package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ModAPI is the declarative mod API version this server understands.
const ModAPI = 1

const (
	maxModFile    = 256 << 10
	maxModEntries = 200
	maxMods       = 32
)

var (
	modIDRe   = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
	semverRe  = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	localIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
)

// ModRules lets mods nudge rule multipliers. Each field is a factor applied
// to the current value (nil = unchanged); results are clamped.
type ModRules struct {
	XPMultiplier     *float64 `json:"xpMultiplier,omitempty"`
	GatherYield      *float64 `json:"gatherYield,omitempty"`
	RegrowMultiplier *float64 `json:"regrowMultiplier,omitempty"`
	MobHPMultiplier  *float64 `json:"mobHpMultiplier,omitempty"`
	MobDamageMult    *float64 `json:"mobDamageMultiplier,omitempty"`
	QuestRewardMult  *float64 `json:"questRewardMultiplier,omitempty"`
}

// ModContent is the subset of Pack a mod may supply.
type ModContent struct {
	Items   []Item      `json:"items"`
	Recipes []Recipe    `json:"recipes"`
	Quests  []Quest     `json:"quests"`
	Shop    []ShopEntry `json:"shop"`
}

type Manifest struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	API         int        `json:"api"`
	Description string     `json:"description"`
	Content     ModContent `json:"content"`
	Rules       ModRules   `json:"rules"`
}

// ModStatus reports what happened to each discovered mod.
type ModStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Dir     string `json:"dir"`
	Enabled bool   `json:"enabled"`
	Loaded  bool   `json:"loaded"`
	Error   string `json:"error,omitempty"`
}

// ParseManifest strictly decodes and validates a mod manifest in isolation.
func ParseManifest(b []byte) (*Manifest, error) {
	if len(b) > maxModFile {
		return nil, fmt.Errorf("mod.json too large (%d bytes, max %d)", len(b), maxModFile)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid mod.json: %w", err)
	}
	if !modIDRe.MatchString(m.ID) {
		return nil, fmt.Errorf("invalid mod id %q (lowercase letters, digits, - or _, 2-32 chars)", m.ID)
	}
	if m.API != ModAPI {
		return nil, fmt.Errorf("mod %s targets mod API %d; this server supports API %d", m.ID, m.API, ModAPI)
	}
	if !semverRe.MatchString(m.Version) {
		return nil, fmt.Errorf("mod %s: version must be MAJOR.MINOR.PATCH", m.ID)
	}
	if len(m.Name) == 0 || len(m.Name) > 64 || len(m.Description) > 500 {
		return nil, fmt.Errorf("mod %s: name 1-64 chars, description <=500", m.ID)
	}
	c := m.Content
	if len(c.Items) > maxModEntries || len(c.Recipes) > maxModEntries || len(c.Quests) > maxModEntries || len(c.Shop) > maxModEntries {
		return nil, fmt.Errorf("mod %s: too many entries (max %d per kind)", m.ID, maxModEntries)
	}
	pre := m.ID + ":"
	check := func(kind, id string) error {
		if !strings.HasPrefix(id, pre) || !localIDRe.MatchString(strings.TrimPrefix(id, pre)) {
			return fmt.Errorf("mod %s: %s id %q must look like %s<name>", m.ID, kind, id, pre)
		}
		return nil
	}
	for i := range c.Items {
		it := &c.Items[i]
		if err := check("item", it.ID); err != nil {
			return nil, err
		}
		if it.Kind == KindBlock || it.Kind == KindCurrency || it.Block != 0 {
			return nil, fmt.Errorf("mod %s: item %s: mods cannot add blocks or currencies in API 1", m.ID, it.ID)
		}
		if len(it.Name) == 0 || len(it.Name) > 40 || len(it.Desc) > 200 {
			return nil, fmt.Errorf("mod %s: item %s: bad name/desc length", m.ID, it.ID)
		}
		if !hexColor(it.Color) {
			it.Color = "#cccccc"
		}
	}
	for _, r := range c.Recipes {
		if err := check("recipe", r.ID); err != nil {
			return nil, err
		}
	}
	for _, q := range c.Quests {
		if err := check("quest", q.ID); err != nil {
			return nil, err
		}
		if len(q.Name) > 60 || len(q.Intro) > 600 || len(q.Outro) > 600 {
			return nil, fmt.Errorf("mod %s: quest %s: text too long", m.ID, q.ID)
		}
	}
	return &m, nil
}

func hexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, ch := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", ch) {
			return false
		}
	}
	return true
}

