package plg_editor_codemirror

import (
	"encoding/json"
	"sync"
)

type subscriber struct {
	clientID string
	ch       chan []Change
}

type Change struct {
	From     json.RawMessage `json:"from"`
	To       json.RawMessage `json:"to"`
	Text     []string        `json:"text"`
	Removed  []string        `json:"removed"`
	ClientID string          `json:"clientID"`
}

type document struct {
	mu          sync.Mutex
	changes     []Change
	subscribers []subscriber
}

func (this *document) Transaction(fn func()) {
	this.mu.Lock()
	fn()
	this.mu.Unlock()
}
