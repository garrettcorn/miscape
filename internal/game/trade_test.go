package game

import (
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"lanternfall/internal/protocol"
)

func tradePair(t *testing.T) (*fixture, *Player, *Player, uint32) {
	f := newFixture(t)
	a, b := f.join("Ann"), f.join("Ben")
	f.atWild(a, 0, 0)
	f.atWild(b, 1, 0)
	a.Char.Inv = map[string]int{"log": 10, "coin": 50, "copper_sword": 1, "warden_token": 1}
	b.Char.Inv = map[string]int{"copper_ore": 6, "coin": 5}
	f.mustOK(a, protocol.In{T: "trade_request", Target: b.ID})
	id := a.trade.ID
	f.mustOK(b, protocol.In{T: "trade_respond", Trade: id, Accept: true})
	return f, a, b, id
}

func TestTradeHappyPathConservesItems(t *testing.T) {
	f, a, b, id := tradePair(t)
	before := f.g.TotalItems()
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"log": 4, "coin": 20}})
	f.mustOK(b, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"copper_ore": 6}})
	f.mustFail(a, protocol.In{T: "trade_confirm", Trade: id}) // must lock first
	f.mustOK(a, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(a, protocol.In{T: "trade_confirm", Trade: id})
	if a.Char.Inv["log"] != 10 {
		t.Fatal("nothing may move before both confirm")
	}
	f.mustOK(b, protocol.In{T: "trade_confirm", Trade: id})
	if a.Char.Inv["log"] != 6 || a.Char.Inv["coin"] != 30 || a.Char.Inv["copper_ore"] != 6 {
		t.Fatalf("A after trade: %v", a.Char.Inv)
	}
	if b.Char.Inv["log"] != 4 || b.Char.Inv["coin"] != 25 || b.Char.Inv["copper_ore"] != 0 {
		t.Fatalf("B after trade: %v", b.Char.Inv)
	}
	if !reflect.DeepEqual(before, f.g.TotalItems()) {
		t.Fatalf("items not conserved: %v -> %v", before, f.g.TotalItems())
	}
	if a.trade != nil || b.trade != nil || len(f.g.Trades) != 0 {
		t.Fatal("trade should be closed")
	}
	if !f.g.SaveSoon {
		t.Fatal("completed trade should trigger a save")
	}
}

func TestTradeOfferChangeResetsLocks(t *testing.T) {
	f, a, b, id := tradePair(t)
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"log": 1}})
	f.mustOK(a, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_confirm", Trade: id})
	// A swaps the offer at the last second (classic scam): both locks clear.
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"log": 1, "coin": 0 + 1}})
	tr := f.g.Trades[id]
	if tr.Locked != [2]bool{} || tr.Confirmed != [2]bool{} {
		t.Fatal("offer change must reset agreement")
	}
	f.mustFail(a, protocol.In{T: "trade_confirm", Trade: id})
}

func TestTradeRejectsUntradeableAndUnowned(t *testing.T) {
	f, a, _, id := tradePair(t)
	f.mustFail(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"warden_token": 1}})
	f.mustFail(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"log": 11}})
	f.mustFail(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"glimmer_blade": 1}})
}

func TestTradeRevalidatesAtCommit(t *testing.T) {
	f, a, b, id := tradePair(t)
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"log": 10}})
	f.mustOK(a, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(a, protocol.In{T: "trade_confirm", Trade: id})
	// A spends the logs before B confirms.
	before := f.g.TotalItems()
	a.Char.Inv["log"] = 3
	before["log"] -= 7
	f.mustFail(b, protocol.In{T: "trade_confirm", Trade: id})
	if b.Char.Inv["log"] != 0 || a.Char.Inv["log"] != 3 {
		t.Fatal("partial trade happened")
	}
	if !reflect.DeepEqual(before, f.g.TotalItems()) {
		t.Fatal("items changed on failed trade")
	}
}

func TestTradeCapacityFailureMovesNothing(t *testing.T) {
	f, a, b, id := tradePair(t)
	for i := 0; i < InventorySlots-1; i++ {
		b.Char.Inv["junk"+string(rune('a'+i))] = 1
	}
	before := f.g.TotalItems()
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"log": 1, "copper_sword": 1}})
	f.mustOK(a, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(a, protocol.In{T: "trade_confirm", Trade: id})
	f.mustFail(b, protocol.In{T: "trade_confirm", Trade: id})
	if !reflect.DeepEqual(before, f.g.TotalItems()) || a.Char.Inv["copper_sword"] != 1 {
		t.Fatal("failed trade moved items")
	}
}

func TestTradeDisconnectCancels(t *testing.T) {
	f, a, b, id := tradePair(t)
	before := f.g.TotalItems()
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"coin": 50}})
	f.mustOK(a, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(a, protocol.In{T: "trade_confirm", Trade: id})
	f.g.Leave(b.ID) // B disconnects mid-trade
	if a.trade != nil || len(f.g.Trades) != 0 {
		t.Fatal("trade should be cancelled")
	}
	if !reflect.DeepEqual(before, f.g.TotalItems()) || a.Char.Inv["coin"] != 50 {
		t.Fatal("disconnect moved items")
	}
	if m := f.out.last(a.ID, "trade_end"); m == nil || m["ok"] != false {
		t.Fatal("A should be told the trade ended")
	}
}

