package game

import (
	"fmt"

	"lanternfall/internal/content"
)

// ---------- slaying ----------
//
// A magistrate (any NPC that gives bounties) hands out warrants: kill a
// number of one kind of creature suited to your Slaying level. Each kill of
// that creature while the warrant is open trains Slaying; reporting back
// when it's done pays coins. One warrant at a time.

// BountyState is a character's open warrant.
type BountyState struct {
	Giver string `json:"giver"`
	Mob   string `json:"mob"`
	Total int    `json:"total"`
	Left  int    `json:"left"`
}

func (g *Game) bountyDef(giver, mob string) *content.Bounty {
	for i := range g.C.Bounties {
		if b := &g.C.Bounties[i]; b.Giver == giver && b.Mob == mob {
			return b
		}
	}
	return nil
}

// TakeBounty asks a magistrate for a warrant.
func (g *Game) TakeBounty(p *Player, npc string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	if !g.nearNPC(p, npc) {
		return fail("Walk closer first.")
	}
	if b := p.Char.Bounty; b != nil {
		if b.Left == 0 {
			return fail("Report back on your last warrant first.")
		}
		return fail("You've a warrant open already: %d %s left.", b.Left, g.mobName(b.Mob))
	}
	lvl := p.Char.Level("slaying")
	var ok []*content.Bounty
	gives := false
	for i := range g.C.Bounties {
		b := &g.C.Bounties[i]
		if b.Giver != npc {
			continue
		}
		gives = true
		if b.Level <= lvl {
			ok = append(ok, b)
		}
	}
	if !gives {
		return fail("They don't hand out warrants.")
	}
	if len(ok) == 0 {
		return fail("Nothing suitable for you yet.")
	}
	b := ok[g.rng.IntN(len(ok))]
	n := b.Min + g.rng.IntN(b.Max-b.Min+1)
	p.Char.Bounty = &BountyState{Giver: npc, Mob: b.Mob, Total: n, Left: n}
	p.dirtyMe, g.Dirty = true, true
	g.toast(p, fmt.Sprintf("Warrant issued: see off %d %s.", n, g.mobName(b.Mob)), "quest")
	return nil
}

// ClaimBounty reports a finished warrant for payment.
func (g *Game) ClaimBounty(p *Player, npc string) error {
	b := p.Char.Bounty
	if b == nil || b.Giver != npc {
		return fail("You've no warrant from them.")
	}
	if !g.nearNPC(p, npc) {
		return fail("Walk closer first.")
	}
	if b.Left > 0 {
		return fail("Not done yet: %d %s left.", b.Left, g.mobName(b.Mob))
	}
	coins := 0
	if def := g.bountyDef(b.Giver, b.Mob); def != nil {
		coins = def.Coins * b.Total
	}
	if coins > 0 && !g.grant(p, map[string]int{"coin": coins}) {
		return fail("Your pack is full.")
	}
	p.Char.Bounty = nil
	p.dirtyMe, g.Dirty = true, true
	g.SaveSoon = true
	g.toast(p, fmt.Sprintf("Warrant discharged. The magistrate pays you %d coins.", coins), "good")
	return nil
}

// bountyKill counts a kill against the killer's warrant.
func (g *Game) bountyKill(p *Player, m *Mob) {
	b := p.Char.Bounty
	if b == nil || b.Mob != m.Def.ID || b.Left == 0 {
		return
	}
	b.Left--
	p.dirtyMe = true
	if def := g.bountyDef(b.Giver, b.Mob); def != nil {
		g.addXP(p, "slaying", def.XP)
	}
	if b.Left == 0 {
		g.toast(p, fmt.Sprintf("Warrant complete! Report back to %s.", g.npcName(b.Giver)), "quest")
	}
}

func (g *Game) mobName(id string) string {
	if m := g.C.Mobs[id]; m != nil {
		return m.Name + "s"
	}
	return id
}

func (g *Game) npcName(id string) string {
	if n := g.C.NPCs[id]; n != nil {
		return n.Name
	}
	return id
}
