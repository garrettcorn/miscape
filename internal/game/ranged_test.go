package game

import (
	"strings"
	"testing"
	"time"

	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

// rangeSetup puts p in the countryside with a boggart d blocks away along a
// clear line, and returns it.
func rangeSetup(t *testing.T, f *fixture, p *Player, d float64) *Mob {
	t.Helper()
	f.atWild(p, 0, 0)
	var m *Mob
	for _, mm := range f.g.Mobs {
		if mm.Def.ID == "bramblet" {
			m = mm
			break
		}
	}
	c := p.Char
	for _, dir := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		x, z := f.g.W.WrapX(c.X+dir[0]*d), c.Z+dir[1]*d
		if f.g.lineClear(c.X, c.Y+1.5, c.Z, x, c.Y+0.8, z) && f.g.W.ZoneAt(x, z) == world.ZoneWild {
			m.X, m.Y, m.Z = x, c.Y, z
			m.HP = m.Def.HP
			return m
		}
	}
	t.Fatal("no clear line of fire")
	return nil
}

func TestSlingNeedsStonesReachAndAClearShot(t *testing.T) {
	f := newFixture(t)
	p := f.join("Slinger")
	m := rangeSetup(t, f, p, 6)
	p.Char.Inv["sling"] = 1
	f.mustOK(p, protocol.In{T: "equip", Item: "sling"})
	shoot := protocol.In{T: "attack", Target: m.ID}
	if e := f.mustFail(p, shoot); !strings.Contains(e, "nothing to shoot") || !strings.Contains(e, "Stone") {
		t.Fatalf("shooting without stones: %s", e)
	}
	p.Char.Inv["stone"] = 3
	f.mustOK(p, shoot)
	if p.Char.Inv["stone"] != 2 || m.HP >= m.Def.HP {
		t.Fatalf("shot didn't use a stone or hurt: stones %d hp %d", p.Char.Inv["stone"], m.HP)
	}
	if s := f.out.last(p.ID, "shot"); s == nil || s["kind"] != "shot" {
		t.Fatalf("no shot message: %v", s)
	}
	// Too far.
	f.advance(2 * time.Second)
	dx, dz := f.g.W.DX(p.Char.X, m.X), m.Z-p.Char.Z
	m.X, m.Z = f.g.W.WrapX(p.Char.X+dx*2), p.Char.Z+dz*2
	if e := f.mustFail(p, shoot); !strings.Contains(e, "Out of range") {
		t.Fatalf("12 blocks with a sling: %s", e)
	}
	// A wall in the way.
	m.X, m.Z = f.g.W.WrapX(p.Char.X+dx), p.Char.Z+dz
	wx, wz := int(p.Char.X+dx/2), int(p.Char.Z+dz/2)
	for y := int(p.Char.Y); y < int(p.Char.Y)+3; y++ {
		f.g.W.Set(wx, y, wz, world.Cobble)
	}
	if e := f.mustFail(p, shoot); !strings.Contains(e, "clear shot") {
		t.Fatalf("shooting through a wall: %s", e)
	}
	// A sword can't reach that far.
	p.Char.Inv["copper_sword"] = 1
	f.mustOK(p, protocol.In{T: "equip", Item: "copper_sword"})
	if e := f.mustFail(p, shoot); !strings.Contains(e, "Out of reach") {
		t.Fatalf("melee at range: %s", e)
	}
}

func TestBowsUseTheBestArrowsAndTrainRange(t *testing.T) {
	f := newFixture(t)
	p := f.join("Archer")
	m := rangeSetup(t, f, p, 8)
	p.Char.Inv["shortbow"] = 1
	if e := f.mustFail(p, protocol.In{T: "equip", Item: "shortbow"}); !strings.Contains(e, "Range 4") {
		t.Fatalf("shortbow at range 1: %s", e)
	}
	p.Char.XP["range"] = XPForLevel(4)
	f.mustOK(p, protocol.In{T: "equip", Item: "shortbow"})
	p.Char.Inv["copper_arrow"], p.Char.Inv["iron_arrow"] = 5, 5
	for m.HP > 0 {
		f.advance(2 * time.Second)
		f.mustOK(p, protocol.In{T: "attack", Target: m.ID})
	}
	if p.Char.Inv["iron_arrow"] == 5 || p.Char.Inv["copper_arrow"] != 5 {
		t.Fatalf("should shoot iron arrows first: %v", p.Char.Inv)
	}
	if p.Char.XP["range"] <= XPForLevel(4) {
		t.Fatal("bow kills should train Range")
	}
}

