package game

import (
	"math"
	"strings"
	"testing"
	"time"

	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

func TestXPCurve(t *testing.T) {
	if LevelFor(0, 10) != 1 || LevelFor(49, 10) != 1 || LevelFor(50, 10) != 2 || LevelFor(150, 10) != 3 {
		t.Fatalf("curve wrong: %d %d %d %d", LevelFor(0, 10), LevelFor(49, 10), LevelFor(50, 10), LevelFor(150, 10))
	}
	if LevelFor(1_000_000, 10) != 10 || LevelFor(1_000_000, 20) != 20 {
		t.Fatal("level must stop at the cap")
	}
}

func TestGatherTreeYieldsLogsAndRegrows(t *testing.T) {
	f := newFixture(t)
	p := f.join("Ada")
	x, y, z := f.findBlock(world.Log, f.wild)
	f.standNear(p, x, y, z)
	node := f.g.C.Nodes[world.Log]
	for i := 0; i < node.Hits; i++ {
		f.mustOK(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z})
		f.advance(GatherCooldown)
	}
	if p.Char.Inv["log"] != 1 {
		t.Fatalf("want 1 log, got %d", p.Char.Inv["log"])
	}
	if p.Char.XP["woodcutting"] != node.XP {
		t.Fatalf("want %d xp, got %d", node.XP, p.Char.XP["woodcutting"])
	}
	if f.g.W.Get(x, y, z) != world.Air {
		t.Fatal("block should be removed")
	}
	f.mustFail(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z})
	// Regrows after its timer (move the player out of the way first).
	f.atSpawn(p)
	f.g.Tick(f.g.Now.Add(time.Duration(node.RegrowSec+1) * time.Second))
	if f.g.W.Get(x, y, z) != world.Log {
		t.Fatal("tree should regrow")
	}
}

func TestGatherCooldownRangeAndLevel(t *testing.T) {
	f := newFixture(t)
	p := f.join("Bob")
	x, y, z := f.findBlock(world.Log, f.wild)
	f.standNear(p, x, y, z)
	f.mustOK(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z})
	if e := f.mustFail(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z}); !strings.Contains(e, "Steady") {
		t.Fatalf("cooldown not enforced: %s", e)
	}
	f.at(p, float64(x)+20, float64(y), float64(z))
	f.advance(time.Second)
	if e := f.mustFail(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z}); !strings.Contains(e, "far") {
		t.Fatalf("range not enforced: %s", e)
	}
	// Copper needs Mining 2.
	wx, wy, wz := f.wildSpot(0, 0)
	cx, cy, cz := int(wx)+1, int(wy), int(wz)
	f.g.W.Set(cx, cy, cz, world.CopperOre)
	f.at(p, wx-0.5, wy, wz)
	if e := f.mustFail(p, protocol.In{T: "gather", BX: cx, BY: cy, BZ: cz}); !strings.Contains(e, "Mining level 2") {
		t.Fatalf("level not enforced: %s", e)
	}
	// Non-node blocks can't be gathered.
	if e := f.mustFail(p, protocol.In{T: "gather", BX: cx, BY: 0, BZ: cz}); e == "" {
		t.Fatal("bedrock gathered")
	}
}

func TestVillageIsProtected(t *testing.T) {
	f := newFixture(t)
	p := f.join("Cyd")
	// House corner logs are in the village.
	x, y, z := f.findBlock(world.Log, func(x, y, z int) bool {
		return f.g.W.ZoneAt(float64(x)+0.5, float64(z)+0.5) == world.ZoneTown
	})
	f.standNear(p, x, y, z)
	if e := f.mustFail(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z}); !strings.Contains(e, "town") {
		t.Fatalf("town gather allowed: %s", e)
	}
	p.Char.Inv["plank"] = 5
	f.atSpawn(p)
	sp := f.g.W.Spawn()
	if e := f.mustFail(p, protocol.In{T: "place", BX: int(sp.X) + 2, BY: 13, BZ: int(sp.Z), Item: "plank"}); !strings.Contains(e, "No building inside Manchester") {
		t.Fatalf("town build allowed: %s", e)
	}
}

