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
	f.g.addXP(p, "mining", 1_000_000)
	if p.Char.XP["mining"] != XPForLevel(10) || p.Char.Level("mining") != 10 {
		t.Fatalf("XP should stop at the cap: %d xp, level %d", p.Char.XP["mining"], p.Char.Level("mining"))
	}
	before := len(f.out.msgs[p.ID])
	f.g.addXP(p, "mining", 100)
	if len(f.out.msgs[p.ID]) != before {
		t.Fatal("no XP messages once at the cap")
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
