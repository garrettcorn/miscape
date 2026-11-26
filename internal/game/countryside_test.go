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

// v0.7: hunting, farming and agility.

func TestAnimalsFleeAndTrainHunting(t *testing.T) {
	f := newFixture(t)
	p := f.join("Hunter")
	var hare, deer *Mob
	for _, m := range f.g.Mobs {
		switch m.Def.ID {
		case "hare":
			hare = m
		case "deer":
			deer = m
		}
	}
	if hare == nil || deer == nil {
		t.Fatal("no hares or deer spawned")
	}
	// Every animal spawns on dry land in the countryside.
	for _, m := range f.g.Mobs {
		if m.Def.Flees && (f.g.W.ZoneAt(m.X, m.Z) != world.ZoneWild || f.g.wet(int(m.X), int(m.Z))) {
			t.Fatalf("%s spawned in zone %v or water at %.0f,%.0f", m.Def.ID, f.g.W.ZoneAt(m.X, m.Z), m.X, m.Z)
		}
	}
	// A hare runs away and never bites.
	f.at(p, hare.X+2, hare.Y, hare.Z)
	hp := p.Char.HP
	before := f.g.W.Dist(hare.X, hare.Z, p.Char.X, p.Char.Z)
	for i := 0; i < 10; i++ {
		f.g.tickMobs()
	}
	if after := f.g.W.Dist(hare.X, hare.Z, p.Char.X, p.Char.Z); after <= before {
		t.Fatalf("hare didn't flee: %.1f -> %.1f", before, after)
	}
	if p.Char.HP != hp {
		t.Fatal("hares don't bite")
	}
	// Killing it trains Hunting, not Melee, and its meat counts as gathered.
	f.g.killMob(hare, p)
	if p.Char.XP["hunting"] == 0 || p.Char.XP["melee"] != 0 || p.Char.Inv["raw_hare"] != 1 {
		t.Fatalf("hare kill: xp %v inv %v", p.Char.XP, p.Char.Inv)
	}
	// Deer need Hunting 6.
	f.at(p, deer.X+1, deer.Y, deer.Z)
	if e := f.mustFail(p, protocol.In{T: "attack", Target: deer.ID}); !strings.Contains(e, "Hunting 6") {
		t.Fatalf("deer at hunting 1: %s", e)
	}
}

func TestSnaresCatchWhileYouWait(t *testing.T) {
	f := newFixture(t)
	p := f.join("Snarer")
	gx, gy, gz := f.findBlock(world.Grass, func(x, y, z int) bool {
		return f.wild(x, y, z) && f.g.W.Get(x, y+1, z) == world.Air && f.g.W.Get(x, y+2, z) == world.Air
	})
	sx, sy, sz := gx, gy+1, gz
	f.standNear(p, gx, gy+1, gz)
	set := protocol.In{T: "set_trap", BX: sx, BY: sy, BZ: sz, Item: "snare"}
	p.Char.Inv["snare"] = 1
	f.mustOK(p, set)
	if f.g.W.Get(sx, sy, sz) != world.Snare || p.Char.Inv["snare"] != 0 {
		t.Fatal("snare not set")
	}
	// Someone else can't empty it.
	o := f.join("Thief")
	f.at(o, p.Char.X, p.Char.Y, p.Char.Z)
	if e := f.mustFail(o, protocol.In{T: "gather", BX: sx, BY: sy, BZ: sz}); !strings.Contains(e, "someone else's") {
		t.Fatalf("emptying another's snare: %s", e)
	}
	// It's saved with the world.
	g2, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), f.g.Snapshot(), newRec())
	if err != nil || len(g2.traps) != 1 {
		t.Fatalf("snare not saved: %v", err)
	}
	f.advance(46 * time.Second)
	f.g.tickPlots()
	if f.g.W.Get(sx, sy, sz) != world.SnareFull {
		t.Fatal("snare should show a catch")
	}
	f.advance(time.Second)
	f.mustOK(p, protocol.In{T: "gather", BX: sx, BY: sy, BZ: sz})
	if p.Char.Inv["raw_hare"] != 1 || p.Char.Inv["hide"] != 1 || p.Char.Inv["snare"] != 1 || p.Char.XP["hunting"] == 0 {
		t.Fatalf("catch: %v xp %v", p.Char.Inv, p.Char.XP)
	}
	if f.g.W.Get(sx, sy, sz) != world.Air {
		t.Fatal("snare should be gone")
	}
	// Not in town.
	f.atSpawn(p)
	f.advance(time.Second)
	if e := f.mustFail(p, protocol.In{T: "set_trap", BX: int(p.Char.X) + 1, BY: int(p.Char.Y), BZ: int(p.Char.Z), Item: "snare"}); !strings.Contains(e, "countryside") {
		t.Fatalf("snare in town: %s", e)
	}
}

