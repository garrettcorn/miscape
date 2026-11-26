package world

import (
	"fmt"
	"math"
	"sort"
)

// ---------- resource hotspots ----------
//
// Resources bunch up in sites: mines (1-3 kinds of ore), woods (1-2 kinds
// of tree), fishing grounds (coast thick with fishing spots) and hunting
// grounds (where game animals gather). What a site holds depends on its
// danger level, so the better ores and timber are further from home. Sites
// come from a hash of the world seed and the map, like trees, so the server,
// the in-browser engine and the maps all agree and nothing is saved. Every
// town also gets sites for the skills it's known for, close by.

const (
	SiteMine    = "mine"
	SiteWood    = "wood"
	SiteFishing = "fishing"
	SiteHunting = "hunting"

	siteCell = 64 // blocks per site cell (× the level's scale)
)

// Site is one resource hotspot.
type Site struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	X      float64 `json:"x"`
	Z      float64 `json:"z"`
	R      float64 `json:"r"`
	Danger int     `json:"danger"`
	Blocks []uint8 `json:"blocks,omitempty"` // the ores in a mine, the trees in a wood
	Town   string  `json:"town,omitempty"`   // the town it was placed for, if any
}

type band struct {
	b      uint8
	lo, hi int // danger levels (on a 1-10 scale) it appears at
}

// OreBands and TreeBands say which ores and trees appear at which danger.
var (
	OreBands  = []band{{CopperOre, 1, 3}, {TinOre, 1, 4}, {Coal, 2, 6}, {IronOre, 4, 8}, {Glimmer, 6, 10}, {SilverOre, 8, 10}}
	TreeBands = []band{{Log, 1, 5}, {Rowan, 2, 7}, {Yew, 5, 10}}
)

// BandsAt lists the blocks of bands that appear at danger level t (1-10 scale).
func BandsAt(bands []band, t int) []uint8 {
	var out []uint8
	for _, b := range bands {
		if t >= b.lo && t <= b.hi {
			out = append(out, b.b)
		}
	}
	return out
}

// Tier is a danger level on the 1-10 scale bands and catches use, whatever
// the world's level.
func (w *World) Tier(danger int) int {
	return max(1, min(10, int(math.Ceil(float64(danger)*10/float64(w.MaxDanger())))))
}

// Sites lists every resource hotspot.
func (w *World) Sites() []*Site { return w.sites }

// SiteAt is the hotspot covering (x,z), or nil.
func (w *World) SiteAt(x, z float64) *Site {
	for _, s := range w.siteIdx[w.chunkKey(int(math.Floor(wrapF(x, w.W)))/Chunk, int(math.Floor(z))/Chunk)] {
		if w.Dist2(x, z, s.X, s.Z) <= s.R*s.R {
			return s
		}
	}
	return nil
}

// skillSites says which hotspots a town known for a skill gets nearby.
var skillSites = map[string][]string{
	"mining": {SiteMine}, "crafting": {SiteMine}, "woodcutting": {SiteWood}, "firemaking": {SiteWood},
	"magic": {SiteWood}, "fishing": {SiteFishing}, "cooking": {SiteFishing}, "hunting": {SiteHunting},
	"range": {SiteHunting}, "slaying": {SiteHunting},
}

func (w *World) buildSites() {
	w.siteIdx = map[int64][]*Site{}
	scale := max(1, w.Def.Scale)
	// Sites for the towns first, so they get the best spots near home.
	for _, p := range w.Def.Places {
		if p.Structure == "" {
			continue
		}
		want := map[string]bool{}
		for _, sk := range p.Known {
			for _, k := range skillSites[sk] {
				want[k] = true
			}
		}
		if p.ID == w.Def.Spawn { // new players need a bit of everything
			for _, k := range []string{SiteMine, SiteWood, SiteFishing, SiteHunting} {
				want[k] = true
			}
		}
		kinds := make([]string, 0, len(want))
		for k := range want {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			w.townSite(p, k)
		}
	}
	cell := siteCell * scale
	for gz := 0; gz < w.H/cell; gz++ {
		for gx := 0; gx < w.W/cell; gx++ {
			w.cellSite(gx, gz, cell)
		}
	}
}

func siteRadius(kind string, scale int) float64 {
	return map[string]float64{SiteMine: 7, SiteWood: 9, SiteFishing: 9, SiteHunting: 12}[kind] * math.Sqrt(float64(scale))
}

