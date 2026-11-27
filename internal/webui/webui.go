// Package webui embeds the built Vue interface (web/ builds into ui/).
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:ui
var files embed.FS

// FS returns the interface files.
func FS() fs.FS {
	sub, err := fs.Sub(files, "ui")
	if err != nil {
		panic(err) // "ui" is embedded at build time
	}
	return sub
}
