package game

import (
	"fmt"
	"math"
	"strings"
)

// ---------- skills ----------

// XPForLevel is the total XP needed to reach level L (L>=1).
func XPForLevel(l int) int { return 25 * (l - 1) * l }

func LevelFor(xp int) int {
	l := 1
	for l < MaxSkillLevel && xp >= XPForLevel(l+1) {
		l++
	}
	return l
}

func (c *Character) Level(skill string) int { return LevelFor(c.XP[skill]) }

func (c *Character) MaxHP() int { return 18 + 2*c.Level("combat") }

func (g *Game) addXP(p *Player, skill string, amount int) {
	if amount <= 0 {
		return
	}
	amt := int(math.Round(float64(amount) * g.C.Rules.XPMultiplier))
	before := p.Char.Level(skill)
	p.Char.XP[skill] += amt
	after := p.Char.Level(skill)
	p.dirtyMe = true
	g.Dirty = true
	if after > before {
		g.Out.Send(p.ID, M{"t": "levelup", "skill": skill, "level": after})
		g.toast(p, fmt.Sprintf("%s reached level %d!", title(skill), after), "good")
	}
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
