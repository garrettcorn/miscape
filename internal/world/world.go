// Package world holds the voxel grid, the curated world builder, zone
// queries and chunk encoding. It has no knowledge of players or networking.
package world

import "math"

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
	NumBlocks        = 22
)

// Solid reports whether a block blocks movement.
func Solid(b uint8) bool {
	return b != Air && b != Water && b != Flowers
}

const (
	SizeX   = 128
	SizeY   = 40
	SizeZ   = 128
	Chunk   = 16
	ChunksX = SizeX / Chunk
	ChunksZ = SizeZ / Chunk
)

// Zone identifiers.
const (
	ZoneWild    = "wild"
	ZoneVillage = "village"
	ZoneGloam   = "gloam"
)

var (
	VillageCX, VillageCZ, VillageR = 64.0, 64.0, 17.0
	// GloamMinX: everything at or beyond this x (inside the ridge) is the Gloam.
	GloamMinX              = 101.0
	SpawnX, SpawnY, SpawnZ = 64.5, 13.0, 70.5
	AnvilX, AnvilY, AnvilZ = 58, 13, 67
)

type World struct {
	Blocks []uint8 // index = (y*SizeZ+z)*SizeX+x
}

func New() *World { return &World{Blocks: make([]uint8, SizeX*SizeY*SizeZ)} }

func In(x, y, z int) bool {
	return x >= 0 && y >= 0 && z >= 0 && x < SizeX && y < SizeY && z < SizeZ
}

func Index(x, y, z int) int { return (y*SizeZ+z)*SizeX + x }

func (w *World) Get(x, y, z int) uint8 {
	if !In(x, y, z) {
		if y < 0 {
			return Bedrock
		}
		return Air
	}
	return w.Blocks[Index(x, y, z)]
}

func (w *World) Set(x, y, z int, b uint8) {
	if In(x, y, z) {
		w.Blocks[Index(x, y, z)] = b
	}
}

// SolidAt reports whether the block containing world point (x,y,z) is solid.
// Outside the horizontal bounds counts as solid (invisible walls).
func (w *World) SolidAt(x, y, z float64) bool {
	ix, iy, iz := int(math.Floor(x)), int(math.Floor(y)), int(math.Floor(z))
	if ix < 0 || iz < 0 || ix >= SizeX || iz >= SizeZ || iy < 0 {
		return true
	}
	return Solid(w.Get(ix, iy, iz))
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

// ZoneAt classifies a world position.
func ZoneAt(x, z float64) string {
	if x >= GloamMinX {
		return ZoneGloam
	}
	dx, dz := x-VillageCX, z-VillageCZ
	if dx*dx+dz*dz <= VillageR*VillageR {
		return ZoneVillage
	}
	return ZoneWild
}

// EncodeChunk run-length encodes a 16x40x16 chunk column as pairs
// (count-1, block) in x-fastest, then z, then y order.
func (w *World) EncodeChunk(cx, cz int) []byte {
	out := make([]byte, 0, 512)
	var cur uint8
	n := 0
	flush := func() {
		if n > 0 {
			out = append(out, byte(n-1), cur)
		}
	}
	for y := 0; y < SizeY; y++ {
		for z := 0; z < Chunk; z++ {
			for x := 0; x < Chunk; x++ {
				b := w.Get(cx*Chunk+x, y, cz*Chunk+z)
				if n > 0 && b == cur && n < 256 {
					n++
					continue
				}
				flush()
				cur, n = b, 1
			}
		}
	}
	flush()
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