func TestCraftingRequirements(t *testing.T) {
	f := newFixture(t)
	p := f.join("Dee")
	f.mustFail(p, protocol.In{T: "craft", Recipe: "planks"}) // no logs
	p.Char.Inv["log"] = 1
	f.mustOK(p, protocol.In{T: "craft", Recipe: "planks"})
	if p.Char.Inv["plank"] != 4 || p.Char.Inv["log"] != 0 {
		t.Fatalf("planks: %v", p.Char.Inv)
	}
	f.mustOK(p, protocol.In{T: "craft", Recipe: "wooden_sword"})
	if p.Char.Inv["wooden_sword"] != 1 || p.Char.Equipped != "wooden_sword" {
		t.Fatalf("sword not crafted/auto-equipped: %v eq=%s", p.Char.Inv, p.Char.Equipped)
	}
	// Copper sword needs Crafting 3 and the anvil.
	p.Char.Inv["copper_ore"], p.Char.Inv["plank"] = 3, 2
	if e := f.mustFail(p, protocol.In{T: "craft", Recipe: "copper_sword"}); !strings.Contains(e, "Crafting level 3") {
		t.Fatal(e)
	}
	p.Char.XP["crafting"] = XPForLevel(3)
	f.atWild(p, 0, 0)
	if e := f.mustFail(p, protocol.In{T: "craft", Recipe: "copper_sword"}); !strings.Contains(e, "anvil") {
		t.Fatal(e)
	}
	a, _ := f.g.W.Point("anvil")
	f.at(p, a.X+1.5, a.Y, a.Z+0.5)
	f.mustOK(p, protocol.In{T: "craft", Recipe: "copper_sword"})
	if p.Char.Inv["copper_sword"] != 1 || p.Char.Inv["copper_ore"] != 0 {
		t.Fatalf("copper sword: %v", p.Char.Inv)
	}
}

func TestInventoryCapacity(t *testing.T) {
	f := newFixture(t)
	p := f.join("Eve")
	for i := 0; i < InventorySlots; i++ {
		p.Char.Inv[string(rune('a'+i))+"x"] = 1 // unknown items still take space
	}
	p.Char.Inv["log"] = 1
	f.mustFail(p, protocol.In{T: "craft", Recipe: "planks"}) // log->4 planks needs a new slot
	if p.Char.Inv["log"] != 1 {
		t.Fatal("failed craft must not consume inputs")
	}
}

func TestCombatKillsGiveXPLootAndQuestProgress(t *testing.T) {
	f := newFixture(t)
	p := f.join("Fin")
	var m *Mob
	for _, mm := range f.g.Mobs {
		if mm.Def.ID == "bramblet" {
			m = mm
			break
		}
	}
	f.atSpawn(p)
	f.g.Talk(p, "mabel")
	f.atNPC(p, "mabel")
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "lights_for_wickhollow"})
	f.at(p, m.X+1, m.Y, m.Z)
	hits := 0
	for m.HP > 0 && hits < 50 {
		f.mustOK(p, protocol.In{T: "attack", Target: m.ID})
		f.mustFail(p, protocol.In{T: "attack", Target: m.ID}) // cooldown
		f.advance(time.Second)
		hits++
	}
	if m.HP > 0 {
		t.Fatal("mob never died")
	}
	if p.Char.XP["melee"] != m.Def.XP {
		t.Fatalf("melee xp %d", p.Char.XP["melee"])
	}
	if p.Char.Quests["lights_for_wickhollow"].Progress[2] != 1 {
		t.Fatalf("kill progress not counted: %v", p.Char.Quests["lights_for_wickhollow"].Progress)
	}
	f.mustFail(p, protocol.In{T: "attack", Target: m.ID})
	// Respawns.
	f.g.Tick(f.g.Now.Add(MobRespawn + time.Second))
	if m.HP != m.MaxHP {
		t.Fatal("mob should respawn")
	}
}

