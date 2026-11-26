package world

import (
	"testing"

	"lanternfall/maps"
)

func level(t testing.TB, name string) *World {
	t.Helper()
	d, err := LoadDef(maps.FS, "earth", name)
	if err != nil {
		t.Fatal(err)
	}
	return New(d)
}

func TestDangerRisesWithDistance(t *testing.T) {
	w := level(t, "level1")
	at := func(id string) (float64, float64) { p := w.Def.Place(id); return p.X, p.Z }
	mx, mz := at("manchester")
	if d := w.Danger(mx, mz); d != 0 {
		t.Fatalf("Manchester itself should be safe, danger %d", d)
	}
	if d := w.Danger(mx+25, mz); d != 1 {
		t.Fatalf("just outside Manchester: danger %d, want 1", d)
	}
	moors := w.Def.Zones[0]
	if d := w.Danger(moors.X, moors.Z); d != 3 {
		t.Fatalf("the Highland Moors: danger %d, want 3", d)
	}
	sx, sz := at("sydney")
	if d := w.Danger(sx+30, sz); d < 8 {
		t.Fatalf("outside Sydney: danger %d, want 8+", d)
	}
	// It rises the further you go.
	prev := 0
	for _, id := range []string{"bergen", "boston", "rio", "sydney"} {
		x, z := at(id)
		d := w.distanceDanger(x, z)
		if d < prev {
			t.Fatalf("%s is danger %d, less than a nearer town (%d)", id, d, prev)
		}
		prev = d
	}
	if lvl2 := level(t, "level2"); lvl2.MaxDanger() != 20 {
		t.Fatal("a level 2 world goes to danger 20")
	}
}

func TestEveryTownStandsOnDryLand(t *testing.T) {
	w := level(t, "level1")
	for _, p := range w.Def.Places {
		if p.Structure == "" {
			continue
		}
		if w.ZoneAt(p.X, p.Z) != ZoneTown || w.ZoneName(p.X, p.Z) != p.Name {
			t.Errorf("%s isn't a town zone", p.ID)
		}
		sp, err := w.Point("town:" + p.ID + ":spawn")
		if err != nil {
			t.Fatal(err)
		}
		if b := w.Get(int(sp.X), int(sp.Y)-1, int(sp.Z)); !Solid(b) {
			t.Errorf("%s: nothing to stand on at its spawn point (block %d)", p.ID, b)
		}
		if b := w.Get(int(sp.X), int(sp.Y), int(sp.Z)); Solid(b) {
			t.Errorf("%s: its spawn point is inside a block (%d)", p.ID, b)
		}
	}
}

func TestHotspots(t *testing.T) {
	w := level(t, "level1")
	sites := w.Sites()
	if len(sites) < 60 {
		t.Fatalf("only %d hotspots", len(sites))
	}
	// The same seed gives the same sites (the browser engine and the maps agree).
	again := level(t, "level1").Sites()
	if len(again) != len(sites) || again[10].ID != sites[10].ID || again[10].X != sites[10].X {
		t.Fatal("sites aren't deterministic")
	}
	high := map[uint8]bool{IronOre: true, Glimmer: true}
	for _, s := range sites {
		if w.ZoneAt(s.X, s.Z) == ZoneTown {
			t.Errorf("site %s is inside a town", s.ID)
		}
		tier := w.Tier(s.Danger)
		switch s.Kind {
		case SiteMine:
			if len(s.Blocks) < 1 || len(s.Blocks) > 3 {
				t.Errorf("mine %s has %d ores", s.ID, len(s.Blocks))
			}
			for _, b := range s.Blocks {
				ok := false
				for _, bb := range BandsAt(OreBands, tier) {
					ok = ok || bb == b
				}
				if !ok || (s.Danger <= 2 && high[b]) {
					t.Errorf("mine %s at danger %d has ore %d", s.ID, s.Danger, b)
				}
			}
		case SiteWood:
			if len(s.Blocks) < 1 || len(s.Blocks) > 2 {
				t.Errorf("wood %s has %d trees", s.ID, len(s.Blocks))
			}
		}
	}
	// Every town has hotspots for what it's known for, close by.
	kinds := map[string]bool{}
	for _, p := range w.Def.Places {
		for _, sk := range p.Known {
			for _, k := range skillSites[sk] {
				found := false
				for _, s := range sites {
					found = found || (s.Kind == k && w.Dist(s.X, s.Z, p.X, p.Z) < p.Radius+90)
				}
				if !found {
					t.Errorf("%s is known for %s but has no %s nearby", p.ID, sk, k)
				}
				kinds[k] = true
			}
		}
	}
	// A mine really has ore in it.
	for _, s := range sites {
		if s.Kind != SiteMine || len(s.Blocks) == 0 {
			continue
		}
		n := 0
		for dz := -int(s.R); dz <= int(s.R); dz++ {
			for dx := -int(s.R); dx <= int(s.R); dx++ {
				x, z := wrapI(int(s.X)+dx, w.W), int(s.Z)+dz
				for y := w.Def.GroundY; y < w.Def.GroundY+5; y++ {
					for _, b := range s.Blocks {
						if w.Get(x, y, z) == b {
							n++
						}
					}
				}
			}
		}
		if n < 15 {
			t.Errorf("mine %s has only %d ore blocks", s.ID, n)
		}
		break
	}
}
