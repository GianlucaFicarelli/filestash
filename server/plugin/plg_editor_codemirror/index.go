package plg_editor_codemirror

import (
	_ "embed"

	. "github.com/mickael-kerjean/filestash/server/common"
	. "github.com/mickael-kerjean/filestash/server/middleware"

	"github.com/gorilla/mux"
)

//go:embed assets/application_editor.patch
var PATCH []byte

func init() {
	Hooks.Register.OnConfig(func() {
		if PluginEnable() {
			Hooks.Register.StaticPatch(PATCH, WithID("plg_editor_codemirror"))
		} else {
			Hooks.Register.StaticPatch([]byte(""), WithID("plg_editor_codemirror"))
		}
	})
	Hooks.Register.HttpEndpoint(func(r *mux.Router) error {
		middlewares := []Middleware{ApiHeaders, SecureHeaders, SessionStart, LoggedInOnly}
		r.HandleFunc(
			WithBase("/api/codemirror/pull"),
			NewMiddlewareChain(HandlerPull, middlewares),
		).Methods("GET")
		r.HandleFunc(
			WithBase("/api/codemirror/push"),
			NewMiddlewareChain(HandlerPush, middlewares),
		).Methods("POST")
		r.HandleFunc(
			WithBase("/api/codemirror/reset"),
			NewMiddlewareChain(HandlerReset, middlewares),
		).Methods("POST")
		return nil
	})
}
