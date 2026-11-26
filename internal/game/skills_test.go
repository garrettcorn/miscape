package game

import (
	"strings"
	"testing"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
)

func TestFourteenSkillsInGroups(t *testing.T) {
	c := content.MustBase()
	want := []string{"melee", "range", "magic", "slaying", "mining", "woodcutting", "fishing", "hunting", "farming",
		"crafting", "cooking", "firemaking", "agility", "thieving"}
	if len(c.Skills) != len(want) {
		t.Fatalf("%d skills", len(c.Skills))
	}
	for i, id := range want {
		if c.Skills[i].ID != id || c.Skills[i].Name == "" || c.Skills[i].Icon == "" {
			t.Errorf("skill %d: %+v, want %s", i, c.Skills[i], id)
		}
	}
	// Every station a recipe can use has a block in the world.
	for _, s := range content.Stations {
		if _, ok := stationBlocks[s]; !ok || stationName(s) == "" {
			t.Errorf("station %q has no block or name", s)
		}
	}
}

func TestLevelCapFollowsWorldLevel(t *testing.T) {
	f := newFixture(t)
	if f.g.LevelCap() != 10 {
		t.Fatalf("level 1 world cap %d", f.g.LevelCap())
	}
	p := f.join("Capper")
	capAt := XPForLevel(10)
	// Reaching the cap: full XP up to it, half of the rest.
	f.g.addXP(p, "mining", capAt+100)
	if p.Char.XP["mining"] != capAt+50 || p.Char.Level("mining") != 10 {
		t.Fatalf("past the cap: %d xp (want %d), level %d", p.Char.XP["mining"], capAt+50, p.Char.Level("mining"))
	}
	// At the cap, XP keeps coming at half rate, with a popup, and the level stays.
	f.g.addXP(p, "mining", 40)
	if p.Char.XP["mining"] != capAt+70 || p.Char.Level("mining") != 10 {
		t.Fatalf("at the cap: %d xp, level %d", p.Char.XP["mining"], p.Char.Level("mining"))
	}
	if m := f.out.last(p.ID, "xp"); m == nil || m["amount"] != 20 {
		t.Fatalf("xp popup at the cap: %v", m)
	}
	if lu := f.out.last(p.ID, "levelup"); lu != nil && lu["level"] != 10 {
		t.Fatalf("no level past the cap: %v", lu)
	}
	// Small awards still count (half, rounded up).
	f.g.addXP(p, "mining", 1)
	if p.Char.XP["mining"] != capAt+71 {
		t.Fatalf("1 xp at the cap: %d", p.Char.XP["mining"])
	}
	if m := f.g.meMsg(p); m["cap"] != 10 {
		t.Fatalf("me cap %v", m["cap"])
	}
	// A level-2 world allows level 20.
	f.g.W.Def.Level = 2
	defer func() { f.g.W.Def.Level = 1 }()
	c2 := &Character{XP: map[string]int{"mining": XPForLevel(25)}}
	c2.levelCap = f.g.LevelCap()
	if f.g.LevelCap() != 20 || c2.Level("mining") != 20 {
		t.Fatalf("level 2 cap %d, level %d", f.g.LevelCap(), c2.Level("mining"))
	}
	// XP banked past the level 1 cap turns into levels in a level 2 world.
	p.Char.XP["mining"] = XPForLevel(12)
	p.Char.levelCap = f.g.LevelCap()
	if p.Char.Level("mining") != 12 {
		t.Fatalf("banked xp in a level 2 world: level %d", p.Char.Level("mining"))
	}
}

func TestQuestsGiveAdventurePoints(t *testing.T) {
	c := content.MustBase()
	for id, q := range c.Quests {
		if q.Reward.AP < 1 {
			t.Errorf("quest %s gives no adventure points", id)
		}
	}
	// Some quests need points, and doing the quests you can always unlocks
	// the rest eventually, so nobody is ever stuck.
	gated := 0
	for _, q := range c.Quests {
		if q.NeedsAP > 0 {
			gated++
		}
	}
	if gated == 0 {
		t.Fatal("no quest needs adventure points")
	}
	ap, done := 0, map[string]bool{}
	for progress := true; progress; {
		progress = false
		for id, q := range c.Quests {
			if !done[id] && ap >= q.NeedsAP && (q.Requires == "" || done[q.Requires]) {
				done[id], ap, progress = true, ap+q.Reward.AP, true
			}
		}
	}
	for id, q := range c.Quests {
		if !done[id] {
			t.Errorf("quest %s (needs %d points) can never be reached; all reachable quests give %d", id, q.NeedsAP, ap)
		}
	}
}

