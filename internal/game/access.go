package game

import (
	"crypto/rand"
	"errors"
	"sort"
	"strings"
	"time"

	"lanternfall/internal/protocol"
)

var inviteWords = []string{"MOSS", "LAMP", "FERN", "WICK", "PEBBLE", "BRAMBLE", "GLOW", "ACORN", "DEW", "THIMBLE"}

const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I

// CreateInvite makes a new invite code. uses<=0 means unlimited.
func (g *Game) CreateInvite(label string, uses int, ttl time.Duration) *Invite {
	var b [7]byte
	rand.Read(b[:])
	var sb strings.Builder
	sb.WriteString(inviteWords[int(b[0])%len(inviteWords)])
	sb.WriteByte('-')
	for _, c := range b[1:] {
		sb.WriteByte(inviteAlphabet[int(c)%len(inviteAlphabet)])
	}
	inv := &Invite{Code: sb.String(), Label: trimLen(label, 40), UsesLeft: uses, Created: g.Now}
	if uses <= 0 {
		inv.UsesLeft = -1
	}
	if ttl > 0 {
		inv.Expires = g.Now.Add(ttl)
	}
	g.Invites[inv.Code] = inv
	g.SaveSoon = true
	return inv
}

func trimLen(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func (g *Game) RevokeInvite(code string) bool {
	if _, ok := g.Invites[code]; !ok {
		return false
	}
	delete(g.Invites, code)
	g.SaveSoon = true
	return true
}

func (g *Game) useInvite(code string) bool {
	inv := g.Invites[strings.ToUpper(strings.TrimSpace(code))]
	if inv == nil {
		return false
	}
	if !inv.Expires.IsZero() && g.Now.After(inv.Expires) {
		return false
	}
	if inv.UsesLeft == 0 {
		return false
	}
	if inv.UsesLeft > 0 {
		inv.UsesLeft--
	}
	g.SaveSoon = true
	return true
}

func (g *Game) InviteList() []*Invite {
	out := make([]*Invite, 0, len(g.Invites))
	for _, i := range g.Invites {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Created.Before(out[b].Created) })
	return out
}

// BanByName bans a character and disconnects it if online.
func (g *Game) BanByName(name, reason string) error {
	id, ok := g.names[strings.ToLower(name)]
	if !ok {
		return errors.New("no character with that name")
	}
	g.Bans[id] = &Ban{CharID: id, Name: g.Chars[id].Name, Reason: trimLen(reason, 100), At: g.Now}
	if pid, online := g.byChar[id]; online {
		g.Out.Kick(pid, protocol.ErrBanned, "You have been banned from this world.")
		g.Leave(pid)
	}
	g.SaveSoon = true
	return nil
}

func (g *Game) Unban(name string) bool {
	for id, b := range g.Bans {
		if strings.EqualFold(b.Name, name) {
			delete(g.Bans, id)
			g.SaveSoon = true
			return true
		}
	}
	return false
}

// Kick disconnects an online player without banning.
func (g *Game) KickByName(name string) bool {
	id, ok := g.names[strings.ToLower(name)]
	if !ok {
		return false
	}
	pid, online := g.byChar[id]
	if !online {
		return false
	}
	g.Out.Kick(pid, "kicked", "The host removed you from the world.")
	g.Leave(pid)
	return true
}

// CharacterSummary is what the admin screen lists.
type CharacterSummary struct {
	Name     string         `json:"name"`
	Online   bool           `json:"online"`
	Banned   bool           `json:"banned"`
	Levels   map[string]int `json:"levels"`
	LastSeen time.Time      `json:"lastSeen"`
}

func (g *Game) Characters() []CharacterSummary {
	var out []CharacterSummary
	for id, c := range g.Chars {
		_, on := g.byChar[id]
		lv := map[string]int{}
		for s := range c.XP {
			lv[s] = c.Level(s)
		}
		out = append(out, CharacterSummary{c.Name, on, g.Bans[id] != nil, lv, c.LastSeen})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}
