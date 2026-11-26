// Package protocol defines the versioned client/server wire format and
// validates untrusted client messages before they reach game rules.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// ProtoMin..ProtoMax is the range of client protocol versions accepted.
	// Version 2: the wrapping Earth world (16-bit chunk coordinates, world
	// shape sent in welcome). Version 3: skills (xp and levelup messages),
	// blocks 24-31 (fishing spots, campfires, new trees and ores) and the
	// light_fire message.
	// Version 4: snares, crops and agility courses (blocks 32-39, the set_trap
	// and plant messages).
	// Version 5: thieving and slaying (stall blocks 40-41; steal, bounty_take
	// and bounty_claim).
	ProtoMin = 5
	ProtoMax = 5
	// Current is what this server speaks natively.
	Current = 5

	// MaxCoord bounds block and chunk coordinates; the game checks them
	// against the actual world size.
	MaxCoord = 1 << 20

	MaxMessageBytes = 4096
	MaxChatLen      = 200
	MaxOfferEntries = 8
	MaxChunkRequest = 16

	// Binary frame types (server -> client).
	BinChunk byte = 0x01
)

// Error codes sent to clients in {"t":"error","code":...}.
const (
	ErrIncompatible   = "incompatible_version"
	ErrBadMessage     = "bad_message"
	ErrRateLimited    = "rate_limited"
	ErrNeedHello      = "hello_required"
	ErrBadToken       = "bad_token"
	ErrNameTaken      = "name_taken"
	ErrBadName        = "bad_name"
	ErrInviteRequired = "invite_required"
	ErrInviteInvalid  = "invite_invalid"
	ErrBanned         = "banned"
	ErrFull           = "world_full"
	ErrReplaced       = "session_replaced"
	ErrServer         = "server_error"
)

// In is every client->server message. A flat struct keeps decoding strict and
// validation in one place; each type uses a subset of fields.
type In struct {
	T   string `json:"t"`
	Rid uint32 `json:"rid,omitempty"`

	// hello
	Proto  int    `json:"proto,omitempty"`
	Client string `json:"client,omitempty"`
	Token  string `json:"token,omitempty"`
	Name   string `json:"name,omitempty"`
	Invite string `json:"invite,omitempty"`
	Color  string `json:"color,omitempty"`

	// move
	X   float64 `json:"x,omitempty"`
	Y   float64 `json:"y,omitempty"`
	Z   float64 `json:"z,omitempty"`
	Yaw float64 `json:"yaw,omitempty"`

	// block / entity targets
	BX     int    `json:"bx,omitempty"`
	BY     int    `json:"by,omitempty"`
	BZ     int    `json:"bz,omitempty"`
	Target uint32 `json:"target,omitempty"`

	Recipe string         `json:"recipe,omitempty"`
	Charm  string         `json:"charm,omitempty"`
	Item   string         `json:"item,omitempty"`
	Count  int            `json:"count,omitempty"`
	NPC    string         `json:"npc,omitempty"`
	Quest  string         `json:"quest,omitempty"`
	Text   string         `json:"text,omitempty"`
	Trade  uint32         `json:"trade,omitempty"`
	Accept bool           `json:"accept,omitempty"`
	Offer  map[string]int `json:"offer,omitempty"`
	Chunks [][2]int       `json:"chunks,omitempty"`
}

// Message types and whether they need a request id (state-changing).
var types = map[string]bool{
	"hello": false, "move": false, "chunks": false, "ping": false,
	"gather": true, "attack": true, "place": true, "break": true, "light_fire": true, "set_trap": true, "plant": true,
	"craft": true, "eat": true, "equip": true, "drop": true, "cast": true,
	"talk": true, "quest_accept": true, "quest_turnin": true, "steal": true, "bounty_take": true, "bounty_claim": true,
	"buy": true, "sell": true,
	"trade_request": true, "trade_respond": true, "trade_offer": true,
	"trade_lock": true, "trade_confirm": true, "trade_cancel": true,
	"chat": true, "hearth": true,
}

// NeedsRid reports whether a message type must carry a request id.
func NeedsRid(t string) bool { return types[t] }

// Types lists every client message type and whether it needs a request id.
func Types() map[string]bool {
	out := make(map[string]bool, len(types))
	for k, v := range types {
		out[k] = v
	}
	return out
}

