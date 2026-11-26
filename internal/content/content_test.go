package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseContentValid(t *testing.T) {
	c, err := Base()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Items) < 10 || len(c.Recipes) < 5 || len(c.Quests) < 2 || len(c.Mobs) != 2 {
		t.Fatalf("unexpected content counts: %d items %d recipes %d quests %d mobs", len(c.Items), len(c.Recipes), len(c.Quests), len(c.Mobs))
	}
	if c.Hash == "" {
		t.Fatal("hash")
	}
}

const goodMod = `{
 "id": "tea", "name": "Tea Time", "version": "1.2.0", "api": 1, "description": "Adds tea.",
 "content": {
   "items": [{"id": "tea:leaf", "name": "Tea Leaf", "kind": "material", "tradeable": true, "stack": 32, "color": "#77aa55"},
             {"id": "tea:cup", "name": "Cup of Tea", "kind": "food", "heal": 6, "tradeable": true, "stack": 8, "color": "#aa7744"}],
   "recipes": [{"id": "tea:brew", "output": "tea:cup", "count": 1, "inputs": {"tea:leaf": 2, "plank": 1}, "level": 1, "xp": 5}],
   "quests": [], "shop": [{"npc": "bertram", "item": "tea:leaf", "price": 1}]
 },
 "rules": {"xpMultiplier": 1.5}
}`

func TestApplyMod(t *testing.T) {
	c := MustBase()
	h := c.Hash
	m, err := ParseManifest([]byte(goodMod))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Apply(m); err != nil {
		t.Fatal(err)
	}
	if c.Items["tea:cup"] == nil || c.Recipes["tea:brew"] == nil || c.Rules.XPMultiplier != 1.5 {
		t.Fatal("mod not applied")
	}
	if c.Hash == h || len(c.Mods) != 1 || c.Mods[0] != "tea@1.2.0" {
		t.Fatal("hash/mod list not updated")
	}
	c.RecomputeRules(DefaultRules())
	if c.Rules.XPMultiplier != 1.5 {
		t.Fatal("recompute must keep mod factors")
	}
}

func TestRejectsBadMods(t *testing.T) {
	bad := map[string]string{
		"wrong api":       strings.Replace(goodMod, `"api": 1`, `"api": 2`, 1),
		"no namespace":    strings.Replace(goodMod, `"tea:leaf", "name": "Tea Leaf"`, `"leaf", "name": "Tea Leaf"`, 1),
		"other namespace": strings.Replace(goodMod, `"id": "tea:brew"`, `"id": "core:brew"`, 1),
		"unknown field":   strings.Replace(goodMod, `"api": 1`, `"api": 1, "script": "rm -rf /"`, 1),
		"bad version":     strings.Replace(goodMod, `"1.2.0"`, `"latest"`, 1),
		"bad id":          strings.Replace(goodMod, `"id": "tea", "name"`, `"id": "../tea", "name"`, 1),
		"adds block":      strings.Replace(goodMod, `"kind": "material"`, `"kind": "block", "block": 3`, 1),
	}
	for name, s := range bad {
		if _, err := ParseManifest([]byte(s)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// Valid shape but broken references fail at Apply, leaving content untouched.
	c := MustBase()
	before := len(c.Items)
	m, _ := ParseManifest([]byte(strings.Replace(goodMod, `"plank": 1`, `"unobtainium": 1`, 1)))
	if err := c.Apply(m); err == nil {
		t.Error("dangling reference accepted")
	}
	if len(c.Items) != before || len(c.Mods) != 0 {
		t.Error("failed apply mutated content")
	}
	// Absurd rule factors are clamped.
	m2, _ := ParseManifest([]byte(strings.Replace(goodMod, `"xpMultiplier": 1.5`, `"xpMultiplier": 1000`, 1)))
	c2 := MustBase()
	c2.Apply(m2)
	if c2.Rules.XPMultiplier > 5 {
		t.Error("rules not clamped")
	}
}

func writeMod(t *testing.T, dir, name, body string) {
	os.MkdirAll(filepath.Join(dir, name), 0o755)
	if err := os.WriteFile(filepath.Join(dir, name, "mod.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadModsReportsAndSkips(t *testing.T) {
	dir := t.TempDir()
	writeMod(t, dir, "tea", goodMod)
	writeMod(t, dir, "broken", `{"id":`)
	writeMod(t, dir, "future", strings.Replace(strings.Replace(goodMod, `"api": 1`, `"api": 7`, 1), `"id": "tea"`, `"id": "future"`, 1))
	os.MkdirAll(filepath.Join(dir, "empty"), 0o755)

	c := MustBase()
	st := c.LoadMods(dir, []string{"tea", "future"}, true)
	byDir := map[string]ModStatus{}
	for _, s := range st {
		byDir[s.Dir] = s
	}
	if !byDir["tea"].Loaded || byDir["broken"].Error == "" || !strings.Contains(byDir["future"].Error, "API 7") || byDir["empty"].Error == "" {
		t.Fatalf("statuses: %+v", st)
	}
	if c.Items["tea:cup"] == nil {
		t.Fatal("good mod should load despite broken neighbours")
	}
	// Disallowed: nothing applied.
	c2 := MustBase()
	st2 := c2.LoadMods(dir, []string{"tea"}, false)
	for _, s := range st2 {
		if s.Loaded {
			t.Fatal("mods loaded while disallowed")
		}
	}
	// Not enabled: discovered but not loaded.
	c3 := MustBase()
	for _, s := range c3.LoadMods(dir, nil, true) {
		if s.Loaded {
			t.Fatal("unenabled mod loaded")
		}
	}
}

func TestExampleModInRepoIsValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "mods", "example-sweetberries", "mod.json"))
	if err != nil {
		t.Skip("example mod not present")
	}
	m, err := ParseManifest(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := MustBase().Apply(m); err != nil {
		t.Fatal(err)
	}
}
