package game

import (
	"fmt"
	"time"

	"lanternfall/internal/content"
	"lanternfall/internal/world"
)

// ---------- snares (hunting) and crops (farming) ----------
//
// Both are things a player puts in one block of the world that are ready
// after a while: a snare has caught something, a crop has ripened. They
// belong to whoever set them (only they can collect), keep going while
// nobody is online, and are saved with the world.

const (
	MaxTrapsPerPlayer = 4
	MaxCropsPerPlayer = 8
	SeedBackChance    = 0.5 // chance a harvest also gives the seed back
)

type plot struct {
	P     world.Pos `json:"p"`
	Owner string    `json:"owner"` // character id
	Item  string    `json:"item"`  // the snare or seed
	Ready time.Time `json:"ready"`
}

// placeCheck validates putting something small in the empty cell (bx,by,bz)
// on top of a block that ground(b) accepts.
func (g *Game) placeCheck(p *Player, bx, by, bz int, ground func(uint8) bool, what string) error {
	if !g.W.In(by, bz) || by < 1 || by >= world.SizeY-1 {
		return fail("You can't put a %s there.", what)
	}
	if g.eyeDist(p, bx, by, bz) > ReachBlocks {
		return fail("That's too far away.")
	}
	if cur := g.W.Get(bx, by, bz); cur != world.Air && cur != world.Flowers {
		return fail("There's no room for a %s there.", what)
	}
	if !ground(g.W.Get(bx, by-1, bz)) {
		return nil // the caller's message is more specific
	}
	if g.entityInCell(bx, by, bz) {
		return fail("Someone is standing there.")
	}
	if g.Now.Before(p.nextGather) {
		return fail("Steady now…")
	}
	return nil
}

func countOwned(ps []plot, owner string) int {
	n := 0
	for _, pl := range ps {
		if pl.Owner == owner {
			n++
		}
	}
	return n
}

func plotAt(ps []plot, pos world.Pos) int {
	for i, pl := range ps {
		if pl.P == pos {
			return i
		}
	}
	return -1
}

func (g *Game) takeOne(p *Player, item string) {
	p.Char.Inv[item]--
	if p.Char.Inv[item] <= 0 {
		delete(p.Char.Inv, item)
	}
	g.ensureEquip(p.Char)
	p.dirtyMe, g.Dirty = true, true
}

// SetTrap sets a snare on open ground in the countryside.
func (g *Game) SetTrap(p *Player, bx, by, bz int, item string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	it := g.C.Items[item]
	if it == nil || it.Trap == nil {
		return fail("That isn't a snare.")
	}
	if p.Char.Inv[item] < 1 {
		return fail("You don't have a %s.", it.Name)
	}
	if lvl := p.Char.Level("hunting"); lvl < it.Trap.Level {
		return fail("You need Hunting level %d to set a %s (you have %d).", it.Trap.Level, it.Name, lvl)
	}
	if g.W.ZoneAt(float64(bx)+0.5, float64(bz)+0.5) == world.ZoneTown {
		return fail("Nothing to catch in town. Set snares out in the countryside.")
	}
	open := func(b uint8) bool {
		return b == world.Grass || b == world.Dirt || b == world.Snow || b == world.GloamMoss || b == world.Sand
	}
	if err := g.placeCheck(p, bx, by, bz, open, "snare"); err != nil {
		return err
	}
	if !open(g.W.Get(bx, by-1, bz)) {
		return fail("Snares go on open ground: grass, earth, sand or snow.")
	}
	if countOwned(g.traps, p.Char.ID) >= MaxTrapsPerPlayer {
		return fail("You already have %d snares set.", MaxTrapsPerPlayer)
	}
	p.nextGather = g.Now.Add(GatherCooldown)
	g.takeOne(p, item)
	g.traps = append(g.traps, plot{P: g.W.PosAt(bx, by, bz), Owner: p.Char.ID, Item: item, Ready: g.Now.Add(time.Duration(it.Trap.Sec) * time.Second)})
	g.setBlock(bx, by, bz, world.Snare)
	g.toast(p, "Snare set. Come back in a little while to see what it's caught.", "good")
	return nil
}

// collectTrap empties a snare: a catch if it's sprung, or the empty snare.
func (g *Game) collectTrap(p *Player, i int) error {
	tr := g.traps[i]
	if tr.Owner != p.Char.ID {
		return fail("That's someone else's snare.")
	}
	it := g.C.Items[tr.Item]
	got := map[string]int{tr.Item: 1}
	var ct *content.Catch
	if it != nil && it.Trap != nil && !g.Now.Before(tr.Ready) {
		c := g.pickCatch(p.Char.Level("hunting"), it.Trap.Catches)
		ct = &c
		got[c.Item]++
		for k, n := range c.Extra {
			got[k] += n
		}
	}
	if !g.grant(p, got) {
		return fail("Your pack is full.")
	}
	g.traps = append(g.traps[:i], g.traps[i+1:]...)
	g.setBlock(int(tr.P.X), int(tr.P.Y), int(tr.P.Z), world.Air)
	if ct == nil {
		g.toast(p, "Nothing yet. You take the snare back up.", "info")
		return nil
	}
	g.addXP(p, "hunting", ct.XP)
	g.progress(p, "gather", ct.Item, 1)
	for k, n := range ct.Extra {
		g.progress(p, "gather", k, n)
	}
	g.toast(p, fmt.Sprintf("Your snare caught: %s", g.itemName(ct.Item)), "item")
	return nil
}

