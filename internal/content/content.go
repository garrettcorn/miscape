// Package content defines data-driven game content (items, recipes, mobs,
// quests, shops, gatherable nodes, rule multipliers). The base game is itself
// expressed as a content pack; mods use the same schema with restrictions.
package content

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed base.json
var baseJSON []byte

// Item kinds.
const (
	KindMaterial = "material"
	KindWeapon   = "weapon"
	KindFood     = "food"
	KindTool     = "tool"
	KindBlock    = "block"
	KindCurrency = "currency"
)

type Item struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Desc      string `json:"desc,omitempty"`
	Kind      string `json:"kind"`
	Damage    int    `json:"damage,omitempty"`     // weapons
	Cooldown  int    `json:"cooldownMs,omitempty"` // weapons
	Heal      int    `json:"heal,omitempty"`       // food
	Block     uint8  `json:"block,omitempty"`      // placeable block id
	Tradeable bool   `json:"tradeable"`
	Stack     int    `json:"stack"`
	Value     int    `json:"value,omitempty"` // shop sell price
	Color     string `json:"color"`           // UI icon tint
	Light     bool   `json:"light,omitempty"` // lantern: widens Gloam vision
}

type Recipe struct {
	ID      string         `json:"id"`
	Output  string         `json:"output"`
	Count   int            `json:"count"`
	Inputs  map[string]int `json:"inputs"`
	Level   int            `json:"level"` // Crafting level required
	XP      int            `json:"xp"`
	Station string         `json:"station,omitempty"` // "anvil" or ""
}

type Drop struct {
	Item   string  `json:"item"`
	Min    int     `json:"min"`
	Max    int     `json:"max"`
	Chance float64 `json:"chance"`
}

type Mob struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Model      string  `json:"model"`
	HP         int     `json:"hp"`
	Damage     int     `json:"damage"`
	CooldownMs int     `json:"cooldownMs"`
	Speed      float64 `json:"speed"`
	Aggro      float64 `json:"aggro"`
	XP         int     `json:"xp"`
	Drops      []Drop  `json:"drops"`
}

type Spawn struct {
	Mob   string  `json:"mob"`
	X     float64 `json:"x"`
	Z     float64 `json:"z"`
	R     float64 `json:"r"` // wander radius
	Count int     `json:"count"`
}

type Node struct {
	Block     uint8  `json:"block"`
	Name      string `json:"name"`
	Item      string `json:"item"`
	Skill     string `json:"skill"`
	Level     int    `json:"level"`
	XP        int    `json:"xp"`
	Hits      int    `json:"hits"`
	RegrowSec int    `json:"regrowSec"`
}

// Quest step kinds.
const (
	StepGather  = "gather"  // obtain N of item by gathering
	StepCraft   = "craft"   // craft N of item
	StepKill    = "kill"    // defeat N of mob
	StepDeliver = "deliver" // hand in N of item at turn-in (consumed)
)

type Step struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Count  int    `json:"count"`
	Text   string `json:"text"`
}

type Reward struct {
	Coins int            `json:"coins"`
	Items map[string]int `json:"items,omitempty"`
	XP    map[string]int `json:"xp,omitempty"`
}

type Quest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Giver    string `json:"giver"`
	Intro    string `json:"intro"`
	Outro    string `json:"outro"`
	Steps    []Step `json:"steps"`
	Reward   Reward `json:"reward"`
	Requires string `json:"requires,omitempty"` // prerequisite quest id
}

type ShopEntry struct {
	NPC   string `json:"npc"`
	Item  string `json:"item"`
	Price int    `json:"price"` // buy price in coins
}

type NPC struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	Z     float64  `json:"z"`
	Yaw   float64  `json:"yaw"`
	Color string   `json:"color"`
	Lines []string `json:"lines"`
}

// Rules are tunable multipliers. Mods and world config may adjust them within
// bounds enforced by Clamp.
type Rules struct {
	XPMultiplier       float64 `json:"xpMultiplier"`
	GatherYield        float64 `json:"gatherYield"`
	RegrowMultiplier   float64 `json:"regrowMultiplier"`
	MobHPMultiplier    float64 `json:"mobHpMultiplier"`
	MobDamageMult      float64 `json:"mobDamageMultiplier"`
	QuestRewardMult    float64 `json:"questRewardMultiplier"`
	GloamDeathCoinLoss float64 `json:"gloamDeathCoinLoss"`
}

func DefaultRules() Rules {
	return Rules{1, 1, 1, 1, 1, 1, 0.1}
}

