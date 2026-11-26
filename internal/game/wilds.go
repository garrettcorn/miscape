package game

import (
	"math"
	"time"

	"lanternfall/internal/content"
	"lanternfall/internal/world"
)

// ---------- roaming creatures ----------
//
// Out in the countryside, creatures appear around each player: whatever
// roams that region (content "wilds") at that danger level, and game animals
// at hunting grounds (content "hunting"). They're as strong as the danger
// level where they appear, so the further you travel, the tougher they get.
// They vanish again once nobody is near. Hand-placed creatures (the ones
// around Manchester) are separate and always come back.

const (
	WildsEvery      = 4 * time.Second
	WildsPerPlayer  = 5
	WildsNear       = 48.0 // counted as "around" a player
	WildsMinDist    = 24.0
	WildsMaxDist    = 40.0
	WildsForget     = 80.0 // no player this close...
	WildsForgetTime = 30 * time.Second
	WildsMax        = 600 // across the whole world
)

func (g *Game) tickWilds() {
	if g.Now.Before(g.nextWilds) {
		return
	}
	g.nextWilds = g.Now.Add(WildsEvery)
	// Forget roaming creatures nobody is near.
	wild := 0
	for id, m := range g.Mobs {
		if !m.Wild {
			continue
		}
		near := false
		for _, p := range g.Players {
			if g.W.Dist2(m.X, m.Z, p.Char.X, p.Char.Z) < WildsForget*WildsForget {
				near = true
				break
			}
		}
		switch {
		case near:
			m.lonely = time.Time{}
		case m.lonely.IsZero():
			m.lonely = g.Now
		case g.Now.Sub(m.lonely) > WildsForgetTime:
			delete(g.Mobs, id)
			continue
		}
		wild++
	}
	for _, p := range g.Players {
		if wild >= WildsMax {
			return
		}
		c := p.Char
		if p.dead || g.W.ZoneAt(c.X, c.Z) == world.ZoneTown {
			continue
		}
		n := 0
		for _, m := range g.Mobs {
			if m.Wild && m.HP > 0 && g.W.Dist2(m.X, m.Z, c.X, c.Z) < WildsNear*WildsNear {
				n++
			}
		}
		if n < WildsPerPlayer && g.spawnWild(c.X, c.Z) {
			wild++
		}
	}
}

// spawnWild tries to put one roaming creature somewhere around (x,z).
func (g *Game) spawnWild(px, pz float64) bool {
	for try := 0; try < 6; try++ {
		a := g.rng.Float64() * 2 * math.Pi
		d := WildsMinDist + g.rng.Float64()*(WildsMaxDist-WildsMinDist)
		x, z := g.W.WrapX(px+math.Cos(a)*d), pz+math.Sin(a)*d
		ix, iz := int(math.Floor(x)), int(math.Floor(z))
		if z < 4 || z > float64(g.W.H-4) || g.wet(ix, iz) || g.W.GroundY(ix, iz) <= 0 || g.W.ZoneAt(x, z) != world.ZoneWild {
			continue
		}
		danger := max(1, g.W.Danger(x, z))
		def := g.pickWild(x, z, g.W.Tier(danger))
		if def == nil {
			continue
		}
		m := &Mob{ID: g.allocID(), Def: def, Level: danger, Wild: true,
			Spawn: content.Spawn{Mob: def.ID, X: x, Z: z, R: 6}}
		g.respawnMob(m)
		g.Mobs[m.ID] = m
		return true
	}
	return false
}

// pickWild chooses what appears at (x,z): game animals at hunting grounds,
// otherwise the region's roaming creatures for that tier.
func (g *Game) pickWild(x, z float64, tier int) *content.Mob {
	if s := g.W.SiteAt(x, z); s != nil && s.Kind == world.SiteHunting {
		var ok []string
		for _, q := range g.C.Hunting {
			if q.MinTier <= tier {
				ok = append(ok, q.Mob)
			}
		}
		if len(ok) > 0 {
			return g.C.Mobs[ok[g.rng.IntN(len(ok))]]
		}
	}
	region := ""
	if r := g.W.RegionAt(x, z); r != nil {
		region = r.Name
	}
	total := 0
	var ok []content.Wild
	for _, wd := range g.C.Wilds {
		if (wd.Region == "" || wd.Region == region) && tier >= wd.MinTier && tier <= wd.MaxTier {
			ok = append(ok, wd)
			total += max(1, wd.Weight)
		}
	}
	if total == 0 {
		return nil
	}
	r := g.rng.IntN(total)
	for _, wd := range ok {
		if r -= max(1, wd.Weight); r < 0 {
			return g.C.Mobs[wd.Mob]
		}
	}
	return nil
}
