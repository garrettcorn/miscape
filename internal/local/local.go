// Package local runs a single-player world inside the player's browser
// (compiled to WebAssembly and hosted in a Web Worker). It reuses the exact
// game rules, protocol and snapshot format of the Go server, but has no
// networking or file system: messages arrive as JSON strings and snapshots are
// handed to the host page for storage (IndexedDB).
//
// Browser worlds are their own realm kind ("browser"). Their saves can't be
// loaded into hosted worlds and hosted saves can't be imported here, so
// progress stays separate.
package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/game"
	"lanternfall/internal/protocol"
	"lanternfall/internal/store"
)

// Autosave interval for browser worlds (they also save on important events
// and when the page is hidden).
const Autosave = 20 * time.Second

// Out receives messages for the client: text (JSON) or binary (chunk frames).
type Out func(text []byte, bin []byte)

// Save receives a complete, checksummed snapshot to persist.
type Save func(snapshot []byte)

// Host owns one browser world and its single player.
type Host struct {
	mu       sync.Mutex
	g        *game.Game
	out      Out
	save     Save
	pid      uint32
	lastSave time.Time
	// Recovered reports that the newest save was unusable and an older one was loaded.
	Recovered bool
}

// WorldConfig is the fixed configuration of a browser world.
func WorldConfig() config.World {
	w := config.DefaultWorld(config.ModePrivate)
	w.Name = "My Browser World"
	w.Description = "A world that lives in this browser."
	w.MaxPlayers = 1
	w.Access = "open"
	w.Trading = "off" // nobody to trade with
	return w
}

type outbox struct{ h *Host }

func (o outbox) Send(pid uint32, msg any) {
	b, err := json.Marshal(msg)
	if err == nil {
		o.h.out(b, nil)
	}
}
func (o outbox) SendBin(pid uint32, b []byte) { o.h.out(nil, b) }
func (o outbox) Kick(pid uint32, code, msg string) {
	b, _ := json.Marshal(map[string]any{"t": "error", "code": code, "text": msg})
	o.h.out(b, nil)
}

// Validate checks that a snapshot is a readable browser-world save.
func Validate(snapshot []byte) error {
	var d game.SaveData
	if err := store.Decode(snapshot, &d); err != nil {
		return fmt.Errorf("this file isn't a readable Lanternfall save (%v)", err)
	}
	if d.Realm.Kind != config.ModeBrowser {
		return fmt.Errorf("this save belongs to a %q world; only browser-world saves can be imported here, so progress stays separate between worlds", d.Realm.Kind)
	}
	if d.Version != game.SaveVersion && d.Version != 1 { // version 1 saves are migrated on load
		return fmt.Errorf("this save uses version %d, which this game can't read", d.Version)
	}
	return nil
}

// Open starts a world from the newest usable snapshot in saves (newest
// first), or a fresh world if there are none. Corrupt snapshots are skipped.
func Open(saves [][]byte, out Out, save Save) (*Host, error) {
	h := &Host{out: out, save: save}
	c := content.MustBase()
	cfg := WorldConfig()
	c.RecomputeRules(cfg.Rules())
	var problems []string
	for i, s := range saves {
		if len(s) == 0 {
			continue
		}
		if err := Validate(s); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		var d game.SaveData
		store.Decode(s, &d)
		g, err := game.Load(c, cfg, &d, outbox{h})
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		h.g = g
		h.Recovered = i > 0
		break
	}
	if h.g == nil {
		if len(problems) > 0 && allNonEmpty(saves) {
			return nil, errors.New("no usable save found: " + problems[0])
		}
		h.g = game.New(c, cfg, config.ModeBrowser, outbox{h})
	}
	h.lastSave = time.Now()
	return h, nil
}

func allNonEmpty(s [][]byte) bool {
	n := 0
	for _, b := range s {
		if len(b) > 0 {
			n++
		}
	}
	return n > 0
}

// Info mirrors the server's /api/info for the join screen.
func (h *Host) Info() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, _ := json.Marshal(map[string]any{
		"game": "lanternfall", "proto": []int{protocol.ProtoMin, protocol.ProtoMax},
		"realm": h.g.Realm.ID, "kind": h.g.Realm.Kind, "name": h.g.Cfg.Name, "desc": h.g.Cfg.Description,
		"modded": false, "players": len(h.g.Players), "max": 1, "access": "open", "pvp": false, "trading": false,
		"recovered": h.Recovered, "characters": h.g.CharacterNames(),
	})
	return b
}

// Hello joins (or rejoins) the single local player. Errors are sent to the
// client as {"t":"error"} messages, exactly like the network server.
func (h *Host) Hello(text []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, err := protocol.Decode(text)
	if err != nil || m.T != "hello" {
		outbox{h}.Kick(0, protocol.ErrNeedHello, "Expected a hello message.")
		return
	}
	if !protocol.Compatible(m.Proto) {
		outbox{h}.Kick(0, protocol.ErrIncompatible, "This page's game files are out of date. Reload the page.")
		return
	}
	if h.pid != 0 && h.g.Players[h.pid] != nil {
		h.g.Leave(h.pid)
	}
	// Single-player: the local player owns every character in this save, so
	// "continue as <name>" works even without the browser's stored token
	// (e.g. after importing the world on another device).
	reissued := ""
	if m.Token == "" {
		if tok, ok := h.g.ReissueToken(m.Name); ok {
			m.Token, m.Name, reissued = tok, "", tok
		}
	}
	pid, token, err := h.g.Join(game.Session{Local: true}, m)
	if token == "" {
		token = reissued
	}
	if err != nil {
		var je *game.JoinError
		if errors.As(err, &je) {
			outbox{h}.Kick(0, je.Code, je.Msg)
		} else {
			outbox{h}.Kick(0, protocol.ErrServer, "Could not join.")
		}
		return
	}
	h.pid = pid
	outbox{h}.Send(pid, h.g.Welcome(pid, token))
	h.saveNow() // make the new character durable right away
}

// Message handles one client message after hello.
func (h *Host) Message(text []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pid == 0 {
		return
	}
	m, err := protocol.Decode(text)
	if err != nil {
		b, _ := json.Marshal(map[string]any{"t": "ack", "rid": 0, "ok": false, "err": "invalid message"})
		h.out(b, nil)
		return
	}
	if m.T == "hello" {
		return
	}
	h.g.Handle(h.pid, m)
}

// Tick advances the world and autosaves when due.
func (h *Host) Tick(now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.g.Tick(now)
	if h.g.SaveSoon || (h.g.Dirty && now.Sub(h.lastSave) >= Autosave) {
		h.saveNow()
	}
}

// Flush saves immediately (e.g. when the page is hidden or closing).
func (h *Host) Flush() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.saveNow()
}

// Export returns the current snapshot for downloading.
func (h *Host) Export() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, _ := store.Encode(h.g.Snapshot(), time.Now())
	return b
}

func (h *Host) saveNow() {
	b, err := store.Encode(h.g.Snapshot(), time.Now())
	if err != nil {
		return
	}
	h.g.SaveSoon, h.g.Dirty = false, false
	h.lastSave = time.Now()
	h.save(b)
}
