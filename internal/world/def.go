package world

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"path"
	"sync"
)

// Def describes one level of a world: which map image it is drawn from, how
// much it is scaled up, and the places and zones on it. Levels can inherit
// from an earlier level, so level 2 has everything level 1 has, plus more.
//
// Places and zones use latitude/longitude (or offsets from a place), so they
// land on the same spot at every level's scale.
type Def struct {
	Name     string    `json:"name"`
	Level    int       `json:"level"`
	Inherits string    `json:"inherits,omitempty"` // file name of the parent level, e.g. "level1"
	Map      string    `json:"map,omitempty"`      // terrain image (PNG), relative to the level file
	Legend   string    `json:"legend,omitempty"`   // color -> terrain JSON
	Relief   string    `json:"relief,omitempty"`   // optional grey relief + ocean-depth layer for the world map
	Scale    int       `json:"scale"`              // blocks per map pixel
	GroundY  int       `json:"groundY,omitempty"`  // y of the top ground block
	Seed     uint64    `json:"seed,omitempty"`
	Spawn    string    `json:"spawn,omitempty"` // place id where new characters start
	Places   []Place   `json:"places"`
	Zones    []ZoneDef `json:"zones"`

	// Resolved by LoadDef.
	Dir     string `json:"-"` // the folder the level was loaded from, e.g. "earth"
	terrain *TerrainMap
	W, H    int
}

// Place is a named spot, optionally with a structure (e.g. a town) and a safe zone.
type Place struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Structure string  `json:"structure,omitempty"` // "town" or ""
	Zone      string  `json:"zone,omitempty"`      // "town" makes the circle a safe zone
	Radius    float64 `json:"radius,omitempty"`
	Level     int     `json:"-"` // the level that added it
	X, Z      float64 `json:"-"`
}

// ZoneDef is a circular region with a gameplay kind (e.g. "danger").
type ZoneDef struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	At     string   `json:"at,omitempty"` // place id; DX/DZ are block offsets from it
	DX     float64  `json:"dx,omitempty"`
	DZ     float64  `json:"dz,omitempty"`
	Lat    *float64 `json:"lat,omitempty"`
	Lon    *float64 `json:"lon,omitempty"`
	Radius float64  `json:"radius"`
	X, Z   float64  `json:"-"`
}

// Width and Height are the level's size in blocks (columns).
func (d *Def) Width() int  { return d.W }
func (d *Def) Height() int { return d.H }

// LatLon converts geographic coordinates to block coordinates (equirectangular,
// x grows east from 180°W, z grows south from the North Pole).
func (d *Def) LatLon(lat, lon float64) (x, z float64) {
	return math.Floor((lon+180)/360*float64(d.W)) + 0.5, math.Floor((90-lat)/180*float64(d.H)) + 0.5
}

type mapCacheKey struct {
	fsys fs.FS
	path string
}

var (
	mapCacheMu sync.Mutex
	mapCache   = map[mapCacheKey]*TerrainMap{}
)

// LoadDef reads <dir>/<name>.json from fsys, following "inherits" chains.
// The decoded terrain image is cached, since every game in a process uses it.
func LoadDef(fsys fs.FS, dir, name string) (*Def, error) {
	return loadDef(fsys, dir, name, 0)
}

func loadDef(fsys fs.FS, dir, name string, depth int) (*Def, error) {
	if depth > 10 {
		return nil, fmt.Errorf("level %s: inherits too deeply", name)
	}
	b, err := fs.ReadFile(fsys, path.Join(dir, name+".json"))
	if err != nil {
		return nil, err
	}
	var d Def
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("level %s: %w", name, err)
	}
	if d.Inherits != "" {
		parent, err := loadDef(fsys, dir, d.Inherits, depth+1)
		if err != nil {
			return nil, err
		}
		d.inherit(parent)
	} else if d.Map == "" || d.Legend == "" {
		return nil, fmt.Errorf("level %s: map and legend are required", name)
	}
	if d.Scale < 1 {
		d.Scale = 1
	}
	if d.GroundY == 0 {
		d.GroundY = 12
	}
	if d.GroundY < 8 || d.GroundY > SizeY-12 {
		return nil, fmt.Errorf("level %s: groundY must be 8..%d", name, SizeY-12)
	}
	for i := range d.Places {
		if d.Places[i].Level == 0 {
			d.Places[i].Level = d.Level
		}
	}
	tm, err := loadTerrain(fsys, path.Join(dir, d.Map), path.Join(dir, d.Legend))
	if err != nil {
		return nil, fmt.Errorf("level %s: %w", name, err)
	}
	d.terrain = tm
	d.Dir = dir
	d.W, d.H = tm.W*d.Scale, tm.H*d.Scale
	if d.W%Chunk != 0 || d.H%Chunk != 0 {
		return nil, fmt.Errorf("level %s: size %dx%d must be a multiple of %d", name, d.W, d.H, Chunk)
	}
	return &d, d.resolve()
}

