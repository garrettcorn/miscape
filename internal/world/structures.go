package world

import (
	"fmt"
	"math"
)

// Structures are hand-designed builds (a town) stamped at a place. They are
// laid out once when the world is created and stored as an overlay that chunk
// generation applies, so they cost memory only for their own blocks.

// builder collects structure blocks around an origin.
type builder struct {
	w      *World
	blocks map[Pos]uint8
	ox, oz int // origin (canonical x)
	g      int // ground y (top ground block)
}

func (b *builder) set(dx, y, dz int, blk uint8) {
	z := b.oz + dz
	if y < 1 || y >= SizeY || z < 0 || z >= b.w.H {
		return
	}
	b.blocks[Pos{int32(wrapI(b.ox+dx, b.w.W)), int32(y), int32(z)}] = blk
}

func (b *builder) box(x0, y0, z0, x1, y1, z1 int, blk uint8) {
	for y := y0; y <= y1; y++ {
		for z := z0; z <= z1; z++ {
			for x := x0; x <= x1; x++ {
				b.set(x, y, z, blk)
			}
		}
	}
}

// land reports whether the ground at offset (dx,dz) is dry land.
func (b *builder) land(dx, dz int) bool {
	t := b.w.terrainAt(wrapI(b.ox+dx, b.w.W), b.oz+dz)
	return !t.IsWater() && t != TIce
}

func (w *World) buildStructures() {
	all := map[Pos]uint8{}
	for _, p := range w.Def.Places {
		ox, oz := int(math.Floor(p.X)), int(math.Floor(p.Z))
		w.Points["place:"+p.ID] = Point{float64(ox) + 0.5, float64(w.Def.GroundY + 1), float64(oz) + 0.5}
		if p.Structure != "town" && p.Structure != "village" {
			continue
		}
		r := p.Radius
		if r <= 0 {
			r = map[string]float64{"town": 18, "village": 10}[p.Structure]
		}
		// Trees stay out of town and off the roads just outside it.
		w.clear = append(w.clear, clearing{float64(ox) + 0.5, float64(oz) + 0.5, r + 8})
		b := &builder{w: w, blocks: all, ox: ox, oz: oz, g: w.Def.GroundY}
		if p.Structure == "village" {
			b.village(p.ID, int(r), w.gateDir(p.X, p.Z))
		} else {
			b.town(p.ID, int(r), w.gateDir(p.X, p.Z))
		}
	}
	for p, blk := range all {
		k := w.chunkKey(int(p.X)/Chunk, int(p.Z)/Chunk)
		w.overlay[k] = append(w.overlay[k], overlayBlock{p, blk})
	}
}

