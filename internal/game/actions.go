package game

import (
	"fmt"
	"math"
	"time"

	"lanternfall/internal/world"
)

const (
	GatherCooldown = 420 * time.Millisecond
	EatCooldown    = 900 * time.Millisecond
)

func (g *Game) eyeDist(p *Player, bx, by, bz int) float64 {
	c := p.Char
	return math.Sqrt(sq(g.W.DX(c.X, float64(bx)+0.5)) + sq(c.Y+1.5-(float64(by)+0.5)) + sq(c.Z-(float64(bz)+0.5)))
}

func sq(v float64) float64 { return v * v }

func (g *Game) alive(p *Player) error {
	if p.dead || p.Char.HP <= 0 {
		return fail("You need to recover first.")
	}
	return nil
}

// Gather swings at a resource block. Nodes take several hits; the last hit
// yields items and XP and schedules the block to regrow.
func (g *Game) Gather(p *Player, bx, by, bz int) error {
	if err := g.alive(p); err != nil {
		return err
	}
	if !g.W.In(by, bz) {
		return fail("Nothing there.")
	}
	if g.eyeDist(p, bx, by, bz) > ReachBlocks {
		return fail("That's too far away.")
	}
	b := g.W.Get(bx, by, bz)
	node := g.C.Nodes[b]
	idx := g.W.PosAt(bx, by, bz)
	if node == nil || g.placed[idx] != "" {
		return fail("You can't gather that.")
	}
	if g.W.ZoneAt(float64(bx)+0.5, float64(bz)+0.5) == world.ZoneTown {
		return fail("The folk of %s would rather you didn't chop up the town.", g.W.ZoneName(float64(bx)+0.5, float64(bz)+0.5))
	}
	if lvl := p.Char.Level(node.Skill); lvl < node.Level {
		return fail("You need %s level %d for %s (you have %d).", title(node.Skill), node.Level, node.Name, lvl)
	}
	if g.Now.Before(p.nextGather) {
		return fail("Steady now…")
	}
	p.nextGather = g.Now.Add(GatherCooldown)
	// Check space before spending effort.
	probe, _ := applyDelta(p.Char.Inv, nil, map[string]int{node.Item: 1})
	if !g.fits(probe) {
		return fail("Your pack is full.")
	}
	p.hearthStart = time.Time{}
	g.nodeHits[idx]++
	if g.nodeHits[idx] < node.Hits {
		g.Out.Send(p.ID, M{"t": "chip", "x": bx, "y": by, "z": bz, "hit": g.nodeHits[idx], "of": node.Hits})
		return nil
	}
	delete(g.nodeHits, idx)
	n := g.yield()
	got := map[string]int{node.Item: n}
	if !g.grant(p, got) {
		got[node.Item] = 1
		if !g.grant(p, got) {
			return fail("Your pack is full.")
		}
		n = 1
	}
	g.addXP(p, node.Skill, node.XP)
	g.progress(p, "gather", node.Item, n)
	g.setBlock(bx, by, bz, world.Air)
	regrow := time.Duration(float64(node.RegrowSec)*g.C.Rules.RegrowMultiplier) * time.Second
	g.regens = append(g.regens, regen{P: idx, Block: b, At: g.Now.Add(regrow)})
	g.toast(p, fmt.Sprintf("+%d %s", n, g.itemName(node.Item)), "item")
	return nil
}

func (g *Game) yield() int {
	y := g.C.Rules.GatherYield
	n := int(math.Floor(y))
	if g.rng.Float64() < y-float64(n) {
		n++
	}
	return max(1, n)
}

func (g *Game) itemName(id string) string {
	if it := g.C.Items[id]; it != nil {
		return it.Name
	}
	return id + " (missing)"
}

