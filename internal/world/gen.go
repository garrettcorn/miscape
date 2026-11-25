package world

import (
	"math"
	"math/rand/v2"
)

// Build creates the curated Lanternfall map. It is deterministic: the same
// seed always yields the same world, so only player/world diffs need saving.
func Build(seed uint64) *World {
	w := New()
	rng := rand.New(rand.NewPCG(seed, 0x4c616e7465726e))
	h := heightmap(seed)

	// Terrain columns.
	for z := 0; z < SizeZ; z++ {
		for x := 0; x < SizeX; x++ {
			top := h[z*SizeX+x]
			gloam := float64(x) >= GloamMinX
			for y := 0; y <= top; y++ {
				var b uint8
				switch {
				case y == 0:
					b = Bedrock
				case y < top-3:
					b = Stone
				case gloam:
					b = Gloamstone
					if y == top {
						b = GloamMoss
					}
				case y < top:
					b = Dirt
				default:
					b = Grass
				}
				w.Set(x, y, z, b)
			}
		}
	}

	ridge(w)
	pond(w)
	quarry(w, rng)
	paths(w)
	village(w)
	gloam(w, rng, h)
	trees(w, rng)
	decorate(w, rng)
	return w
}

func smoothstep(a, b, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-a)/(b-a)))
	return t * t * (3 - 2*t)
}

func hash2(seed uint64, x, z int) float64 {
	v := seed ^ uint64(x)*0x9E3779B97F4A7C15 ^ uint64(z)*0xC2B2AE3D27D4EB4F
	v ^= v >> 31
	v *= 0xBF58476D1CE4E5B9
	v ^= v >> 29
	return float64(v%10000) / 10000
}

func valueNoise(seed uint64, x, z, scale float64) float64 {
	fx, fz := x/scale, z/scale
	x0, z0 := int(math.Floor(fx)), int(math.Floor(fz))
	tx, tz := fx-float64(x0), fz-float64(z0)
	tx, tz = tx*tx*(3-2*tx), tz*tz*(3-2*tz)
	a := hash2(seed, x0, z0)
	b := hash2(seed, x0+1, z0)
	c := hash2(seed, x0, z0+1)
	d := hash2(seed, x0+1, z0+1)
	return (a*(1-tx)+b*tx)*(1-tz) + (c*(1-tx)+d*tx)*tz
}

func heightmap(seed uint64) []int {
	h := make([]int, SizeX*SizeZ)
	for z := 0; z < SizeZ; z++ {
		for x := 0; x < SizeX; x++ {
			fx, fz := float64(x), float64(z)
			n := valueNoise(seed, fx, fz, 18)*4 + valueNoise(seed+7, fx, fz, 7)*1.5
			y := 11 + n
			// Rolling hills at the map edge make a soft border.
			edge := math.Min(math.Min(fx, fz), math.Min(SizeX-1-fx, SizeZ-1-fz))
			y += (1 - smoothstep(0, 8, edge)) * 9
			// Flatten the village and the gate approach.
			dv := math.Hypot(fx-VillageCX, fz-VillageCZ)
			t := smoothstep(VillageR+1, VillageR+8, dv)
			y = 12*(1-t) + y*t
			if fx >= 84 && fx < 101 {
				g := smoothstep(4, 9, math.Abs(fz-63))
				y = 12*(1-g) + y*g
			}
			if fx >= GloamMinX {
				y = 9 + valueNoise(seed+13, fx, fz, 9)*3 + (1-smoothstep(0, 6, SizeX-1-fx))*9
			}
			h[z*SizeX+x] = int(math.Round(y))
		}
	}
	return h
}

// ridge builds the stone wall separating the Gloam, with one lit gate.
func ridge(w *World) {
	for z := 0; z < SizeZ; z++ {
		for x := 98; x < 101; x++ {
			gate := z >= 61 && z <= 65
			top := 24
			if gate {
				// Clear the gate passage down to path level.
				for y := 13; y < SizeY; y++ {
					w.Set(x, y, z, Air)
				}
				for y := 1; y <= 12; y++ {
					w.Set(x, y, z, Stone)
				}
				w.Set(x, 12, z, Cobble)
				// Lintel over the gate.
				for y := 18; y <= 20; y++ {
					w.Set(x, y, z, Gloamstone)
				}
				continue
			}
			for y := 1; y <= top; y++ {
				w.Set(x, y, z, Stone)
			}
			if (x+z)%5 == 0 {
				w.Set(x, top, z, Gloamstone)
			}
		}
	}
	// Lantern pillars flanking the gate, and a warning sign.
	for _, z := range []int{59, 67} {
		for y := 13; y <= 16; y++ {
			w.Set(97, y, z, Plank)
		}
		w.Set(97, 17, z, Lantern)
	}
	w.Set(96, 13, 59, Sign)
}

