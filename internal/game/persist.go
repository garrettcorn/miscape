package game

import (
	"fmt"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/world"
)

const SaveVersion = 1

// SaveData is the full persistent state of one world (one realm).
type SaveData struct {
	Version    int                   `json:"version"`
	Realm      Realm                 `json:"realm"`
	Characters map[string]*Character `json:"characters"`
	Blocks     [][2]int              `json:"blocks"` // [index, block] diffs vs the curated build
	Placed     map[int]string        `json:"placed"`
	Regens     []regen               `json:"regens"`
	Invites    map[string]*Invite    `json:"invites"`
	Bans       map[string]*Ban       `json:"bans"`
	SavedAt    time.Time             `json:"savedAt"`
}

// Snapshot captures a consistent copy of state. Because it runs on the loop
// goroutine, it can never observe half of a trade or craft.
func (g *Game) Snapshot() *SaveData {
	// Persist online characters' live state (positions etc.) — they are the same pointers.
	chars := make(map[string]*Character, len(g.Chars))
	for id, c := range g.Chars {
		cp := *c
		cp.Inv = cloneInv(c.Inv)
		cp.XP = map[string]int{}
		for k, v := range c.XP {
			cp.XP[k] = v
		}
		cp.Quests = map[string]*QuestState{}
		for k, v := range c.Quests {
			q := *v
			q.Progress = append([]int(nil), v.Progress...)
			cp.Quests[k] = &q
		}
		chars[id] = &cp
	}
	var diffs [][2]int
	for i, b := range g.W.Blocks {
		if b != g.base.Blocks[i] {
			diffs = append(diffs, [2]int{i, int(b)})
		}
	}
	placed := make(map[int]string, len(g.placed))
	for k, v := range g.placed {
		placed[k] = v
	}
	invites := map[string]*Invite{}
	for k, v := range g.Invites {
		cp := *v
		invites[k] = &cp
	}
	bans := map[string]*Ban{}
	for k, v := range g.Bans {
		cp := *v
		bans[k] = &cp
	}
	return &SaveData{
		Version: SaveVersion, Realm: g.Realm, Characters: chars, Blocks: diffs, Placed: placed,
		Regens: append([]regen(nil), g.regens...), Invites: invites, Bans: bans, SavedAt: g.Now,
	}
}

// Load builds a game from saved data. Characters from another realm are
// refused, so progress can never cross between worlds.
func Load(c *content.Content, cfg config.World, d *SaveData, out Outbox) (*Game, error) {
	if d.Version != SaveVersion {
		return nil, fmt.Errorf("unsupported save version %d", d.Version)
	}
	if d.Realm.ID == "" {
		return nil, fmt.Errorf("save has no realm id")
	}
	g := New(c, cfg, d.Realm.Kind, out)
	g.Realm = d.Realm
	g.noteMods()
	for id, ch := range d.Characters {
		if ch.Realm != d.Realm.ID {
			return nil, fmt.Errorf("character %s belongs to realm %s, not %s", ch.Name, ch.Realm, d.Realm.ID)
		}
		if ch.Inv == nil {
			ch.Inv = map[string]int{}
		}
		if ch.XP == nil {
			ch.XP = map[string]int{}
		}
		if ch.Quests == nil {
			ch.Quests = map[string]*QuestState{}
		}
		ch.ID = id
		g.addChar(ch)
	}
	for _, bd := range d.Blocks {
		if bd[0] >= 0 && bd[0] < len(g.W.Blocks) && bd[1] >= 0 && bd[1] < world.NumBlocks {
			g.W.Blocks[bd[0]] = uint8(bd[1])
		}
	}
	for k, v := range d.Placed {
		g.placed[k] = v
	}
	g.regens = d.Regens
	if d.Invites != nil {
		g.Invites = d.Invites
	}
	if d.Bans != nil {
		g.Bans = d.Bans
	}
	return g, nil
}