// town is the starting town: a lamp-lit plaza with a well, three houses, a
// market stall, an anvil, and cobbled roads leading out (east toward the
// danger zone). It reports its spawn and anvil as named points.
func (b *builder) town(id string, r int, gate [2]int) {
	g := b.g
	// Plaza.
	for z := -6; z <= 6; z++ {
		for x := -6; x <= 6; x++ {
			if math.Hypot(float64(x), float64(z)) <= 6.3 {
				b.set(x, g, z, Cobble)
			}
		}
	}
	// Roads (laid before buildings so floors win).
	road := func(x1, z1 int) {
		steps := max(abs(x1), abs(z1))
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(max(steps, 1))
			x, z := int(math.Round(t*float64(x1))), int(math.Round(t*float64(z1)))
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					if b.land(x+dx, z+dz) {
						b.set(x+dx, g, z+dz, Cobble)
					}
				}
			}
		}
	}
	reach := r + 6
	road(0, -reach)
	road(0, reach)
	road(reach, -1)
	road(-reach, 4)
	// Lamp posts ringing the plaza.
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x, z := int(math.Round(math.Cos(a)*7.5)), int(math.Round(math.Sin(a)*7.5))
		b.lamp(x, z)
	}
	// Well.
	b.box(-1, g+1, -1, 1, g+1, 1, Cobble)
	b.set(0, g+1, 0, Water)
	b.house(-14, -12, true)
	b.house(8, -13, true)
	b.house(-15, 10, false)
	// Market stall.
	b.box(6, g+1, 4, 8, g+1, 4, Plank)
	b.set(6, g+2, 4, Stall)
	b.set(8, g+2, 4, Stall)
	// Anvil by the lamplighter's workshop.
	b.set(-6, g, 3, Cobble)
	b.set(-6, g+1, 3, Anvil)
	// A kitchen range across the plaza, for cooking.
	b.set(5, g+1, -3, Range)
	// Allotments: a field of tilled soil with a path round it, for farming.
	for z := 6; z <= 11; z++ {
		for x := 7; x <= 14; x++ {
			if !b.land(x, z) {
				continue
			}
			if x == 7 || x == 14 || z == 6 || z == 11 {
				b.set(x, g, z, Cobble)
			} else {
				b.set(x, g, z, Soil)
			}
		}
	}
	// Lantern posts and a warning sign where the road toward danger leaves town.
	gx, gz := gate[0]*(r+2), gate[1]*(r+2)
	sx, sz := gate[1], gate[0] // sideways
	b.lamp(gx-3*sx, gz-3*sz)
	b.lamp(gx+3*sx, gz+3*sz)
	b.set(gx+gate[0]-3*sx, g+1, gz+gate[1]-3*sz, Sign)

	w := b.w
	pt := func(name string, dx, dz float64) {
		w.Points[name] = Point{w.WrapX(float64(b.ox) + dx), float64(g + 1), float64(b.oz) + dz}
	}
	townPt := func(name string, dx, dz float64) {
		pt("town:"+id+":"+name, dx, dz)
		if w.Def.Spawn == id {
			pt(name, dx, dz)
		}
	}
	townPt("anvil", -6, 3)
	townPt("range", 5, -3)
	townPt("allotment", 10.5, 8.5)
	townPt("spawn", 0.5, 6.5)
	pt("town:"+id+":gate", float64(gate[0]*(r+4))+0.5, float64(gate[1]*(r+4))+0.5)
}

// village is a small settlement: a cobbled green with a well, a ring of
// lamps, two cottages, a market stall, an anvil and a kitchen range, and two
// roads out. Its points are town:<id>:spawn, :gate, :anvil, :range and :stall.
func (b *builder) village(id string, r int, gate [2]int) {
	g := b.g
	for z := -4; z <= 4; z++ {
		for x := -4; x <= 4; x++ {
			if math.Hypot(float64(x), float64(z)) <= 4.3 {
				b.set(x, g, z, Cobble)
			}
		}
	}
	reach := r + 6
	for _, d := range [][2]int{gate, {-gate[0], -gate[1]}} {
		for i := 0; i <= reach; i++ {
			x, z := d[0]*i, d[1]*i
			for k := -1; k <= 1; k++ {
				if b.land(x+k*d[1], z+k*d[0]) {
					b.set(x+k*d[1], g, z+k*d[0], Cobble)
				}
			}
		}
	}
	for i := 0; i < 6; i++ {
		a := float64(i)*math.Pi/3 + math.Pi/6
		b.lamp(int(math.Round(math.Cos(a)*5.5)), int(math.Round(math.Sin(a)*5.5)))
	}
	b.box(-1, g+1, -1, 1, g+1, 1, Cobble)
	b.set(0, g+1, 0, Water)
	// Cottages either side of the road, clear of the green and the lamps.
	if gate[0] != 0 {
		b.house(-8, 6, false)
		b.house(-8, -11, true)
	} else {
		b.house(6, -8, true)
		b.house(-11, -8, true)
	}
	// A stall, an anvil and a range around the green.
	b.box(3, g+1, 3, 4, g+1, 3, Plank)
	b.set(3, g+2, 3, Stall)
	b.set(-3, g, 3, Cobble)
	b.set(-3, g+1, 3, Anvil)
	b.set(3, g+1, -3, Range)

	w := b.w
	pt := func(name string, dx, dz float64) {
		w.Points["town:"+id+":"+name] = Point{w.WrapX(float64(b.ox) + dx), float64(g + 1), float64(b.oz) + dz}
	}
	pt("spawn", 0.5, -2.5)
	pt("anvil", -3, 3)
	pt("range", 3, -3)
	pt("stall", 3, 3)
	pt("gate", float64(gate[0]*(r+4))+0.5, float64(gate[1]*(r+4))+0.5)
}

