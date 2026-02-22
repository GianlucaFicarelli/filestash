//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"os"
)

func main() {
	output := "mime_generated.go"     // path to output Go file
	input := "../../config/mime.json" // path to input mime file

	f, err := os.OpenFile(input, os.O_RDONLY, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	j, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	mTypes := make(map[string]string, 0)
	json.Unmarshal(j, &mTypes)

	var buf bytes.Buffer
	buf.WriteString("package common\n")
	buf.WriteString("func init() {\n")
	for key, value := range mTypes {
		fmt.Fprintf(&buf, "MimeTypes[\"%s\"] = \"%s\"\n", key, value)
	}
	buf.WriteString("}\n")
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error formatting: %v\n", err)
		os.Exit(1)
	}
	if err = os.WriteFile(output, formatted, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
		os.Exit(1)
	}
}