func TestMobsDamagePlayersAndDeathRespawns(t *testing.T) {
	f := newFixture(t)
	p := f.join("Gus")
	var m *Mob
	for _, mm := range f.g.Mobs {
		if mm.Def.ID == "gloamwisp" {
			m = mm
			break
		}
	}
	p.Char.Inv["coin"] = 100
	f.at(p, m.X+0.5, m.Y, m.Z)
	start := p.Char.HP
	for i := 0; i < 400 && p.Char.HP == start; i++ {
		f.g.Tick(f.g.Now.Add(50 * time.Millisecond))
	}
	if p.Char.HP >= start {
		t.Fatal("wisp never attacked")
	}
	// Knocked out in the danger zone: back in town, minus 10% of coins.
	f.at(p, m.X+0.5, m.Y, m.Z)
	f.g.damagePlayer(p, 999, m.ID)
	if p.Char.HP != p.Char.MaxHP() || f.g.W.ZoneAt(p.Char.X, p.Char.Z) != world.ZoneTown {
		t.Fatal("should wake up in town at full HP")
	}
	if p.Char.Inv["coin"] != 90 {
		t.Fatalf("coin loss: %d", p.Char.Inv["coin"])
	}
}

func TestPvPRules(t *testing.T) {
	f := newFixture(t)
	a, b := f.join("Hal"), f.join("Ivy")
	f.atWild(a, 0, 0)
	f.atWild(b, 1, 0)
	if e := f.mustFail(a, protocol.In{T: "attack", Target: b.ID}); !strings.Contains(e, "PvP is off") {
		t.Fatal(e)
	}
	f.g.Cfg.PvP = true
	f.mustOK(a, protocol.In{T: "attack", Target: b.ID})
	if b.Char.HP >= b.Char.MaxHP() {
		t.Fatal("pvp damage not applied")
	}
	f.advance(time.Second)
	f.atSpawn(a)
	f.atSpawn(b)
	b.Char.X += 1
	if e := f.mustFail(a, protocol.In{T: "attack", Target: b.ID}); !strings.Contains(e, "safe") {
		t.Fatal(e)
	}
}

func TestQuestFlow(t *testing.T) {
	f := newFixture(t)
	p := f.join("Joy")
	f.atNPC(p, "mabel")
	f.mustFail(p, protocol.In{T: "quest_accept", Quest: "what_glimmers_below"}) // wrong giver range / locked
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "lights_for_wickhollow"})
	f.mustFail(p, protocol.In{T: "quest_accept", Quest: "lights_for_wickhollow"})
	f.mustFail(p, protocol.In{T: "quest_turnin", Quest: "lights_for_wickhollow"})
	f.g.progress(p, "gather", "log", 10)
	f.g.progress(p, "craft", "wooden_sword", 1)
	f.g.progress(p, "kill", "bramblet", 3)
	f.mustOK(p, protocol.In{T: "quest_turnin", Quest: "lights_for_wickhollow"})
	if p.Char.Quests["lights_for_wickhollow"].Status != "done" || p.Char.Inv["coin"] != 25 || p.Char.Inv["copper_ore"] != 2 {
		t.Fatalf("rewards: %v %v", p.Char.Quests["lights_for_wickhollow"], p.Char.Inv)
	}
	f.mustFail(p, protocol.In{T: "quest_turnin", Quest: "lights_for_wickhollow"}) // no double reward
	// The first quest paid 2 adventure points; Ott wants 4.
	if p.Char.AP != 2 {
		t.Fatalf("adventure points after the first quest: %d", p.Char.AP)
	}
	// Second quest: deliver shards to Ott, once you have 4 adventure points.
	f.atNPC(p, "ott")
	if e := f.mustFail(p, protocol.In{T: "quest_accept", Quest: "what_glimmers_below"}); !strings.Contains(e, "4 adventure points") {
		t.Fatalf("Ott with 2 points: %s", e)
	}
	p.Char.AP = 4
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "what_glimmers_below"})
	p.Char.Inv["glimmer_shard"] = 2
	f.mustFail(p, protocol.In{T: "quest_turnin", Quest: "what_glimmers_below"})
	p.Char.Inv["glimmer_shard"] = 3
	f.mustOK(p, protocol.In{T: "quest_turnin", Quest: "what_glimmers_below"})
	if p.Char.Inv["glimmer_shard"] != 0 || p.Char.Inv["warden_token"] != 1 {
		t.Fatalf("deliver: %v", p.Char.Inv)
	}
}

