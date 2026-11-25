package game

import (
	"math"
	"time"

	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

// Movement constants shared with the client (web/js/physics.js).
const (
	WalkSpeed   = 4.6
	SprintSpeed = 6.8
	BodyHalf    = 0.3
	BodyHeight  = 1.7
	MaxJumpRise = 1.75 // max rise above last supported height (jump ~1.3 + step slack)
	MaxAirTime  = 2500 * time.Millisecond
)

func (g *Game) bodyBlocked(x, y, z float64) bool {
	for _, dy := range []float64{0.05, 0.9, BodyHeight - 0.05} {
		for _, dx := range []float64{-BodyHalf + 0.02, BodyHalf - 0.02} {
			for _, dz := range []float64{-BodyHalf + 0.02, BodyHalf - 0.02} {
				if g.W.SolidAt(x+dx, y+dy, z+dz) {
					return true
				}
			}
		}
	}
	return false
}

func (g *Game) supported(x, y, z float64) bool {
	for _, dx := range []float64{-BodyHalf, BodyHalf} {
		for _, dz := range []float64{-BodyHalf, BodyHalf} {
			if g.W.SolidAt(x+dx, y-0.15, z+dz) {
				return true
			}
		}
	}
	ix, iy, iz := int(math.Floor(x)), int(math.Floor(y+0.5)), int(math.Floor(z))
	return g.W.Get(ix, iy, iz) == world.Water || g.W.Get(ix, iy-1, iz) == world.Water
}

func (g *Game) entityInCell(x, y, z int) bool {
	fx, fy, fz := float64(x), float64(y), float64(z)
	overlap := func(px, py, pz, half, h float64) bool {
		return px+half > fx && px-half < fx+1 && pz+half > fz && pz-half < fz+1 && py+h > fy && py < fy+1
	}
	for _, p := range g.Players {
		if overlap(p.Char.X, p.Char.Y, p.Char.Z, BodyHalf, BodyHeight) {
			return true
		}
	}
	for _, m := range g.Mobs {
		if m.HP > 0 && overlap(m.X, m.Y, m.Z, 0.4, 1) {
			return true
		}
	}
	return false
}

// MoveCheck validates a proposed position against the last accepted one.
// It returns "" when acceptable or a reason string.
func (g *Game) MoveCheck(p *Player, x, y, z float64) string {
	c := p.Char
	if x < 0.3 || z < 0.3 || x > world.SizeX-0.3 || z > world.SizeZ-0.3 || y < 0.5 || y > world.SizeY+4 {
		return "out of bounds"
	}
	dt := g.Now.Sub(p.lastMove).Seconds()
	dt = math.Max(0.05, math.Min(dt, 1.0))
	horiz := math.Hypot(x-c.X, z-c.Z)
	if horiz > SprintSpeed*dt*1.3+0.35 {
		return "too fast"
	}
	if y-p.groundY > MaxJumpRise {
		return "too high"
	}
	// Sweep the path so fast or long updates can't tunnel through walls. The
	// path is L-shaped (rise, then move across at the higher height, then
	// descend) so legitimate step-ups and ledge drops don't clip corners.
	top := math.Max(c.Y, y)
	if g.sweepBlocked(c.X, c.Y, c.Z, c.X, top, c.Z) || g.sweepBlocked(c.X, top, c.Z, x, top, z) || g.sweepBlocked(x, top, z, x, y, z) {
		return "blocked"
	}
	if g.bodyBlocked(x, y, z) {
		return "blocked"
	}
	return ""
}

func (g *Game) sweepBlocked(x0, y0, z0, x1, y1, z1 float64) bool {
	d := math.Max(math.Hypot(x1-x0, z1-z0), math.Abs(y1-y0))
	steps := int(math.Ceil(d / 0.25))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		if g.bodyBlocked(x0+(x1-x0)*t, y0+(y1-y0)*t, z0+(z1-z0)*t) {
			return true
		}
	}
	return false
}

func (g *Game) handleMove(p *Player, m *protocol.In) {
	if p.dead {
		return
	}
	c := p.Char
	reason := g.MoveCheck(p, m.X, m.Y, m.Z)
	if reason == "" && !g.supported(m.X, m.Y, m.Z) {
		if p.airSince.IsZero() {
			p.airSince = g.Now
		} else if g.Now.Sub(p.airSince) > MaxAirTime && m.Y >= c.Y-0.01 {
			reason = "floating"
		}
	}
	if reason != "" {
		p.violations++
		g.Out.Send(p.ID, M{"t": "correct", "x": c.X, "y": c.Y, "z": c.Z, "why": reason})
		p.lastMove = g.Now
		return
	}
	prevZone := world.ZoneAt(c.X, c.Z)
	c.X, c.Y, c.Z, c.Yaw = m.X, m.Y, m.Z, m.Yaw
	p.lastMove = g.Now
	if g.supported(c.X, c.Y, c.Z) {
		p.groundY = c.Y
		p.airSince = time.Time{}
	} else if c.Y < p.groundY {
		// Falling lowers the reference so you can jump again after landing.
		p.groundY = math.Min(p.groundY, c.Y+0.01)
	}
	if z := world.ZoneAt(c.X, c.Z); z != prevZone {
		p.dirtyMe = true
		if z == world.ZoneGloam {
			g.toast(p, "You enter the Gloam. Creatures here hit harder. Hold Hearth to return home.", "danger")
		}
	}
	g.Dirty = true
	if p.trade != nil {
		o := g.Players[p.trade.other(p.ID)]
		if o != nil && math.Hypot(o.Char.X-c.X, o.Char.Z-c.Z) > TradeRange+4 {
			g.cancelTrade(p.trade, "You moved too far apart.")
		}
	}
}
