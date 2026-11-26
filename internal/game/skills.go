package game

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"lanternfall/internal/content"
)

// ---------- skills ----------
//
// Skills are content (base.json "skills"). Levels follow one XP curve, and
// the highest level reachable depends on the world: 10 x the world level
// (level 1 world: 10, level 2: 20, ... level 7: 70). XP stops at the cap, so
// higher levels are earned in the bigger, harder worlds.

// XPForLevel is the total XP needed to reach level L (L>=1).
func XPForLevel(l int) int { return 25 * (l - 1) * l }

// LevelFor is the level XP buys, up to the cap.
func LevelFor(xp, levelCap int) int {
	l := 1
	for l < levelCap && xp >= XPForLevel(l+1) {
		l++
	}
	return l
}

// LevelCap is the highest skill level in this world.
func (g *Game) LevelCap() int { return 10 * max(1, g.W.Def.Level) }

// Level is the character's level in a skill (capped by the world they're in).
func (c *Character) Level(skill string) int {
	cp := c.levelCap
	if cp == 0 {
		cp = 10
	}
	return LevelFor(c.XP[skill], cp)
}

// CombatLevel sums up fighting strength: the best combat style, plus a
// quarter of the other two.
func (c *Character) CombatLevel() int {
	m, r, g := c.Level("melee"), c.Level("range"), c.Level("magic")
	best := max(m, r, g)
	return best + (m+r+g-best)/4
}

// MaxHP grows with combat level.
func (c *Character) MaxHP() int { return 18 + 2*c.CombatLevel() }

// meets checks skill requirements, returning a player-facing reason if not.
func (g *Game) meets(c *Character, needs map[string]int) error {
	keys := make([]string, 0, len(needs))
	for k := range needs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, sk := range keys {
		if lv := c.Level(sk); lv < needs[sk] {
			return fail("Needs %s %d (you have %d).", g.skillName(sk), needs[sk], lv)
		}
	}
	return nil
}

func (g *Game) skillName(id string) string {
	if s := g.C.Skill(id); s != nil {
		return s.Name
	}
	return title(id)
}

// addXP grants skill XP (scaled by the world's XP rule), stopping at the
// world's level cap. It tells the client (for the XP popup) and announces
// level-ups with what they unlock.
func (g *Game) addXP(p *Player, skill string, amount int) {
	if amount <= 0 {
		return
	}
	amt := int(math.Round(float64(amount) * g.C.Rules.XPMultiplier))
	capXP := XPForLevel(g.LevelCap())
	before := p.Char.Level(skill)
	got := min(amt, max(0, capXP-p.Char.XP[skill]))
	if got <= 0 {
		return // at the cap: the next world raises it
	}
	p.Char.XP[skill] += got
	after := p.Char.Level(skill)
	p.dirtyMe = true
	g.Dirty = true
	g.Out.Send(p.ID, M{"t": "xp", "skill": skill, "amount": got})
	if after > before {
		unlocks := g.unlocksAt(skill, after)
		g.Out.Send(p.ID, M{"t": "levelup", "skill": skill, "level": after, "unlocks": unlocks})
		msg := fmt.Sprintf("%s reached level %d!", g.skillName(skill), after)
		if len(unlocks) > 0 {
			msg += " Unlocked: " + strings.Join(unlocks, ", ") + "."
		}
		if after == g.LevelCap() {
			msg += " That's the most this world allows."
		}
		g.toast(p, msg, "good")
	}
}

// unlocksAt lists what reaching a level in a skill opens up, from content:
// recipes, gathering nodes, equipment and quests that need exactly that level.
func (g *Game) unlocksAt(skill string, level int) []string {
	var out []string
	add := func(s string) {
		for _, o := range out {
			if o == s {
				return
			}
		}
		out = append(out, s)
	}
	for _, r := range g.C.Recipes {
		if (r.Skill == skill && r.Level == level) || r.Needs[skill] == level {
			add(g.itemName(r.Output))
		}
	}
	for _, n := range g.C.Nodes {
		if (n.Skill == skill && n.Level == level) || n.Needs[skill] == level {
			add(n.Name)
		}
	}
	for _, it := range g.C.Items {
		if it.Needs[skill] == level {
			add(it.Name)
		}
	}
	for _, q := range g.C.Quests {
		if q.Needs[skill] == level {
			add("the quest " + q.Name)
		}
	}
	sort.Strings(out)
	return out
}

// skillsView is the me-message skill table.
func (g *Game) skillsView(c *Character) M {
	cp := g.LevelCap()
	skills := M{}
	for _, s := range g.C.Skills {
		l := c.Level(s.ID)
		skills[s.ID] = M{"xp": c.XP[s.ID], "lvl": l, "cur": XPForLevel(l), "next": XPForLevel(min(l+1, cp))}
	}
	return skills
}

// styleOf is the combat skill a weapon trains (melee for fists).
func styleOf(it *content.Item) string {
	if it == nil || it.Style == "" {
		return "melee"
	}
	return it.Style
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
