package game

import (
	"strings"
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

// The v0.5 loop: catch a fish, light a fire, cook it, eat it.

func TestFishingNeedsARodAndKeepsTheSpot(t *testing.T) {
	f := newFixture(t)
	p := f.join("Angler")
	// There are fishing spots on the coast near Manchester.
	x, y, z := f.findBlock(world.FishSpot, nil)
	f.standNear(p, x, y, z)
	if e := f.mustFail(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z}); !strings.Contains(e, "Fishing Rod") {
		t.Fatalf("fishing without a rod: %s", e)
	}
	p.Char.Inv["fishing_rod"] = 1
	node := f.g.C.Nodes[world.FishSpot]
	for i := 0; i < node.Hits; i++ {
		f.advance(time.Second)
		f.mustOK(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z})
	}
	if p.Char.Inv["raw_herring"] < 1 || p.Char.XP["fishing"] == 0 {
		t.Fatalf("no herring caught: %v xp %v", p.Char.Inv, p.Char.XP)
	}
	if f.g.W.Get(x, y, z) != world.FishSpot {
		t.Fatal("a fishing spot shouldn't be used up")
	}
	// Better fish come with levels: a novice only ever catches herring.
	for i := 0; i < 50; i++ {
		if it, _ := f.g.catch(p.Char, node); it != "raw_herring" {
			t.Fatalf("level 1 caught %s", it)
		}
	}
	p.Char.XP["fishing"] = XPForLevel(10)
	seen := map[string]bool{}
	for i := 0; i < 400; i++ {
		it, _ := f.g.catch(p.Char, node)
		seen[it] = true
	}
	if !seen["raw_cod"] || !seen["raw_salmon"] || !seen["raw_trout"] {
		t.Fatalf("level 10 catches: %v", seen)
	}
}

func TestCampfireCooksAndBurnsOut(t *testing.T) {
	f := newFixture(t)
	p := f.join("Kindler")
	f.atWild(p, 0, 0)
	fx, fy, fz := int(p.Char.X)+1, int(p.Char.Y), int(p.Char.Z)
	light := protocol.In{T: "light_fire", BX: fx, BY: fy, BZ: fz, Item: "log"}
	p.Char.Inv["log"] = 3
	if e := f.mustFail(p, light); !strings.Contains(e, "tinderbox") {
		t.Fatalf("fire without a tinderbox: %s", e)
	}
	p.Char.Inv["tinderbox"] = 1
	p.Char.Inv["coal"] = 1
	if e := f.mustFail(p, protocol.In{T: "light_fire", BX: fx, BY: fy, BZ: fz, Item: "coal"}); !strings.Contains(e, "Firemaking level 6") {
		t.Fatalf("coal at level 1: %s", e)
	}
	f.mustOK(p, light)
	if f.g.W.Get(fx, fy, fz) != world.Campfire || p.Char.Inv["log"] != 2 || p.Char.XP["firemaking"] == 0 {
		t.Fatalf("fire not lit: block %d inv %v", f.g.W.Get(fx, fy, fz), p.Char.Inv)
	}
	// It's a cooking station.
	p.Char.Inv["raw_herring"] = 1
	p.Char.XP["cooking"] = XPForLevel(5) // past the burn level, so it never burns
	f.mustOK(p, protocol.In{T: "craft", Recipe: "cook_herring"})
	if p.Char.Inv["herring"] != 1 || p.Char.Inv["raw_herring"] != 0 {
		t.Fatalf("not cooked: %v", p.Char.Inv)
	}
	// Fires are saved with the world.
	g2, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), f.g.Snapshot(), newRec())
	if err != nil || len(g2.fires) != 1 || g2.W.Get(fx, fy, fz) != world.Campfire {
		t.Fatalf("fire not saved: %v %d", err, len(g2.fires))
	}
	// It burns out after a minute (an oak log).
	f.advance(61 * time.Second)
	f.g.tickFires()
	if f.g.W.Get(fx, fy, fz) != world.Air || len(f.g.fires) != 0 {
		t.Fatal("the fire should have burned out")
	}
	p.Char.Inv["raw_herring"] = 1
	if e := f.mustFail(p, protocol.In{T: "craft", Recipe: "cook_herring"}); !strings.Contains(e, "campfire") {
		t.Fatalf("cooking with no fire: %s", e)
	}
	// No fires in town; the kitchen range is there instead.
	f.atNPC(p, "hannah")
	tx, ty, tz := int(p.Char.X)+1, int(p.Char.Y), int(p.Char.Z)
	f.advance(2 * time.Second)
	if e := f.mustFail(p, protocol.In{T: "light_fire", BX: tx, BY: ty, BZ: tz, Item: "log"}); !strings.Contains(e, "range") {
		t.Fatalf("fire in town: %s", e)
	}
	f.mustOK(p, protocol.In{T: "craft", Recipe: "cook_herring"})
}