func TestMovementValidation(t *testing.T) {
	f := newFixture(t)
	p := f.join("Kit")
	f.atSpawn(p)
	p.lastMove = f.g.Now
	move := func(x, y, z float64) bool {
		f.advance(50 * time.Millisecond)
		f.g.Handle(p.ID, &protocol.In{T: "move", X: x, Y: y, Z: z})
		return p.Char.X == x && p.Char.Y == y && p.Char.Z == z
	}
	sp := f.g.W.Spawn()
	if !move(sp.X+0.3, sp.Y, sp.Z) {
		t.Fatal("normal step rejected")
	}
	if move(sp.X+5, sp.Y, sp.Z) {
		t.Fatal("teleport accepted")
	}
	if f.out.last(p.ID, "correct") == nil {
		t.Fatal("expected correction")
	}
	if move(sp.X+0.3, sp.Y+3, sp.Z) {
		t.Fatal("super jump accepted")
	}
	// Into the plaza well (solid cobble rim at y=13), from right beside it.
	wx, wz := f.town(-1, 0)
	f.at(p, wx-1.2, 13, wz)
	if move(wx, 13.0, wz) {
		t.Fatal("moved into a solid block")
	}
	// Long gap then a big move through a wall must still be blocked.
	x, y, z := f.wildSpot(0, 0)
	for dz := -3; dz <= 3; dz++ {
		for yy := int(y); yy < int(y)+4; yy++ {
			f.g.W.Set(int(x)+2, yy, int(z)+dz, world.Stone)
		}
	}
	f.at(p, x, y, z)
	f.advance(time.Second)
	f.g.Handle(p.ID, &protocol.In{T: "move", X: x + 4, Y: y, Z: z})
	if p.Char.X != x {
		t.Fatal("tunneled through a wall")
	}
}

// The world wraps east–west: walking over the date line is a short step, but
// jumping to the far side of the world is still a teleport.
func TestMoveAcrossDateLine(t *testing.T) {
	f := newFixture(t)
	p := f.join("Magellan")
	w := f.g.W
	// Find dry, open ground on both sides of the line (Chukotka, Antarctica…).
	stand := w.Def.GroundY + 1
	z := -1
	for zz := 4; zz < w.H-4 && z < 0; zz++ {
		ok := true
		for _, x := range []int{w.W - 2, w.W - 1, 0, 1} {
			ok = ok && !f.g.wet(x, zz) && w.SurfaceY(x, zz) == stand && w.GroundY(x, zz) == stand
		}
		if ok {
			z = zz
		}
	}
	if z < 0 {
		t.Fatal("no dry ground across the date line")
	}
	fz := float64(z) + 0.5
	f.at(p, float64(w.W)-0.2, float64(stand), fz)
	p.lastMove = f.g.Now
	f.advance(50 * time.Millisecond)
	f.g.Handle(p.ID, &protocol.In{T: "move", X: 0.1, Y: float64(stand), Z: fz})
	if p.Char.X != 0.1 {
		t.Fatalf("step across the date line rejected: %v", f.out.last(p.ID, "correct"))
	}
	// A client that sends x just past the edge is wrapped, not rejected.
	f.advance(50 * time.Millisecond)
	f.g.Handle(p.ID, &protocol.In{T: "move", X: -0.15, Y: float64(stand), Z: fz})
	if p.Char.X != float64(w.W)-0.15 {
		t.Fatalf("x not wrapped: %v", p.Char.X)
	}
	f.advance(50 * time.Millisecond)
	f.g.Handle(p.ID, &protocol.In{T: "move", X: float64(w.W) / 2, Y: float64(stand), Z: fz})
	if p.Char.X == float64(w.W)/2 {
		t.Fatal("teleport to the far side accepted")
	}
}

func TestPlaceAndBreakOwnership(t *testing.T) {
	f := newFixture(t)
	a, b := f.join("Lua"), f.join("Moe")
	a.Char.Inv["plank"] = 2
	// Creatures spawn at random spots and could stand in the target cell.
	clear(f.g.Mobs)
	wx, wy, wz := f.wildSpot(0, 0)
	x, y, z := int(wx)-1, int(wy), int(wz)
	f.at(a, wx+1, wy, wz)
	f.at(b, wx+1, wy, wz+1)
	f.mustOK(a, protocol.In{T: "place", BX: x, BY: y, BZ: z, Item: "plank"})
	if f.g.W.Get(x, y, z) != world.Plank || a.Char.Inv["plank"] != 1 {
		t.Fatal("place failed")
	}
	f.mustFail(a, protocol.In{T: "place", BX: x, BY: y, BZ: z, Item: "plank"}) // occupied
	if e := f.mustFail(b, protocol.In{T: "break", BX: x, BY: y, BZ: z}); !strings.Contains(e, "Someone else") {
		t.Fatal(e)
	}
	f.mustOK(a, protocol.In{T: "break", BX: x, BY: y, BZ: z})
	if a.Char.Inv["plank"] != 2 || f.g.W.Get(x, y, z) != world.Air {
		t.Fatal("break should return the plank")
	}
	// Natural blocks can't be "broken" (only gathered).
	f.mustFail(a, protocol.In{T: "break", BX: x, BY: y - 1, BZ: z})
}

