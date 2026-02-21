package plg_override_download

import (
	_ "embed"

	. "github.com/mickael-kerjean/filestash/server/common"
)

//go:embed assets/pages/filespage/thing.js
var PATCH []byte

func init() {
	Hooks.Register.StaticPatch(PATCH)
}