func pond(w *World) {
	cx, cz, r := 30.0, 70.0, 9.0
	for z := int(cz - r - 2); z <= int(cz+r+2); z++ {
		for x := int(cx - r - 2); x <= int(cx+r+2); x++ {
			d := math.Hypot(float64(x)-cx, float64(z)-cz)
			if d > r+2 {
				continue
			}
			if d <= r {
				depth := int(8 + (d/r)*3)
				for y := depth + 1; y < SizeY; y++ {
					w.Set(x, y, z, Air)
				}
				w.Set(x, depth, z, Sand)
				for y := depth + 1; y <= 11; y++ {
					w.Set(x, y, z, Water)
				}
			} else {
				// Sandy shore.
				top := w.SurfaceY(x, z) - 1
				if top >= 0 && w.Get(x, top, z) == Grass {
					w.Set(x, top, z, Sand)
				}
			}
		}
	}
}

func quarry(w *World, rng *rand.Rand) {
	cx, cz, r := 64.0, 104.0, 10.0
	for z := int(cz - r - 1); z <= int(cz+r+1); z++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			d := math.Hypot(float64(x)-cx, float64(z)-cz)
			if d > r+1 {
				continue
			}
			// Terraced pit: deeper toward the center.
			floor := 6 + int(math.Floor(d/r*5))
			if d > r {
				floor = 12
			}
			for y := floor + 1; y < SizeY; y++ {
				w.Set(x, y, z, Air)
			}
			for y := 1; y <= floor; y++ {
				b := Stone
				if y >= floor-1 && rng.Float64() < 0.07 {
					b = CopperOre
				}
				w.Set(x, y, z, b)
			}
		}
	}
	// A few chunky boulders and a ramp down.
	for i := 0; i < 6; i++ {
		x := int(cx) + rng.IntN(13) - 6
		z := int(cz) + rng.IntN(13) - 6
		y := w.SurfaceY(x, z)
		w.Set(x, y, z, CopperOre)
		w.Set(x+1, y, z, Stone)
	}
}

func paths(w *World) {
	seg := func(x0, z0, x1, z1 int) {
		steps := int(math.Max(math.Abs(float64(x1-x0)), math.Abs(float64(z1-z0))))
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(max(steps, 1))
			x := int(math.Round(float64(x0) + t*float64(x1-x0)))
			z := int(math.Round(float64(z0) + t*float64(z1-z0)))
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					px, pz := x+dx, z+dz
					y := w.SurfaceY(px, pz) - 1
					if y > 0 {
						b := w.Get(px, y, pz)
						if b == Grass || b == Dirt || b == Sand {
							w.Set(px, y, pz, Cobble)
						}
					}
				}
			}
		}
	}
	seg(64, 64, 64, 36) // north to the woods
	seg(64, 64, 64, 92) // south to the quarry
	seg(64, 64, 97, 63) // east to the gate
	seg(64, 64, 41, 68) // west to the pond
}

func box(w *World, x0, y0, z0, x1, y1, z1 int, b uint8) {
	for y := y0; y <= y1; y++ {
		for z := z0; z <= z1; z++ {
			for x := x0; x <= x1; x++ {
				w.Set(x, y, z, b)
			}
		}
	}
}

func house(w *World, x0, z0 int, doorSouth bool) {
	x1, z1 := x0+5, z0+5
	box(w, x0, 12, z0, x1, 12, z1, Plank) // floor
	for y := 13; y <= 15; y++ {
		for z := z0; z <= z1; z++ {
			for x := x0; x <= x1; x++ {
				edge := x == x0 || x == x1 || z == z0 || z == z1
				if !edge {
					w.Set(x, y, z, Air)
					continue
				}
				corner := (x == x0 || x == x1) && (z == z0 || z == z1)
				b := Plank
				if corner {
					b = Log
				}
				w.Set(x, y, z, b)
			}
		}
	}
	// Door and windows.
	dz := z0
	if doorSouth {
		dz = z1
	}
	w.Set(x0+2, 13, dz, Air)
	w.Set(x0+2, 14, dz, Air)
	w.Set(x0, 14, z0+2, Air)
	w.Set(x1, 14, z0+3, Air)
	// Stepped roof.
	for i := 0; i < 3; i++ {
		box(w, x0-1+i, 16+i, z0-1+i, x1+1-i, 16+i, z1+1-i, Roof)
	}
	w.Set(x0+2, 15, z0+2, Lantern) // warm light inside
}