// Place puts a block item into the world.
func (g *Game) Place(p *Player, bx, by, bz int, item string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	it := g.C.Items[item]
	if it == nil || it.Kind != "block" || it.Block == 0 {
		return fail("That can't be placed.")
	}
	if p.Char.Inv[item] < 1 {
		return fail("You don't have any %s.", it.Name)
	}
	if !g.W.In(by, bz) || by < 1 || by >= world.SizeY-1 {
		return fail("You can't build there.")
	}
	if g.W.ZoneAt(float64(bx)+0.5, float64(bz)+0.5) == world.ZoneTown {
		return fail("No building inside %s — try the countryside!", g.W.ZoneName(float64(bx)+0.5, float64(bz)+0.5))
	}
	if g.eyeDist(p, bx, by, bz) > ReachBlocks {
		return fail("That's too far away.")
	}
	cur := g.W.Get(bx, by, bz)
	if cur != world.Air && cur != world.Water && cur != world.Flowers {
		return fail("Something is already there.")
	}
	if g.entityInCell(bx, by, bz) {
		return fail("Someone is standing there.")
	}
	if p.Char.Placed >= MaxPlacedPerCharacter {
		return fail("You've placed the maximum of %d blocks.", MaxPlacedPerCharacter)
	}
	p.Char.Inv[item]--
	if p.Char.Inv[item] == 0 {
		delete(p.Char.Inv, item)
	}
	p.Char.Placed++
	p.dirtyMe = true
	g.placed[g.W.PosAt(bx, by, bz)] = p.Char.ID
	g.setBlock(bx, by, bz, it.Block)
	return nil
}

// Break removes a block the player placed, returning its item.
func (g *Game) Break(p *Player, bx, by, bz int) error {
	if err := g.alive(p); err != nil {
		return err
	}
	if !g.W.In(by, bz) {
		return fail("Nothing there.")
	}
	idx := g.W.PosAt(bx, by, bz)
	owner := g.placed[idx]
	if owner == "" {
		return fail("You can only remove blocks you placed.")
	}
	if owner != p.Char.ID {
		return fail("Someone else built that.")
	}
	if g.eyeDist(p, bx, by, bz) > ReachBlocks {
		return fail("That's too far away.")
	}
	b := g.W.Get(bx, by, bz)
	var item string
	for id, it := range g.C.Items {
		if it.Kind == "block" && it.Block == b {
			item = id
		}
	}
	if item != "" && !g.grant(p, map[string]int{item: 1}) {
		return fail("Your pack is full.")
	}
	delete(g.placed, idx)
	p.Char.Placed = max(0, p.Char.Placed-1)
	g.setBlock(bx, by, bz, world.Air)
	return nil
}

func (g *Game) nearAnvil(p *Player) bool {
	c := p.Char
	a, err := g.W.Point("anvil")
	return err == nil && g.W.Dist(c.X, c.Z, a.X+0.5, a.Z+0.5) <= AnvilRange
}

// Craft turns inputs into an output if the player meets the requirements.
func (g *Game) Craft(p *Player, recipe string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	r := g.C.Recipes[recipe]
	if r == nil {
		return fail("Unknown recipe.")
	}
	if lvl := p.Char.Level("crafting"); lvl < r.Level {
		return fail("You need Crafting level %d (you have %d).", r.Level, lvl)
	}
	if r.Station == "anvil" && !g.nearAnvil(p) {
		return fail("You need to be at the anvil in %s.", g.W.ZoneName(g.W.Spawn().X, g.W.Spawn().Z))
	}
	if !has(p.Char.Inv, r.Inputs) {
		return fail("You're missing materials.")
	}
	n, ok := applyDelta(p.Char.Inv, r.Inputs, map[string]int{r.Output: r.Count})
	if !ok || !g.fits(n) {
		return fail("Your pack is full.")
	}
	p.Char.Inv = n
	g.ensureEquip(p.Char)
	p.dirtyMe = true
	g.Dirty = true
	g.addXP(p, "crafting", r.XP)
	g.progress(p, "craft", r.Output, r.Count)
	if r.Level >= 3 {
		g.SaveSoon = true
	}
	g.toast(p, fmt.Sprintf("Crafted %s ×%d", g.itemName(r.Output), r.Count), "good")
	return nil
}

