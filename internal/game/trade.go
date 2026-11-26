package game

import (
	"fmt"
	"time"
)

// Trade design
//
// Items never leave inventories while a trade is open: offers are only
// promises. The swap happens in a single step inside the game loop (Execute)
// after both players locked AND confirmed the *same* offers. Any offer change
// clears both locks. Execute re-validates ownership and capacity, computes
// both new inventories, and only then assigns them — so it either fully
// happens or not at all. Disconnect, death, or walking apart cancels an open
// trade with nothing moved. A completed trade is removed, so a duplicated or
// replayed confirm cannot apply twice (and the rid cache returns the
// original ack).

const TradeRequestTTL = 30 * time.Second

type Trade struct {
	ID        uint32
	P         [2]uint32 // P[0] requested, P[1] was asked
	Offer     [2]map[string]int
	Locked    [2]bool
	Confirmed [2]bool
	Open      bool // false while the request is pending
	Created   time.Time
	Version   int
}

func (t *Trade) side(pid uint32) int {
	if t.P[0] == pid {
		return 0
	}
	if t.P[1] == pid {
		return 1
	}
	return -1
}

func (t *Trade) other(pid uint32) uint32 {
	if t.P[0] == pid {
		return t.P[1]
	}
	return t.P[0]
}

func (g *Game) tradeView(p *Player) M {
	t := p.trade
	if t == nil {
		return nil
	}
	s := t.side(p.ID)
	o := g.Players[t.other(p.ID)]
	name := "?"
	if o != nil {
		name = o.Char.Name
	}
	return M{"id": t.ID, "open": t.Open, "with": name, "mine": t.Offer[s], "theirs": t.Offer[1-s],
		"myLock": t.Locked[s], "theirLock": t.Locked[1-s], "myConfirm": t.Confirmed[s], "theirConfirm": t.Confirmed[1-s],
		"incoming": s == 1 && !t.Open, "v": t.Version}
}

func (g *Game) tradeChanged(t *Trade) {
	t.Version++
	for _, id := range t.P {
		if p := g.Players[id]; p != nil {
			p.dirtyMe = true
		}
	}
}

func (g *Game) getTrade(p *Player, id uint32) (*Trade, int, error) {
	t := g.Trades[id]
	if t == nil || p.trade != t {
		return nil, -1, fail("That trade is no longer open.")
	}
	return t, t.side(p.ID), nil
}

func (g *Game) TradeRequest(p *Player, target uint32) error {
	if g.Cfg.Trading != "on" {
		return fail("Trading is turned off in this world.")
	}
	if err := g.alive(p); err != nil {
		return err
	}
	o := g.Players[target]
	if o == nil || o == p {
		return fail("Nobody to trade with.")
	}
	if p.trade != nil {
		return fail("Finish your current trade first.")
	}
	if o.trade != nil {
		return fail("%s is busy trading.", o.Char.Name)
	}
	if g.W.Dist(o.Char.X, o.Char.Z, p.Char.X, p.Char.Z) > TradeRange {
		return fail("Get closer to %s to trade.", o.Char.Name)
	}
	t := &Trade{ID: g.allocID(), P: [2]uint32{p.ID, o.ID}, Offer: [2]map[string]int{{}, {}}, Created: g.Now}
	g.Trades[t.ID] = t
	p.trade, o.trade = t, t
	g.tradeChanged(t)
	g.toast(o, p.Char.Name+" would like to trade.", "info")
	return nil
}

func (g *Game) TradeRespond(p *Player, id uint32, accept bool) error {
	t, s, err := g.getTrade(p, id)
	if err != nil {
		return err
	}
	if s != 1 || t.Open {
		return fail("Nothing to respond to.")
	}
	if !accept {
		g.cancelTrade(t, p.Char.Name+" declined.")
		return nil
	}
	t.Open = true
	g.tradeChanged(t)
	return nil
}

