package game

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ---------- charms (magic) ----------
//
// Charms are content (base.json "charms"): cast with reagents, they ward,
// heal or bane. Attacking with magic is done with a wand, a magic weapon
// whose ammunition is salt (see Attack).

type effect struct {
	until time.Time
	power int
}

// Cast works a charm the player knows (has the Magic level for).
func (g *Game) Cast(p *Player, id string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	ch := g.C.Charm(id)
	if ch == nil {
		return fail("You don't know that charm.")
	}
	c := p.Char
	if lvl := c.Level("magic"); lvl < ch.Level {
		return fail("You need Magic level %d for %s (you have %d).", ch.Level, ch.Name, lvl)
	}
	if g.Now.Before(p.charmReady[id]) {
		return fail("%s isn't ready yet.", ch.Name)
	}
	if !has(c.Inv, ch.Reagents) {
		return fail("%s needs %s.", ch.Name, g.listItems(ch.Reagents))
	}
	if ch.Effect == "heal" && c.HP >= c.MaxHP() {
		return fail("You're already at full health.")
	}
	n, _ := applyDelta(c.Inv, ch.Reagents, nil)
	c.Inv = n
	g.ensureEquip(c)
	if p.charmReady == nil {
		p.charmReady = map[string]time.Time{}
	}
	p.charmReady[id] = g.Now.Add(time.Duration(ch.CooldownMs) * time.Millisecond)
	switch ch.Effect {
	case "heal":
		c.HP = min(c.MaxHP(), c.HP+ch.Power)
	case "ward":
		p.addEffect("ward", ch.Sec, ch.Power, g.Now)
	case "bane":
		p.addEffect("bane:"+ch.Against, ch.Sec, ch.Power, g.Now)
	}
	p.hearthStart = time.Time{}
	p.dirtyMe, g.Dirty = true, true
	g.addXP(p, "magic", ch.XP)
	g.progress(p, "cast", id, 1)
	g.broadcastNear(c.X, c.Z, M{"t": "cast", "src": p.ID, "charm": id})
	msg := ch.Name + "!"
	if ch.Words != "" {
		msg = fmt.Sprintf("“%s”", ch.Words)
	}
	g.toast(p, msg, "good")
	return nil
}

func (p *Player) addEffect(name string, sec, power int, now time.Time) {
	if p.effects == nil {
		p.effects = map[string]effect{}
	}
	p.effects[name] = effect{until: now.Add(time.Duration(sec) * time.Second), power: power}
}

// hasEffect reports whether an effect is active (expired ones are dropped
// by effectsView).
func (g *Game) hasEffect(p *Player, name string) bool {
	e, ok := p.effects[name]
	return ok && g.Now.Before(e.until)
}

// effectsView is the me-message list of active effects: name -> seconds left.
func (g *Game) effectsView(p *Player) M {
	out := M{}
	for k, e := range p.effects {
		if left := e.until.Sub(g.Now); left > 0 {
			out[k] = int(math.Ceil(left.Seconds()))
		} else {
			delete(p.effects, k)
		}
	}
	return out
}

func (g *Game) listItems(m map[string]int) string {
	s := ""
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for i, id := range ids {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%d %s", m[id], g.itemName(id))
	}
	return s
}