func TestNoShootingFromTown(t *testing.T) {
	f := newFixture(t)
	p := f.join("Sniper")
	m := rangeSetup(t, f, p, 5)
	f.atSpawn(p)
	m.X, m.Y, m.Z = p.Char.X+3, p.Char.Y, p.Char.Z
	p.Char.Inv["rowan_wand"], p.Char.Inv["salt"] = 1, 5
	f.mustOK(p, protocol.In{T: "equip", Item: "rowan_wand"})
	if e := f.mustFail(p, protocol.In{T: "attack", Target: m.ID}); !strings.Contains(e, "No shooting in town") {
		t.Fatalf("shooting from town: %s", e)
	}
}

func TestCharmsHealWardAndBane(t *testing.T) {
	f := newFixture(t)
	p := f.join("Cunning")
	c := p.Char
	if e := f.mustFail(p, protocol.In{T: "cast", Charm: "hearth_charm"}); !strings.Contains(e, "Magic level 2") {
		t.Fatalf("charm at magic 1: %s", e)
	}
	c.XP["magic"] = XPForLevel(7)
	if e := f.mustFail(p, protocol.In{T: "cast", Charm: "hearth_charm"}); !strings.Contains(e, "Salt") {
		t.Fatalf("charm without reagents: %s", e)
	}
	c.Inv["salt"], c.Inv["berry"], c.Inv["rowan_log"], c.Inv["glimmer_shard"] = 10, 2, 1, 1
	c.HP = c.MaxHP() // Magic 7 raises combat level, and so max health
	if e := f.mustFail(p, protocol.In{T: "cast", Charm: "hearth_charm"}); !strings.Contains(e, "full health") {
		t.Fatalf("healing at full health: %s", e)
	}
	c.HP = 5
	f.mustOK(p, protocol.In{T: "cast", Charm: "hearth_charm"})
	if c.HP != 13 || c.Inv["salt"] != 8 || c.Inv["berry"] != 1 {
		t.Fatalf("heal: hp %d inv %v", c.HP, c.Inv)
	}
	if e := f.mustFail(p, protocol.In{T: "cast", Charm: "hearth_charm"}); !strings.Contains(e, "isn't ready") {
		t.Fatalf("no cooldown: %s", e)
	}
	// The ward takes 30% off.
	f.mustOK(p, protocol.In{T: "cast", Charm: "rowan_ward"})
	hp := c.HP
	f.g.damagePlayer(p, 10, 0)
	if hp-c.HP != 7 {
		t.Fatalf("warded hit took %d, want 7", hp-c.HP)
	}
	if fx := f.g.meMsg(p)["fx"].(M); fx["ward"] != 30 {
		t.Fatalf("me fx: %v", fx)
	}
	f.advance(31 * time.Second)
	hp = c.HP
	f.g.damagePlayer(p, 4, 0)
	if hp-c.HP != 4 {
		t.Fatal("the ward should have worn off")
	}
	// Boggart-bane doubles damage against boggarts.
	c.HP = c.MaxHP()
	m := rangeSetup(t, f, p, 2)
	before := m.HP
	f.mustOK(p, protocol.In{T: "attack", Target: m.ID})
	plain := before - m.HP
	f.mustOK(p, protocol.In{T: "cast", Charm: "boggart_bane"})
	m.HP = m.Def.HP
	f.advance(2 * time.Second)
	f.mustOK(p, protocol.In{T: "attack", Target: m.ID})
	if got := m.Def.HP - m.HP; got != 2*plain {
		t.Fatalf("bane hit %d, plain %d", got, plain)
	}
	if c.XP["magic"] <= XPForLevel(7) {
		t.Fatal("charms should train Magic")
	}
}

func TestCunningWomanTeachesCharms(t *testing.T) {
	f := newFixture(t)
	p := f.join("Apprentice")
	f.atNPC(p, "nance")
	p.Char.AP = 2 // Nance wants a little experience first
	f.mustOK(p, protocol.In{T: "quest_accept", Quest: "salt_and_rowan"})
	if p.Char.Inv["rowan_wand"] != 1 || p.Char.Inv["salt"] != 24 {
		t.Fatalf("quest gear: %v", p.Char.Inv)
	}
	p.Char.XP["magic"] = XPForLevel(2)
	p.Char.HP = 5
	f.mustOK(p, protocol.In{T: "cast", Charm: "hearth_charm"})
	if st := p.Char.Quests["salt_and_rowan"]; st.Progress[1] != 1 {
		t.Fatalf("cast step: %v", st.Progress)
	}
	f.g.addXP(p, "magic", XPForLevel(4))
	if lu := f.out.last(p.ID, "levelup"); lu == nil || !strings.Contains(strings.Join(lu["unlocks"].([]string), ","), "Rowan Ward") {
		t.Fatalf("magic 4 should unlock the Rowan Ward: %v", lu)
	}
}
