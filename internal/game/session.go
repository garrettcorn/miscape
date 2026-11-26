package game

import (
	"fmt"
	"sort"
	"strings"

	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

// ---------- join / leave ----------

// Session describes an incoming connection to Join.
type Session struct {
	Local bool // host's own loopback connection (trusted for access control)
}

// JoinError is an access failure with a protocol error code.
type JoinError struct{ Code, Msg string }

func (e *JoinError) Error() string { return e.Code + ": " + e.Msg }

// Join authenticates a hello and puts the character in the world. On success
// it returns the new player id and, for new characters, the issued token.
func (g *Game) Join(s Session, h *protocol.In) (pid uint32, token string, err error) {
	var c *Character
	if h.Token != "" {
		id, ok := g.tokens[HashToken(h.Token)]
		if !ok {
			return 0, "", &JoinError{protocol.ErrBadToken, "This world doesn't recognise your character. It may belong to a different world."}
		}
		c = g.Chars[id]
	} else {
		if !protocol.ValidName(h.Name) {
			return 0, "", &JoinError{protocol.ErrBadName, "Names are 3-16 letters, digits or _, starting with a letter."}
		}
		if _, taken := g.names[strings.ToLower(h.Name)]; taken {
			return 0, "", &JoinError{protocol.ErrNameTaken, "That name is already taken in this world."}
		}
		if !s.Local && g.Cfg.Access == "invite" {
			if h.Invite == "" {
				return 0, "", &JoinError{protocol.ErrInviteRequired, "This world is invite-only. Ask the host for an invite code."}
			}
			if !g.useInvite(h.Invite) {
				return 0, "", &JoinError{protocol.ErrInviteInvalid, "That invite code is invalid, used up, or expired."}
			}
		}
	}
	if c != nil {
		if b := g.Bans[c.ID]; b != nil {
			return 0, "", &JoinError{protocol.ErrBanned, "You have been banned from this world."}
		}
		if old, online := g.byChar[c.ID]; online {
			// Reconnect: the newest session wins.
			g.Out.Kick(old, protocol.ErrReplaced, "You connected from somewhere else.")
			g.Leave(old)
		}
	}
	if len(g.Players) >= g.Cfg.MaxPlayers {
		return 0, "", &JoinError{protocol.ErrFull, fmt.Sprintf("This world is full (%d players).", g.Cfg.MaxPlayers)}
	}
	if c == nil {
		token = NewToken()
		color := h.Color
		if color == "" {
			color = "#4a90d9"
		}
		c = &Character{
			ID: newID(), Realm: g.Realm.ID, Name: h.Name, TokenHash: HashToken(token), Color: color,
			X: g.W.Spawn().X, Y: g.W.Spawn().Y, Z: g.W.Spawn().Z,
			Inv: map[string]int{}, XP: map[string]int{}, Quests: map[string]*QuestState{},
			Created: g.Now,
		}
		c.HP = c.MaxHP()
		g.addChar(c)
		g.SaveSoon = true
	}
	p := &Player{ID: g.allocID(), Char: c, Local: s.Local, lastMove: g.Now, groundY: c.Y, lastRegen: g.Now, dirtyMe: true}
	if c.HP <= 0 {
		c.HP = c.MaxHP()
	}
	// Never resume inside a block or outside the world (it may have changed
	// while they were away).
	if !g.W.InZ(c.Z) || g.bodyBlocked(c.X, c.Y, c.Z) {
		sp := g.W.Spawn()
		c.X, c.Y, c.Z = sp.X, sp.Y, sp.Z
	}
	c.X = g.W.WrapX(c.X)
	g.Players[p.ID] = p
	g.byChar[c.ID] = p.ID
	g.ensureEquip(c)
	g.Dirty = true
	g.broadcastChat("", fmt.Sprintf("%s arrived in %s.", c.Name, g.W.ZoneName(g.W.Spawn().X, g.W.Spawn().Z)), "system")
	return p.ID, token, nil
}

func (g *Game) addChar(c *Character) {
	g.Chars[c.ID] = c
	g.tokens[c.TokenHash] = c.ID
	g.names[strings.ToLower(c.Name)] = c.ID
}

// Leave removes a player (disconnect). Any open trade is cancelled; since
// trades never move items until the atomic commit, nothing is lost.
func (g *Game) Leave(pid uint32) {
	p := g.Players[pid]
	if p == nil {
		return
	}
	if p.trade != nil {
		g.cancelTrade(p.trade, p.Char.Name+" left.")
	}
	p.Char.LastSeen = g.Now
	delete(g.Players, pid)
	delete(g.byChar, p.Char.ID)
	g.Dirty, g.SaveSoon = true, true
	g.broadcastChat("", p.Char.Name+" headed home.", "system")
}

// Welcome is sent after a successful join.
func (g *Game) Welcome(pid uint32, token string) M {
	p := g.Players[pid]
	return M{
		"t": "welcome", "proto": protocol.Current, "you": pid, "token": token,
		"realm": M{"id": g.Realm.ID, "kind": g.Realm.Kind, "name": g.Cfg.Name, "desc": g.Cfg.Description,
			"modded": g.Realm.EverModded, "mods": g.C.Mods, "pvp": g.Cfg.PvP, "trading": g.Cfg.Trading == "on"},
		"world":   g.worldView(),
		"content": g.C.View(), "contentHash": g.C.Hash,
		"pos":  []float64{p.Char.X, p.Char.Y, p.Char.Z, p.Char.Yaw},
		"chat": g.chatLog,
	}
}

func (g *Game) meMsg(p *Player) M {
	c := p.Char
	skills := M{}
	for _, s := range content.Skills {
		l := c.Level(s)
		next := XPForLevel(min(l+1, MaxSkillLevel))
		skills[s] = M{"xp": c.XP[s], "lvl": l, "cur": XPForLevel(l), "next": next}
	}
	var hearth float64
	if !p.hearthStart.IsZero() {
		hearth = g.Now.Sub(p.hearthStart).Seconds() / HearthSeconds
	}
	return M{"t": "me", "name": c.Name, "color": c.Color, "hp": c.HP, "mhp": c.MaxHP(), "inv": c.Inv, "eq": c.Equipped, "skills": skills,
		"quests": c.Quests, "slots": g.slotsUsed(c.Inv), "maxSlots": InventorySlots,
		"zone": g.W.ZoneAt(c.X, c.Z), "zoneName": g.W.ZoneName(c.X, c.Z), "hearth": hearth, "trade": g.tradeView(p)}
}

// OnlineNames lists online players (for /api/info and admin).
func (g *Game) OnlineNames() []string {
	var out []string
	for _, p := range g.Players {
		out = append(out, p.Char.Name)
	}
	sort.Strings(out)
	return out
}

// worldView tells the client the shape of the world: its size (x wraps),
// the towns and danger zones, and named points such as the anvil.
func (g *Game) worldView() M {
	w := g.W
	var zones []M
	for _, p := range w.Def.Places {
		if p.Zone == world.ZoneTown {
			zones = append(zones, M{"id": p.ID, "name": p.Name, "kind": world.ZoneTown, "x": p.X, "z": p.Z, "r": p.Radius})
		}
	}
	for _, z := range w.Def.Zones {
		zones = append(zones, M{"id": z.ID, "name": z.Name, "kind": z.Kind, "x": z.X, "z": z.Z, "r": z.Radius})
	}
	sp := w.Spawn()
	a, _ := w.Point("anvil")
	return M{
		"name": w.Def.Name, "level": w.Def.Level,
		"sx": w.W, "sy": world.SizeY, "sz": w.H, "chunk": world.Chunk, "wrapX": true,
		"groundY": w.Def.GroundY, "zones": zones,
		// The terrain image for the world map: pixel = block / scale.
		"map":   g.mapView(),
		"spawn": []float64{sp.X, sp.Y, sp.Z}, "anvil": []float64{a.X, a.Y, a.Z},
	}
}

// mapView says where the world-map images are (served under /maps/).
func (g *Game) mapView() M {
	d := g.W.Def
	m := M{"url": "maps/" + d.Dir + "/" + d.Map, "scale": d.Scale}
	if d.Relief != "" {
		m["relief"] = "maps/" + d.Dir + "/" + d.Relief
	}
	return m
}
