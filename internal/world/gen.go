package world

import (
	"math"
)

// ---------- noise ----------

func smoothstep(a, b, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-a)/(b-a)))
	return t * t * (3 - 2*t)
}

func hashU(seed uint64, x, z int) uint64 {
	v := seed ^ uint64(x)*0x9E3779B97F4A7C15 ^ uint64(z)*0xC2B2AE3D27D4EB4F
	v ^= v >> 31
	v *= 0xBF58476D1CE4E5B9
	v ^= v >> 29
	v *= 0x94D049BB133111EB
	v ^= v >> 32
	return v
}

func hash2(seed uint64, x, z int) float64 { return float64(hashU(seed, x, z)%1000003) / 1000003 }

// valueNoise is smooth noise in 0..1 that tiles every `period` cells east–west
// (so it is continuous across the date line).
func valueNoise(seed uint64, x, z, scale float64, period int) float64 {
	fx, fz := x/scale, z/scale
	x0, z0 := int(math.Floor(fx)), int(math.Floor(fz))
	tx, tz := fx-float64(x0), fz-float64(z0)
	tx, tz = tx*tx*(3-2*tx), tz*tz*(3-2*tz)
	w := func(i int) int {
		if period > 0 {
			return wrapI(i, period)
		}
		return i
	}
	a := hash2(seed, w(x0), z0)
	b := hash2(seed, w(x0+1), z0)
	c := hash2(seed, w(x0), z0+1)
	d := hash2(seed, w(x0+1), z0+1)
	return (a*(1-tx)+b*tx)*(1-tz) + (c*(1-tx)+d*tx)*tz
}

// ---------- terrain ----------

// townLand is how far past a town's edge the ground is kept dry: its roads
// run 6 blocks out and the gate (lamps, sign, watchman) stands about 4 out.
const townLand = 7

// terrainAt samples the map image for canonical column (x,z). Scaled-up
// levels wobble the sample point so borders between terrain types aren't
// blocky staircases.
func (w *World) terrainAt(x, z int) Terrain {
	if z < 0 || z >= w.H {
		return TIce
	}
	fx, fz := float64(x)+0.5, float64(z)+0.5
	// Towns always stand on open grassland, whatever the map says there,
	// including their roads and gate (which reach townLand blocks past the edge).
	for _, p := range w.Def.Places {
		if p.Structure == "town" && w.Dist2(fx, fz, p.X, p.Z) <= (p.Radius+townLand)*(p.Radius+townLand) {
			return TGrass
		}
	}
	for _, zd := range w.Def.Zones {
		// A ragged edge, so the moor doesn't look like a drawn circle.
		r := zd.Radius + (valueNoise(w.seed+55, fx, fz, 4, w.W/4)-0.5)*5
		if zd.Kind == ZoneDanger && w.Dist2(fx, fz, zd.X, zd.Z) <= r*r {
			return TMoor
		}
	}
	tm, s := w.Def.terrain, w.Def.Scale
	if s == 1 {
		return tm.At(x, z)
	}
	sc := float64(s) * 1.7
	period := int(math.Ceil(float64(w.W) / sc))
	jx := (valueNoise(w.seed+101, float64(x), float64(z), sc, period) - 0.5) * float64(s) * 1.2
	jz := (valueNoise(w.seed+202, float64(x), float64(z), sc, period) - 0.5) * float64(s) * 1.2
	return tm.At(int(math.Floor((float64(x)+jx)/float64(s))), int(math.Floor((float64(z)+jz)/float64(s))))
}

