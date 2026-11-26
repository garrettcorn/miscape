package game

import (
	"fmt"
	"math"
	"time"

	"lanternfall/internal/content"
)

// ---------- thieving ----------
//
// Players can pick townsfolk's pockets (NPCs with a "pocket") and lift goods
// from market stalls (gathering nodes that train thieving). Never from other
// players. Success gets likelier with Thieving level; getting caught means a
// stun and a fine from the watch.

const (
	StealCooldown = 1500 * time.Millisecond
	CaughtStun    = 4 * time.Second
	StallFine     = 5
)

// stealChance is the chance a theft at the given difficulty succeeds.
func stealChance(lvl, need int) float64 {
	return math.Max(0.3, math.Min(0.95, 0.6+0.06*float64(lvl-need)))
}

// Steal picks an NPC's pocket.
func (g *Game) Steal(p *Player, npc string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	n := g.C.NPCs[npc]
	if n == nil {
		return fail("Nobody by that name.")
	}
	if n.Pocket == nil {
		return fail("You'd never get away with it.")
	}
	if !g.nearNPC(p, npc) {
		return fail("Get closer first.")
	}
	pk := n.Pocket
	if lvl := p.Char.Level("thieving"); lvl < pk.Level {
		return fail("You need Thieving level %d to pick %s's pocket (you have %d).", pk.Level, firstName(n.Name), lvl)
	}
	if g.Now.Before(p.nextSteal) {
		return fail("Steady now…")
	}
	p.nextSteal = g.Now.Add(StealCooldown)
	if g.rng.Float64() >= stealChance(p.Char.Level("thieving"), pk.Level) {
		return g.caught(p, pk.Fine, fmt.Sprintf("%s catches your wrist!", firstName(n.Name)))
	}
	got := map[string]int{}
	if c := pk.Coins[0] + g.rng.IntN(pk.Coins[1]-pk.Coins[0]+1); c > 0 {
		got["coin"] = c
	}
	for _, d := range pk.Loot {
		if g.rng.Float64() < d.Chance {
			got[d.Item] += max(1, d.Min+g.rng.IntN(d.Max-d.Min+1))
		}
	}
	if !g.grant(p, got) {
		return fail("Your pack is full.")
	}
	g.addXP(p, "thieving", pk.XP)
	g.toast(p, "You lift "+g.describe(got)+".", "item")
	return nil
}

// caught stuns the thief and has the watch fine them.
func (g *Game) caught(p *Player, fine int, why string) error {
	p.stunUntil = g.Now.Add(CaughtStun)
	fine = min(fine, p.Char.Inv["coin"])
	if fine > 0 {
		p.Char.Inv["coin"] -= fine
		if p.Char.Inv["coin"] == 0 {
			delete(p.Char.Inv, "coin")
		}
	}
	p.dirtyMe, g.Dirty = true, true
	msg := why + " The watch gives you a clip round the ear"
	if fine > 0 {
		msg += fmt.Sprintf(" and fines you %d coins", fine)
	}
	return fail("%s.", msg)
}

// stealFromStall is gathering from a thieving node (a market stall).
func (g *Game) stealFromStall(p *Player, node *content.Node) error {
	if g.rng.Float64() >= stealChance(p.Char.Level("thieving"), node.Level) {
		return g.caught(p, StallFine, "The stallholder spots you!")
	}
	return nil
}

func (g *Game) describe(got map[string]int) string {
	s := ""
	for i, id := range sortedKeys(got) {
		if i > 0 {
			s += ", "
		}
		if id == "coin" {
			s += fmt.Sprintf("%d coins", got[id])
		} else {
			s += fmt.Sprintf("%d %s", got[id], g.itemName(id))
		}
	}
	return s
}

func sortedKeys(m map[string]int) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

func firstName(full string) string {
	for i, r := range full {
		if r == ' ' || r == ',' {
			return full[:i]
		}
	}
	return full
}
