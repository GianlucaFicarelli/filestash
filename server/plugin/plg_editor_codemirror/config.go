package plg_editor_codemirror

import (
	. "github.com/mickael-kerjean/filestash/server/common"
)

var PluginEnable = func() bool {
	return Config.Get("features.collaborative.enable").Schema(func(f *FormElement) *FormElement {
		if f == nil {
			f = &FormElement{}
		}
		f.Name = "enable"
		f.Type = "enable"
		f.Target = []string{}
		f.Description = "Enable/Disable collaborative editing"
		f.Default = true
		return f
	}).Bool()
}
