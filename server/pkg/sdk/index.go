package sdk

import (
	"net/http"
	"net/url"

	. "github.com/mickael-kerjean/filestash/server/common"
)

type Filestash struct {
	Token    string
	URL      string
	Insecure bool
	Storage  string
	Client   *http.Client
}

func NewClient() Filestash {
	baseURL, _ := url.Parse(localURL())
	insecure := baseURL.Hostname() == "localhost" || baseURL.Hostname() == "127.0.0.1"
	opts := []HTTPClientOption{WithoutTimeout}
	if insecure {
		opts = append(opts, WithInsecure)
	}
	return Filestash{
		URL:      baseURL.String(),
		Insecure: insecure,
		Client:   HTTPClient(opts...),
	}
}
