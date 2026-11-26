// Package world holds the voxel world: a large, east–west wrapping grid of
// columns generated on demand from a level definition (a map image plus
// places and zones), with player edits layered on top. It has no knowledge of
// players or networking.
//
// Only edited blocks are stored; everything else is regenerated from the
// map, so memory and save size scale with what players change, not with the
// size of the world.
package world

import (
	"math"
	"sort"
)

// Block ids. Shared with the client (web/js/blocks.js); changing them is a
// protocol-breaking change.
const (
	Air        uint8 = 0
	Grass      uint8 = 1
	Dirt       uint8 = 2
	Stone      uint8 = 3
	Sand       uint8 = 4
	Water      uint8 = 5
	Log        uint8 = 6
	Leaves     uint8 = 7
	Plank      uint8 = 8
	Cobble     uint8 = 9
	CopperOre  uint8 = 10
	Gloamstone uint8 = 11
	Glimmer    uint8 = 12
	Lantern    uint8 = 13
	Roof       uint8 = 14
	Flowers    uint8 = 15
	Anvil      uint8 = 16
	Bedrock    uint8 = 17
	Sign       uint8 = 18
	BerryBush  uint8 = 19
	Deadwood   uint8 = 20
	GloamMoss  uint8 = 21
	Snow       uint8 = 22
	Ice        uint8 = 23
	NumBlocks        = 24
)

// Solid reports whether a block blocks movement.
func Solid(b uint8) bool {
	return b != Air && b != Water && b != Flowers
}

const (
	SizeY = 40 // world height in blocks
	Chunk = 16 // chunk width and depth
	// MaxCachedChunks bounds memory for generated chunks; edits are kept
	// separately and never evicted.
	MaxCachedChunks = 2048
)

// Zone kinds returned by ZoneAt.
const (
	ZoneWild   = "wild"
	ZoneTown   = "town"   // safe: no building, no fighting players, no monsters
	ZoneDanger = "danger" // monsters roam, knock-outs cost coins, dark
)

// Pos is an integer block position. X is always canonical (0 <= X < W).
type Pos struct{ X, Y, Z int32 }

// Point is a named spot reported by a structure (spawn, anvil, NPC places).
type Point struct{ X, Y, Z float64 }

type chunk struct {
	base, cur []uint8 // generated, and with edits applied
	used      uint64
}

// World is one level's voxel world.
type World struct {
	Def    *Def
	W, H   int // columns east–west (wraps) and north–south
	CX, CZ int // chunk counts
	Points map[string]Point

	seed     uint64
	overlay  map[int64][]overlayBlock // structures, by chunk
	clear    []clearing               // structure footprints kept free of trees
	edits    map[int64]map[Pos]uint8  // player edits, by chunk
	nEdits   int
	cache    map[int64]*chunk
	useClock uint64
}

type overlayBlock struct {
	p Pos
	b uint8
}

type clearing struct{ x, z, r float64 }

// New builds a world for a level. Structures are laid out once; terrain is
// generated lazily as chunks are needed.
func New(d *Def) *World {
	w := &World{
		Def: d, W: d.W, H: d.H, CX: d.W / Chunk, CZ: d.H / Chunk,
		seed: d.Seed, overlay: map[int64][]overlayBlock{}, edits: map[int64]map[Pos]uint8{},
		cache: map[int64]*chunk{}, Points: map[string]Point{},
	}
	w.buildStructures()
	return w
}

// ---------- coordinates ----------

func wrapF(x float64, w int) float64 {
	x = math.Mod(x, float64(w))
	if x < 0 {
		x += float64(w)
	}
	return x
}

func wrapI(x, w int) int { return ((x % w) + w) % w }

// WrapX returns x moved into 0 <= x < W.
func (w *World) WrapX(x float64) float64 { return wrapF(x, w.W) }

// DX is the shortest east–west difference b-a, across the date line if shorter.
func (w *World) DX(a, b float64) float64 {
	d := math.Mod(b-a, float64(w.W))
	if d > float64(w.W)/2 {
		d -= float64(w.W)
	} else if d < -float64(w.W)/2 {
		d += float64(w.W)
	}
	return d
}

// Dist2 is the squared horizontal distance between two points, wrapping east–west.
func (w *World) Dist2(ax, az, bx, bz float64) float64 {
	dx, dz := w.DX(ax, bx), bz-az
	return dx*dx + dz*dz
}

// Dist is the horizontal distance between two points, wrapping east–west.
func (w *World) Dist(ax, az, bx, bz float64) float64 { return math.Sqrt(w.Dist2(ax, az, bx, bz)) }