func (g *Game) TradeOffer(p *Player, id uint32, offer map[string]int) error {
	t, s, err := g.getTrade(p, id)
	if err != nil {
		return err
	}
	if !t.Open {
		return fail("The trade hasn't been accepted yet.")
	}
	clean := map[string]int{}
	for k, v := range offer {
		it := g.C.Items[k]
		if it == nil {
			return fail("Unknown item.")
		}
		if !it.Tradeable {
			return fail("%s can't be traded.", it.Name)
		}
		if p.Char.Inv[k] < v {
			return fail("You don't have %d %s.", v, it.Name)
		}
		clean[k] = v
	}
	t.Offer[s] = clean
	// Any change invalidates both sides' agreement.
	t.Locked = [2]bool{}
	t.Confirmed = [2]bool{}
	g.tradeChanged(t)
	return nil
}

func (g *Game) TradeLock(p *Player, id uint32) error {
	t, s, err := g.getTrade(p, id)
	if err != nil {
		return err
	}
	if !t.Open {
		return fail("The trade hasn't been accepted yet.")
	}
	t.Locked[s] = true
	g.tradeChanged(t)
	return nil
}

func (g *Game) TradeConfirm(p *Player, id uint32) error {
	t, s, err := g.getTrade(p, id)
	if err != nil {
		return err
	}
	if !t.Locked[0] || !t.Locked[1] {
		return fail("Both of you need to lock in your offers first.")
	}
	t.Confirmed[s] = true
	g.tradeChanged(t)
	if t.Confirmed[0] && t.Confirmed[1] {
		return g.executeTrade(t)
	}
	return nil
}

func (g *Game) TradeCancel(p *Player, id uint32) error {
	t, _, err := g.getTrade(p, id)
	if err != nil {
		return err
	}
	g.cancelTrade(t, p.Char.Name+" cancelled the trade.")
	return nil
}

func (g *Game) endTrade(t *Trade) {
	delete(g.Trades, t.ID)
	for _, id := range t.P {
		if p := g.Players[id]; p != nil && p.trade == t {
			p.trade = nil
			p.dirtyMe = true
		}
	}
}

func (g *Game) cancelTrade(t *Trade, why string) {
	g.endTrade(t)
	for _, id := range t.P {
		if g.Players[id] != nil {
			g.Out.Send(id, M{"t": "trade_end", "ok": false, "text": why})
		}
	}
}

// executeTrade performs the atomic swap.
func (g *Game) executeTrade(t *Trade) error {
	a, b := g.Players[t.P[0]], g.Players[t.P[1]]
	if a == nil || b == nil {
		g.cancelTrade(t, "Your trade partner left.")
		return fail("Your trade partner left.")
	}
	newA, okA := applyDelta(a.Char.Inv, t.Offer[0], t.Offer[1])
	newB, okB := applyDelta(b.Char.Inv, t.Offer[1], t.Offer[0])
	if !okA || !okB {
		g.cancelTrade(t, "Someone no longer has the offered items. Nothing was exchanged.")
		return fail("Trade failed: items changed. Nothing was exchanged.")
	}
	if !g.fits(newA) || !g.fits(newB) {
		g.cancelTrade(t, "Not enough pack space. Nothing was exchanged.")
		return fail("Trade failed: not enough pack space. Nothing was exchanged.")
	}
	// Commit both sides together.
	a.Char.Inv, b.Char.Inv = newA, newB
	g.ensureEquip(a.Char)
	g.ensureEquip(b.Char)
	g.endTrade(t)
	g.Dirty, g.SaveSoon = true, true
	for _, p := range []*Player{a, b} {
		g.Out.Send(p.ID, M{"t": "trade_end", "ok": true, "text": fmt.Sprintf("Trade complete with %s.", g.Players[t.other(p.ID)].Char.Name)})
	}
	return nil
}

func (g *Game) tickTrades() {
	for _, t := range g.Trades {
		if !t.Open && g.Now.Sub(t.Created) > TradeRequestTTL {
			g.cancelTrade(t, "The trade request expired.")
		}
	}
}