// Characters from before adventure points existed get them for the quests
// they had already finished.
func TestOldSavesGetAdventurePoints(t *testing.T) {
	f := newFixture(t)
	p := f.join("Veteran")
	p.Char.Quests["lights_for_wickhollow"] = &QuestState{Status: "done", Progress: []int{4, 1, 3}}
	p.Char.Quests["salt_and_rowan"] = &QuestState{Status: "active", Progress: []int{0, 0}}
	snap := f.g.Snapshot()
	snap.Version = 3
	snap.Characters[p.Char.ID].AP = 0
	g2, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), snap, newRec())
	if err != nil {
		t.Fatal(err)
	}
	if ap := g2.Chars[p.Char.ID].AP; ap != 2 {
		t.Fatalf("migrated adventure points: %d, want 2", ap)
	}
}

func TestRequirementsAreEnforced(t *testing.T) {
	f := newFixture(t)
	p := f.join("Novice")
	// A recipe gated by another skill.
	f.g.C.Recipes["planks"].Needs = map[string]int{"woodcutting": 3}
	p.Char.Inv["log"] = 1
	if e := f.mustFail(p, protocol.In{T: "craft", Recipe: "planks"}); !strings.Contains(e, "Needs Woodcutting 3 (you have 1)") {
		t.Fatalf("recipe needs: %s", e)
	}
	p.Char.XP["woodcutting"] = XPForLevel(3)
	f.mustOK(p, protocol.In{T: "craft", Recipe: "planks"})
	// Equipment.
	f.g.C.Items["copper_sword"].Needs = map[string]int{"melee": 5}
	p.Char.Inv["copper_sword"] = 1
	if e := f.mustFail(p, protocol.In{T: "equip", Item: "copper_sword"}); !strings.Contains(e, "Melee 5") {
		t.Fatalf("equip needs: %s", e)
	}
	// Quests.
	f.g.C.Quests["lights_for_wickhollow"].Needs = map[string]int{"agility": 2}
	f.atNPC(p, "mabel")
	if e := f.mustFail(p, protocol.In{T: "quest_accept", Quest: "lights_for_wickhollow"}); !strings.Contains(e, "Agility 2") {
		t.Fatalf("quest needs: %s", e)
	}
}

func TestWeaponStyleTrainsItsSkill(t *testing.T) {
	f := newFixture(t)
	p := f.join("Styler")
	var m *Mob
	for _, mm := range f.g.Mobs {
		if mm.Def.ID == "bramblet" {
			m = mm
			break
		}
	}
	// A pretend magic weapon: kills train magic, and magic level adds damage.
	staff := *f.g.C.Items["wooden_sword"]
	staff.ID, staff.Style = "test_staff", "magic"
	f.g.C.Items["test_staff"] = &staff
	p.Char.Inv["test_staff"] = 1
	p.Char.Equipped = "test_staff"
	p.Char.XP["magic"] = XPForLevel(6)
	if d := f.g.PlayerDamage(p.Char); d != staff.Damage+3 {
		t.Fatalf("damage %d, want %d", d, staff.Damage+3)
	}
	f.g.killMob(m, p)
	if p.Char.XP["magic"] != XPForLevel(6)+m.Def.XP || p.Char.XP["melee"] != 0 {
		t.Fatalf("xp went to the wrong skill: %v", p.Char.XP)
	}
	if m := f.out.last(p.ID, "xp"); m == nil || m["skill"] != "magic" {
		t.Fatalf("no XP popup message: %v", m)
	}
}

func TestLevelUpNamesUnlocks(t *testing.T) {
	f := newFixture(t)
	p := f.join("Learner")
	f.g.addXP(p, "crafting", XPForLevel(3))
	lu := f.out.last(p.ID, "levelup")
	if lu == nil || lu["level"] != 3 {
		t.Fatalf("levelup %v", lu)
	}
	if u := lu["unlocks"].([]string); len(u) == 0 || !strings.Contains(strings.Join(u, ","), "Copper Sword") {
		t.Fatalf("crafting 3 should unlock the copper sword: %v", u)
	}
}

func TestModRecipesMustUseKnownSkills(t *testing.T) {
	c := content.MustBase()
	m := &content.Manifest{ID: "x", Name: "X", Version: "1", API: 1, Content: content.ModContent{
		Recipes: []content.Recipe{{ID: "x:juggle", Output: "plank", Count: 1, Inputs: map[string]int{"log": 1}, Skill: "juggling", Level: 1, XP: 1}},
	}}
	if err := c.Apply(m); err == nil || !strings.Contains(err.Error(), "unknown skill") {
		t.Fatalf("recipe with an unknown skill accepted: %v", err)
	}
	_ = config.ModePrivate
}
