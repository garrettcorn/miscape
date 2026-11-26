// Package maps embeds the world map data: per world, a terrain image, its
// color legend, and one JSON file per level (see docs/EXTENDING.md).
package maps

import "embed"

//go:embed earth/*.json earth/*.png
var FS embed.FS
