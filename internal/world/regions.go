package world

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"sort"
	"strconv"
	"strings"
)

// Region is a named part of the map: a continent or an ocean. Regions come
// from a color-coded image (one pixel per level-1 column, like the terrain)
// and a JSON file naming each color.
type Region struct {
	Name        string     `json:"name"`
	Countryside string     `json:"countryside"` // how the wild parts read, e.g. "the European countryside"
	Sea         bool       `json:"sea,omitempty"`
	Label       [2]float64 `json:"label"`              // [lat, lon] for the world-map label
	MinZoom     float64    `json:"minZoom,omitempty"`  // only label when zoomed in this far (pixels per block)
	MapLabel    string     `json:"mapLabel,omitempty"` // shorter text for the world-map label (default: Name)
	Color       string     `json:"-"`
}

// RegionMap is a grid of region indexes (0 = none), one per map pixel.
type RegionMap struct {
	W, H    int
	R       []uint8
	Regions []*Region // index-1
}

// At returns the region at map pixel (x,z), or nil.
func (m *RegionMap) At(x, z int) *Region {
	if m == nil {
		return nil
	}
	x = ((x % m.W) + m.W) % m.W
	z = min(max(z, 0), m.H-1)
	i := m.R[z*m.W+x]
	if i == 0 {
		return nil
	}
	return m.Regions[i-1]
}

// ParseRegions reads regions.json: {"#rrggbb": {"name": ..., ...}}. Regions
// come back sorted by name, with Color set.
func ParseRegions(b []byte) ([]*Region, error) {
	var m map[string]*Region
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("regions: %w", err)
	}
	var out []*Region
	for hex, r := range m {
		if len(strings.TrimPrefix(hex, "#")) != 6 {
			return nil, fmt.Errorf("regions: bad color %q", hex)
		}
		if _, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32); err != nil {
			return nil, fmt.Errorf("regions: bad color %q", hex)
		}
		if r.Name == "" || r.Countryside == "" {
			return nil, fmt.Errorf("regions: %s needs a name and a countryside", hex)
		}
		if r.Label[0] < -90 || r.Label[0] > 90 || r.Label[1] < -180 || r.Label[1] > 180 {
			return nil, fmt.Errorf("regions: %s label out of range", r.Name)
		}
		r.Color = strings.ToLower(hex)
		out = append(out, r)
	}
	if len(out) == 0 || len(out) > 254 {
		return nil, fmt.Errorf("regions: need 1 to 254 regions")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// DecodeRegions turns a region image into a RegionMap. Colors not in the
// list take the nearest one, so soft brushes are fine.
func DecodeRegions(pngData []byte, regions []*Region) (*RegionMap, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("region image: %w", err)
	}
	type rgb struct{ r, g, b int }
	cols := make([]rgb, len(regions))
	for i, r := range regions {
		v, _ := strconv.ParseUint(strings.TrimPrefix(r.Color, "#"), 16, 32)
		cols[i] = rgb{int(v >> 16), int(v>>8) & 255, int(v) & 255}
	}
	b := img.Bounds()
	m := &RegionMap{W: b.Dx(), H: b.Dy(), R: make([]uint8, b.Dx()*b.Dy()), Regions: regions}
	cache := map[uint32]uint8{}
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			cr, cg, cb, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			key := (cr>>8)<<16 | (cg>>8)<<8 | cb>>8
			idx, ok := cache[key]
			if !ok {
				best, bd := 0, 1<<30
				for i, c := range cols {
					d := (c.r-int(cr>>8))*(c.r-int(cr>>8)) + (c.g-int(cg>>8))*(c.g-int(cg>>8)) + (c.b-int(cb>>8))*(c.b-int(cb>>8))
					if d < bd {
						best, bd = i, d
					}
				}
				idx = uint8(best + 1)
				cache[key] = idx
			}
			m.R[y*m.W+x] = idx
		}
	}
	return m, nil
}
