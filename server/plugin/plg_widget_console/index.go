package plg_widget_console

import (
	"embed"
	"net/http"

	. "github.com/mickael-kerjean/filestash/server/common"

	"github.com/gorilla/mux"
)

//go:generate go run generator.go
//go:embed assets/*
var assets embed.FS

func init() {
	Hooks.Register.StaticPatch(load("assets/shell.patch"))
	Hooks.Register.HttpEndpoint(func(r *mux.Router) error {
		for _, endpoint := range []struct {
			url     string
			content []byte
			mime    string
		}{
			{"/components/shell.js", load("assets/shell.js"), "application/javascript"},
			{"/components/shell.css", load("assets/shell.css"), "text/css"},
			{"/components/registry.js", load("assets/registry.js"), "application/javascript"},
			{"/components/commands.js", load("assets/commands.js"), "application/javascript"},
			{"/components/commands.js", load("assets/commands.js"), "application/javascript"},

			{"/lib/vendor/xterm/xterm.js", load("assets/vendor/xterm.js"), "application/javascript"},
			{"/lib/vendor/xterm/xterm.css", load("assets/vendor/xterm.css"), "text/css"},
		} {
			r.HandleFunc(WithBase("/assets/"+BUILD_REF+endpoint.url), func(res http.ResponseWriter, req *http.Request) {
				res.Header().Set("Content-Type", endpoint.mime)
				res.Write(endpoint.content)
			}).Methods("GET")
		}
		return nil
	})
}

func load(path string) []byte {
	data, _ := assets.ReadFile(path)
	return data
}