// column fills one column of a chunk with ground for its terrain.
func (w *World) column(c []uint8, lx, lz, x, z int) {
	g := w.Def.GroundY
	set := func(y int, b uint8) {
		if y >= 0 && y < SizeY {
			c[(y*Chunk+lz)*Chunk+lx] = b
		}
	}
	set(0, Bedrock)
	// The poles are a wall of ice cliffs.
	if z < 2 || z >= w.H-2 {
		for y := 1; y < SizeY-6; y++ {
			set(y, Ice)
		}
		return
	}
	t := w.terrainAt(x, z)
	h := hash2(w.seed+7, x, z)
	switch t {
	case TOcean, TShallows:
		floor := g - 6
		if t == TShallows {
			floor = g - 2
		}
		for y := 1; y < floor-1; y++ {
			set(y, Stone)
		}
		set(floor-1, Sand)
		set(floor, Sand)
		for y := floor + 1; y <= g; y++ {
			set(y, Water)
		}
		if h < 0.08 && w.coastal(x, z) {
			set(g, FishSpot)
		}
		return
	case TIce:
		for y := 1; y <= g-3; y++ {
			set(y, Stone)
		}
		for y := g - 2; y <= g; y++ {
			set(y, Ice)
		}
		return
	}
	for y := 1; y <= g-4; y++ {
		set(y, Stone)
	}
	under, top := Dirt, Grass
	switch t {
	case TDesert:
		under, top = Sand, Sand
	case TSnow:
		top = Snow
	case TTundra:
		// Patches of snow over grass (smooth noise, so it isn't a checkerboard).
		if valueNoise(w.seed+9, float64(x), float64(z), 8, w.W/8) < 0.45 {
			top = Snow
		}
	case TRock:
		under, top = Stone, Stone
		if h < 0.05 {
			top = CopperOre
		}
	case TMoor:
		under, top = Gloamstone, GloamMoss
	}
	for y := g - 3; y < g; y++ {
		set(y, under)
	}
	set(g, top)
	// Ground cover: flowers on grassland and in woods.
	if (t == TGrass || t == TForest) && h > 0.955 && !w.cleared(float64(x)+0.5, float64(z)+0.5) {
		set(g+1, Flowers)
	}
}

// ---------- features ----------

// Features (trees, bushes, boulders, crystals) sit one per 4x4 cell at a
// hashed spot, so neighboring chunks agree on them and trunks never touch.
const cell = 4

type feature uint8

const (
	fNone feature = iota
	fTree
	fBerry
	fBoulder
	fDeadwood
	fGlimmer
	fRowan
	fYew
)

func (w *World) featureAt(gx, gz int) (x, z int, f feature, h float64) {
	u := hashU(w.seed+31, gx, gz)
	x, z = wrapI(gx*cell+int(u%2), w.W), gz*cell+int(u>>8%2)
	if z < 3 || z >= w.H-3 || w.cleared(float64(x)+0.5, float64(z)+0.5) {
		return x, z, fNone, 0
	}
	r := float64(u>>16%100000) / 100000
	h = float64(u>>40%1000) / 1000
	switch w.terrainAt(x, z) {
	case TForest:
		if r < 0.66 {
			f = fTree
		} else if r < 0.7 {
			f = fBerry
		} else if r < 0.74 {
			f = fYew
		}
	case TGrass:
		if r < 0.1 {
			f = fTree
		} else if r < 0.15 {
			f = fBerry
		} else if r < 0.18 {
			f = fBoulder
		} else if r < 0.195 {
			f = fRowan
		} else if r < 0.202 {
			f = fYew
		}
	case TTundra, TSnow:
		if r < 0.04 {
			f = fTree
		}
	case TRock:
		if r < 0.2 {
			f = fBoulder
		}
	case TMoor:
		if r < 0.22 {
			f = fDeadwood
		} else if r < 0.42 {
			f = fGlimmer
		} else if r < 0.45 {
			f = fRowan
		} else if r < 0.47 {
			f = fBoulder
		}
	}
	return x, z, f, h
}

