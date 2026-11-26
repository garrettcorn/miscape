package game

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

// The wide world: danger levels, roaming creatures, Hearth travel.

// placeOut puts p just outside a town, on dry wild land.
func (f *fixture) placeOut(p *Player, town string) {
	f.t.Helper()
	pl := f.g.W.Def.Place(town)
	for r := pl.Radius + 12; r < pl.Radius+60; r += 2 {
		for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, z := f.g.W.WrapX(pl.X+d[0]*r), pl.Z+d[1]*r
			ix, iz := int(x), int(z)
			if f.g.W.ZoneAt(x, z) == world.ZoneWild && !f.g.wet(ix, iz) && f.g.W.GroundY(ix, iz) > 0 {
				f.at(p, x, float64(f.g.W.SurfaceY(ix, iz)), z)
				return
			}
		}
	}
	f.t.Fatalf("no dry land outside %s", town)
}

func TestRoamingCreaturesScaleWithDanger(t *testing.T) {
	f := newFixture(t)
	p := f.join("Wanderer")
	count := func(near *Player) (n int, lv int) {
		for _, m := range f.g.Mobs {
			if m.Wild && f.g.W.Dist(m.X, m.Z, near.Char.X, near.Char.Z) < WildsNear {
				n++
				lv = max(lv, m.Level)
				if f.g.W.ZoneAt(m.X, m.Z) == world.ZoneTown {
					t.Fatalf("a roaming %s appeared in town", m.Def.ID)
				}
			}
		}
		return
	}
	// In town: nothing appears.
	f.atSpawn(p)
	for i := 0; i < 5; i++ {
		f.advance(WildsEvery)
		f.g.tickWilds()
	}
	if n, _ := count(p); n != 0 {
		t.Fatalf("%d roaming creatures around a player in town", n)
	}
	// Near Manchester: weak ones, never more than the cap.
	f.placeOut(p, "manchester")
	for i := 0; i < 20; i++ {
		f.advance(WildsEvery)
		f.g.tickWilds()
	}
	n, lv := count(p)
	if n == 0 || n > WildsPerPlayer || lv > 2 {
		t.Fatalf("near Manchester: %d creatures, highest level %d", n, lv)
	}
	// Far away: tougher ones, with more health.
	q := f.join("Explorer")
	f.placeOut(q, "sydney")
	for i := 0; i < 20; i++ {
		f.advance(WildsEvery)
		f.g.tickWilds()
	}
	n2, lv2 := count(q)
	if n2 == 0 || lv2 < 8 {
		t.Fatalf("near Sydney: %d creatures, highest level %d", n2, lv2)
	}
	for _, m := range f.g.Mobs {
		if m.Wild && m.Level >= 8 && m.MaxHP <= f.g.mobMaxHP(m.Def, 1)*3 {
			t.Fatalf("a level %d %s has only %d health", m.Level, m.Def.ID, m.MaxHP)
		}
	}
	// Left alone, they go away: send both players off to Rio.
	rio, _ := f.g.townSpawn("rio")
	f.at(p, rio.X, rio.Y, rio.Z)
	f.at(q, rio.X, rio.Y, rio.Z)
	for i := 0; i < 12; i++ {
		f.advance(WildsEvery)
		f.g.tickWilds()
	}
	for _, m := range f.g.Mobs {
		if m.Wild {
			t.Fatalf("a roaming %s stayed with nobody near", m.Def.ID)
		}
	}
}

