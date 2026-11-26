package local

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/game"
	"lanternfall/internal/store"
)

type sink struct {
	texts []map[string]any
	bins  int
	saves [][]byte
}

func (s *sink) out(text, bin []byte) {
	if bin != nil {
		s.bins++
		return
	}
	var m map[string]any
	json.Unmarshal(text, &m)
	s.texts = append(s.texts, m)
}
func (s *sink) save(b []byte) { s.saves = append(s.saves, append([]byte(nil), b...)) }
func (s *sink) last(t string) map[string]any {
	for i := len(s.texts) - 1; i >= 0; i-- {
		if s.texts[i]["t"] == t {
			return s.texts[i]
		}
	}
	return nil
}

func open(t *testing.T, saves ...[]byte) (*Host, *sink) {
	t.Helper()
	s := &sink{}
	h, err := Open(saves, s.out, s.save)
	if err != nil {
		t.Fatal(err)
	}
	return h, s
}

func TestFreshWorldJoinPlayAndReload(t *testing.T) {
	h, s := open(t)
	var info map[string]any
	json.Unmarshal(h.Info(), &info)
	if info["kind"] != config.ModeBrowser {
		t.Fatalf("kind: %v", info["kind"])
	}
	h.Hello([]byte(`{"t":"hello","proto":2,"name":"Solo","color":"#4a90d9"}`))
	w := s.last("welcome")
	if w == nil || w["token"] == "" {
		t.Fatalf("no welcome: %v", s.texts)
	}
	if len(s.saves) == 0 {
		t.Fatal("new character should be saved immediately")
	}
	token := w["token"].(string)
	// Play: request a chunk and craft (needs a log).
	h.Message([]byte(`{"t":"chunks","chunks":[[4,4]]}`))
	if s.bins != 1 {
		t.Fatal("chunk not sent")
	}
	h.mu.Lock()
	for _, p := range h.g.Players {
		p.Char.Inv["log"] = 1
	}
	h.mu.Unlock()
	h.Message([]byte(`{"t":"craft","rid":5,"recipe":"planks"}`))
	if a := s.last("ack"); a["ok"] != true {
		t.Fatalf("craft failed: %v", a)
	}
	h.Tick(time.Now())
	h.Flush()
	snap := s.saves[len(s.saves)-1]

	// "Reload the page": a new host from the saved snapshot, same token.
	h2, s2 := open(t, snap)
	h2.Hello([]byte(`{"t":"hello","proto":2,"token":"` + token + `"}`))
	if s2.last("welcome") == nil {
		t.Fatalf("rejoin failed: %v", s2.texts)
	}
	h2.Tick(time.Now())
	if me := s2.last("me"); me == nil || me["inv"].(map[string]any)["plank"] != float64(4) {
		t.Fatalf("progress not restored: %v", me)
	}
}

func TestCorruptNewestFallsBackToBackup(t *testing.T) {
	h, s := open(t)
	h.Hello([]byte(`{"t":"hello","proto":2,"name":"Backup"}`))
	good := s.saves[len(s.saves)-1]
	h2, _ := open(t, []byte(`{"format":1,"sha256":"00","payload":{}}`), good)
	if !h2.Recovered || len(h2.g.Chars) != 1 {
		t.Fatal("should recover from the backup")
	}
	if _, err := Open([][]byte{[]byte("garbage")}, (&sink{}).out, (&sink{}).save); err == nil {
		t.Fatal("all-corrupt saves must be an error, not a silent fresh world")
	}
}

func TestOnlyBrowserSavesCanBeImported(t *testing.T) {
	// A save from a player-hosted world must be refused (realm separation).
	g := game.New(content.MustBase(), config.DefaultWorld(config.ModePrivate), config.ModePrivate, nil)
	b, _ := store.Encode(g.Snapshot(), time.Now())
	if err := Validate(b); err == nil || !strings.Contains(err.Error(), "only browser-world saves") {
		t.Fatalf("hosted-world save accepted: %v", err)
	}
	if err := Validate([]byte("not json")); err == nil {
		t.Fatal("garbage accepted")
	}
	h, _ := open(t)
	if err := Validate(h.Export()); err != nil {
		t.Fatalf("own export rejected: %v", err)
	}
}

func TestRulesStillEnforced(t *testing.T) {
	h, s := open(t)
	h.Hello([]byte(`{"t":"hello","proto":2,"name":"Cheater"}`))
	h.Message([]byte(`{"t":"craft","rid":1,"recipe":"glimmer_blade"}`))
	if a := s.last("ack"); a["ok"] != false {
		t.Fatal("craft without materials accepted")
	}
	h.Message([]byte(`{"t":"move","x":120,"y":13,"z":120}`))
	if s.last("correct") == nil {
		t.Fatal("teleport accepted")
	}
	h.Hello([]byte(`{"t":"hello","proto":99,"name":"Future"}`))
	if e := s.last("error"); e == nil || e["code"] != "incompatible_version" {
		t.Fatal("version check missing")
	}
}

func TestImportedWorldResumesByName(t *testing.T) {
	// Device A creates a character and exports the world.
	h, s := open(t)
	h.Hello([]byte(`{"t":"hello","proto":2,"name":"Traveler"}`))
	export := h.Export()
	// Device B imports it: no stored token, so it continues by name.
	h2, s2 := open(t, export)
	var info map[string]any
	json.Unmarshal(h2.Info(), &info)
	if names := info["characters"].([]any); len(names) != 1 || names[0] != "Traveler" {
		t.Fatalf("characters: %v", info["characters"])
	}
	h2.Hello([]byte(`{"t":"hello","proto":2,"name":"Traveler"}`))
	w := s2.last("welcome")
	if w == nil || w["token"] == "" {
		t.Fatalf("resume by name failed: %v", s2.texts)
	}
	if len(h2.g.Chars) != 1 {
		t.Fatal("must resume the existing character, not create a duplicate")
	}
	_ = s
}