// inherit fills unset fields from the parent and prepends its places and zones.
func (d *Def) inherit(p *Def) {
	if d.Name == "" {
		d.Name = p.Name
	}
	if d.Map == "" {
		d.Map, d.Legend, d.Relief = p.Map, p.Legend, p.Relief
	}
	if d.Scale == 0 {
		d.Scale = p.Scale * 2
	}
	if d.GroundY == 0 {
		d.GroundY = p.GroundY
	}
	if d.Seed == 0 {
		d.Seed = p.Seed
	}
	if d.Spawn == "" {
		d.Spawn = p.Spawn
	}
	d.Places = append(append([]Place(nil), p.Places...), d.Places...)
	d.Zones = append(append([]ZoneDef(nil), p.Zones...), d.Zones...)
}

func loadTerrain(fsys fs.FS, mapPath, legendPath string) (*TerrainMap, error) {
	key := mapCacheKey{fsys, mapPath + "|" + legendPath}
	mapCacheMu.Lock()
	defer mapCacheMu.Unlock()
	if tm := mapCache[key]; tm != nil {
		return tm, nil
	}
	lb, err := fs.ReadFile(fsys, legendPath)
	if err != nil {
		return nil, err
	}
	legend, err := ParseLegend(lb)
	if err != nil {
		return nil, err
	}
	pb, err := fs.ReadFile(fsys, mapPath)
	if err != nil {
		return nil, err
	}
	tm, err := DecodeTerrain(pb, legend)
	if err != nil {
		return nil, err
	}
	mapCache[key] = tm
	return tm, nil
}

// resolve turns lat/lon and place-relative positions into block coordinates
// and checks that everything is inside the world.
func (d *Def) resolve() error {
	byID := map[string]*Place{}
	for i := range d.Places {
		p := &d.Places[i]
		if p.ID == "" || byID[p.ID] != nil {
			return fmt.Errorf("place %q: missing or duplicate id", p.ID)
		}
		if p.Lat < -90 || p.Lat > 90 || p.Lon < -180 || p.Lon > 180 {
			return fmt.Errorf("place %s: lat/lon out of range", p.ID)
		}
		if p.Structure != "" && p.Structure != "town" {
			return fmt.Errorf("place %s: unknown structure %q", p.ID, p.Structure)
		}
		p.X, p.Z = d.LatLon(p.Lat, p.Lon)
		byID[p.ID] = p
	}
	if d.Spawn == "" || byID[d.Spawn] == nil {
		return fmt.Errorf("spawn place %q not found", d.Spawn)
	}
	for i := range d.Zones {
		z := &d.Zones[i]
		switch {
		case z.At != "":
			p := byID[z.At]
			if p == nil {
				return fmt.Errorf("zone %s: unknown place %q", z.ID, z.At)
			}
			z.X, z.Z = p.X+z.DX, p.Z+z.DZ
		case z.Lat != nil && z.Lon != nil:
			z.X, z.Z = d.LatLon(*z.Lat, *z.Lon)
		default:
			return fmt.Errorf("zone %s: needs \"at\" or lat/lon", z.ID)
		}
		if z.Kind != ZoneDanger && z.Kind != ZoneTown {
			return fmt.Errorf("zone %s: unknown kind %q", z.ID, z.Kind)
		}
		if z.Radius <= 0 || z.Z-z.Radius < 4 || z.Z+z.Radius > float64(d.H-4) {
			return fmt.Errorf("zone %s: radius or position out of bounds", z.ID)
		}
		z.X = wrapF(z.X, d.W)
	}
	for _, p := range d.Places {
		if p.Z-p.Radius < 4 || p.Z+p.Radius > float64(d.H-4) {
			return fmt.Errorf("place %s: too close to the pole", p.ID)
		}
	}
	return nil
}

// Place looks up a place by id.
func (d *Def) Place(id string) *Place {
	for i := range d.Places {
		if d.Places[i].ID == id {
			return &d.Places[i]
		}
	}
	return nil
}