// In reports whether a block position exists (x always does: it wraps).
func (w *World) In(y, z int) bool { return y >= 0 && y < SizeY && z >= 0 && z < w.H }

// InZ reports whether a world point is inside the north–south bounds.
func (w *World) InZ(z float64) bool { return z >= 0 && z < float64(w.H) }

func (w *World) chunkKey(cx, cz int) int64 { return int64(cz)*int64(w.CX) + int64(wrapI(cx, w.CX)) }

// PosAt returns the canonical position of block (x,y,z).
func (w *World) PosAt(x, y, z int) Pos { return Pos{int32(wrapI(x, w.W)), int32(y), int32(z)} }

func local(x, y, z int) int { return (y*Chunk+z%Chunk)*Chunk + x%Chunk }

// ---------- blocks ----------

func (w *World) chunk(cx, cz int) *chunk {
	k := w.chunkKey(cx, cz)
	w.useClock++
	if c := w.cache[k]; c != nil {
		c.used = w.useClock
		return c
	}
	if len(w.cache) >= MaxCachedChunks {
		w.evict()
	}
	base := w.genChunk(wrapI(cx, w.CX), cz)
	cur := append([]uint8(nil), base...)
	for p, b := range w.edits[k] {
		cur[local(int(p.X), int(p.Y), int(p.Z))] = b
	}
	c := &chunk{base: base, cur: cur, used: w.useClock}
	w.cache[k] = c
	return c
}

// evict drops the least recently used quarter of the cache.
func (w *World) evict() {
	type kv struct {
		k int64
		u uint64
	}
	all := make([]kv, 0, len(w.cache))
	for k, c := range w.cache {
		all = append(all, kv{k, c.used})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].u < all[j].u })
	for _, e := range all[:len(all)/4] {
		delete(w.cache, e.k)
	}
}

// Get returns the block at (x,y,z). Below the world is bedrock; above it and
// past the poles is air.
func (w *World) Get(x, y, z int) uint8 {
	if y < 0 {
		return Bedrock
	}
	if y >= SizeY || z < 0 || z >= w.H {
		return Air
	}
	x = wrapI(x, w.W)
	return w.chunk(x/Chunk, z/Chunk).cur[local(x, y, z)]
}

// Set changes a block. The change is kept as an edit (saved with the world)
// unless it restores the generated block.
func (w *World) Set(x, y, z int, b uint8) {
	if !w.In(y, z) {
		return
	}
	x = wrapI(x, w.W)
	cx, cz := x/Chunk, z/Chunk
	c := w.chunk(cx, cz)
	i := local(x, y, z)
	c.cur[i] = b
	k := w.chunkKey(cx, cz)
	p := Pos{int32(x), int32(y), int32(z)}
	m := w.edits[k]
	if b == c.base[i] {
		if _, ok := m[p]; ok {
			delete(m, p)
			w.nEdits--
			if len(m) == 0 {
				delete(w.edits, k)
			}
		}
		return
	}
	if m == nil {
		m = map[Pos]uint8{}
		w.edits[k] = m
	}
	if _, ok := m[p]; !ok {
		w.nEdits++
	}
	m[p] = b
}