func TestDuplicateRequestIsNotReapplied(t *testing.T) {
	f := newFixture(t)
	p := f.join("Nia")
	p.Char.Inv["log"] = 2
	m := protocol.In{T: "craft", Recipe: "planks", Rid: 777}
	f.g.Handle(p.ID, &m)
	f.g.Handle(p.ID, &m) // retry with the same rid
	if p.Char.Inv["plank"] != 4 || p.Char.Inv["log"] != 1 {
		t.Fatalf("duplicate rid re-applied: %v", p.Char.Inv)
	}
	// Survives reconnect: same character, new session.
	tokChar := p.Char
	f.g.Leave(p.ID)
	pid, _, _ := f.joinWithChar(tokChar)
	f.g.Handle(pid, &m)
	if tokChar.Inv["plank"] != 4 {
		t.Fatal("duplicate rid re-applied after reconnect")
	}
}

func (f *fixture) joinWithChar(c *Character) (uint32, string, error) {
	tok := NewToken()
	delete(f.g.tokens, c.TokenHash)
	c.TokenHash = HashToken(tok)
	f.g.tokens[c.TokenHash] = c.ID
	return f.g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Token: tok})
}

func TestHearthChannelAndInterrupt(t *testing.T) {
	f := newFixture(t)
	p := f.join("Ozzy")
	clear(f.g.Mobs) // creatures in the danger zone would interrupt the channel
	dx, dy, dz := f.danger()
	f.at(p, dx, dy, dz)
	f.mustOK(p, protocol.In{T: "hearth"})
	f.g.Tick(f.g.Now.Add(2 * time.Second))
	f.g.damagePlayer(p, 1, 0)
	f.g.Tick(f.g.Now.Add(4 * time.Second))
	if f.g.W.ZoneAt(p.Char.X, p.Char.Z) == world.ZoneTown {
		t.Fatal("damage should interrupt hearth")
	}
	f.mustOK(p, protocol.In{T: "hearth"})
	f.g.Tick(f.g.Now.Add(time.Duration(HearthSeconds*float64(time.Second)) + time.Millisecond))
	if f.g.W.ZoneAt(p.Char.X, p.Char.Z) != world.ZoneTown {
		t.Fatal("hearth should return home")
	}
}

func TestShop(t *testing.T) {
	f := newFixture(t)
	p := f.join("Pip")
	f.atWild(p, 0, 0)
	p.Char.Inv["coin"] = 10
	f.mustFail(p, protocol.In{T: "buy", Item: "berry", Count: 1}) // not near Bertram
	f.atNPC(p, "bertram")
	f.mustOK(p, protocol.In{T: "buy", Item: "berry", Count: 3})
	if p.Char.Inv["coin"] != 4 || p.Char.Inv["berry"] != 3 {
		t.Fatal(p.Char.Inv)
	}
	f.mustFail(p, protocol.In{T: "buy", Item: "lantern", Count: 1}) // too expensive
	p.Char.Inv["log"] = 5
	f.mustOK(p, protocol.In{T: "sell", Item: "log", Count: 5})
	if p.Char.Inv["coin"] != 9 {
		t.Fatal(p.Char.Inv)
	}
	f.mustFail(p, protocol.In{T: "sell", Item: "warden_token", Count: 1})
}