func TestHearthToVisitedTowns(t *testing.T) {
	f := newFixture(t)
	p := f.join("Traveller")
	if !slices.Equal(p.Char.Visited, []string{"manchester"}) {
		t.Fatalf("new characters know Manchester: %v", p.Char.Visited)
	}
	if e := f.mustFail(p, protocol.In{T: "hearth", Place: "boston"}); !strings.Contains(e, "been to") {
		t.Fatalf("hearth to an unvisited town: %s", e)
	}
	// Walk into Boston.
	b := f.g.W.Def.Place("boston")
	f.placeOut(p, "boston")
	f.g.handleMove(p, &protocol.In{T: "move", X: b.X, Y: float64(f.g.W.SurfaceY(int(b.X), int(b.Z))), Z: b.Z})
	if f.g.W.TownAt(p.Char.X, p.Char.Z) != "boston" {
		// The move check may refuse a long step; stand there directly.
		f.at(p, b.X, float64(f.g.W.SurfaceY(int(b.X), int(b.Z))), b.Z)
		f.g.visit(p)
	}
	if !slices.Contains(p.Char.Visited, "boston") {
		t.Fatalf("Boston not remembered: %v", p.Char.Visited)
	}
	// Hearth home from the wilds, then back to Boston.
	f.placeOut(p, "manchester")
	f.mustOK(p, protocol.In{T: "hearth", Place: "boston"})
	f.advance(time.Duration(HearthSeconds*float64(time.Second)) + time.Second)
	f.g.tickPlayers()
	if f.g.W.TownAt(p.Char.X, p.Char.Z) != "boston" {
		t.Fatalf("hearth landed at %.0f,%.0f (%s)", p.Char.X, p.Char.Z, f.g.W.ZoneName(p.Char.X, p.Char.Z))
	}
	// Knocked out near Boston, you wake in Boston.
	f.placeOut(p, "boston")
	f.g.playerDied(p)
	if f.g.W.TownAt(p.Char.X, p.Char.Z) != "boston" {
		t.Fatal("knocked out near Boston should wake in Boston")
	}
	// Visits are saved; older saves start with the spawn town.
	snap := f.g.Snapshot()
	if !slices.Contains(snap.Characters[p.Char.ID].Visited, "boston") {
		t.Fatal("visits not saved")
	}
	snap.Version = 4
	snap.Characters[p.Char.ID].Visited = nil
	g2, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), snap, newRec())
	if err != nil {
		t.Fatal(err)
	}
	if v := g2.Chars[p.Char.ID].Visited; !slices.Equal(v, []string{"manchester"}) {
		t.Fatalf("migrated visits: %v", v)
	}
}

func TestDangerAndHotspotsReachTheClient(t *testing.T) {
	f := newFixture(t)
	p := f.join("Mapper")
	f.placeOut(p, "manchester")
	if me := f.g.meMsg(p); me["danger"] != 1 {
		t.Fatalf("danger near Manchester: %v", me["danger"])
	}
	wv := f.g.worldView()
	sites := wv["sites"].([]M)
	if len(sites) < 60 {
		t.Fatalf("only %d sites sent", len(sites))
	}
	mines := 0
	for _, s := range sites {
		if s["kind"] == world.SiteMine {
			mines++
			if len(s["what"].([]string)) == 0 {
				t.Fatalf("a mine with nothing listed: %v", s)
			}
		}
	}
	if mines == 0 {
		t.Fatal("no mines on the map")
	}
}

// Each town's quests can be done nearby: what they ask you to mine, chop,
// catch or fight is found within reach of the town.
func TestTownQuestsCanBeDoneNearby(t *testing.T) {
	f := newFixture(t)
	near := func(town string, kind string, ok func(s *world.Site) bool) bool {
		p := f.g.W.Def.Place(town)
		for _, s := range f.g.W.Sites() {
			if s.Kind == kind && f.g.W.Dist(s.X, s.Z, p.X, p.Z) < p.Radius+90 && ok(s) {
				return true
			}
		}
		return false
	}
	has := func(b uint8) func(*world.Site) bool {
		return func(s *world.Site) bool { return slices.Contains(s.Blocks, b) }
	}
	tier := func(min int) func(*world.Site) bool {
		return func(s *world.Site) bool { return f.g.W.Tier(s.Danger) >= min }
	}
	any := func(*world.Site) bool { return true }
	for _, c := range []struct {
		town, kind string
		ok         func(*world.Site) bool
	}{
		{"falun", world.SiteMine, has(world.CopperOre)},
		{"potosi", world.SiteMine, has(world.IronOre)},
		{"timbuktu", world.SiteWood, has(world.Rowan)},
		{"bergen", world.SiteFishing, any},
		{"boston", world.SiteFishing, tier(2)},    // trout
		{"hobart", world.SiteFishing, tier(6)},    // cod
		{"quebec", world.SiteHunting, any},        // hides
		{"cape_town", world.SiteHunting, tier(4)}, // deer
	} {
		if !near(c.town, c.kind, c.ok) {
			t.Errorf("%s has no suitable %s nearby for its quest", c.town, c.kind)
		}
	}
	// Quest creatures roam near their towns.
	for town, mob := range map[string]string{"boston": "jersey_devil", "cape_town": "grootslang", "canton": "jiangshi", "sydney": "yowie"} {
		p := f.g.W.Def.Place(town)
		found := false
		for i := 0; i < 600 && !found; i++ {
			a, d := float64(i)*0.7, p.Radius+10+float64(i%30)
			x, z := f.g.W.WrapX(p.X+math.Cos(a)*d), p.Z+math.Sin(a)*d
			if f.g.wet(int(x), int(z)) {
				continue
			}
			if d := f.g.pickWild(x, z, f.g.W.Tier(max(1, f.g.W.Danger(x, z)))); d != nil && d.ID == mob {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s roams near %s", mob, town)
		}
	}
	// Villages have soil for farming quests.
	if _, err := f.g.W.Point("town:calcutta:allotment"); err != nil {
		t.Fatal(err)
	}
}