// Edits returns every player edit as [x,y,z,block], sorted, for saving.
func (w *World) Edits() [][4]int {
	out := make([][4]int, 0, w.nEdits)
	for _, m := range w.edits {
		for p, b := range m {
			out = append(out, [4]int{int(p.X), int(p.Y), int(p.Z), int(b)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a[2] != b[2] {
			return a[2] < b[2]
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		return a[1] < b[1]
	})
	return out
}

// NumEdits is the number of edited blocks.
func (w *World) NumEdits() int { return w.nEdits }

// CachedChunks is how many generated chunks are in memory.
func (w *World) CachedChunks() int { return len(w.cache) }

// SolidAt reports whether the block containing world point (x,y,z) is solid.
// Past the poles counts as solid (an invisible wall behind the ice cliffs).
func (w *World) SolidAt(x, y, z float64) bool {
	iz, iy := int(math.Floor(z)), int(math.Floor(y))
	if iz < 0 || iz >= w.H || iy < 0 {
		return true
	}
	return Solid(w.Get(int(math.Floor(x)), iy, iz))
}

// SurfaceY returns the y of the first non-solid cell above the highest solid
// block in column (x,z), i.e. where something standing there would put its feet.
func (w *World) SurfaceY(x, z int) int {
	for y := SizeY - 1; y >= 0; y-- {
		if Solid(w.Get(x, y, z)) {
			return y + 1
		}
	}
	return 0
}

// GroundY is where a creature walking on terrain would stand in column (x,z):
// above the highest solid block that isn't foliage, so creatures never end up
// on tree canopies. Returns -1 if that spot has no headroom (e.g. a trunk).
func (w *World) GroundY(x, z int) int {
	for y := SizeY - 1; y >= 0; y-- {
		b := w.Get(x, y, z)
		if b == Leaves || !Solid(b) {
			continue
		}
		if Solid(w.Get(x, y+1, z)) || Solid(w.Get(x, y+2, z)) {
			return -1
		}
		return y + 1
	}
	return 0
}

// ---------- zones ----------

// ZoneAt classifies a world position: inside a town, inside a danger zone,
// or the wild.
func (w *World) ZoneAt(x, z float64) string {
	k, _ := w.zoneAt(x, z)
	return k
}

// ZoneName is the display name of the zone at (x,z): a town or danger zone's
// name, or in the wild the region's countryside ("the European countryside",
// "the Atlantic Ocean").
func (w *World) ZoneName(x, z float64) string {
	_, n := w.zoneAt(x, z)
	return n
}

func (w *World) zoneAt(x, z float64) (kind, name string) {
	for _, p := range w.Def.Places {
		if p.Zone == ZoneTown && w.Dist2(x, z, p.X, p.Z) <= p.Radius*p.Radius {
			return ZoneTown, p.Name
		}
	}
	for _, zd := range w.Def.Zones {
		if w.Dist2(x, z, zd.X, zd.Z) <= zd.Radius*zd.Radius {
			return zd.Kind, zd.Name
		}
	}
	if r := w.RegionAt(x, z); r != nil {
		if r.Sea && !w.terrainAt(wrapI(int(math.Floor(x)), w.W), int(math.Floor(z))).IsWater() {
			// Dry land the regions map calls sea (a coast, or ground a town
			// keeps dry): it belongs to the nearest continent.
			if l := w.landRegionNear(x, z); l != nil {
				r = l
			}
		}
		return ZoneWild, r.Countryside
	}
	return ZoneWild, ""
}

// RegionAt is the continent or ocean at a point, or nil if the level has no
// regions image.
func (w *World) RegionAt(x, z float64) *Region {
	if w.Def.regions == nil {
		return nil
	}
	s := float64(w.Def.Scale)
	return w.Def.regions.At(int(math.Floor(w.WrapX(x)/s)), int(math.Floor(z/s)))
}

// Terrain returns the ground type of column (x,z), from the map image (scaled
// up with a little noise along borders) and danger zones.
func (w *World) Terrain(x, z int) Terrain {
	return w.terrainAt(wrapI(x, w.W), z)
}

// ---------- chunk encoding ----------

// EncodeChunk run-length encodes a chunk column as pairs (count-1, block) in
// x-fastest, then z, then y order.
func (w *World) EncodeChunk(cx, cz int) []byte {
	out := make([]byte, 0, 512)
	if cz < 0 || cz >= w.CZ {
		return out
	}
	c := w.chunk(wrapI(cx, w.CX), cz).cur
	var cur uint8
	n := 0
	for _, b := range c {
		if n > 0 && b == cur && n < 256 {
			n++
			continue
		}
		if n > 0 {
			out = append(out, byte(n-1), cur)
		}
		cur, n = b, 1
	}
	if n > 0 {
		out = append(out, byte(n-1), cur)
	}
	return out
}

// DecodeChunk is the inverse of EncodeChunk (used by tests and tools).
func DecodeChunk(data []byte) ([]uint8, bool) {
	out := make([]uint8, 0, Chunk*Chunk*SizeY)
	for i := 0; i+1 < len(data); i += 2 {
		for k := 0; k <= int(data[i]); k++ {
			out = append(out, data[i+1])
		}
	}
	return out, len(out) == Chunk*Chunk*SizeY && len(data)%2 == 0
}

// landRegionNear finds the nearest continent to (x,z) in the regions image,
// searching a few map pixels out.
func (w *World) landRegionNear(x, z float64) *Region {
	rm, s := w.Def.regions, float64(w.Def.Scale)
	px, pz := int(math.Floor(w.WrapX(x)/s)), int(math.Floor(z/s))
	for r := 1; r <= 12; r++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dz)) != r {
					continue
				}
				if reg := rm.At(px+dx, pz+dz); reg != nil && !reg.Sea {
					return reg
				}
			}
		}
	}
	return nil
}
