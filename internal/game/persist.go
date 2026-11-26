package game

import (
	"fmt"
	"sort"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/world"
)

// SaveVersion 2 stores block edits by position on a world level. Version 1
// (the original 128x128 map) is migrated on load: see migrateV1.
const SaveVersion = 2

// SaveData is the full persistent state of one world (one realm).
type SaveData struct {
	Version    int                   `json:"version"`
	Realm      Realm                 `json:"realm"`
	Level      int                   `json:"level"`
	Characters map[string]*Character `json:"characters"`
	Edits      [][4]int              `json:"edits"`        // [x, y, z, block] changes to the generated world
	Placed     []PlacedBlock         `json:"placedBlocks"` // who placed each player-built block
	Regrow     []regen               `json:"regrow"`
	Invites    map[string]*Invite    `json:"invites"`
	Bans       map[string]*Ban       `json:"bans"`
	Drops      []*Drop               `json:"drops,omitempty"`
	SavedAt    time.Time             `json:"savedAt"`

	// Version 1 fields, read only so old saves can be recognised; their
	// contents referred to the old map and are discarded.
	V1Blocks [][2]int `json:"blocks,omitempty"`
}

// PlacedBlock records the character that placed a block (only they may remove it).
type PlacedBlock struct {
	X    int32  `json:"x"`
	Y    int32  `json:"y"`
	Z    int32  `json:"z"`
	Char string `json:"char"`
}

// Snapshot captures a consistent copy of state. Because it runs on the loop
// goroutine, it can never observe half of a trade or craft. Its cost grows
// with characters and edits, not with the size of the world.
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
	placed := make([]PlacedBlock, 0, len(g.placed))
	for k, v := range g.placed {
		placed = append(placed, PlacedBlock{k.X, k.Y, k.Z, v})
	}
	sort.Slice(placed, func(i, j int) bool {
		a, b := placed[i], placed[j]
		return a.Z < b.Z || a.Z == b.Z && (a.X < b.X || a.X == b.X && a.Y < b.Y)
	})
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
	drops := make([]*Drop, 0, len(g.Drops))
	for _, d := range g.Drops {
		cp := *d
		drops = append(drops, &cp)
	}
	return &SaveData{
		Version: SaveVersion, Realm: g.Realm, Level: g.W.Def.Level, Characters: chars, Edits: g.W.Edits(), Placed: placed,
		Regrow: append([]regen(nil), g.regens...), Invites: invites, Bans: bans, Drops: drops, SavedAt: g.Now,
	}
}

// Load builds a game from saved data. Characters from another realm are
// refused, so progress can never cross between worlds.
func Load(c *content.Content, cfg config.World, d *SaveData, out Outbox) (*Game, error) {
	if d.Version != SaveVersion && d.Version != 1 {
		return nil, fmt.Errorf("unsupported save version %d", d.Version)
	}
	if d.Realm.ID == "" {
		return nil, fmt.Errorf("save has no realm id")
	}
	g := New(c, cfg, d.Realm.Kind, out)
	if d.Version == SaveVersion && d.Level != g.W.Def.Level {
		return nil, fmt.Errorf("this save is from level %d of the world, but the server runs level %d", d.Level, g.W.Def.Level)
	}
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
	if d.Version == 1 {
		g.migrateV1()
	} else {
		for _, e := range d.Edits {
			if e[3] >= 0 && e[3] < world.NumBlocks {
				g.W.Set(e[0], e[1], e[2], uint8(e[3]))
			}
		}
		for _, pb := range d.Placed {
			g.placed[g.W.PosAt(int(pb.X), int(pb.Y), int(pb.Z))] = pb.Char
		}
		g.regens = d.Regrow
		for _, dr := range d.Drops {
			if dr == nil || dr.Count < 1 || dr.Item == "" {
				continue
			}
			cp := *dr
			cp.ID = g.allocID()
			g.Drops[cp.ID] = &cp
		}
	}
	if d.Invites != nil {
		g.Invites = d.Invites
	}
	if d.Bans != nil {
		g.Bans = d.Bans
	}
	return g, nil
}

// migrateV1 brings characters from the original small map into this world.
// They keep their inventories, skills and quests and start at the spawn
// town; block edits, regrowth timers and ground drops referred to the old
// map, so they were not loaded. Quest progress for quests that no longer
// exist is dropped.
func (g *Game) migrateV1() {
	sp := g.W.Spawn()
	for _, c := range g.Chars {
		c.X, c.Y, c.Z, c.Yaw = sp.X, sp.Y, sp.Z, 0
		c.Placed = 0
		for q := range c.Quests {
			if g.C.Quests[q] == nil {
				delete(c.Quests, q)
			}
		}
	}
	g.SaveSoon = true
}