// fits reports whether a site of kind can go at (x,z): the right ground,
// away from towns, the poles and other sites.
func (w *World) siteFits(kind string, x, z, r, townGap float64) bool {
	if z < r+8 || z > float64(w.H)-r-8 {
		return false
	}
	for _, p := range w.Def.Places {
		if p.Structure != "" && w.Dist(x, z, p.X, p.Z) < p.Radius+townLand+r+townGap {
			return false
		}
	}
	for _, s := range w.sites {
		if w.Dist(x, z, s.X, s.Z) < s.R+r+6 {
			return false
		}
	}
	ix, iz := int(math.Floor(x)), int(math.Floor(z))
	t := w.terrainAt(ix, iz)
	if kind == SiteFishing {
		return t.IsWater() && w.coastal(ix, iz)
	}
	if t.IsWater() || t == TIce {
		return false
	}
	// Land sites need dry ground right across them.
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		if tt := w.terrainAt(wrapI(int(math.Floor(x+math.Cos(a)*r)), w.W), int(math.Floor(z+math.Sin(a)*r))); tt.IsWater() || tt == TIce {
			return false
		}
	}
	return true
}

func (w *World) townSite(p Place, kind string) {
	h := hashU(w.seed+97, int(p.X)*31+len(kind), int(p.Z)*17+int(kind[0]))
	r := siteRadius(kind, w.Def.Scale)
	a0 := float64(h%360) * math.Pi / 180
	for i := 0; i < 96; i++ {
		a := a0 + float64(i)*2.39996 // golden angle: spreads the tries around
		d := p.Radius + townLand + r + 4 + float64(i%12)*5
		x, z := w.WrapX(p.X+math.Cos(a)*d), p.Z+math.Sin(a)*d
		if w.inDangerZone(x, z, r) {
			continue // a town's own hotspots are somewhere a newcomer can go
		}
		if w.siteFits(kind, x, z, r, 2) {
			w.addSite(&Site{ID: fmt.Sprintf("%s:%s", p.ID, kind), Kind: kind, X: x, Z: z, R: r, Town: p.ID}, h>>20)
			return
		}
	}
}

func (w *World) cellSite(gx, gz, cell int) {
	u := hashU(w.seed+71, gx, gz)
	roll := float64(u>>40%1000) / 1000
	pick := func(i uint64) (float64, float64) {
		v := hashU(w.seed+73+i, gx, gz)
		return float64(gx*cell) + 8 + float64(v%uint64(cell-16)), float64(gz*cell) + 8 + float64(v>>20%uint64(cell-16))
	}
	x, z := pick(0)
	kind := ""
	switch w.terrainAt(int(x), int(z)) {
	case TOcean, TShallows:
		// Look around the cell for a stretch of coast.
		for i := uint64(1); i < 14 && kind == ""; i++ {
			if cx, cz := pick(i); w.coastal(int(cx), int(cz)) && w.terrainAt(int(cx), int(cz)).IsWater() && roll < 0.55 {
				x, z, kind = cx, cz, SiteFishing
			}
		}
	case TRock:
		if roll < 0.7 {
			kind = SiteMine
		}
	case TMoor:
		if roll < 0.5 {
			kind = SiteMine
		}
	case TTundra, TSnow:
		switch {
		case roll < 0.25:
			kind = SiteMine
		case roll < 0.45:
			kind = SiteHunting
		}
	case TForest:
		switch {
		case roll < 0.45:
			kind = SiteWood
		case roll < 0.65:
			kind = SiteHunting
		}
	case TGrass:
		switch {
		case roll < 0.15:
			kind = SiteMine
		case roll < 0.3:
			kind = SiteWood
		case roll < 0.44:
			kind = SiteHunting
		}
	case TDesert:
		if roll < 0.15 {
			kind = SiteMine
		}
	}
	if kind == "" {
		return
	}
	r := siteRadius(kind, w.Def.Scale)
	if !w.siteFits(kind, x, z, r, 22) {
		return
	}
	w.addSite(&Site{ID: fmt.Sprintf("%s:%d:%d", kind, gx, gz), Kind: kind, X: x, Z: z, R: r}, u)
}

// addSite fills in a site's danger and contents and indexes it by chunk.
func (w *World) addSite(s *Site, h uint64) {
	s.X = math.Floor(s.X) + 0.5
	s.Z = math.Floor(s.Z) + 0.5
	s.Danger = max(1, w.distanceDanger(s.X, s.Z))
	for _, zd := range w.Def.Zones {
		if zd.Danger > 0 && w.Dist(s.X, s.Z, zd.X, zd.Z) <= zd.Radius {
			s.Danger = zd.Danger
		}
	}
	t := w.Tier(s.Danger)
	switch s.Kind {
	case SiteMine:
		n := 1 + int(h%3)
		if s.Town != "" {
			n = max(n, 2) // a town's own mine is worth the walk
		}
		s.Blocks = choose(BandsAt(OreBands, t), n, h>>8)
	case SiteWood:
		s.Blocks = choose(BandsAt(TreeBands, t), 1+int(h>>4%2), h>>12)
	}
	w.sites = append(w.sites, s)
	reach := int(s.R) + 4
	for cz := (int(s.Z) - reach) / Chunk; cz <= (int(s.Z)+reach)/Chunk; cz++ {
		for x := int(s.X) - reach; x <= int(s.X)+reach+Chunk; x += Chunk {
			k := w.chunkKey(wrapI(x, w.W)/Chunk, cz)
			if l := w.siteIdx[k]; len(l) == 0 || l[len(l)-1] != s {
				w.siteIdx[k] = append(l, s)
			}
		}
	}
}

