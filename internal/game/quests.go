package game

import (
	"fmt"
	"math"
	"sort"

	"lanternfall/internal/content"
)

// Talk returns an NPC's dialog: greeting, quests on offer, and quest status.
func (g *Game) Talk(p *Player, npc string) error {
	n := g.C.NPCs[npc]
	if n == nil {
		return fail("Nobody by that name.")
	}
	if !g.nearNPC(p, npc) {
		return fail("Walk closer to talk to %s.", n.Name)
	}
	var quests []M
	ids := make([]string, 0)
	for id, q := range g.C.Quests {
		if q.Giver == npc {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		q := g.C.Quests[id]
		st := p.Char.Quests[id]
		status := "available"
		switch {
		case st != nil && st.Status == "done":
			status = "done"
		case st != nil:
			status = "active"
			if g.questReady(p, q, st) {
				status = "ready"
			}
		case q.Requires != "" && (p.Char.Quests[q.Requires] == nil || p.Char.Quests[q.Requires].Status != "done"):
			status = "locked"
		}
		quests = append(quests, M{"id": id, "status": status})
	}
	var shop []content.ShopEntry
	for _, s := range g.C.Shop {
		if s.NPC == npc {
			shop = append(shop, s)
		}
	}
	g.Out.Send(p.ID, M{"t": "dialog", "npc": npc, "name": n.Name, "lines": n.Lines, "quests": quests, "shop": shop})
	return nil
}

func (g *Game) AcceptQuest(p *Player, id string) error {
	q := g.C.Quests[id]
	if q == nil {
		return fail("Unknown quest.")
	}
	if !g.nearNPC(p, q.Giver) {
		return fail("Talk to %s to start this quest.", g.C.NPCs[q.Giver].Name)
	}
	if p.Char.Quests[id] != nil {
		return fail("You've already started that.")
	}
	if err := g.meets(p.Char, q.Needs); err != nil {
		return err
	}
	if q.Requires != "" {
		if pre := p.Char.Quests[q.Requires]; pre == nil || pre.Status != "done" {
			return fail("Finish \"%s\" first.", g.C.Quests[q.Requires].Name)
		}
	}
	if len(q.Start) > 0 && !g.grant(p, q.Start) {
		return fail("Make some room in your pack first.")
	}
	p.Char.Quests[id] = &QuestState{Status: "active", Progress: make([]int, len(q.Steps))}
	p.dirtyMe, g.Dirty = true, true
	g.toast(p, "Quest started: "+q.Name, "quest")
	return nil
}

// progress advances matching steps of active quests. Gather/craft/kill steps
// count events after the quest was accepted.
func (g *Game) progress(p *Player, kind, target string, n int) {
	for id, st := range p.Char.Quests {
		if st.Status != "active" {
			continue
		}
		q := g.C.Quests[id]
		if q == nil {
			continue // quest from a disabled mod: keep state, don't advance
		}
		for i, s := range q.Steps {
			if s.Kind == kind && s.Target == target && i < len(st.Progress) && st.Progress[i] < s.Count {
				st.Progress[i] = min(s.Count, st.Progress[i]+n)
				p.dirtyMe, g.Dirty = true, true
				if st.Progress[i] == s.Count {
					g.toast(p, "✓ "+s.Text, "quest")
				}
			}
		}
	}
}

func (g *Game) questReady(p *Player, q *content.Quest, st *QuestState) bool {
	deliver := map[string]int{}
	for i, s := range q.Steps {
		if s.Kind == content.StepDeliver {
			deliver[s.Target] += s.Count
			continue
		}
		if i >= len(st.Progress) || st.Progress[i] < s.Count {
			return false
		}
	}
	return has(p.Char.Inv, deliver)
}

func (g *Game) TurnIn(p *Player, id string) error {
	q := g.C.Quests[id]
	if q == nil {
		return fail("Unknown quest.")
	}
	st := p.Char.Quests[id]
	if st == nil || st.Status != "active" {
		return fail("That quest isn't active.")
	}
	if !g.nearNPC(p, q.Giver) {
		return fail("Return to %s.", g.C.NPCs[q.Giver].Name)
	}
	if !g.questReady(p, q, st) {
		return fail("You haven't finished everything yet.")
	}
	deliver := map[string]int{}
	for _, s := range q.Steps {
		if s.Kind == content.StepDeliver {
			deliver[s.Target] += s.Count
		}
	}
	mult := g.C.Rules.QuestRewardMult
	add := map[string]int{}
	for k, v := range q.Reward.Items {
		add[k] += v
	}
	if q.Reward.Coins > 0 {
		add["coin"] += int(math.Round(float64(q.Reward.Coins) * mult))
	}
	n, ok := applyDelta(p.Char.Inv, deliver, add)
	if !ok || !g.fits(n) {
		return fail("Make some room in your pack first.")
	}
	p.Char.Inv = n
	g.ensureEquip(p.Char)
	for i, s := range q.Steps {
		if s.Kind == content.StepDeliver {
			st.Progress[i] = s.Count
		}
	}
	st.Status = "done"
	for sk, xp := range q.Reward.XP {
		g.addXP(p, sk, int(math.Round(float64(xp)*mult)))
	}
	p.dirtyMe, g.Dirty, g.SaveSoon = true, true, true
	g.Out.Send(p.ID, M{"t": "quest_done", "quest": id, "text": q.Outro})
	g.toast(p, fmt.Sprintf("Quest complete: %s", q.Name), "quest")
	return nil
}
