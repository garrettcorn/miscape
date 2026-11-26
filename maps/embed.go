// Package maps embeds the world map data: per world, a terrain image, its
// color legend, and one JSON file per level (see docs/EXTENDING.md).
package maps

import (
	"embed"
	"io/fs"
)

// FS holds what the game needs to generate worlds: level files, legends and
// terrain images.
//
//go:embed earth/*.json earth/*.png
var FS embed.FS

// Display holds images only shown to players (e.g. the world map's relief
// layer) in <world>/display/. They're separate so the in-browser engine,
// which never reads them, doesn't carry them.
//
//go:embed earth/display/*.png
var Display embed.FS

// Served is everything the server offers under /maps/: Display first, then
// FS. (A function, not a variable, so binaries that never serve maps, like
// the in-browser engine, don't keep the Display images.)
func Served() fs.FS { return union{Display, FS} }

type union []fs.FS

func (u union) Open(name string) (fs.File, error) {
	var err error
	for _, f := range u {
		var file fs.File
		if file, err = f.Open(name); err == nil {
			return file, nil
		}
	}
	return nil, err
}