func village(w *World) {
	cx, cz := int(VillageCX), int(VillageCZ)
	for z := cz - 6; z <= cz+6; z++ {
		for x := cx - 6; x <= cx+6; x++ {
			if math.Hypot(float64(x-cx), float64(z-cz)) <= 6.3 {
				w.Set(x, 12, z, Cobble)
				for y := 13; y < SizeY; y++ {
					w.Set(x, y, z, Air)
				}
			}
		}
	}
	// Lamp posts ringing the plaza.
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x := cx + int(math.Round(math.Cos(a)*7.5))
		z := cz + int(math.Round(math.Sin(a)*7.5))
		w.Set(x, 13, z, Plank)
		w.Set(x, 14, z, Plank)
		w.Set(x, 15, z, Lantern)
	}
	// A little well in the center.
	box(w, cx-1, 13, cz-1, cx+1, 13, cz+1, Cobble)
	w.Set(cx, 13, cz, Water)
	house(w, 50, 52, true)
	house(w, 72, 51, true)
	house(w, 49, 74, false)
	// Bertram's stall.
	box(w, 70, 13, 68, 72, 13, 68, Plank)
	w.Set(70, 14, 68, BerryBush)
	// Anvil by Mabel's workshop.
	w.Set(AnvilX, AnvilY-1, AnvilZ, Cobble)
	w.Set(AnvilX, AnvilY, AnvilZ, Anvil)
}

func trees(w *World, rng *rand.Rand) {
	tree := func(x, z int) {
		y := w.SurfaceY(x, z)
		if y < 1 || w.Get(x, y-1, z) != Grass {
			return
		}
		// Keep clear of other trunks.
		for dz := -2; dz <= 2; dz++ {
			for dx := -2; dx <= 2; dx++ {
				if w.Get(x+dx, y, z+dz) == Log {
					return
				}
			}
		}
		hgt := 4 + rng.IntN(2)
		for i := 0; i < hgt; i++ {
			w.Set(x, y+i, z, Log)
		}
		top := y + hgt
		for dy := -2; dy <= 1; dy++ {
			r := 2
			if dy == 1 {
				r = 1
			}
			for dz := -r; dz <= r; dz++ {
				for dx := -r; dx <= r; dx++ {
					if abs(dx) == r && abs(dz) == r && rng.Float64() < 0.6 {
						continue
					}
					if w.Get(x+dx, top+dy, z+dz) == Air {
						w.Set(x+dx, top+dy, z+dz, Leaves)
					}
				}
			}
		}
	}
	for z := 4; z < SizeZ-4; z++ {
		for x := 4; x < int(GloamMinX)-4; x++ {
			fx, fz := float64(x), float64(z)
			if ZoneAt(fx, fz) == ZoneVillage || math.Hypot(fx-30, fz-70) < 12 || math.Hypot(fx-64, fz-104) < 13 {
				continue
			}
			if x > 84 && z > 54 && z < 72 { // gate approach
				continue
			}
			p := 0.008
			if x < 56 && z < 56 {
				p = 0.07 // Mossmeadow woods
			}
			if rng.Float64() < p {
				tree(x, z)
			}
		}
	}
}

func gloam(w *World, rng *rand.Rand, h []int) {
	for i := 0; i < 26; i++ {
		x := int(GloamMinX) + 3 + rng.IntN(SizeX-int(GloamMinX)-8)
		z := 6 + rng.IntN(SizeZ-12)
		y := w.SurfaceY(x, z)
		n := 1 + rng.IntN(3)
		for k := 0; k < n; k++ {
			w.Set(x, y+k, z, Glimmer)
		}
		if rng.Float64() < 0.5 {
			w.Set(x+1, y, z, Glimmer)
		}
	}
	for i := 0; i < 22; i++ {
		x := int(GloamMinX) + 3 + rng.IntN(SizeX-int(GloamMinX)-8)
		z := 6 + rng.IntN(SizeZ-12)
		y := w.SurfaceY(x, z)
		if w.Get(x, y-1, z) != GloamMoss {
			continue
		}
		hgt := 3 + rng.IntN(3)
		for k := 0; k < hgt; k++ {
			w.Set(x, y+k, z, Deadwood)
		}
		w.Set(x+1, y+hgt-1, z, Deadwood)
		w.Set(x-1, y+hgt-2, z, Deadwood)
	}
	_ = h
}

func decorate(w *World, rng *rand.Rand) {
	for z := 2; z < SizeZ-2; z++ {
		for x := 2; x < int(GloamMinX)-2; x++ {
			y := w.SurfaceY(x, z)
			if y < 1 || w.Get(x, y-1, z) != Grass || w.Get(x, y, z) != Air {
				continue
			}
			r := rng.Float64()
			switch {
			case ZoneAt(float64(x), float64(z)) == ZoneVillage:
			case r < 0.006:
				w.Set(x, y, z, BerryBush)
			case r < 0.05:
				w.Set(x, y, z, Flowers)
			}
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