func (g *Game) Eat(p *Player, item string) error {
	if err := g.alive(p); err != nil {
		return err
	}
	it := g.C.Items[item]
	if it == nil || it.Kind != "food" {
		return fail("You can't eat that.")
	}
	if p.Char.Inv[item] < 1 {
		return fail("You don't have any.")
	}
	if g.Now.Before(p.nextEat) {
		return fail("Still chewing…")
	}
	if p.Char.HP >= p.Char.MaxHP() {
		return fail("You're already at full health.")
	}
	p.nextEat = g.Now.Add(EatCooldown)
	p.Char.Inv[item]--
	if p.Char.Inv[item] == 0 {
		delete(p.Char.Inv, item)
	}
	p.Char.HP = min(p.Char.MaxHP(), p.Char.HP+it.Heal)
	p.dirtyMe = true
	g.Dirty = true
	return nil
}

func (g *Game) Equip(p *Player, item string) error {
	if item == "" {
		p.Char.Equipped = ""
		p.dirtyMe = true
		return nil
	}
	it := g.C.Items[item]
	if it == nil || it.Kind != "weapon" {
		return fail("That isn't a weapon.")
	}
	if p.Char.Inv[item] < 1 {
		return fail("You don't have that.")
	}
	p.Char.Equipped = item
	p.dirtyMe = true
	g.Dirty = true
	return nil
}

func (g *Game) nearNPC(p *Player, id string) bool {
	n := g.C.NPCs[id]
	if n == nil {
		return false
	}
	c := p.Char
	return g.W.Dist(c.X, c.Z, n.X, n.Z) <= NPCRange && math.Abs(c.Y-n.Y) < 3
}

func (g *Game) Buy(p *Player, item string, count int) error {
	for _, s := range g.C.Shop {
		if s.Item != item {
			continue
		}
		if !g.nearNPC(p, s.NPC) {
			return fail("Visit %s to buy that.", g.C.NPCs[s.NPC].Name)
		}
		cost := s.Price * count
		if p.Char.Inv["coin"] < cost {
			return fail("That costs %d coins; you have %d.", cost, p.Char.Inv["coin"])
		}
		n, ok := applyDelta(p.Char.Inv, map[string]int{"coin": cost}, map[string]int{item: count})
		if !ok || !g.fits(n) {
			return fail("Your pack is full.")
		}
		p.Char.Inv = n
		g.ensureEquip(p.Char)
		p.dirtyMe, g.Dirty = true, true
		return nil
	}
	return fail("Nobody sells that.")
}

func (g *Game) Sell(p *Player, item string, count int) error {
	it := g.C.Items[item]
	if it == nil || it.Value <= 0 || !it.Tradeable {
		return fail("Nobody wants to buy that.")
	}
	near := false
	for _, s := range g.C.Shop {
		near = near || g.nearNPC(p, s.NPC)
	}
	if !near {
		return fail("Find a shopkeeper to sell.")
	}
	if p.Char.Inv[item] < count {
		return fail("You don't have that many.")
	}
	n, _ := applyDelta(p.Char.Inv, map[string]int{item: count}, map[string]int{"coin": it.Value * count})
	p.Char.Inv = n
	g.ensureEquip(p.Char)
	p.dirtyMe, g.Dirty = true, true
	return nil
}

// Hearth starts a channelled return to the village (cancelled by damage or moving).
func (g *Game) Hearth(p *Player) error {
	if err := g.alive(p); err != nil {
		return err
	}
	if !p.hearthStart.IsZero() {
		p.hearthStart = time.Time{}
		p.dirtyMe = true
		return nil // pressing again cancels
	}
	p.hearthStart = g.Now
	p.hearthX, p.hearthZ = p.Char.X, p.Char.Z
	p.dirtyMe = true
	return nil
}
