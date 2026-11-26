package world

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"
)

// Terrain is the ground type of one column, read from a map image.
type Terrain uint8

const (
	TOcean Terrain = iota
	TShallows
	TGrass
	TForest
	TDesert
	TSnow
	TIce
	TTundra
	TRock
	TMoor // the danger zone's ground; set by zones, not by the image
	NumTerrains
)

var terrainNames = [NumTerrains]string{"ocean", "shallows", "grass", "forest", "desert", "snow", "ice", "tundra", "rock", "moor"}

func (t Terrain) String() string {
	if int(t) < len(terrainNames) {
		return terrainNames[t]
	}
	return "unknown"
}

// ParseTerrain maps a legend name to a terrain type.
func ParseTerrain(s string) (Terrain, bool) {
	for i, n := range terrainNames {
		if n == s {
			return Terrain(i), true
		}
	}
	return 0, false
}

// IsWater reports whether a terrain is sea.
func (t Terrain) IsWater() bool { return t == TOcean || t == TShallows }

// TerrainMap is a grid of terrain types, one per map pixel.
type TerrainMap struct {
	W, H int
	T    []Terrain
}

func (m *TerrainMap) At(x, z int) Terrain {
	x = ((x % m.W) + m.W) % m.W
	z = min(max(z, 0), m.H-1)
	return m.T[z*m.W+x]
}

// Legend maps "#rrggbb" colors to terrain names.
type Legend map[string]string

type legendColor struct {
	r, g, b int
	t       Terrain
}

func (l Legend) colors() ([]legendColor, error) {
	var out []legendColor
	for hex, name := range l {
		t, ok := ParseTerrain(name)
		if !ok {
			return nil, fmt.Errorf("legend: unknown terrain %q", name)
		}
		v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
		if err != nil || len(strings.TrimPrefix(hex, "#")) != 6 {
			return nil, fmt.Errorf("legend: bad color %q", hex)
		}
		out = append(out, legendColor{int(v >> 16), int(v>>8) & 255, int(v) & 255, t})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("legend is empty")
	}
	return out, nil
}

// Hex returns the legend color for a terrain (for tools that draw maps).
func (l Legend) Hex(t Terrain) string {
	for hex, name := range l {
		if name == t.String() {
			return hex
		}
	}
	return ""
}

// ParseLegend reads legend.json: {"#1f4e79": "ocean", ...}.
func ParseLegend(b []byte) (Legend, error) {
	var l Legend
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("legend: %w", err)
	}
	_, err := l.colors()
	return l, err
}

// DecodeTerrain turns a map image into terrain using the legend. Colors not in
// the legend (e.g. from anti-aliased editing) take the nearest legend color.
func DecodeTerrain(pngData []byte, legend Legend) (*TerrainMap, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("map image: %w", err)
	}
	cols, err := legend.colors()
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	m := &TerrainMap{W: b.Dx(), H: b.Dy(), T: make([]Terrain, b.Dx()*b.Dy())}
	cache := map[uint32]Terrain{}
	pal, paletted := img.(*image.Paletted)
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			var r, g, bl uint32
			if paletted {
				r, g, bl, _ = pal.Palette[pal.ColorIndexAt(b.Min.X+x, b.Min.Y+y)].RGBA()
			} else {
				r, g, bl, _ = img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			}
			key := (r>>8)<<16 | (g>>8)<<8 | bl>>8
			t, ok := cache[key]
			if !ok {
				t = nearest(cols, int(r>>8), int(g>>8), int(bl>>8))
				cache[key] = t
			}
			m.T[y*m.W+x] = t
		}
	}
	return m, nil
}

func nearest(cols []legendColor, r, g, b int) Terrain {
	best, bd := cols[0].t, 1<<30
	for _, c := range cols {
		d := (c.r-r)*(c.r-r) + (c.g-g)*(c.g-g) + (c.b-b)*(c.b-b)
		if d < bd {
			best, bd = c.t, d
		}
	}
	return best
}