func clampF(v, lo, hi float64) float64 {
	if v != v { // NaN
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Clamp keeps rules within sane, server-safe bounds.
func (r Rules) Clamp() Rules {
	r.XPMultiplier = clampF(r.XPMultiplier, 0.25, 5)
	r.GatherYield = clampF(r.GatherYield, 0.5, 4)
	r.RegrowMultiplier = clampF(r.RegrowMultiplier, 0.25, 4)
	r.MobHPMultiplier = clampF(r.MobHPMultiplier, 0.25, 4)
	r.MobDamageMult = clampF(r.MobDamageMult, 0, 4)
	r.QuestRewardMult = clampF(r.QuestRewardMult, 0.25, 5)
	r.GloamDeathCoinLoss = clampF(r.GloamDeathCoinLoss, 0, 0.5)
	return r
}

// Pack is the on-disk/JSON form of content.
type Pack struct {
	Items   []Item      `json:"items"`
	Recipes []Recipe    `json:"recipes"`
	Mobs    []Mob       `json:"mobs"`
	Spawns  []Spawn     `json:"spawns"`
	Nodes   []Node      `json:"nodes"`
	Quests  []Quest     `json:"quests"`
	Shop    []ShopEntry `json:"shop"`
	NPCs    []NPC       `json:"npcs"`
}

// Content is the resolved, indexed content used by the game.
type Content struct {
	Items    map[string]*Item
	Recipes  map[string]*Recipe
	Mobs     map[string]*Mob
	Spawns   []Spawn
	Nodes    map[uint8]*Node
	Quests   map[string]*Quest
	Shop     []ShopEntry
	NPCs     map[string]*NPC
	Rules    Rules
	Mods     []string // "id@version" of loaded mods, sorted
	ModRules []ModRules
	Hash     string // content fingerprint
}

// Base loads the embedded base content.
func Base() (*Content, error) {
	var p Pack
	if err := json.Unmarshal(baseJSON, &p); err != nil {
		return nil, fmt.Errorf("base content: %w", err)
	}
	c := &Content{
		Items: map[string]*Item{}, Recipes: map[string]*Recipe{}, Mobs: map[string]*Mob{},
		Nodes: map[uint8]*Node{}, Quests: map[string]*Quest{}, NPCs: map[string]*NPC{},
		Rules: DefaultRules(),
	}
	if err := c.merge(&p, ""); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("base content invalid: %w", err)
	}
	c.rehash()
	return c, nil
}

// MustBase is Base for tests and init paths where failure is a programming error.
func MustBase() *Content {
	c, err := Base()
	if err != nil {
		panic(err)
	}
	return c
}

func (c *Content) merge(p *Pack, ns string) error {
	for i := range p.Items {
		it := p.Items[i]
		if _, dup := c.Items[it.ID]; dup {
			return fmt.Errorf("duplicate item %q", it.ID)
		}
		c.Items[it.ID] = &it
	}
	for i := range p.Recipes {
		r := p.Recipes[i]
		if _, dup := c.Recipes[r.ID]; dup {
			return fmt.Errorf("duplicate recipe %q", r.ID)
		}
		c.Recipes[r.ID] = &r
	}
	for i := range p.Mobs {
		m := p.Mobs[i]
		if _, dup := c.Mobs[m.ID]; dup {
			return fmt.Errorf("duplicate mob %q", m.ID)
		}
		c.Mobs[m.ID] = &m
	}
	c.Spawns = append(c.Spawns, p.Spawns...)
	for i := range p.Nodes {
		n := p.Nodes[i]
		c.Nodes[n.Block] = &n
	}
	for i := range p.Quests {
		q := p.Quests[i]
		if _, dup := c.Quests[q.ID]; dup {
			return fmt.Errorf("duplicate quest %q", q.ID)
		}
		c.Quests[q.ID] = &q
	}
	c.Shop = append(c.Shop, p.Shop...)
	for i := range p.NPCs {
		n := p.NPCs[i]
		c.NPCs[n.ID] = &n
	}
	return nil
}

// Validate checks referential integrity of the whole content set.
func (c *Content) Validate() error {
	for id, it := range c.Items {
		if it.Stack < 1 || it.Stack > 999 {
			return fmt.Errorf("item %s: bad stack %d", id, it.Stack)
		}
		switch it.Kind {
		case KindMaterial, KindWeapon, KindFood, KindTool, KindBlock, KindCurrency:
		default:
			return fmt.Errorf("item %s: bad kind %q", id, it.Kind)
		}
		if it.Damage < 0 || it.Damage > 100 || it.Heal < 0 || it.Heal > 100 {
			return fmt.Errorf("item %s: stats out of range", id)
		}
		if it.Kind == KindWeapon && (it.Cooldown < 200 || it.Cooldown > 5000) {
			return fmt.Errorf("item %s: weapon cooldown out of range", id)
		}
	}
	for id, r := range c.Recipes {
		if c.Items[r.Output] == nil {
			return fmt.Errorf("recipe %s: unknown output %q", id, r.Output)
		}
		if r.Count < 1 || r.Count > 64 || len(r.Inputs) == 0 || r.Level < 1 || r.Level > 10 {
			return fmt.Errorf("recipe %s: bad count/inputs/level", id)
		}
		for in, n := range r.Inputs {
			if c.Items[in] == nil || n < 1 || n > 64 {
				return fmt.Errorf("recipe %s: bad input %q", id, in)
			}
		}
		if r.Station != "" && r.Station != "anvil" {
			return fmt.Errorf("recipe %s: unknown station %q", id, r.Station)
		}
	}
	for id, m := range c.Mobs {
		if m.HP < 1 || m.Damage < 0 || m.Speed <= 0 || m.CooldownMs < 200 {
			return fmt.Errorf("mob %s: bad stats", id)
		}
		for _, d := range m.Drops {
			if c.Items[d.Item] == nil || d.Min < 0 || d.Max < d.Min || d.Chance < 0 || d.Chance > 1 {
				return fmt.Errorf("mob %s: bad drop %q", id, d.Item)
			}
		}
	}
	for _, s := range c.Spawns {
		if c.Mobs[s.Mob] == nil || s.Count < 0 || s.Count > 50 {
			return fmt.Errorf("spawn: bad mob %q", s.Mob)
		}
	}
	for b, n := range c.Nodes {
		if c.Items[n.Item] == nil || n.Hits < 1 || !validSkill(n.Skill) {
			return fmt.Errorf("node %d: bad item/hits/skill", b)
		}
	}
	for id, q := range c.Quests {
		if c.NPCs[q.Giver] == nil {
			return fmt.Errorf("quest %s: unknown giver %q", id, q.Giver)
		}
		if q.Requires != "" && c.Quests[q.Requires] == nil {
			return fmt.Errorf("quest %s: unknown prerequisite %q", id, q.Requires)
		}
		if len(q.Steps) == 0 || len(q.Steps) > 8 {
			return fmt.Errorf("quest %s: needs 1-8 steps", id)
		}
		for _, s := range q.Steps {
			if s.Count < 1 || s.Count > 100 {
				return fmt.Errorf("quest %s: bad step count", id)
			}
			switch s.Kind {
			case StepGather, StepCraft, StepDeliver:
				if c.Items[s.Target] == nil {
					return fmt.Errorf("quest %s: unknown item %q", id, s.Target)
				}
			case StepKill:
				if c.Mobs[s.Target] == nil {
					return fmt.Errorf("quest %s: unknown mob %q", id, s.Target)
				}
			default:
				return fmt.Errorf("quest %s: bad step kind %q", id, s.Kind)
			}
		}
		for it := range q.Reward.Items {
			if c.Items[it] == nil {
				return fmt.Errorf("quest %s: unknown reward item %q", id, it)
			}
		}
		for sk := range q.Reward.XP {
			if !validSkill(sk) {
				return fmt.Errorf("quest %s: unknown skill %q", id, sk)
			}
		}
	}
	for _, s := range c.Shop {
		if c.NPCs[s.NPC] == nil || c.Items[s.Item] == nil || s.Price < 1 {
			return fmt.Errorf("shop: bad entry %s/%s", s.NPC, s.Item)
		}
	}
	return nil
}

// Skills is the fixed skill list for v1.
var Skills = []string{"woodcutting", "mining", "crafting", "combat"}

func validSkill(s string) bool {
	for _, k := range Skills {
		if k == s {
			return true
		}
	}
	return false
}

func (c *Content) rehash() {
	h := sha256.New()
	ids := make([]string, 0, len(c.Items)+len(c.Recipes)+len(c.Quests))
	for k := range c.Items {
		ids = append(ids, "i:"+k)
	}
	for k := range c.Recipes {
		ids = append(ids, "r:"+k)
	}
	for k := range c.Quests {
		ids = append(ids, "q:"+k)
	}
	sort.Strings(ids)
	for _, s := range ids {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	rb, _ := json.Marshal(c.Rules)
	h.Write(rb)
	for _, m := range c.Mods {
		h.Write([]byte(m))
	}
	c.Hash = hex.EncodeToString(h.Sum(nil))[:16]
}

// ClientView is the subset of content shipped to clients for UI.
type ClientView struct {
	Items   []*Item     `json:"items"`
	Recipes []*Recipe   `json:"recipes"`
	Mobs    []ClientMob `json:"mobs"`
	Quests  []*Quest    `json:"quests"`
	NPCs    []*NPC      `json:"npcs"`
	Shop    []ShopEntry `json:"shop"`
	Nodes   []*Node     `json:"nodes"`
}

type ClientMob struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
	HP    int    `json:"hp"`
}

func (c *Content) View() ClientView {
	v := ClientView{Shop: c.Shop}
	for _, k := range sortedKeys(c.Items) {
		v.Items = append(v.Items, c.Items[k])
	}
	for _, k := range sortedKeys(c.Recipes) {
		v.Recipes = append(v.Recipes, c.Recipes[k])
	}
	for _, k := range sortedKeys(c.Mobs) {
		m := c.Mobs[k]
		v.Mobs = append(v.Mobs, ClientMob{m.ID, m.Name, m.Model, m.HP})
	}
	for _, k := range sortedKeys(c.Quests) {
		v.Quests = append(v.Quests, c.Quests[k])
	}
	for _, k := range sortedKeys(c.NPCs) {
		v.NPCs = append(v.NPCs, c.NPCs[k])
	}
	for b := 0; b < 256; b++ {
		if n := c.Nodes[uint8(b)]; n != nil {
			v.Nodes = append(v.Nodes, n)
		}
	}
	return v
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
