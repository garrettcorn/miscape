package game

import (
	"encoding/json"
	"testing"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
)

// A save from the original 128x128 map (version 1) loads into the Earth
// world: characters keep everything except their position, which moves to
// the spawn town; edits that referred to the old map are dropped.
func TestVersion1SaveMigrates(t *testing.T) {
	v1 := `{
	  "version": 1,
	  "realm": {"id": "r1", "kind": "private", "created": "2026-11-20T10:00:00Z"},
	  "characters": {"c1": {"id": "c1", "realm": "r1", "name": "Oldtimer", "tokenHash": "abc", "color": "#4a90d9",
	    "x": 64.5, "y": 13, "z": 70.5, "hp": 20, "inv": {"log": 9, "coin": 31, "wooden_sword": 1}, "equipped": "wooden_sword",
	    "xp": {"woodcutting": 120}, "placed": 3,
	    "quests": {"lights_for_wickhollow": {"status": "done", "progress": [10, 1, 3]},
	               "a_quest_that_was_removed": {"status": "active", "progress": [1]}}}},
	  "blocks": [[5000, 8], [6000, 0]],
	  "placed": {"5000": "c1"},
	  "regens": [{"i": 7000, "b": 6, "at": "2026-11-20T10:05:00Z"}],
	  "invites": {"MOSS-AAAA": {"code": "MOSS-AAAA", "label": "friends", "usesLeft": 2}},
	  "bans": {},
	  "drops": [{"item": "log", "count": 2, "x": 60, "y": 13, "z": 60, "owner": "c1"}],
	  "savedAt": "2026-11-20T10:06:00Z"
	}`
	var d SaveData
	if err := json.Unmarshal([]byte(v1), &d); err != nil {
		t.Fatal(err)
	}
	g, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), &d, newRec())
	if err != nil {
		t.Fatal(err)
	}
	c := g.Chars["c1"]
	sp := g.W.Spawn()
	if c == nil || c.X != sp.X || c.Z != sp.Z || c.Placed != 0 {
		t.Fatalf("character not moved to the spawn town: %+v", c)
	}
	if c.Inv["log"] != 9 || c.Inv["coin"] != 31 || c.XP["woodcutting"] != 120 || c.Equipped != "wooden_sword" {
		t.Fatalf("progress lost: %+v", c)
	}
	if c.Quests["lights_for_wickhollow"] == nil || c.Quests["a_quest_that_was_removed"] != nil {
		t.Fatalf("quests: %v", c.Quests)
	}
	if g.W.NumEdits() != 0 || len(g.placed) != 0 || len(g.regens) != 0 || len(g.Drops) != 0 {
		t.Fatal("old-map edits should not be applied")
	}
	if g.Invites["MOSS-AAAA"] == nil {
		t.Fatal("invites lost")
	}
	// It saves back as version 2 and loads again.
	snap := g.Snapshot()
	if snap.Version != SaveVersion || snap.Level != 1 {
		t.Fatalf("saved as version %d level %d", snap.Version, snap.Level)
	}
	if _, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), snap, newRec()); err != nil {
		t.Fatal(err)
	}
}
