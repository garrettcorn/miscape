package game

import (
	"strings"
	"testing"
	"time"

	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

func TestXPCurve(t *testing.T) {
	if LevelFor(0) != 1 || LevelFor(49) != 1 || LevelFor(50) != 2 || LevelFor(150) != 3 {
		t.Fatalf("curve wrong: %d %d %d %d", LevelFor(0), LevelFor(49), LevelFor(50), LevelFor(150))
	}
	if LevelFor(1_000_000) != MaxSkillLevel {
		t.Fatal("level must cap")
	}
}

func TestGatherTreeYieldsLogsAndRegrows(t *testing.T) {
	f := newFixture(t)
	p := f.join("Ada")
	x, y, z := f.findBlock(world.Log, wild)
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
	f.at(p, world.SpawnX, world.SpawnY, world.SpawnZ)
	f.g.Tick(f.g.Now.Add(time.Duration(node.RegrowSec+1) * time.Second))
	if f.g.W.Get(x, y, z) != world.Log {
		t.Fatal("tree should regrow")
	}
}

func TestGatherCooldownRangeAndLevel(t *testing.T) {
	f := newFixture(t)
	p := f.join("Bob")
	x, y, z := f.findBlock(world.Log, wild)
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
	cx, cy, cz := f.findBlock(world.CopperOre, nil)
	f.standNear(p, cx, cy, cz)
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
		return !wild(x, y, z) && world.ZoneAt(float64(x), float64(z)) == world.ZoneVillage
	})
	f.standNear(p, x, y, z)
	if e := f.mustFail(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z}); !strings.Contains(e, "village") {
		t.Fatalf("village gather allowed: %s", e)
	}
	p.Char.Inv["plank"] = 5
	f.at(p, world.SpawnX, world.SpawnY, world.SpawnZ)
	if e := f.mustFail(p, protocol.In{T: "place", BX: 66, BY: 13, BZ: 71, Item: "plank"}); !strings.Contains(e, "Wickhollow") {
		t.Fatalf("village build allowed: %s", e)
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
	f.at(p, 30, 13, 30)
	if e := f.mustFail(p, protocol.In{T: "craft", Recipe: "copper_sword"}); !strings.Contains(e, "anvil") {
		t.Fatal(e)
	}
	f.at(p, float64(world.AnvilX)+1.5, 13, float64(world.AnvilZ)+0.5)
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
	f.at(p, world.SpawnX, world.SpawnY, world.SpawnZ)
	f.g.Talk(p, "mabel")
	f.at(p, 60.5, 13, 62.5)
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
	if p.Char.XP["combat"] != m.Def.XP {
		t.Fatalf("combat xp %d", p.Char.XP["combat"])
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
	// Knock out in the Gloam: respawn in village, lose 10% coins.
	f.at(p, m.X+0.5, m.Y, m.Z)
	f.g.damagePlayer(p, 999, m.ID)
	if p.Char.HP != p.Char.MaxHP() || world.ZoneAt(p.Char.X, p.Char.Z) != world.ZoneVillage {
		t.Fatal("should respawn in the village at full HP")
	}
	if p.Char.Inv["coin"] != 90 {
		t.Fatalf("coin loss: %d", p.Char.Inv["coin"])
	}
}

func TestPvPRules(t *testing.T) {
	f := newFixture(t)
	a, b := f.join("Hal"), f.join("Ivy")
	f.at(a, 30, 13, 30)
	f.at(b, 31, 13, 30)
	if e := f.mustFail(a, protocol.In{T: "attack", Target: b.ID}); !strings.Contains(e, "PvP is off") {
		t.Fatal(e)
	}
	f.g.Cfg.PvP = true
	f.mustOK(a, protocol.In{T: "attack", Target: b.ID})
	if b.Char.HP >= b.Char.MaxHP() {
		t.Fatal("pvp damage not applied")
	}
	f.advance(time.Second)
	f.at(a, world.SpawnX, world.SpawnY, world.SpawnZ)
	f.at(b, world.SpawnX+1, world.SpawnY, world.SpawnZ)
	if e := f.mustFail(a, protocol.In{T: "attack", Target: b.ID}); !strings.Contains(e, "safe") {
		t.Fatal(e)
	}
}

func TestQuestFlow(t *testing.T) {
	f := newFixture(t)
	p := f.join("Joy")
	f.at(p, 60.5, 13, 62.5)
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
	// Second quest: deliver shards to Ott.
	f.at(p, 95.5, 13, 62.5)
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
	f.at(p, world.SpawnX, world.SpawnY, world.SpawnZ)
	p.lastMove = f.g.Now
	move := func(x, y, z float64) bool {
		f.advance(50 * time.Millisecond)
		f.g.Handle(p.ID, &protocol.In{T: "move", X: x, Y: y, Z: z})
		return p.Char.X == x && p.Char.Y == y && p.Char.Z == z
	}
	if !move(world.SpawnX+0.3, world.SpawnY, world.SpawnZ) {
		t.Fatal("normal step rejected")
	}
	if move(world.SpawnX+5, world.SpawnY, world.SpawnZ) {
		t.Fatal("teleport accepted")
	}
	if f.out.last(p.ID, "correct") == nil {
		t.Fatal("expected correction")
	}
	if move(world.SpawnX+0.3, world.SpawnY+3, world.SpawnZ) {
		t.Fatal("super jump accepted")
	}
	// Into the plaza well (solid cobble rim at y=13).
	if move(64.2, 13.0, 63.0) {
		t.Fatal("moved into a solid block")
	}
	// Long gap then a big move through the ridge wall must still be blocked.
	f.at(p, 96.5, 13, 50.5)
	f.advance(time.Second)
	f.g.Handle(p.ID, &protocol.In{T: "move", X: 102.5, Y: 13, Z: 50.5})
	if p.Char.X != 96.5 {
		t.Fatal("tunneled through the ridge")
	}
}

func TestPlaceAndBreakOwnership(t *testing.T) {
	f := newFixture(t)
	a, b := f.join("Lua"), f.join("Moe")
	a.Char.Inv["plank"] = 2
	// Creatures spawn at random spots and could stand in the target cell.
	clear(f.g.Mobs)
	x, z := 30, 40
	y := f.g.W.SurfaceY(x, z)
	for f.g.W.Get(x, y, z) != world.Air || !wild(x, y, z) {
		x++
		y = f.g.W.SurfaceY(x, z)
	}
	f.at(a, float64(x)+2.5, float64(f.g.W.SurfaceY(x+2, z)), float64(z)+0.5)
	f.at(b, float64(x)+2.5, float64(f.g.W.SurfaceY(x+2, z)), float64(z)+1.5)
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
	f.at(p, 115, 11, 60)
	f.mustOK(p, protocol.In{T: "hearth"})
	f.g.Tick(f.g.Now.Add(2 * time.Second))
	f.g.damagePlayer(p, 1, 0)
	f.g.Tick(f.g.Now.Add(4 * time.Second))
	if world.ZoneAt(p.Char.X, p.Char.Z) == world.ZoneVillage {
		t.Fatal("damage should interrupt hearth")
	}
	f.mustOK(p, protocol.In{T: "hearth"})
	f.g.Tick(f.g.Now.Add(time.Duration(HearthSeconds*float64(time.Second)) + time.Millisecond))
	if world.ZoneAt(p.Char.X, p.Char.Z) != world.ZoneVillage {
		t.Fatal("hearth should return home")
	}
}

func TestShop(t *testing.T) {
	f := newFixture(t)
	p := f.join("Pip")
	f.at(p, 30, 13, 30)
	p.Char.Inv["coin"] = 10
	f.mustFail(p, protocol.In{T: "buy", Item: "berry", Count: 1}) // not near Bertram
	f.at(p, 69.5, 13, 66.5)
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
	x, z := 30, 90
	for !wild(x, 0, z) || f.g.W.SurfaceY(x, z) != f.g.W.SurfaceY(x+1, z) {
		x++
	}
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
