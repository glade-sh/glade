package embed

import _ "embed"

//go:embed glade.out.js
var OutJS []byte

func ScriptURL() string {
	return "/lightning/glade.out.js"
}
