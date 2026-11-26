package game

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"lanternfall/internal/content"
	"lanternfall/internal/world"
)

// The browser client keeps its own copies of a few tables (block ids, which
// blocks are solid, movement constants, drawable models). The server is
// authoritative, so if they drift the client mispredicts and gets snapped
// back. These tests read the client source and fail on any mismatch.

func readClient(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../web/js/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestClientBlockTableMatchesServer(t *testing.T) {
	src := readClient(t, "blocks.js")
	obj := regexp.MustCompile(`(?s)export const B = \{(.*?)\};`).FindStringSubmatch(src)
	if obj == nil {
		t.Fatal("blocks.js: B table not found")
	}
	ids := map[string]int{}
	for _, m := range regexp.MustCompile(`([A-Z_]+):\s*(\d+)`).FindAllStringSubmatch(obj[1], -1) {
		n, _ := strconv.Atoi(m[2])
		ids[m[1]] = n
	}
	if len(ids) != world.NumBlocks {
		t.Fatalf("client has %d block ids, server has %d", len(ids), world.NumBlocks)
	}
	seen := map[int]string{}
	for name, id := range ids {
		if id < 0 || id >= world.NumBlocks {
			t.Errorf("client block %s = %d is out of range", name, id)
		}
		if other, dup := seen[id]; dup {
			t.Errorf("client blocks %s and %s share id %d", name, other, id)
		}
		seen[id] = name
	}
	// Solidity: def(B.X, ..., { ... solid: false ... }) marks non-solid blocks.
	nonSolid := map[int]bool{}
	for _, m := range regexp.MustCompile(`def\(B\.([A-Z_]+),[^\n]*solid: false`).FindAllStringSubmatch(src, -1) {
		nonSolid[ids[m[1]]] = true
	}
	for id := 0; id < world.NumBlocks; id++ {
		if world.Solid(uint8(id)) == nonSolid[id] {
			t.Errorf("block %d (%s): server solid=%v, client solid=%v", id, seen[id], world.Solid(uint8(id)), !nonSolid[id])
		}
	}
}

func TestClientMovementConstantsMatchServer(t *testing.T) {
	src := readClient(t, "physics.js")
	want := map[string]float64{"WALK": WalkSpeed, "SPRINT": SprintSpeed, "HALF": BodyHalf, "HEIGHT": BodyHeight}
	for name, v := range want {
		m := regexp.MustCompile(`\b` + name + `\s*=\s*([0-9.]+)`).FindStringSubmatch(src)
		if m == nil {
			t.Errorf("physics.js: %s not found", name)
			continue
		}
		if got, _ := strconv.ParseFloat(m[1], 64); got != v {
			t.Errorf("%s: client %v, server %v", name, got, v)
		}
	}
}

func TestClientCanDrawEveryContentModel(t *testing.T) {
	src := readClient(t, "models.js")
	section := func(start string) string {
		i := strings.Index(src, start)
		if i < 0 {
			t.Fatalf("models.js: %q not found", start)
		}
		end := strings.Index(src[i:], "\n};")
		return src[i : i+end]
	}
	items, mobs := section("export const ITEM_MODELS = {"), section("export const MOB_MODELS = {")
	for m := range content.Models {
		if !regexp.MustCompile(`\n  ` + m + `: `).MatchString(items) {
			t.Errorf("item model %q has no drawing in ITEM_MODELS", m)
		}
	}
	for m := range content.MobModels {
		if !regexp.MustCompile(`\n  ` + m + `: `).MatchString(mobs) {
			t.Errorf("creature model %q has no drawing in MOB_MODELS", m)
		}
	}
	icons := readClient(t, "icons.js")
	for m := range content.Models {
		if !regexp.MustCompile(`\n  ` + m + `: \(`).MatchString(icons) {
			t.Errorf("item model %q has no icon in icons.js", m)
		}
	}
}