func TestCropsGrowEvenWhileOffline(t *testing.T) {
	f := newFixture(t)
	p := f.join("Grower")
	sx, sy, sz := f.findBlock(world.Soil, nil)
	f.standNear(p, sx, sy, sz)
	plant := protocol.In{T: "plant", BX: sx, BY: sy + 1, BZ: sz, Item: "seed_potato"}
	p.Char.Inv["seed_potato"] = 2
	f.mustOK(p, plant)
	if f.g.W.Get(sx, sy+1, sz) != world.Sprout {
		t.Fatal("no sprout")
	}
	if e := f.mustFail(p, protocol.In{T: "gather", BX: sx, BY: sy + 1, BZ: sz}); !strings.Contains(e, "Not ready") {
		t.Fatalf("harvesting a sprout: %s", e)
	}
	// Cabbages need Farming 5.
	p.Char.Inv["cabbage_seed"] = 1
	f.advance(time.Second)
	if e := f.mustFail(p, protocol.In{T: "plant", BX: sx, BY: sy + 1, BZ: sz, Item: "cabbage_seed"}); !strings.Contains(e, "Farming level 5") {
		t.Fatalf("cabbages at farming 1: %s", e)
	}
	// Save, "go offline" for two minutes, load: it's ripe.
	snap := f.g.Snapshot()
	g2, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), snap, newRec())
	if err != nil {
		t.Fatal(err)
	}
	g2.Now = f.g.Now.Add(2 * time.Minute)
	g2.tickPlots()
	if g2.W.Get(sx, sy+1, sz) != world.Potatoes {
		t.Fatal("crop should ripen across a save and reload")
	}
	// Harvest in the original game.
	f.advance(2 * time.Minute)
	f.g.tickPlots()
	f.mustOK(p, protocol.In{T: "gather", BX: sx, BY: sy + 1, BZ: sz})
	if n := p.Char.Inv["potato"]; n < 2 || n > 4 || p.Char.XP["farming"] == 0 {
		t.Fatalf("harvest: %v", p.Char.Inv)
	}
	if f.g.W.Get(sx, sy+1, sz) != world.Air {
		t.Fatal("harvested plot should be clear")
	}
	// Seeds only grow in soil.
	f.atWild(p, 0, 0)
	f.advance(time.Second)
	if e := f.mustFail(p, protocol.In{T: "plant", BX: int(p.Char.X) + 1, BY: int(p.Char.Y), BZ: int(p.Char.Z), Item: "seed_potato"}); !strings.Contains(e, "soil") {
		t.Fatalf("planting on grass: %s", e)
	}
}

func TestAgilityCourseLapAndSprintBonus(t *testing.T) {
	f := newFixture(t)
	p := f.join("Runner")
	co := f.g.course("irk_valley")
	// Every flag of every course is on dry land in the countryside.
	for _, c := range f.g.C.Courses {
		for i, pt := range c.Points {
			x, z := int(pt[0]), int(pt[1])
			if f.g.W.ZoneAt(pt[0], pt[1]) != world.ZoneWild || f.g.wet(x, z) {
				t.Fatalf("%s flag %d at %.0f,%.0f is in zone %v or water", c.ID, i, pt[0]-1517, pt[1]-312, f.g.W.ZoneAt(pt[0], pt[1]))
			}
		}
	}
	// Visit the flags in order (teleporting stands in for running).
	visit := func(i int) {
		pt := co.Points[i]
		y := float64(f.g.W.SurfaceY(int(pt[0]), int(pt[1])))
		f.at(p, pt[0], y, pt[1])
		f.advance(time.Second)
		f.g.checkCourse(p)
	}
	visit(0)
	if p.run == nil {
		t.Fatal("standing on the start flag should start the course")
	}
	visit(2) // out of order: doesn't count
	if p.run.next != 1 {
		t.Fatalf("skipped a flag: next %d", p.run.next)
	}
	for i := 1; i < len(co.Points); i++ {
		visit(i)
	}
	if p.run != nil || p.Char.XP["agility"] != co.XP {
		t.Fatalf("lap not counted: run %v xp %d", p.run, p.Char.XP["agility"])
	}
	// Too slow fails the lap.
	f.advance(5 * time.Second)
	visit(0)
	f.advance(time.Duration(co.LimitSec+1) * time.Second)
	visit(1)
	if p.run != nil || p.Char.XP["agility"] != co.XP {
		t.Fatal("a slow lap should fail")
	}
	// Agility adds 1% sprint speed per level.
	p.Char.XP["agility"] = XPForLevel(10)
	if s := sprintSpeed(p.Char); s < SprintSpeed*1.099 || s > SprintSpeed*1.101 {
		t.Fatalf("sprint at agility 10: %.3f", s)
	}
	// The harder course needs Agility 5 (fine now) and names itself.
	if u := f.g.unlocksAt("agility", 5); !strings.Contains(strings.Join(u, ","), "Pennine Fell Race") {
		t.Fatalf("agility 5 unlocks: %v", u)
	}
}

func TestCountrysideQuestsHandOverGear(t *testing.T) {
	f := newFixture(t)
	p := f.join("Helper")
	f.atNPC(p, "abel")
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "squires_hares"})
	f.atNPC(p, "jonas")
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "poorhouse_potatoes"})
	if p.Char.Inv["snare"] != 3 || p.Char.Inv["seed_potato"] != 4 {
		t.Fatalf("quest gear: %v", p.Char.Inv)
	}
}