// choose picks up to n of opts, deterministically from h, keeping their order.
func choose(opts []uint8, n int, h uint64) []uint8 {
	if n >= len(opts) {
		return append([]uint8(nil), opts...)
	}
	idx := make([]int, len(opts))
	for i := range idx {
		idx[i] = i
	}
	for i := len(idx) - 1; i > 0; i-- {
		j := int(h % uint64(i+1))
		h = h/uint64(i+1) + 0x9E37
		idx[i], idx[j] = idx[j], idx[i]
	}
	idx = idx[:n]
	sort.Ints(idx)
	out := make([]uint8, n)
	for i, k := range idx {
		out[i] = opts[k]
	}
	return out
}

// stampSites draws the hotspots that reach into chunk (cx,cz).
func (w *World) stampSites(c []uint8, cx, cz int) {
	x0, z0 := cx*Chunk, cz*Chunk
	g := w.Def.GroundY
	at := func(lx, y, lz int) *uint8 { return &c[(y*Chunk+lz)*Chunk+lx] }
	for _, s := range w.siteIdx[w.chunkKey(cx, cz)] {
		if s.Kind == SiteWood {
			w.stampWood(c, x0, z0, s)
			continue
		}
		for lz := 0; lz < Chunk; lz++ {
			for lx := 0; lx < Chunk; lx++ {
				x, z := x0+lx, z0+lz
				d := math.Hypot(w.DX(s.X, float64(x)+0.5), float64(z)+0.5-s.Z)
				if d > s.R {
					continue
				}
				h := hash2(w.seed+131, x, z)
				switch s.Kind {
				case SiteMine:
					if ground := *at(lx, g, lz); ground == Water || ground == Air {
						continue
					}
					// A rocky mound, ore showing through its surface.
					top := g + int((1-d/s.R)*3.4)
					for y := g + 1; y < g+12; y++ {
						*at(lx, y, lz) = Air
					}
					for y := g; y <= top; y++ {
						*at(lx, y, lz) = Stone
					}
					if h < 0.42 && len(s.Blocks) > 0 {
						*at(lx, top, lz) = s.Blocks[int(hash2(w.seed+137, x, z)*float64(len(s.Blocks)))%len(s.Blocks)]
					} else if h > 0.85 {
						*at(lx, top, lz) = Cobble
					}
				case SiteFishing:
					if *at(lx, g, lz) == Water && h < 0.33 {
						*at(lx, g, lz) = FishSpot
					}
				case SiteHunting:
					if *at(lx, g, lz) == Grass && *at(lx, g+1, lz) == Air && h < 0.04 {
						*at(lx, g+1, lz) = BerryBush
					}
				}
			}
		}
		if s.Kind == SiteMine {
			// A timber-framed entrance with a lantern, on the south side.
			ex, ez := int(math.Floor(s.X)), int(math.Floor(s.Z+s.R))-1
			put := func(x, y, z int, b uint8) {
				lx, lz := wrapI(x-x0, w.W), z-z0
				if lx >= 0 && lx < Chunk && lz >= 0 && lz < Chunk && y > 0 && y < SizeY {
					*at(lx, y, lz) = b
				}
			}
			for y := g + 1; y <= g+3; y++ {
				put(ex-1, y, ez, Log)
				put(ex+1, y, ez, Log)
			}
			put(ex-1, g+4, ez, Plank)
			put(ex, g+4, ez, Plank)
			put(ex+1, g+4, ez, Plank)
			put(ex, g+5, ez, Lantern)
		}
	}
}

// stampWood plants a dense stand of the wood's trees.
func (w *World) stampWood(c []uint8, x0, z0 int, s *Site) {
	r := int(s.R)
	for dz := -r; dz <= r; dz += 3 {
		for dx := -r; dx <= r; dx += 3 {
			fx, fz := int(s.X)+dx, int(s.Z)+dz
			h := hashU(w.seed+139, fx, fz)
			fx += int(h % 2)
			if math.Hypot(float64(fx-int(s.X)), float64(fz-int(s.Z))) > s.R || h>>8%10 < 2 {
				continue
			}
			if t := w.terrainAt(wrapI(fx, w.W), fz); t.IsWater() || t == TIce {
				continue
			}
			f := fTree
			if len(s.Blocks) > 0 {
				switch s.Blocks[int(h>>16)%len(s.Blocks)] {
				case Rowan:
					f = fRowan
				case Yew:
					f = fYew
				}
			}
			w.stamp(c, x0, z0, wrapI(fx, w.W), fz, f, float64(h>>24%1000)/1000)
		}
	}
}

func (w *World) inDangerZone(x, z, r float64) bool {
	for _, zd := range w.Def.Zones {
		if zd.Kind == ZoneDanger && w.Dist(x, z, zd.X, zd.Z) < zd.Radius+r+3 {
			return true
		}
	}
	return false
}
