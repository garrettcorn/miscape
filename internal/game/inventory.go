package game

// Inventories are maps of item id -> count. Capacity is measured in slots:
// each item uses ceil(count/stack) slots; coins live in a purse and use none.
// Items whose definition is missing (a disabled mod) are preserved and use
// one slot per stack of 64, so nothing is ever silently deleted.

func (g *Game) stackOf(id string) int {
	if it := g.C.Items[id]; it != nil {
		return it.Stack
	}
	return 64
}

func (g *Game) slotsUsed(inv map[string]int) int {
	n := 0
	for id, c := range inv {
		if c <= 0 || id == "coin" {
			continue
		}
		s := g.stackOf(id)
		n += (c + s - 1) / s
	}
	return n
}

func (g *Game) fits(inv map[string]int) bool { return g.slotsUsed(inv) <= InventorySlots }

func cloneInv(inv map[string]int) map[string]int {
	out := make(map[string]int, len(inv))
	for k, v := range inv {
		if v > 0 {
			out[k] = v
		}
	}
	return out
}

func has(inv map[string]int, need map[string]int) bool {
	for k, v := range need {
		if inv[k] < v {
			return false
		}
	}
	return true
}

// applyDelta returns a new inventory with add added and sub removed, or ok=false
// if something would go negative. It never mutates its inputs.
func applyDelta(inv, sub, add map[string]int) (map[string]int, bool) {
	out := cloneInv(inv)
	for k, v := range sub {
		if out[k] < v {
			return nil, false
		}
		out[k] -= v
		if out[k] == 0 {
			delete(out, k)
		}
	}
	for k, v := range add {
		if v > 0 {
			out[k] += v
		}
	}
	return out, true
}

// grant tries to add items; it changes nothing and returns false if they don't fit.
func (g *Game) grant(p *Player, add map[string]int) bool {
	n, _ := applyDelta(p.Char.Inv, nil, add)
	if !g.fits(n) {
		return false
	}
	p.Char.Inv = n
	p.dirtyMe = true
	g.Dirty = true
	return true
}

// ensureEquip clears the equipped weapon if it is no longer held.
func (g *Game) ensureEquip(c *Character) {
	if c.Equipped != "" && c.Inv[c.Equipped] <= 0 {
		c.Equipped = ""
	}
	if c.Equipped == "" {
		// Auto-equip the best held weapon for convenience.
		best := 0
		for id, n := range c.Inv {
			if it := g.C.Items[id]; n > 0 && it != nil && it.Kind == "weapon" && it.Damage > best {
				best, c.Equipped = it.Damage, id
			}
		}
	}
}

// TotalItems counts every item across all characters (for conservation tests).
func (g *Game) TotalItems() map[string]int {
	t := map[string]int{}
	for _, c := range g.Chars {
		for k, v := range c.Inv {
			t[k] += v
		}
	}
	return t
}
