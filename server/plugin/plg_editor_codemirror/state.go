package plg_editor_codemirror

import (
	"sync"

	. "github.com/mickael-kerjean/filestash/server/common"
)

var (
	documents   = map[string]*document{}
	documentsMu sync.Mutex
)

func getDocument(ctx *App, path string) *document {
	key := GenerateID(ctx.Session) + "::" + path
	documentsMu.Lock()
	doc, ok := documents[key]
	if !ok {
		doc = &document{}
		documents[key] = doc
	}
	documentsMu.Unlock()
	return doc
}

func removeDocument(ctx *App, path string) {
	key := GenerateID(ctx.Session) + "::" + path
	documentsMu.Lock()
	delete(documents, key)
	documentsMu.Unlock()
}