// Apply layers a validated manifest onto content. On any error, c is left
// unchanged (we merge into a clone and swap).
func (c *Content) Apply(m *Manifest) error {
	n := c.clone()
	p := &Pack{Items: m.Content.Items, Recipes: m.Content.Recipes, Quests: m.Content.Quests, Shop: m.Content.Shop}
	if err := n.merge(p, m.ID); err != nil {
		return fmt.Errorf("mod %s: %w", m.ID, err)
	}
	r := n.Rules
	mul := func(dst *float64, f *float64) {
		if f != nil {
			*dst *= *f
		}
	}
	mul(&r.XPMultiplier, m.Rules.XPMultiplier)
	mul(&r.GatherYield, m.Rules.GatherYield)
	mul(&r.RegrowMultiplier, m.Rules.RegrowMultiplier)
	mul(&r.MobHPMultiplier, m.Rules.MobHPMultiplier)
	mul(&r.MobDamageMult, m.Rules.MobDamageMult)
	mul(&r.QuestRewardMult, m.Rules.QuestRewardMult)
	n.Rules = r.Clamp()
	if err := n.Validate(); err != nil {
		return fmt.Errorf("mod %s: %w", m.ID, err)
	}
	n.ModRules = append(n.ModRules, m.Rules)
	n.Mods = append(n.Mods, m.ID+"@"+m.Version)
	sort.Strings(n.Mods)
	n.rehash()
	*c = *n
	return nil
}

func (c *Content) clone() *Content {
	n := &Content{
		Items: map[string]*Item{}, Recipes: map[string]*Recipe{}, Mobs: c.Mobs, Nodes: c.Nodes,
		Quests: map[string]*Quest{}, NPCs: c.NPCs, Rules: c.Rules,
		Spawns: c.Spawns, Shop: append([]ShopEntry(nil), c.Shop...), Mods: append([]string(nil), c.Mods...), ModRules: append([]ModRules(nil), c.ModRules...),
	}
	for k, v := range c.Items {
		n.Items[k] = v
	}
	for k, v := range c.Recipes {
		n.Recipes[k] = v
	}
	for k, v := range c.Quests {
		n.Quests[k] = v
	}
	return n
}

// LoadMods discovers mods in dir and applies the enabled ones, in id order.
// Broken or incompatible mods are skipped and reported; they never prevent
// the world from starting. If allow is false, nothing is applied.
func (c *Content) LoadMods(dir string, enabled []string, allow bool) []ModStatus {
	var out []ModStatus
	want := map[string]bool{}
	for _, e := range enabled {
		want[e] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			out = append(out, ModStatus{Dir: dir, Error: "cannot read mods directory: " + err.Error()})
		}
		return out
	}
	type found struct {
		st ModStatus
		m  *Manifest
	}
	var list []found
	for _, e := range entries {
		if !e.IsDir() || len(list) >= maxMods {
			continue
		}
		st := ModStatus{Dir: e.Name()}
		// Only mod.json is ever read: mods cannot reference other files.
		b, err := readCapped(filepath.Join(dir, e.Name(), "mod.json"), maxModFile)
		if err != nil {
			st.Error = err.Error()
			list = append(list, found{st: st})
			continue
		}
		m, err := ParseManifest(b)
		if err != nil {
			st.Error = err.Error()
			list = append(list, found{st: st})
			continue
		}
		st.ID, st.Name, st.Version = m.ID, m.Name, m.Version
		st.Enabled = want[m.ID]
		list = append(list, found{st, m})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].st.ID+list[i].st.Dir < list[j].st.ID+list[j].st.Dir })
	seen := map[string]bool{}
	for _, f := range list {
		st := f.st
		if f.m != nil {
			switch {
			case seen[f.m.ID]:
				st.Error = "duplicate mod id " + f.m.ID
			case !st.Enabled:
			case !allow:
				st.Error = "mods are not allowed on this world"
			default:
				if err := c.Apply(f.m); err != nil {
					st.Error = err.Error()
				} else {
					st.Loaded = true
				}
			}
			seen[f.m.ID] = true
		}
		out = append(out, st)
	}
	return out
}

func readCapped(path string, max int64) ([]byte, error) {
	// Refuse symlinks so a mod cannot point mod.json at a secret file.
	if li, err := os.Lstat(path); err == nil && li.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("mod.json must not be a symlink")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("missing mod.json")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("mod.json is not a regular file")
	}
	if st.Size() > max {
		return nil, fmt.Errorf("mod.json too large (%d bytes, max %d)", st.Size(), max)
	}
	b := make([]byte, st.Size())
	_, err = io.ReadFull(f, b)
	return b, err
}

// RecomputeRules sets rules from a new base (e.g. world config changed) and
// re-applies every loaded mod's rule factors.
func (c *Content) RecomputeRules(base Rules) {
	r := base
	for _, m := range c.ModRules {
		mul := func(dst *float64, f *float64) {
			if f != nil {
				*dst *= *f
			}
		}
		mul(&r.XPMultiplier, m.XPMultiplier)
		mul(&r.GatherYield, m.GatherYield)
		mul(&r.RegrowMultiplier, m.RegrowMultiplier)
		mul(&r.MobHPMultiplier, m.MobHPMultiplier)
		mul(&r.MobDamageMult, m.MobDamageMult)
		mul(&r.QuestRewardMult, m.QuestRewardMult)
	}
	c.Rules = r.Clamp()
	c.rehash()
}
