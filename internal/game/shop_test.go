package game

import (
	"testing"

	"lanternfall/internal/protocol"
)

// Every shop entry can be bought from its NPC, and everything with a sell
// value can be sold to any shopkeeper.
func TestEveryShopEntryBuysAndSells(t *testing.T) {
	f := newFixture(t)
	p := f.join("Shopper")
	for _, s := range f.g.C.Shop {
		p.Char.Inv = map[string]int{"coin": 500}
		f.atNPC(p, s.NPC)
		if r := f.do(p, protocol.In{T: "buy", Item: s.Item, Count: 1}); r["ok"] != true {
			t.Errorf("buy %s from %s: %v", s.Item, s.NPC, r["err"])
		}
	}
	for id, it := range f.g.C.Items {
		if it.Value <= 0 || !it.Tradeable || id == "coin" {
			continue
		}
		for _, s := range f.g.C.Shop {
			p.Char.Inv = map[string]int{id: 1}
			p.Char.Equipped = ""
			f.atNPC(p, s.NPC)
			if r := f.do(p, protocol.In{T: "sell", Item: id, Count: 1}); r["ok"] != true {
				t.Errorf("sell %s to %s: %v", id, s.NPC, r["err"])
			}
		}
	}
}

// The Sell button sells a whole stack, which can be bigger than 64.
func TestSellAWholeBigStack(t *testing.T) {
	f := newFixture(t)
	p := f.join("Bulk")
	p.Char.Inv = map[string]int{"salt": 99, "log": 130}
	f.atNPC(p, "bertram")
	f.mustOK(p, protocol.In{T: "sell", Item: "salt", Count: 99})
	f.mustOK(p, protocol.In{T: "sell", Item: "log", Count: 130})
	if p.Char.Inv["salt"] != 0 || p.Char.Inv["log"] != 0 || p.Char.Inv["coin"] != 99+130 {
		t.Fatalf("after selling: %v", p.Char.Inv)
	}
}
