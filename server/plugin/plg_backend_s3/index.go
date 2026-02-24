package plg_backend_s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	. "github.com/mickael-kerjean/filestash/server/common"
)

var S3Cache AppCache

type S3Backend struct {
	client     *s3.Client
	config     aws.Config
	params     map[string]string
	Context    context.Context
	threadSize int
	timeout    time.Duration
}

func init() {
	Backend.Register("s3", &S3Backend{})
	S3Cache = NewAppCache(2, 1)
}

func (b *S3Backend) Init(params map[string]string, app *App) (IBackend, error) {
	if params["encryption_key"] != "" && len(params["encryption_key"]) != 32 {
		return nil, NewError(fmt.Sprintf("Encryption key needs to be 32 characters (current: %d)", len(params["encryption_key"])), 400)
	}
	region := params["region"]
	if region == "" {
		region = "us-east-1"
		if strings.HasSuffix(params["endpoint"], ".cloudflarestorage.com") {
			region = "auto"
		}
	}

	var err error
	var cfg aws.Config
	var creds aws.CredentialsProvider
	if params["access_key_id"] != "" && params["secret_access_key"] != "" {
		creds = aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			params["access_key_id"],
			params["secret_access_key"],
			params["session_token"],
		))
	} else {
		creds = aws.AnonymousCredentials{}
	}
	cfg, err = config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(creds),
	)

	if err != nil {
		return nil, err
	}

	if params["role_arn"] != "" {
		stsClient := sts.NewFromConfig(cfg)
		assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, params["role_arn"])
		cfg.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
	}

	var s3Client *s3.Client
	if params["endpoint"] != "" {
		s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(params["endpoint"])
			o.UsePathStyle = true
		})
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}

	var timeout time.Duration
	if secs, err := strconv.Atoi(params["timeout"]); err == nil {
		timeout = time.Duration(secs) * time.Second
	}

	threadSize, err := strconv.Atoi(params["number_thread"])
	if err != nil || threadSize < 1 || threadSize > 5000 {
		threadSize = 50
	}

	backend := &S3Backend{
		client:     s3Client,
		config:     cfg,
		params:     params,
		Context:    app.Context,
		threadSize: threadSize,
		timeout:    timeout,
	}
	return backend, nil
}

func (b *S3Backend) LoginForm() Form {
	return Form{
		Elmnts: []FormElement{
			{
				Name:  "type",
				Type:  "hidden",
				Value: "s3",
			},
			{
				Name:        "access_key_id",
				Type:        "text",
				Placeholder: "Access Key ID",
			},
			{
				Name:        "secret_access_key",
				Type:        "password",
				Placeholder: "Secret Access Key",
			},
			{
				Name:        "advanced",
				Type:        "enable",
				Placeholder: "Advanced",
				Target: []string{
					"s3_region", "s3_endpoint", "s3_role_arn", "s3_session_token",
					"s3_path", "s3_encryption_key", "s3_number_thread", "s3_timeout",
				},
			},
			{
				Id:          "s3_region",
				Name:        "region",
				Type:        "text",
				Placeholder: "Region",
			},
			{
				Id:          "s3_endpoint",
				Name:        "endpoint",
				Type:        "text",
				Placeholder: "Endpoint",
			},
			{
				Id:          "s3_role_arn",
				Name:        "role_arn",
				Type:        "text",
				Placeholder: "Role ARN",
			},
			{
				Id:          "s3_session_token",
				Name:        "session_token",
				Type:        "text",
				Placeholder: "Session Token",
			},
			{
				Id:          "s3_path",
				Name:        "path",
				Type:        "text",
				Placeholder: "Path",
			},
			{
				Id:          "s3_encryption_key",
				Name:        "encryption_key",
				Type:        "text",
				Placeholder: "Encryption Key",
			},
			{
				Id:          "s3_number_thread",
				Name:        "number_thread",
				Type:        "number",
				Placeholder: "Num. Thread",
			},
			{
				Id:          "s3_timeout",
				Name:        "timeout",
				Type:        "number",
				Placeholder: "List Object Timeout",
			},
		},
	}
}

func (b *S3Backend) Meta(path string) Metadata {
	if path == "/" {
		return Metadata{
			CanCreateFile: NewBool(false),
			CanRename:     NewBool(false),
			CanMove:       NewBool(false),
			CanUpload:     NewBool(false),
		}
	}
	return Metadata{}
}

