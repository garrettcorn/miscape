// Package web embeds the browser client so the server is a single binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html style.css js
var files embed.FS

//go:embed admin.html
var AdminPage []byte

// FS returns the static client files.
func FS() fs.FS { return files }
