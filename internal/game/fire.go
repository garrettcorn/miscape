package game

import (
	"fmt"
	"math"
	"time"

	"lanternfall/internal/world"
)

// ---------- campfires (firemaking) ----------
//
// A player with a tinderbox (any item flagged "kindle") lights fuel (any item
// with "burn" stats) on the ground. The fire is a block that lights the dark,
// is a cooking station, and keeps fire-fearing creatures (wisps) away, until
// it burns out. Fires are saved with the world.

const (
	MaxFiresPerPlayer = 3
	FireWard          = 5.0 // blocks around a fire that fire-fearing creatures avoid
	LightCooldown     = 1200 * time.Millisecond
)

type fire struct {
	P     world.Pos `json:"p"`
	Until time.Time `json:"until"`
	Owner string    `json:"owner"` // character id
}

// LightFire lights item as a campfire in the empty cell (bx,by,bz).
func (g *Game) LightFire(p *Player, bx, by, bz int, item string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	it := g.C.Items[item]
	if it == nil || it.Burn == nil {
		return fail("That won't burn.")
	}
	if p.Char.Inv[item] < 1 {
		return fail("You don't have any %s.", it.Name)
	}
	if !g.carriesKindling(p.Char) {
		return fail("You need a tinderbox to light a fire.")
	}
	if lvl := p.Char.Level("firemaking"); lvl < it.Burn.Level {
		return fail("You need Firemaking level %d to light %s (you have %d).", it.Burn.Level, it.Name, lvl)
	}
	if !g.W.In(by, bz) || by < 1 || by >= world.SizeY-1 {
		return fail("You can't light a fire there.")
	}
	if g.W.ZoneAt(float64(bx)+0.5, float64(bz)+0.5) == world.ZoneTown {
		return fail("No fires in the streets of %s. Use the kitchen range.", g.W.ZoneName(float64(bx)+0.5, float64(bz)+0.5))
	}
	if g.eyeDist(p, bx, by, bz) > ReachBlocks {
		return fail("That's too far away.")
	}
	if cur := g.W.Get(bx, by, bz); cur != world.Air && cur != world.Flowers {
		return fail("There's no room for a fire there.")
	}
	if below := g.W.Get(bx, by-1, bz); !world.Solid(below) || below == world.Campfire {
		return fail("A fire needs solid ground.")
	}
	if g.entityInCell(bx, by, bz) {
		return fail("Someone is standing there.")
	}
	if g.Now.Before(p.nextGather) {
		return fail("Steady now…")
	}
	mine := 0
	for _, f := range g.fires {
		if f.Owner == p.Char.ID {
			mine++
		}
	}
	if mine >= MaxFiresPerPlayer {
		return fail("You already have %d fires burning.", MaxFiresPerPlayer)
	}
	p.nextGather = g.Now.Add(LightCooldown)
	p.Char.Inv[item]--
	if p.Char.Inv[item] == 0 {
		delete(p.Char.Inv, item)
	}
	p.dirtyMe, g.Dirty = true, true
	g.fires = append(g.fires, fire{P: g.W.PosAt(bx, by, bz), Until: g.Now.Add(time.Duration(it.Burn.Sec) * time.Second), Owner: p.Char.ID})
	g.setBlock(bx, by, bz, world.Campfire)
	g.addXP(p, "firemaking", it.Burn.XP)
	g.progress(p, "burn", item, 1)
	g.toast(p, fmt.Sprintf("The %s catches. It'll burn for about %s.", it.Name, roundDur(time.Duration(it.Burn.Sec)*time.Second)), "good")
	return nil
}

func (g *Game) carriesKindling(c *Character) bool {
	for id, n := range c.Inv {
		if it := g.C.Items[id]; n > 0 && it != nil && it.Kindle {
			return true
		}
	}
	return false
}

// tickFires puts out fires that have burned down.
func (g *Game) tickFires() {
	keep := g.fires[:0]
	for _, f := range g.fires {
		if g.Now.Before(f.Until) {
			keep = append(keep, f)
			continue
		}
		x, y, z := int(f.P.X), int(f.P.Y), int(f.P.Z)
		if g.W.Get(x, y, z) == world.Campfire {
			g.setBlock(x, y, z, world.Air)
		}
	}
	g.fires = keep
}

// nearFire reports whether (x,z) is within r blocks of a burning fire.
func (g *Game) nearFire(x, y, z, r float64) bool {
	for _, f := range g.fires {
		if g.W.Dist(x, z, float64(f.P.X)+0.5, float64(f.P.Z)+0.5) <= r && math.Abs(y-float64(f.P.Y)) < 4 {
			return true
		}
	}
	return false
}

func roundDur(d time.Duration) string {
	if d < 90*time.Second {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	return fmt.Sprintf("%d minutes", int(math.Round(d.Minutes())))
}

// warded reports whether a fire-fearing creature is kept away from (x,y,z).
func (g *Game) warded(m *Mob, x, y, z, r float64) bool {
	return m.Def.FearsFire && len(g.fires) > 0 && g.nearFire(x, y, z, r)
}
