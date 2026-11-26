package game

import (
	"math"
	"time"
)

// Items dropped on the ground. Dropping and picking up each run as one step
// on the game loop, so an item is always in exactly one place: a pack or the
// ground. Drops are saved with the world and vanish after DropLifetime.
const (
	DropLifetime    = 10 * time.Minute
	DropPrivateFor  = 60 * time.Second // only the dropper can take it at first
	DropPickupDelay = 1500 * time.Millisecond
	PickupRadius    = 1.4
	MaxDrops        = 300 // per world
	MaxDropsPerChar = 40
)

// Drop is a stack of items lying in the world.
type Drop struct {
	ID    uint32  `json:"-"`
	Item  string  `json:"item"`
	Count int     `json:"count"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Owner string  `json:"owner"` // character id of the dropper
	// OwnerLeft is set once the dropper has stepped away, so standing still
	// after dropping doesn't scoop the item straight back up.
	OwnerLeft bool      `json:"ownerLeft,omitempty"`
	At        time.Time `json:"at"`
	Expires   time.Time `json:"expires"`
}

// canTake reports whether p may pick up d right now.
func (g *Game) canTake(p *Player, d *Drop) bool {
	if d.Owner == p.Char.ID {
		return d.OwnerLeft && g.Now.Sub(d.At) >= DropPickupDelay
	}
	// With trading off, dropping can't be used to hand items to someone else.
	if g.Cfg.Trading != "on" {
		return false
	}
	return g.Now.Sub(d.At) >= DropPrivateFor
}

// DropItem moves count of an item from p's pack to the ground at p's feet.
func (g *Game) DropItem(p *Player, item string, count int) error {
	if err := g.alive(p); err != nil {
		return err
	}
	it := g.C.Items[item]
	if it == nil {
		return fail("That item is from a disabled mod; it stays safe in your pack.")
	}
	if !it.Tradeable {
		return fail("%s can't be dropped.", it.Name)
	}
	if count < 1 || p.Char.Inv[item] < count {
		return fail("You don't have that many.")
	}
	if p.trade != nil {
		return fail("Finish or cancel your trade first.")
	}
	if len(g.Drops) >= MaxDrops {
		return fail("There's too much on the ground already.")
	}
	mine := 0
	for _, d := range g.Drops {
		if d.Owner == p.Char.ID {
			mine++
		}
	}
	if mine >= MaxDropsPerChar {
		return fail("You've dropped a lot already. Pick some up first.")
	}
	n, _ := applyDelta(p.Char.Inv, map[string]int{item: count}, nil)
	p.Char.Inv = n
	g.ensureEquip(p.Char)
	// Land a little in front of the player so it doesn't vanish underfoot.
	c := p.Char
	// A little scatter keeps several drops from stacking on one spot.
	jx, jz := (g.rng.Float64()-0.5)*0.5, (g.rng.Float64()-0.5)*0.5
	x, z := c.X+math.Sin(c.Yaw)*0.9+jx, c.Z+math.Cos(c.Yaw)*0.9+jz
	if g.bodyBlocked(x, c.Y, z) {
		x, z = c.X, c.Z
	}
	d := &Drop{ID: g.allocID(), Item: item, Count: count, X: x, Y: c.Y, Z: z, Owner: c.ID, At: g.Now, Expires: g.Now.Add(DropLifetime)}
	g.Drops[d.ID] = d
	p.dirtyMe, g.Dirty, g.SaveSoon = true, true, true
	return nil
}

// pickup moves as much of d as fits into p's pack. It returns how many moved.
func (g *Game) pickup(p *Player, d *Drop) int {
	take := d.Count
	for take > 0 {
		n, _ := applyDelta(p.Char.Inv, nil, map[string]int{d.Item: take})
		if g.fits(n) {
			p.Char.Inv = n
			break
		}
		take--
	}
	if take == 0 {
		return 0
	}
	d.Count -= take
	if d.Count == 0 {
		delete(g.Drops, d.ID)
	}
	g.ensureEquip(p.Char)
	p.dirtyMe, g.Dirty, g.SaveSoon = true, true, true
	return take
}

// tickDrops expires old drops and picks up the ones players walk over.
func (g *Game) tickDrops() {
	for id, d := range g.Drops {
		if !g.Now.Before(d.Expires) {
			delete(g.Drops, id)
			g.Dirty = true
		}
	}
	if len(g.Drops) == 0 {
		return
	}
	for _, p := range g.Players {
		if p.dead || p.trade != nil {
			continue
		}
		c := p.Char
		for _, d := range g.Drops {
			near := math.Abs(d.Y-c.Y) <= 2 && dist2(d.X, d.Z, c.X, c.Z) <= PickupRadius*PickupRadius
			if d.Owner == c.ID && !d.OwnerLeft && !near {
				d.OwnerLeft = true
			}
			if !near || !g.canTake(p, d) {
				continue
			}
			item := d.Item
			if n := g.pickup(p, d); n > 0 {
				g.Out.Send(p.ID, M{"t": "pickup", "item": item, "count": n})
			} else if g.Now.Sub(p.fullWarned) > 5*time.Second {
				p.fullWarned = g.Now
				g.toast(p, "Your pack is full.", "warn")
			}
		}
	}
}

// dropView lists drops near p for snapshots.
func (g *Game) dropView(p *Player) []M {
	var out []M
	for id, d := range g.Drops {
		if dist2(d.X, d.Z, p.Char.X, p.Char.Z) < InterestRadius*InterestRadius {
			out = append(out, M{"id": id, "i": d.Item, "n": d.Count, "x": r2(d.X), "y": r2(d.Y), "z": r2(d.Z), "own": d.Owner == p.Char.ID})
		}
	}
	return out
}
