package game

import (
	"strings"
	"testing"
	"time"

	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

// v0.8: thieving and slaying.

func TestPickpocketingPaysOrGetsYouFined(t *testing.T) {
	f := newFixture(t)
	p := f.join("Dodger")
	f.atNPC(p, "bertram")
	steal := protocol.In{T: "steal", NPC: "bertram"}
	if e := f.mustFail(p, steal); !strings.Contains(e, "Thieving level 3") {
		t.Fatalf("bertram at thieving 1: %s", e)
	}
	p.Char.XP["thieving"] = XPForLevel(3)
	p.Char.Inv["coin"] = 100
	wins, caught := 0, 0
	for i := 0; i < 60; i++ {
		f.advance(5 * time.Second) // past the cooldown and any stun
		r := f.do(p, steal)
		if r["ok"] == true {
			wins++
		} else if strings.Contains(r["err"].(string), "catches your wrist") && strings.Contains(r["err"].(string), "clip round the ear") {
			caught++
		} else {
			t.Fatalf("unexpected: %v", r)
		}
	}
	// At the NPC's level the odds are 60/40.
	if wins < 20 || caught < 10 {
		t.Fatalf("60 tries: %d clean, %d caught", wins, caught)
	}
	if p.Char.XP["thieving"] <= XPForLevel(3) {
		t.Fatal("pickpocketing should train Thieving")
	}
	// Getting caught stuns you for a moment.
	for {
		f.advance(5 * time.Second)
		if r := f.do(p, steal); r["ok"] != true {
			break
		}
	}
	if e := f.mustFail(p, protocol.In{T: "eat", Item: "berry"}); !strings.Contains(e, "seeing stars") {
		t.Fatalf("not stunned after being caught: %s", e)
	}
	// Players' pockets are never fair game: there's no way to target one.
	f.advance(5 * time.Second)
	if e := f.mustFail(p, protocol.In{T: "steal", NPC: "nobody"}); !strings.Contains(e, "Nobody") {
		t.Fatalf("stealing from a non-NPC: %s", e)
	}
}

func TestMarketStallsRestock(t *testing.T) {
	f := newFixture(t)
	p := f.join("Lifter")
	x, y, z := f.findBlock(world.Stall, nil)
	f.standNear(p, x, y, z)
	p.Char.XP["thieving"] = XPForLevel(10) // 95%: nearly always clean
	var got bool
	for i := 0; i < 5 && !got; i++ {
		f.advance(5 * time.Second)
		if r := f.do(p, protocol.In{T: "gather", BX: x, BY: y, BZ: z}); r["ok"] == true {
			got = true
		}
	}
	if !got || f.g.W.Get(x, y, z) != world.StallEmpty {
		t.Fatalf("stall not robbed: block %d inv %v", f.g.W.Get(x, y, z), p.Char.Inv)
	}
	f.advance(31 * time.Second)
	f.g.tickRegen()
	if f.g.W.Get(x, y, z) != world.Stall {
		t.Fatal("stall should restock")
	}
}

func TestMagistrateWarrants(t *testing.T) {
	f := newFixture(t)
	p := f.join("Warden")
	f.atNPC(p, "rawcliffe")
	f.mustOK(p, protocol.In{T: "bounty_take", NPC: "rawcliffe"})
	b := p.Char.Bounty
	if b == nil || b.Mob != "bramblet" || b.Left < 5 || b.Left > 9 {
		t.Fatalf("a level 1 slayer gets boggarts: %+v", b)
	}
	if e := f.mustFail(p, protocol.In{T: "bounty_take", NPC: "rawcliffe"}); !strings.Contains(e, "already") {
		t.Fatalf("two warrants: %s", e)
	}
	if e := f.mustFail(p, protocol.In{T: "bounty_claim", NPC: "rawcliffe"}); !strings.Contains(e, "Not done") {
		t.Fatalf("claiming early: %s", e)
	}
	var bog, wisp *Mob
	for _, m := range f.g.Mobs {
		switch m.Def.ID {
		case "bramblet":
			bog = m
		case "gloamwisp":
			wisp = m
		}
	}
	// Only assigned kills count.
	f.g.killMob(wisp, p)
	if b.Left != b.Total || p.Char.XP["slaying"] != 0 {
		t.Fatal("a wisp shouldn't count toward a boggart warrant")
	}
	for n := b.Total; n > 0; n-- {
		bog.HP = bog.Def.HP
		f.g.killMob(bog, p)
	}
	if b.Left != 0 || p.Char.XP["slaying"] != 14*b.Total {
		t.Fatalf("warrant: %+v slaying xp %d", b, p.Char.XP["slaying"])
	}
	coins := p.Char.Inv["coin"]
	f.mustOK(p, protocol.In{T: "bounty_claim", NPC: "rawcliffe"})
	if p.Char.Inv["coin"] != coins+3*b.Total || p.Char.Bounty != nil {
		t.Fatalf("claim: coins %d -> %d", coins, p.Char.Inv["coin"])
	}
	// The warrant is saved with the character.
	f.mustOK(p, protocol.In{T: "bounty_take", NPC: "rawcliffe"})
	if snap := f.g.Snapshot(); snap.Characters[p.Char.ID].Bounty == nil {
		t.Fatal("warrant not saved")
	}
}

func TestCloughBoggartNeedsSlaying8(t *testing.T) {
	f := newFixture(t)
	p := f.join("Brave")
	var boss *Mob
	for _, m := range f.g.Mobs {
		if m.Def.ID == "clough_boggart" {
			boss = m
		}
	}
	if boss == nil || f.g.W.ZoneAt(boss.X, boss.Z) != world.ZoneWild || f.g.wet(int(boss.X), int(boss.Z)) {
		t.Fatal("the Clough Boggart should lurk on dry land outside town")
	}
	f.at(p, boss.X+1, boss.Y, boss.Z)
	if e := f.mustFail(p, protocol.In{T: "attack", Target: boss.ID}); !strings.Contains(e, "Slaying 8") {
		t.Fatalf("boss at slaying 1: %s", e)
	}
	p.Char.XP["slaying"] = XPForLevel(8)
	f.mustOK(p, protocol.In{T: "attack", Target: boss.ID})
	if u := f.g.unlocksAt("slaying", 8); !strings.Contains(strings.Join(u, ","), "Boggart of the Clough") {
		t.Fatalf("slaying 8 unlocks: %v", u)
	}
}

// The fine never takes more than you have, and the watch still cuffs you
// when your purse is empty.
func TestBeingCaughtWithEmptyPockets(t *testing.T) {
	f := newFixture(t)
	p := f.join("Skint")
	delete(p.Char.Inv, "coin")
	e := f.g.caught(p, 10, "Caught!")
	if e == nil || strings.Contains(e.Error(), "fines") || p.Char.Inv["coin"] != 0 {
		t.Fatalf("empty purse: %v, coins %d", e, p.Char.Inv["coin"])
	}
	p.Char.Inv["coin"] = 4
	if e := f.g.caught(p, 10, "Caught!"); !strings.Contains(e.Error(), "fines you 4 coins") || p.Char.Inv["coin"] != 0 {
		t.Fatalf("fine larger than purse: %v", e)
	}
}