func TestCookingBurnsLessWithLevel(t *testing.T) {
	r := content.MustBase().Recipes["cook_herring"]
	if c := burnChance(r, 1); c != 0.3 {
		t.Fatalf("burn chance at level 1: %v", c)
	}
	if burnChance(r, 3) >= burnChance(r, 2) || burnChance(r, 5) != 0 || burnChance(r, 9) != 0 {
		t.Fatal("burn chance should fall to zero by the safe level")
	}
	f := newFixture(t)
	p := f.join("Cook")
	f.atNPC(p, "hannah")
	p.Char.Inv["raw_herring"] = 30
	for i := 0; i < 30; i++ {
		f.mustOK(p, protocol.In{T: "craft", Recipe: "cook_herring"})
	}
	burnt, cooked := p.Char.Inv["burnt_fish"], p.Char.Inv["herring"]
	if burnt == 0 || cooked == 0 || burnt+cooked != 30 {
		t.Fatalf("30 herring at level 1 gave %d cooked, %d burnt", cooked, burnt)
	}
}

func TestWispsKeepAwayFromFire(t *testing.T) {
	f := newFixture(t)
	p := f.join("Warded")
	var wisp, boggart *Mob
	for _, m := range f.g.Mobs {
		switch m.Def.ID {
		case "gloamwisp":
			wisp = m
		case "bramblet":
			boggart = m
		}
	}
	f.at(p, wisp.X+2, wisp.Y, wisp.Z)
	f.g.fires = append(f.g.fires, fire{P: f.g.W.PosAt(int(p.Char.X)+1, int(p.Char.Y), int(p.Char.Z)), Until: f.g.Now.Add(time.Minute), Owner: p.Char.ID})
	if !f.g.warded(wisp, p.Char.X, p.Char.Y, p.Char.Z, FireWard) || f.g.warded(boggart, p.Char.X, p.Char.Y, p.Char.Z, FireWard) {
		t.Fatal("only wisps fear fire")
	}
	wisp.target = p.ID
	f.g.tickMobs()
	if wisp.target != 0 {
		t.Fatal("a wisp kept chasing a player by a fire")
	}
	f.g.fires = nil
	f.g.tickMobs()
	if wisp.target != p.ID {
		t.Fatal("without the fire the wisp should come for you")
	}
}

func TestFishwifeQuestHandsOverGear(t *testing.T) {
	f := newFixture(t)
	p := f.join("Helper")
	f.atNPC(p, "hannah")
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "supper_for_the_mill"})
	if p.Char.Inv["fishing_rod"] != 1 || p.Char.Inv["tinderbox"] != 1 {
		t.Fatalf("quest gear: %v", p.Char.Inv)
	}
}

func TestNewTiersGrowNearManchester(t *testing.T) {
	f := newFixture(t)
	for _, b := range []uint8{world.Rowan, world.Yew, world.TinOre, world.Coal, world.IronOre, world.CopperOre} {
		f.findBlock(b, nil)
	}
}