// stamp writes the part of a feature that falls inside the chunk.
func (w *World) stamp(c []uint8, x0, z0 int, fx, fz int, f feature, h float64) {
	g := w.Def.GroundY + 1 // first air block above ground
	set := func(dx, y, dz int, b uint8, onlyAir bool) {
		lx := wrapI(fx+dx-x0, w.W)
		lz := fz + dz - z0
		if lx < 0 || lx >= Chunk || lz < 0 || lz >= Chunk || y < 1 || y >= SizeY {
			return
		}
		i := (y*Chunk+lz)*Chunk + lx
		if onlyAir && c[i] != Air {
			return
		}
		c[i] = b
	}
	switch f {
	case fTree, fRowan, fYew:
		trunk, hgt := Log, 4+int(h*2)
		switch f {
		case fRowan:
			trunk, hgt = Rowan, 3+int(h*2)
		case fYew:
			trunk, hgt = Yew, 4
		}
		for i := 0; i < hgt; i++ {
			set(0, g+i, 0, trunk, false)
		}
		top := g + hgt
		for dy := -2; dy <= 1; dy++ {
			r := 2
			if dy == 1 {
				r = 1
			}
			for dz := -r; dz <= r; dz++ {
				for dx := -r; dx <= r; dx++ {
					corner := abs(dx) == r && abs(dz) == r
					if corner && hash2(w.seed+uint64(dy+5), fx+dx, fz+dz) < 0.6 {
						continue
					}
					set(dx, top+dy, dz, Leaves, true)
				}
			}
		}
	case fBerry:
		set(0, g, 0, BerryBush, false)
	case fBoulder:
		set(0, g, 0, w.ore(fx, fz), false)
		set(1, g, 0, Stone, false)
		if h < 0.5 {
			set(0, g, 1, Stone, false)
			set(0, g+1, 0, Stone, false)
		}
	case fDeadwood:
		hgt := 3 + int(h*3)
		for k := 0; k < hgt; k++ {
			set(0, g+k, 0, Deadwood, false)
		}
		set(1, g+hgt-1, 0, Deadwood, false)
		set(-1, g+hgt-2, 0, Deadwood, false)
	case fGlimmer:
		n := 1 + int(h*3)
		for k := 0; k < n; k++ {
			set(0, g+k, 0, Glimmer, false)
		}
		if h > 0.5 {
			set(1, g, 0, Glimmer, false)
		}
	}
}

// genChunk generates the pristine blocks of chunk (cx,cz): ground, then
// features (including ones from neighboring cells that reach in), then
// structures.
func (w *World) genChunk(cx, cz int) []uint8 {
	c := make([]uint8, Chunk*Chunk*SizeY)
	x0, z0 := cx*Chunk, cz*Chunk
	for lz := 0; lz < Chunk; lz++ {
		for lx := 0; lx < Chunk; lx++ {
			w.column(c, lx, lz, x0+lx, z0+lz)
		}
	}
	// Features within 2 blocks of the chunk can reach into it.
	gx0, gx1 := floorDiv(x0-3, cell), floorDiv(x0+Chunk+2, cell)
	gz0, gz1 := floorDiv(z0-3, cell), floorDiv(z0+Chunk+2, cell)
	for gz := gz0; gz <= gz1; gz++ {
		for gx := gx0; gx <= gx1; gx++ {
			fx, fz, f, h := w.featureAt(wrapI(gx, w.W/cell), gz)
			if f != fNone {
				w.stamp(c, x0, z0, fx, fz, f, h)
			}
		}
	}
	for _, o := range w.overlay[w.chunkKey(cx, cz)] {
		c[local(int(o.p.X), int(o.p.Y), int(o.p.Z))] = o.b
	}
	return c
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// cleared reports whether a point is inside a structure's footprint, where
// trees and flowers aren't placed.
func (w *World) cleared(x, z float64) bool {
	for _, c := range w.clear {
		if w.Dist2(x, z, c.x, c.z) <= c.r*c.r {
			return true
		}
	}
	return false
}

// coastal reports whether a water column touches land, where fish gather.
func (w *World) coastal(x, z int) bool {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if t := w.terrainAt(wrapI(x+d[0], w.W), z+d[1]); !t.IsWater() && t != TIce {
			return true
		}
	}
	return false
}

// ore picks what an outcrop holds: copper is common, iron is rare, and the
// moors are coal country.
func (w *World) ore(x, z int) uint8 {
	r := hash2(w.seed+53, x, z)
	if w.terrainAt(x, z) == TMoor {
		if r < 0.6 {
			return Coal
		}
		return IronOre
	}
	switch {
	case r < 0.4:
		return CopperOre
	case r < 0.65:
		return TinOre
	case r < 0.85:
		return Coal
	}
	return IronOre
}