// Plant sows a seed in allotment soil.
func (g *Game) Plant(p *Player, bx, by, bz int, item string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	it := g.C.Items[item]
	if it == nil || it.Crop == nil {
		return fail("That won't grow.")
	}
	if p.Char.Inv[item] < 1 {
		return fail("You don't have any %s.", it.Name)
	}
	if lvl := p.Char.Level("farming"); lvl < it.Crop.Level {
		return fail("You need Farming level %d to grow that (you have %d).", it.Crop.Level, lvl)
	}
	soil := func(b uint8) bool { return b == world.Soil }
	if err := g.placeCheck(p, bx, by, bz, soil, "seed"); err != nil {
		return err
	}
	if !soil(g.W.Get(bx, by-1, bz)) {
		return fail("Seeds need tilled soil: try the allotments.")
	}
	if countOwned(g.crops, p.Char.ID) >= MaxCropsPerPlayer {
		return fail("You're already tending %d crops.", MaxCropsPerPlayer)
	}
	p.nextGather = g.Now.Add(GatherCooldown)
	g.takeOne(p, item)
	g.crops = append(g.crops, plot{P: g.W.PosAt(bx, by, bz), Owner: p.Char.ID, Item: item, Ready: g.Now.Add(time.Duration(it.Crop.Sec) * time.Second)})
	g.setBlock(bx, by, bz, world.Sprout)
	g.toast(p, fmt.Sprintf("Planted. It'll be ready in about %s, even if you log off.", roundDur(time.Duration(it.Crop.Sec)*time.Second)), "good")
	return nil
}

// harvest picks a ripe crop.
func (g *Game) harvest(p *Player, i int) error {
	cr := g.crops[i]
	if cr.Owner != p.Char.ID {
		return fail("Someone else is growing that.")
	}
	it := g.C.Items[cr.Item]
	if it == nil || it.Crop == nil {
		return fail("That won't come up.")
	}
	if left := cr.Ready.Sub(g.Now); left > 0 {
		return fail("Not ready yet: about %s to go.", roundDur(left.Round(time.Second)))
	}
	n := it.Crop.Min
	if it.Crop.Max > n {
		n += g.rng.IntN(it.Crop.Max - n + 1)
	}
	got := map[string]int{it.Crop.Yield: n}
	if g.rng.Float64() < SeedBackChance {
		got[cr.Item]++
	}
	if !g.grant(p, got) {
		return fail("Your pack is full.")
	}
	g.crops = append(g.crops[:i], g.crops[i+1:]...)
	g.setBlock(int(cr.P.X), int(cr.P.Y), int(cr.P.Z), world.Air)
	g.addXP(p, "farming", it.Crop.XP)
	g.progress(p, "gather", it.Crop.Yield, n)
	g.toast(p, fmt.Sprintf("+%d %s", n, g.itemName(it.Crop.Yield)), "item")
	return nil
}

// tickPlots springs snares and ripens crops whose time has come.
func (g *Game) tickPlots() {
	for _, tr := range g.traps {
		x, y, z := int(tr.P.X), int(tr.P.Y), int(tr.P.Z)
		if !g.Now.Before(tr.Ready) && g.W.Get(x, y, z) == world.Snare {
			g.setBlock(x, y, z, world.SnareFull)
		}
	}
	for _, cr := range g.crops {
		x, y, z := int(cr.P.X), int(cr.P.Y), int(cr.P.Z)
		if it := g.C.Items[cr.Item]; it != nil && it.Crop != nil && !g.Now.Before(cr.Ready) && g.W.Get(x, y, z) == world.Sprout {
			g.setBlock(x, y, z, it.Crop.Ripe)
		}
	}
}

// pickCatch chooses a random catch among those the level allows (the
// lowest one if none do).
func (g *Game) pickCatch(lvl int, cs []content.Catch) content.Catch {
	var ok []content.Catch
	for _, ct := range cs {
		if ct.Level <= lvl {
			ok = append(ok, ct)
		}
	}
	if len(ok) == 0 {
		return cs[0]
	}
	return ok[g.rng.IntN(len(ok))]
}

// collectAt handles "gather" on a snare or crop; ok is false if there is none.
func (g *Game) collectAt(p *Player, pos world.Pos) (handled bool, err error) {
	if i := plotAt(g.traps, pos); i >= 0 {
		return true, g.collectTrap(p, i)
	}
	if i := plotAt(g.crops, pos); i >= 0 {
		return true, g.harvest(p, i)
	}
	return false, nil
}