var (
	nameRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{2,15}$`)
	idRe    = regexp.MustCompile(`^[a-z][a-z0-9_]*(:[a-z][a-z0-9_]*)?$`)
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	tokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	invRe   = regexp.MustCompile(`^[A-Z0-9-]{4,24}$`)
)

func ValidName(s string) bool { return nameRe.MatchString(s) }

func finite(fs ...float64) bool {
	for _, f := range fs {
		if math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) > 1e6 {
			return false
		}
	}
	return true
}

func validID(s string) bool { return len(s) <= 80 && idRe.MatchString(s) }

// Decode parses and validates a client message. Unknown fields are rejected
// so malformed or hostile payloads fail fast.
func Decode(b []byte) (*In, error) {
	if len(b) > MaxMessageBytes {
		return nil, fmt.Errorf("message too large (%d bytes)", len(b))
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var m In
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if dec.More() {
		return nil, errors.New("trailing data")
	}
	return &m, Validate(&m)
}

// Validate checks field shapes for a message's type. It does not check game
// rules (ownership, range, cooldowns) — those live in package game.
func Validate(m *In) error {
	needRid, ok := types[m.T]
	if !ok {
		return fmt.Errorf("unknown message type %q", m.T)
	}
	if needRid && m.Rid == 0 {
		return errors.New("missing rid")
	}
	if !finite(m.X, m.Y, m.Z, m.Yaw) {
		return errors.New("non-finite number")
	}
	bad := func(f string) error { return fmt.Errorf("%s: bad %s", m.T, f) }
	switch m.T {
	case "hello":
		if m.Proto < 1 || m.Proto > 1000 {
			return bad("proto")
		}
		if len(m.Client) > 40 {
			return bad("client")
		}
		if m.Token != "" && !tokenRe.MatchString(m.Token) {
			return bad("token")
		}
		if m.Token == "" && !ValidName(m.Name) {
			return bad("name")
		}
		if m.Invite != "" && !invRe.MatchString(m.Invite) {
			return bad("invite")
		}
		if m.Color != "" && !colorRe.MatchString(m.Color) {
			return bad("color")
		}
	case "gather", "place", "break", "light_fire", "set_trap", "plant":
		if m.BX < -MaxCoord || m.BX > MaxCoord || m.BY < 0 || m.BY > 255 || m.BZ < 0 || m.BZ > MaxCoord {
			return bad("block")
		}
		if (m.T == "place" || m.T == "light_fire" || m.T == "set_trap" || m.T == "plant") && !validID(m.Item) {
			return bad("item")
		}
	case "attack", "trade_request":
		if m.Target == 0 {
			return bad("target")
		}
	case "craft":
		if !validID(m.Recipe) {
			return bad("recipe")
		}
	case "cast":
		if !validID(m.Charm) {
			return bad("charm")
		}
	case "eat", "equip":
		if m.T == "equip" && m.Item == "" {
			return nil // unequip
		}
		if !validID(m.Item) {
			return bad("item")
		}
	case "drop":
		if !validID(m.Item) || m.Count < 1 || m.Count > 999 {
			return bad("item/count")
		}
	case "buy", "sell":
		if !validID(m.Item) || m.Count < 1 || m.Count > 64 {
			return bad("item/count")
		}
	case "talk", "steal", "bounty_take", "bounty_claim":
		if !validID(m.NPC) {
			return bad("npc")
		}
	case "quest_accept", "quest_turnin":
		if !validID(m.Quest) {
			return bad("quest")
		}
	case "trade_respond", "trade_lock", "trade_confirm", "trade_cancel":
		if m.Trade == 0 {
			return bad("trade")
		}
	case "trade_offer":
		if m.Trade == 0 || len(m.Offer) > MaxOfferEntries {
			return bad("offer")
		}
		for k, v := range m.Offer {
			if !validID(k) || v < 1 || v > 999 {
				return bad("offer entry")
			}
		}
	case "chat":
		t := strings.TrimSpace(m.Text)
		if t == "" || utf8.RuneCountInString(t) > MaxChatLen || !utf8.ValidString(t) {
			return bad("text")
		}
		for _, r := range t {
			if r < 0x20 || (r >= 0x7f && r < 0xa0) {
				return bad("text")
			}
		}
	case "chunks":
		if len(m.Chunks) == 0 || len(m.Chunks) > MaxChunkRequest {
			return bad("chunks")
		}
		for _, c := range m.Chunks {
			if c[0] < 0 || c[1] < 0 || c[0] > 0xffff || c[1] > 0xffff {
				return bad("chunk coords")
			}
		}
	}
	return nil
}

// Compatible reports whether a client protocol version is accepted.
func Compatible(v int) bool { return v >= ProtoMin && v <= ProtoMax }

// ChunkFrame builds a binary chunk frame: type, chunk x and z as
// little-endian uint16, then the run-length encoded blocks.
func ChunkFrame(cx, cz int, rle []byte) []byte {
	b := make([]byte, 0, 5+len(rle))
	b = append(b, BinChunk, byte(cx), byte(cx>>8), byte(cz), byte(cz>>8))
	return append(b, rle...)
}
