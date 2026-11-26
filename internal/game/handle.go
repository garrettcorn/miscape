package game

import (
	"strings"

	"lanternfall/internal/protocol"
)

// ---------- request handling ----------

type ridCache struct {
	order []uint32
	acks  map[uint32]M
}

func (g *Game) cachedAck(p *Player, rid uint32) (M, bool) {
	rc := g.rids[p.Char.ID]
	if rc == nil {
		return nil, false
	}
	a, ok := rc.acks[rid]
	return a, ok
}

func (g *Game) rememberAck(p *Player, rid uint32, a M) {
	rc := g.rids[p.Char.ID]
	if rc == nil {
		rc = &ridCache{acks: map[uint32]M{}}
		g.rids[p.Char.ID] = rc
	}
	rc.acks[rid] = a
	rc.order = append(rc.order, rid)
	if len(rc.order) > 128 {
		delete(rc.acks, rc.order[0])
		rc.order = rc.order[1:]
	}
}

// Message handlers. To add a message type:
//  1. declare it in protocol.types (and validate its fields in protocol.Validate);
//  2. add a handler here — "request" handlers return an error that becomes the
//     player-facing ack, "stream" handlers (no rid) answer on their own.
//
// TestEveryMessageHasAHandler keeps the two lists in step.
type (
	requestHandler func(g *Game, p *Player, m *protocol.In) error
	streamHandler  func(g *Game, p *Player, m *protocol.In)
)

var streamHandlers = map[string]streamHandler{
	"move":   func(g *Game, p *Player, m *protocol.In) { g.handleMove(p, m) },
	"chunks": func(g *Game, p *Player, m *protocol.In) { g.sendChunks(p, m.Chunks) },
	"ping":   func(g *Game, p *Player, m *protocol.In) { g.Out.Send(p.ID, M{"t": "pong"}) },
}

var requestHandlers = map[string]requestHandler{
	"gather":        func(g *Game, p *Player, m *protocol.In) error { return g.Gather(p, m.BX, m.BY, m.BZ) },
	"place":         func(g *Game, p *Player, m *protocol.In) error { return g.Place(p, m.BX, m.BY, m.BZ, m.Item) },
	"break":         func(g *Game, p *Player, m *protocol.In) error { return g.Break(p, m.BX, m.BY, m.BZ) },
	"attack":        func(g *Game, p *Player, m *protocol.In) error { return g.Attack(p, m.Target) },
	"craft":         func(g *Game, p *Player, m *protocol.In) error { return g.Craft(p, m.Recipe) },
	"eat":           func(g *Game, p *Player, m *protocol.In) error { return g.Eat(p, m.Item) },
	"equip":         func(g *Game, p *Player, m *protocol.In) error { return g.Equip(p, m.Item) },
	"drop":          func(g *Game, p *Player, m *protocol.In) error { return g.DropItem(p, m.Item, m.Count) },
	"talk":          func(g *Game, p *Player, m *protocol.In) error { return g.Talk(p, m.NPC) },
	"quest_accept":  func(g *Game, p *Player, m *protocol.In) error { return g.AcceptQuest(p, m.Quest) },
	"quest_turnin":  func(g *Game, p *Player, m *protocol.In) error { return g.TurnIn(p, m.Quest) },
	"buy":           func(g *Game, p *Player, m *protocol.In) error { return g.Buy(p, m.Item, m.Count) },
	"sell":          func(g *Game, p *Player, m *protocol.In) error { return g.Sell(p, m.Item, m.Count) },
	"trade_request": func(g *Game, p *Player, m *protocol.In) error { return g.TradeRequest(p, m.Target) },
	"trade_respond": func(g *Game, p *Player, m *protocol.In) error { return g.TradeRespond(p, m.Trade, m.Accept) },
	"trade_offer":   func(g *Game, p *Player, m *protocol.In) error { return g.TradeOffer(p, m.Trade, m.Offer) },
	"trade_lock":    func(g *Game, p *Player, m *protocol.In) error { return g.TradeLock(p, m.Trade) },
	"trade_confirm": func(g *Game, p *Player, m *protocol.In) error { return g.TradeConfirm(p, m.Trade) },
	"trade_cancel":  func(g *Game, p *Player, m *protocol.In) error { return g.TradeCancel(p, m.Trade) },
	"hearth":        func(g *Game, p *Player, m *protocol.In) error { return g.Hearth(p) },
	"chat": func(g *Game, p *Player, m *protocol.In) error {
		g.broadcastChat(p.Char.Name, strings.TrimSpace(m.Text), "say")
		return nil
	},
}

// Handle processes one validated client message.
func (g *Game) Handle(pid uint32, m *protocol.In) {
	p := g.Players[pid]
	if p == nil {
		return
	}
	if h, ok := streamHandlers[m.T]; ok {
		h(g, p, m)
		return
	}
	h, ok := requestHandlers[m.T]
	if !ok {
		return // "hello" is handled by Join; protocol.Validate rejects unknown types
	}
	if m.Rid != 0 {
		if a, ok := g.cachedAck(p, m.Rid); ok {
			// Duplicate request (retry / reconnect resend): same answer, no re-apply.
			g.Out.Send(pid, a)
			return
		}
	}
	err := h(g, p, m)
	ack := M{"t": "ack", "rid": m.Rid, "ok": err == nil}
	if err != nil {
		ack["err"] = err.Error()
	}
	g.rememberAck(p, m.Rid, ack)
	g.Out.Send(pid, ack)
}

func (g *Game) broadcastChat(from, text, kind string) {
	msg := M{"t": "chat", "from": from, "text": text, "kind": kind}
	g.chatLog = append(g.chatLog, msg)
	if len(g.chatLog) > 30 {
		g.chatLog = g.chatLog[1:]
	}
	for id := range g.Players {
		g.Out.Send(id, msg)
	}
}

func (g *Game) sendChunks(p *Player, cs [][2]int) {
	for _, c := range cs {
		cx, cz := ((c[0]%g.W.CX)+g.W.CX)%g.W.CX, c[1]
		if cz < 0 || cz >= g.W.CZ {
			continue
		}
		g.Out.SendBin(p.ID, protocol.ChunkFrame(cx, cz, g.W.EncodeChunk(cx, cz)))
	}
}

func (g *Game) setBlock(x, y, z int, b uint8) {
	g.W.Set(x, y, z, b)
	g.Dirty = true
	x = int(g.W.WrapX(float64(x)))
	msg := M{"t": "block", "x": x, "y": y, "z": z, "b": b}
	// Only players who could have that chunk loaded need to hear about it.
	for id, p := range g.Players {
		if g.W.Dist2(p.Char.X, p.Char.Z, float64(x), float64(z)) < BlockUpdateRadius*BlockUpdateRadius {
			g.Out.Send(id, msg)
		}
	}
}