func (b *S3Backend) Ls(path string) ([]os.FileInfo, error) {
	files := []os.FileInfo{}
	p := b.path(path)
	ctx := b.Context

	if p.bucket == "" {
		output, err := b.client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			return nil, err
		}
		for _, bucket := range output.Buckets {
			files = append(files, &File{
				FName: *bucket.Name,
				FType: "directory",
				FTime: bucket.CreationDate.Unix(),
			})
		}
		return files, nil
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(p.bucket),
		Prefix:    aws.String(p.path),
		Delimiter: aws.String("/"),
	}

	paginator := s3.NewListObjectsV2Paginator(b.client, input)

	start := time.Now()
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, object := range page.Contents {
			if *object.Key == p.path {
				continue
			}
			size := int64(-1)
			if object.Size != aws.Int64(0) {
				size = *object.Size
			}
			isOffline := object.StorageClass == types.ObjectStorageClassGlacier
			files = append(files, &File{
				FName:   filepath.Base(*object.Key),
				FType:   "file",
				FTime:   object.LastModified.Unix(),
				FSize:   size,
				Offline: isOffline,
			})
		}
		for _, prefix := range page.CommonPrefixes {
			files = append(files, &File{
				FName: filepath.Base(*prefix.Prefix),
				FType: "directory",
				FTime: 0,
			})
		}
		if b.timeout > 0 && time.Since(start) > b.timeout {
			break
		}
	}
	return files, nil
}

func (b *S3Backend) Stat(path string) (os.FileInfo, error) {
	p := b.path(path)
	ctx := b.Context

	if p.path == "" {
		output, err := b.client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			return nil, err
		}
		for _, bucket := range output.Buckets {
			if bucket.Name != nil && *bucket.Name == p.bucket {
				return &File{
					FName: *bucket.Name,
					FType: "directory",
					FTime: bucket.CreationDate.Unix(),
				}, nil
			}
		}
		return nil, ErrNotFound
	}

	input := &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(p.path),
	}

	output, err := b.client.HeadObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			// file missing -> assume virtual folder
			return &File{
				FName: filepath.Base(path),
				FType: "directory",
				FTime: -1,
			}, nil
		}
		return nil, err
	}
	size := int64(0)
	if output.ContentLength != nil {
		size = *output.ContentLength
	}
	mtime := int64(0)
	if output.LastModified != nil {
		mtime = output.LastModified.Unix()
	}
	return &File{
		FName: filepath.Base(path),
		FType: "file",
		FSize: size,
		FTime: mtime,
	}, nil
}

func (b *S3Backend) Cat(path string) (io.ReadCloser, error) {
	p := b.path(path)
	ctx := b.Context

	input := &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(p.path),
	}

	if b.params["encryption_key"] != "" {
		input.SSECustomerAlgorithm = aws.String("AES256")
		input.SSECustomerKey = aws.String(b.params["encryption_key"])
	}

	obj, err := b.client.GetObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "InvalidRequest":
				input.SSECustomerAlgorithm = nil
				input.SSECustomerKey = nil
				obj, err = b.client.GetObject(ctx, input)
				if err != nil {
					return nil, err
				}
			case "AccessDenied":
				return nil, ErrNotAllowed
			case "InvalidObjectState":
				return nil, ErrNotReachable
			default:
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return obj.Body, nil
}

func (b *S3Backend) Mkdir(path string) error {
	p := b.path(path)
	ctx := b.Context
	if p.path == "" {
		_, err := b.client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(path),
		})
		return err
	}
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(EnforceDirectory(p.path)),
	})
	return err
}

func (b *S3Backend) Rm(path string) error {
	p := b.path(path)
	if p.bucket == "" {
		return ErrNotFound
	}

	ctx, cancel := context.WithCancel(b.Context)
	defer cancel()

	// Check if it is a file or directory
	finfo, err := b.Stat(path)
	if err != nil {
		return err
	}

	client := b.client

	// CASE 1: Remove a single file
	if !finfo.IsDir() {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(p.bucket),
			Key:    aws.String(p.path),
		})
		return err
	}

	// CASE 2: Remove a folder recursively using parallel workers
	jobChan := make(chan string, b.threadSize)
	errChan := make(chan error, b.threadSize)
	var wg sync.WaitGroup

	for i := 0; i < b.threadSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobChan {
				if ctx.Err() != nil {
					return
				}
				_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(p.bucket),
					Key:    aws.String(key),
				})
				if err != nil {
					errChan <- err
					cancel()
					return
				}
			}
		}()
	}

	// Use paginator to list all objects under the prefix
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(p.bucket),
		Prefix: aws.String(p.path),
	}
	paginator := s3.NewListObjectsV2Paginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			close(jobChan)
			wg.Wait()
			return err
		}
		for _, obj := range page.Contents {
			jobChan <- *obj.Key
		}
	}

	close(jobChan)
	wg.Wait()
	close(errChan)

	for e := range errChan {
		return e
	}

	// Remove the "folder object" itself if it exists
	if p.path != "" {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(p.bucket),
			Key:    aws.String(EnforceDirectory(p.path)),
		})
		return err
	}

	return nil
}