func TestTradeDuplicateConfirmAppliesOnce(t *testing.T) {
	f, a, b, id := tradePair(t)
	f.mustOK(a, protocol.In{T: "trade_offer", Trade: id, Offer: map[string]int{"coin": 10}})
	f.mustOK(a, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(b, protocol.In{T: "trade_lock", Trade: id})
	f.mustOK(a, protocol.In{T: "trade_confirm", Trade: id})
	confirm := protocol.In{T: "trade_confirm", Trade: id, Rid: 9001}
	f.g.Handle(b.ID, &confirm)
	f.g.Handle(b.ID, &confirm)                                               // network retry
	f.g.Handle(b.ID, &protocol.In{T: "trade_confirm", Trade: id, Rid: 9002}) // new rid, stale trade
	if a.Char.Inv["coin"] != 40 || b.Char.Inv["coin"] != 15 {
		t.Fatalf("applied more than once: a=%d b=%d", a.Char.Inv["coin"], b.Char.Inv["coin"])
	}
}

func TestTradeRequestRules(t *testing.T) {
	f := newFixture(t)
	a, b, c := f.join("Cal"), f.join("Dot"), f.join("Eli")
	f.atWild(a, 0, 0)
	f.atWild(b, 50, 0)
	f.mustFail(a, protocol.In{T: "trade_request", Target: b.ID}) // too far
	f.atWild(b, 1, 0)
	f.atWild(c, 0, 1)
	f.mustOK(a, protocol.In{T: "trade_request", Target: b.ID})
	f.mustFail(c, protocol.In{T: "trade_request", Target: a.ID})                                      // busy
	f.mustFail(c, protocol.In{T: "trade_offer", Trade: a.trade.ID, Offer: map[string]int{"coin": 1}}) // not a party
	f.mustFail(a, protocol.In{T: "trade_respond", Trade: a.trade.ID, Accept: true})                   // requester can't accept
	f.g.Tick(f.g.Now.Add(TradeRequestTTL + time.Second))
	if a.trade != nil {
		t.Fatal("pending request should expire")
	}
	f.g.Cfg.Trading = "off"
	f.mustFail(a, protocol.In{T: "trade_request", Target: b.ID})
}

// Randomized invariant check: whatever sequence of trade actions happens,
// the total number of every item never changes and nothing goes negative.
func TestTradeFuzzConservation(t *testing.T) {
	f, a, b, _ := tradePair(t)
	rng := rand.New(rand.NewPCG(1, 2))
	items := []string{"log", "coin", "copper_ore", "copper_sword", "warden_token"}
	ps := []*Player{a, b}
	total := f.g.TotalItems()
	for i := 0; i < 5000; i++ {
		p := ps[rng.IntN(2)]
		o := ps[1-indexOf(ps, p)]
		var m protocol.In
		tid := uint32(0)
		if p.trade != nil {
			tid = p.trade.ID
		}
		switch rng.IntN(8) {
		case 0:
			m = protocol.In{T: "trade_request", Target: o.ID}
		case 1:
			m = protocol.In{T: "trade_respond", Trade: tid, Accept: rng.IntN(4) > 0}
		case 2, 3:
			off := map[string]int{}
			for k := 0; k < rng.IntN(3); k++ {
				off[items[rng.IntN(len(items))]] = 1 + rng.IntN(8)
			}
			m = protocol.In{T: "trade_offer", Trade: tid, Offer: off}
		case 4:
			m = protocol.In{T: "trade_lock", Trade: tid}
		case 5, 6:
			m = protocol.In{T: "trade_confirm", Trade: tid}
		case 7:
			if rng.IntN(10) == 0 {
				m = protocol.In{T: "trade_cancel", Trade: tid}
			} else {
				m = protocol.In{T: "trade_lock", Trade: tid}
			}
		}
		f.rid++
		m.Rid = f.rid
		if rng.IntN(5) == 0 && f.rid > 1 {
			m.Rid = f.rid - 1 // replay
		}
		if m.Trade == 0 && m.T != "trade_request" {
			m.Trade = 1
		}
		f.g.Handle(p.ID, &m)
		for _, pp := range ps {
			for k, v := range pp.Char.Inv {
				if v < 0 {
					t.Fatalf("negative %s", k)
				}
			}
		}
		if got := f.g.TotalItems(); !reflect.DeepEqual(clean(got), clean(total)) {
			t.Fatalf("step %d (%s): items not conserved %v -> %v", i, m.T, total, got)
		}
	}
}

func clean(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		if v != 0 {
			out[k] = v
		}
	}
	return out
}

func indexOf(ps []*Player, p *Player) int {
	for i, x := range ps {
		if x == p {
			return i
		}
	}
	return -1
}