func TestStepUpIsAccepted(t *testing.T) {
	f := newFixture(t)
	p := f.join("Stepper")
	// Build a one-block step in an open wild spot: ground at 13 with a block at x+1.
	wx, _, wz := f.wildSpot(-1, 0)
	x, z := int(wx), int(wz)
	y := f.g.W.SurfaceY(x, z)
	f.g.W.Set(x+1, y, z, world.Stone)
	for yy := y + 1; yy < y+4; yy++ {
		f.g.W.Set(x, yy, z, world.Air)
		f.g.W.Set(x+1, yy, z, world.Air)
	}
	f.at(p, float64(x)+0.6, float64(y), float64(z)+0.5)
	p.lastMove = f.g.Now
	f.advance(60 * time.Millisecond)
	// Client auto-step: rises one block while moving onto the step.
	f.g.Handle(p.ID, &protocol.In{T: "move", X: float64(x) + 1.0, Y: float64(y) + 1.01, Z: float64(z) + 0.5})
	if p.Char.Y != float64(y)+1.01 {
		t.Fatalf("legit step-up rejected: %v", f.out.last(p.ID, "correct"))
	}
}

func TestMobsNeverStandOnCanopies(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 200; i++ {
		f.g.Tick(f.g.Now.Add(50 * time.Millisecond))
	}
	for _, m := range f.g.Mobs {
		below := f.g.W.Get(int(m.X), int(m.Y)-1, int(m.Z))
		if below == world.Leaves {
			t.Fatalf("%s standing on leaves at %.1f,%.1f,%.1f", m.Def.ID, m.X, m.Y, m.Z)
		}
		if m.Y > 20 {
			t.Fatalf("%s too high: %.1f", m.Def.ID, m.Y)
		}
	}
}

// A swimmer floating at the surface can climb out onto the bank (the client
// pulls itself up when it swims into a ledge within reach).
func TestClimbOutOfWater(t *testing.T) {
	f := newFixture(t)
	p := f.join("Swimmer")
	clear(f.g.Mobs)
	wx, wy, wz := f.wildSpot(0, 0)
	x, z, g := int(wx)-1, int(wz), int(wy)-1 // dig a pool in the west column of the patch
	for y := g - 3; y <= g; y++ {
		f.g.W.Set(x, y, z, world.Water)
	}
	float := float64(g) + 1 - 1.25 // chest at the surface
	f.at(p, float64(x)+0.5, float, wz)
	p.lastMove = f.g.Now
	f.advance(100 * time.Millisecond)
	f.g.Handle(p.ID, &protocol.In{T: "move", X: float64(x) + 1.35, Y: wy, Z: wz})
	if p.Char.Y != wy {
		t.Fatalf("climbing out of the water was rejected: %v", f.out.last(p.ID, "correct"))
	}
}

// Every NPC stands on dry ground (not over water), as do the town gate and
// its warning sign — on every level.
func TestNPCsAndGateStandOnDryLand(t *testing.T) {
	f := newFixture(t)
	w := f.g.W
	dry := func(what string, x, z float64) {
		t.Helper()
		ix, iz := int(math.Floor(x)), int(math.Floor(z))
		under := w.Get(ix, w.Def.GroundY, iz)
		if under == world.Water || !world.Solid(under) {
			t.Errorf("%s at %.1f,%.1f stands on block %d", what, x, z, under)
		}
	}
	for id, n := range f.g.C.NPCs {
		dry("npc "+id, n.X, n.Z)
	}
	g, err := w.Point("town:manchester:gate")
	if err != nil {
		t.Fatal(err)
	}
	dry("gate", g.X, g.Z)
	found := false
	for dz := -30.0; dz <= 30 && !found; dz++ {
		for dx := -30.0; dx <= 30; dx++ {
			x, z := int(math.Floor(g.X+dx)), int(math.Floor(g.Z+dz))
			if w.Get(x, w.Def.GroundY+1, z) == world.Sign {
				dry("warning sign", float64(x)+0.5, float64(z)+0.5)
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no warning sign near the gate")
	}
}

// The welcome message tells the client which images to draw the world map from.
func TestWelcomeNamesTheMapImages(t *testing.T) {
	f := newFixture(t)
	p := f.join("Cartographer")
	m := f.g.Welcome(p.ID, "")["world"].(M)["map"].(M)
	if m["url"] != "maps/earth/terrain.png" || m["relief"] != "maps/earth/display/relief.png" || m["scale"] != 1 {
		t.Fatalf("map: %v", m)
	}
}