func (b *builder) lamp(x, z int) {
	g := b.g
	b.set(x, g+1, z, Plank)
	b.set(x, g+2, z, Plank)
	b.set(x, g+3, z, Lantern)
}

func (b *builder) house(x0, z0 int, doorSouth bool) {
	g := b.g
	x1, z1 := x0+5, z0+5
	b.box(x0, g, z0, x1, g, z1, Plank) // floor
	for y := g + 1; y <= g+3; y++ {
		for z := z0; z <= z1; z++ {
			for x := x0; x <= x1; x++ {
				edge := x == x0 || x == x1 || z == z0 || z == z1
				if !edge {
					b.set(x, y, z, Air)
					continue
				}
				blk := Plank
				if (x == x0 || x == x1) && (z == z0 || z == z1) {
					blk = Log
				}
				b.set(x, y, z, blk)
			}
		}
	}
	dz := z0
	if doorSouth {
		dz = z1
	}
	b.set(x0+2, g+1, dz, Air)
	b.set(x0+2, g+2, dz, Air)
	b.set(x0, g+2, z0+2, Air)
	b.set(x1, g+2, z0+3, Air)
	for i := 0; i < 3; i++ {
		b.box(x0-1+i, g+4+i, z0-1+i, x1+1-i, g+4+i, z1+1-i, Roof)
	}
	b.set(x0+2, g+3, z0+2, Lantern)
}

// Point returns a named point, or an error naming what's missing.
func (w *World) Point(name string) (Point, error) {
	p, ok := w.Points[name]
	if !ok {
		return Point{}, fmt.Errorf("world has no point %q", name)
	}
	return p, nil
}

// Spawn is where new characters start and the Hearth Lantern returns to.
func (w *World) Spawn() Point { return w.Points["spawn"] }

// gateDir is the road direction (a unit step along x or z) from a town toward
// the nearest danger zone; east if there is none.
func (w *World) gateDir(x, z float64) [2]int {
	best, dir := math.Inf(1), [2]int{1, 0}
	for _, zd := range w.Def.Zones {
		if zd.Kind != ZoneDanger {
			continue
		}
		if d := w.Dist2(x, z, zd.X, zd.Z); d < best {
			best = d
			dx, dz := w.DX(x, zd.X), zd.Z-z
			if math.Abs(dx) >= math.Abs(dz) {
				dir = [2]int{int(math.Copysign(1, dx)), 0}
			} else {
				dir = [2]int{0, int(math.Copysign(1, dz))}
			}
		}
	}
	return dir
}

// Locate finds a place, zone or named point (for content "at" fields): its
// center, and the height someone standing there would have.
func (w *World) Locate(name string) (x, y, z float64, ok bool) {
	stand := float64(w.Def.GroundY + 1)
	if p := w.Def.Place(name); p != nil {
		pt := w.Points["place:"+name]
		return pt.X, pt.Y, pt.Z, true
	}
	for _, zd := range w.Def.Zones {
		if zd.ID == name {
			return zd.X, stand, zd.Z, true
		}
	}
	if pt, ok := w.Points[name]; ok {
		return pt.X, pt.Y, pt.Z, true
	}
	return 0, 0, 0, false
}
