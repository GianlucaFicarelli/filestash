//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	output := "constants_generated.go" // path to output Go file

	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed: %v\n%s", cmd.String(), err, out)
		os.Exit(1)
	}

	hash := strings.TrimSpace(string(out))

	f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
		return
	}
	f.Write([]byte(fmt.Sprintf(`
package common

func init() {
    BUILD_REF = "%s"
    BUILD_DATE = "%s"
}
`, hash, time.Now().Format("20060102"))))
	f.Close()
}