func (b *S3Backend) Mv(from string, to string) error {
	if from == to {
		return nil
	}
	f := b.path(from)
	t := b.path(to)
	ctx, cancel := context.WithCancel(b.Context)
	defer cancel()
	client := b.client

	finfo, err := b.Stat(from)
	if err != nil {
		return err
	}

	// CASE 1: Rename/Move a file
	if !finfo.IsDir() {
		copyInput := &s3.CopyObjectInput{
			CopySource: aws.String(fmt.Sprintf("%s/%s", f.bucket, f.path)),
			Bucket:     aws.String(t.bucket),
			Key:        aws.String(t.path),
		}
		if b.params["encryption_key"] != "" {
			copyInput.CopySourceSSECustomerAlgorithm = aws.String("AES256")
			copyInput.CopySourceSSECustomerKey = aws.String(b.params["encryption_key"])
			copyInput.SSECustomerAlgorithm = aws.String("AES256")
			copyInput.SSECustomerKey = aws.String(b.params["encryption_key"])
		}
		_, err := client.CopyObject(ctx, copyInput)
		if err != nil {
			return err
		}
		_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(f.bucket),
			Key:    aws.String(f.path),
		})
		return err
	}

	// CASE 2: Rename/Move a folder recursively
	jobChan := make(chan [2]string, b.threadSize) // [sourceKey, targetKey]
	errChan := make(chan error, b.threadSize)
	var wg sync.WaitGroup

	for i := 0; i < b.threadSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pair := range jobChan {
				srcKey := pair[0]
				dstKey := pair[1]
				copyInput := &s3.CopyObjectInput{
					CopySource: aws.String(fmt.Sprintf("%s/%s", f.bucket, srcKey)),
					Bucket:     aws.String(t.bucket),
					Key:        aws.String(dstKey),
				}
				if b.params["encryption_key"] != "" {
					copyInput.CopySourceSSECustomerAlgorithm = aws.String("AES256")
					copyInput.CopySourceSSECustomerKey = aws.String(b.params["encryption_key"])
					copyInput.SSECustomerAlgorithm = aws.String("AES256")
					copyInput.SSECustomerKey = aws.String(b.params["encryption_key"])
				}
				_, err := client.CopyObject(ctx, copyInput)
				if err != nil {
					errChan <- err
					cancel()
					return
				}
				_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(f.bucket),
					Key:    aws.String(srcKey),
				})
				if err != nil {
					errChan <- err
					cancel()
					return
				}
			}
		}()
	}

	// List all objects under the source prefix
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(f.bucket),
		Prefix: aws.String(f.path),
	}
	paginator := s3.NewListObjectsV2Paginator(client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			close(jobChan)
			wg.Wait()
			return err
		}
		for _, obj := range page.Contents {
			relative := strings.TrimPrefix(*obj.Key, f.path)
			jobChan <- [2]string{*obj.Key, t.path + relative}
		}
	}

	close(jobChan)
	wg.Wait()
	close(errChan)

	for e := range errChan {
		return e
	}

	return nil
}

func (b *S3Backend) Touch(path string) error {
	p := b.path(path)
	ctx := b.Context

	input := &s3.PutObjectInput{
		Body:          strings.NewReader(""),
		ContentLength: aws.Int64(0),
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(p.path),
		ContentType:   aws.String(GetMimeType(path)),
	}
	if b.params["encryption_key"] != "" {
		input.SSECustomerAlgorithm = aws.String("AES256")
		input.SSECustomerKey = aws.String(b.params["encryption_key"])
	}
	_, err := b.client.PutObject(ctx, input)
	return err
}

func (b *S3Backend) Save(path string, file io.Reader) error {
	p := b.path(path)
	ctx := b.Context

	tm := transfermanager.New(b.client)
	input := &transfermanager.UploadObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(p.path),
		Body:        file,
		ContentType: aws.String(GetMimeType(path)),
	}

	if b.params["encryption_key"] != "" {
		input.SSECustomerAlgorithm = aws.String("AES256")
		input.SSECustomerKey = aws.String(b.params["encryption_key"])
	}

	_, err := tm.UploadObject(ctx, input)
	return err
}

type S3Path struct {
	bucket string
	path   string
}

func (b *S3Backend) path(p string) S3Path {
	sp := strings.Split(p, "/")
	bucket := ""
	if len(sp) > 1 {
		bucket = sp[1]
	}
	path := ""
	if len(sp) > 2 {
		path = strings.Join(sp[2:], "/")
	}
	return S3Path{
		bucket,
		path,
	}
}
