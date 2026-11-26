package game

import (
	"fmt"
	"slices"

	"lanternfall/internal/world"
)

// ---------- travel: visited towns and the Hearth Lantern ----------
//
// A character remembers every town they've walked into. The Hearth Lantern
// carries them to any of those (oceans are a long swim the first time), and
// a knock-out wakes them in the nearest one.

// visit records that p has entered a town.
func (g *Game) visit(p *Player) {
	id := g.W.TownAt(p.Char.X, p.Char.Z)
	if id == "" || slices.Contains(p.Char.Visited, id) {
		return
	}
	p.Char.Visited = append(p.Char.Visited, id)
	p.dirtyMe, g.Dirty, g.SaveSoon = true, true, true
	if len(p.Char.Visited) > 1 {
		g.toast(p, fmt.Sprintf("You can now Hearth to %s.", g.W.Def.Place(id).Name), "good")
	}
}

// townSpawn is where someone arriving in a town stands.
func (g *Game) townSpawn(id string) (world.Point, bool) {
	pt, err := g.W.Point("town:" + id + ":spawn")
	return pt, err == nil
}

// nearestVisited is the visited town closest to (x,z), or the spawn town.
func (g *Game) nearestVisited(c *Character, x, z float64) string {
	best, bd := g.W.Def.Spawn, -1.0
	for _, id := range c.Visited {
		p := g.W.Def.Place(id)
		if p == nil {
			continue // a town from another map
		}
		if d := g.W.Dist(x, z, p.X, p.Z); bd < 0 || d < bd {
			best, bd = id, d
		}
	}
	return best
}

// visitedView lists the visited towns that exist on this map.
func (g *Game) visitedView(c *Character) []string {
	out := []string{}
	for _, id := range c.Visited {
		if g.W.Def.Place(id) != nil {
			out = append(out, id)
		}
	}
	return out
}

// dangerView lets the client draw the danger overlay with the same formula
// as world.Danger: the spawn town's position and the top danger level.
func (g *Game) dangerView() M {
	sp := g.W.Def.Place(g.W.Def.Spawn)
	return M{"lat": sp.Lat, "lon": sp.Lon, "max": g.W.MaxDanger()}
}

// siteView lists the resource hotspots for the maps, with what each holds
// in words: "Copper Ore, Tin Ore", "Rowan Tree", or the best catch there.
func (g *Game) siteView() []M {
	var out []M
	for _, s := range g.W.Sites() {
		var what []string
		for _, b := range s.Blocks {
			if n := g.C.Nodes[b]; n != nil {
				if n.Item != "" && s.Kind == world.SiteMine {
					what = append(what, g.itemName(n.Item))
				} else {
					what = append(what, n.Name)
				}
			}
		}
		switch s.Kind {
		case world.SiteFishing:
			if n := g.C.Nodes[world.FishSpot]; n != nil {
				for _, ct := range n.Catches {
					if ct.Level <= catchCap(g.W.Tier(s.Danger)) {
						what = append(what, g.itemName(ct.Item))
					}
				}
			}
		case world.SiteHunting:
			for _, h := range g.C.Hunting {
				if g.W.Tier(s.Danger) >= h.MinTier {
					if m := g.C.Mobs[h.Mob]; m != nil {
						what = append(what, m.Name)
					}
				}
			}
		}
		out = append(out, M{"id": s.ID, "kind": s.Kind, "x": s.X, "z": s.Z, "r": s.R, "danger": s.Danger, "what": what})
	}
	return out
}

// catchCap is the highest-level catch found at a danger tier (1-10): herring
// near home, trout from 2, salmon from 4, cod from 6.
func catchCap(tier int) int { return 1 + tier*3/2 }
