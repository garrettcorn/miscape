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
	"regexp"
	"slices"
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

// Models are the shapes the client knows how to draw for held and dropped
// items. An empty model picks a default from the item kind.
var Models = map[string]bool{"sword": true, "axe": true, "pickaxe": true, "club": true, "lantern": true, "gem": true, "food": true, "block": true, "bundle": true, "coin": true, "rod": true, "fish": true, "tinderbox": true, "log": true, "sling": true, "bow": true, "arrow": true, "wand": true, "snare": true}

// MobModels and NPCHats are the creature models and NPC hats the client can
// draw (web/js/models.js). Add a drawing there before adding a name here.
var (
	MobModels = map[string]bool{"bramblet": true, "wisp": true, "hare": true, "grouse": true, "deer": true, "clough": true, "shuck": true, "jerseydevil": true, "curupira": true, "boitata": true, "tokoloshe": true, "grootslang": true, "jiangshi": true, "yeti": true, "bunyip": true, "yowie": true}
	NPCHats   = map[string]bool{"": true, "tall": true, "cap": true, "helm": true}
)

type Item struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Desc      string         `json:"desc,omitempty"`
	Kind      string         `json:"kind"`
	Damage    int            `json:"damage,omitempty"`     // weapons
	Cooldown  int            `json:"cooldownMs,omitempty"` // weapons
	Heal      int            `json:"heal,omitempty"`       // food
	Block     uint8          `json:"block,omitempty"`      // placeable block id
	Tradeable bool           `json:"tradeable"`
	Stack     int            `json:"stack"`
	Value     int            `json:"value,omitempty"`    // shop sell price
	Color     string         `json:"color"`              // UI icon tint
	Light     bool           `json:"light,omitempty"`    // lantern: widens Gloam vision
	Model     string         `json:"model,omitempty"`    // how it looks when held or dropped (see Models)
	Glow      bool           `json:"glow,omitempty"`     // drawn glowing
	Style     string         `json:"style,omitempty"`    // weapons: melee (default), range or magic — the skill it trains
	Needs     map[string]int `json:"needs,omitempty"`    // skill levels needed to equip or use it
	Burn      *Burn          `json:"burn,omitempty"`     // fuel: can be lit as a campfire
	Kindle    bool           `json:"kindle,omitempty"`   // lights fires (a tinderbox)
	Reach     float64        `json:"reach,omitempty"`    // range and magic weapons: how far they shoot
	AmmoKind  string         `json:"ammoKind,omitempty"` // range and magic weapons: what they use up (arrow, shot, salt)
	Ammo      *Ammo          `json:"ammo,omitempty"`     // ammunition for weapons with this AmmoKind
	Trap      *Trap          `json:"trap,omitempty"`     // a snare you set on the ground (hunting)
	Crop      *Crop          `json:"crop,omitempty"`     // a seed you plant in soil (farming)
}

// Trap makes an item a snare: set on open ground, after Sec seconds it has
// caught one of Catches (by Hunting level), which the setter collects.
type Trap struct {
	Level   int     `json:"level"`
	Sec     int     `json:"sec"`
	Catches []Catch `json:"catches"`
}

// Crop makes an item a seed: planted in soil, it grows for Sec seconds
// (in real time, even with nobody online) into the Ripe block, which the
// planter harvests for Min..Max of Yield.
type Crop struct {
	Level int    `json:"level"`
	XP    int    `json:"xp"`
	Sec   int    `json:"sec"`
	Ripe  uint8  `json:"ripe"`
	Yield string `json:"yield"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

// Ammo makes an item ammunition: one is used per shot, adding Damage.
type Ammo struct {
	Kind   string `json:"kind"`
	Damage int    `json:"damage"`
}

// Burn describes an item as campfire fuel: the firemaking level to light it,
// the XP for doing so, and how long the fire lasts.
type Burn struct {
	Level int `json:"level"`
	XP    int `json:"xp"`
	Sec   int `json:"sec"`
}

type Recipe struct {
	ID      string         `json:"id"`
	Output  string         `json:"output"`
	Count   int            `json:"count"`
	Inputs  map[string]int `json:"inputs"`
	Skill   string         `json:"skill,omitempty"` // the skill it uses and trains (default crafting)
	Level   int            `json:"level"`           // level needed in that skill
	XP      int            `json:"xp"`
	Station string         `json:"station,omitempty"` // a station block to stand near: "anvil", "fire", or ""
	Needs   map[string]int `json:"needs,omitempty"`   // other skill levels needed
	// Cooking: below SafeAt there's a chance of getting Burnt instead of
	// Output (falling as the skill rises); at SafeAt and above, never.
	Burnt  string `json:"burnt,omitempty"`
	SafeAt int    `json:"safeAt,omitempty"`
}

type Drop struct {
	Item   string  `json:"item"`
	Min    int     `json:"min"`
	Max    int     `json:"max"`
	Chance float64 `json:"chance"`
}

type Mob struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Model      string         `json:"model"`
	HP         int            `json:"hp"`
	Damage     int            `json:"damage"`
	CooldownMs int            `json:"cooldownMs"`
	Speed      float64        `json:"speed"`
	Aggro      float64        `json:"aggro"`
	XP         int            `json:"xp"`
	Drops      []Drop         `json:"drops"`
	FearsFire  bool           `json:"fearsFire,omitempty"` // won't come near a campfire
	Flees      bool           `json:"flees,omitempty"`     // runs from players instead of fighting (game animals)
	Skill      string         `json:"skill,omitempty"`     // the skill a kill trains, instead of the weapon's style
	Needs      map[string]int `json:"needs,omitempty"`     // skill levels needed to attack it
}

// Spawn is a group of creatures wandering around a point. Positions are
// offsets from a named place, zone or point of the world ("at"), so the same
// content works on every level; X and Z are filled in by Resolve.
type Spawn struct {
	Mob   string  `json:"mob"`
	At    string  `json:"at"`
	DX    float64 `json:"dx,omitempty"`
	DZ    float64 `json:"dz,omitempty"`
	X     float64 `json:"-"`
	Z     float64 `json:"-"`
	R     float64 `json:"r"` // wander radius
	Count int     `json:"count"`
}

type Node struct {
	Block     uint8          `json:"block"`
	Name      string         `json:"name"`
	Item      string         `json:"item"`
	Skill     string         `json:"skill"`
	Level     int            `json:"level"`
	XP        int            `json:"xp"`
	Hits      int            `json:"hits"`
	RegrowSec int            `json:"regrowSec"`
	Needs     map[string]int `json:"needs,omitempty"`   // other skill levels needed
	Tool      string         `json:"tool,omitempty"`    // an item you must carry (a fishing rod)
	Spent     uint8          `json:"spent,omitempty"`   // the block left while it restocks (an empty stall), instead of air
	Keep      bool           `json:"keep,omitempty"`    // the block stays when gathered (fishing spots)
	Catches   []Catch        `json:"catches,omitempty"` // what it yields by level, instead of Item
}

// Catch is one possible yield of a node: a random catch among those your
// level allows.
type Catch struct {
	Item  string         `json:"item"`
	Level int            `json:"level"`
	XP    int            `json:"xp"`
	Extra map[string]int `json:"extra,omitempty"` // also given with the catch
}

// Quest step kinds.
const (
	StepGather  = "gather"  // obtain N of item by gathering
	StepCraft   = "craft"   // craft N of item
	StepKill    = "kill"    // defeat N of mob
	StepDeliver = "deliver" // hand in N of item at turn-in (consumed)
	StepBurn    = "burn"    // light N campfires of item
	StepCast    = "cast"    // cast N of a charm
)

type Step struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Count  int    `json:"count"`
	Text   string `json:"text"`
}

type Reward struct {
	Coins int            `json:"coins"`
	AP    int            `json:"ap"` // adventure points
	Items map[string]int `json:"items,omitempty"`
	XP    map[string]int `json:"xp,omitempty"`
}

type Quest struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Giver    string         `json:"giver"`
	Intro    string         `json:"intro"`
	Outro    string         `json:"outro"`
	Steps    []Step         `json:"steps"`
	Reward   Reward         `json:"reward"`
	Requires string         `json:"requires,omitempty"` // prerequisite quest id
	Needs    map[string]int `json:"needs,omitempty"`    // skill levels needed to start it
	Start    map[string]int `json:"start,omitempty"`    // items handed over when you accept it
	NeedsAP  int            `json:"needsAP,omitempty"`  // adventure points needed to start it
}

type ShopEntry struct {
	NPC   string `json:"npc"`
	Item  string `json:"item"`
	Price int    `json:"price"` // buy price in coins
}

// NPC is a townsperson. Like spawns, it stands at an offset from a named
// place or point ("at"); X, Y and Z are filled in by Resolve.
type NPC struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	At     string   `json:"at"`
	DX     float64  `json:"dx,omitempty"`
	DZ     float64  `json:"dz,omitempty"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Z      float64  `json:"z"`
	Yaw    float64  `json:"yaw"`
	Color  string   `json:"color"`
	Hat    string   `json:"hat,omitempty"` // tall | cap | helm | "" (hair)
	Lines  []string `json:"lines"`
	Pocket *Pocket  `json:"pocket,omitempty"` // can be pickpocketed (thieving)
}

// Pocket is what picking an NPC's pocket takes: the Thieving level, the XP,
// a coin range and a chance at items. Getting caught costs Fine coins.
type Pocket struct {
	Level int    `json:"level"`
	XP    int    `json:"xp"`
	Coins [2]int `json:"coins"`
	Loot  []Drop `json:"loot,omitempty"`
	Fine  int    `json:"fine"`
}

// Bounty is a kind of warrant a magistrate (Giver) hands out: kill Min..Max
// of Mob, for XP Slaying XP per kill and Coins per kill when you report back.
type Bounty struct {
	Giver string `json:"giver"`
	Mob   string `json:"mob"`
	Level int    `json:"level"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
	XP    int    `json:"xp"`
	Coins int    `json:"coins"`
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
	Skills   []Skill     `json:"skills,omitempty"`
	Charms   []Charm     `json:"charms,omitempty"`
	Courses  []Course    `json:"courses,omitempty"`
	Bounties []Bounty    `json:"bounties,omitempty"`
	Wilds    []Wild      `json:"wilds,omitempty"`
	Hunting  []Quarry    `json:"hunting,omitempty"`
	Items    []Item      `json:"items"`
	Recipes  []Recipe    `json:"recipes"`
	Mobs     []Mob       `json:"mobs"`
	Spawns   []Spawn     `json:"spawns"`
	Nodes    []Node      `json:"nodes"`
	Quests   []Quest     `json:"quests"`
	Shop     []ShopEntry `json:"shop"`
	NPCs     []NPC       `json:"npcs"`
}

// Content is the resolved, indexed content used by the game.
type Content struct {
	Skills   []*Skill // in display order
	Charms   []*Charm // in display order
	Courses  []*Course
	Bounties []Bounty
	Wilds    []Wild
	Hunting  []Quarry
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
	if len(p.Skills) > 0 {
		if ns != "" {
			return fmt.Errorf("mods can't add skills in mod API 1")
		}
		for i := range p.Skills {
			s := p.Skills[i]
			if c.Skill(s.ID) != nil {
				return fmt.Errorf("duplicate skill %q", s.ID)
			}
			c.Skills = append(c.Skills, &s)
		}
	}
	if len(p.Bounties) > 0 && ns != "" {
		return fmt.Errorf("mods can't add bounties in mod API 1")
	}
	c.Bounties = append(c.Bounties, p.Bounties...)
	c.Wilds = append(c.Wilds, p.Wilds...)
	c.Hunting = append(c.Hunting, p.Hunting...)
	for i := range p.Courses {
		if ns != "" {
			return fmt.Errorf("mods can't add agility courses in mod API 1")
		}
		co := p.Courses[i]
		c.Courses = append(c.Courses, &co)
	}
	if len(p.Charms) > 0 {
		if ns != "" {
			return fmt.Errorf("mods can't add charms in mod API 1")
		}
		for i := range p.Charms {
			ch := p.Charms[i]
			if c.Charm(ch.ID) != nil {
				return fmt.Errorf("duplicate charm %q", ch.ID)
			}
			c.Charms = append(c.Charms, &ch)
		}
	}
	for i := range p.Items {
		it := p.Items[i]
		if _, dup := c.Items[it.ID]; dup {
			return fmt.Errorf("duplicate item %q", it.ID)
		}
		if it.Kind == KindWeapon && it.Style == "" {
			it.Style = "melee"
		}
		c.Items[it.ID] = &it
	}
	for i := range p.Recipes {
		r := p.Recipes[i]
		if _, dup := c.Recipes[r.ID]; dup {
			return fmt.Errorf("duplicate recipe %q", r.ID)
		}
		if r.Skill == "" {
			r.Skill = "crafting"
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
		// "combat" was one skill before it split into melee, range and magic.
		if xp, ok := q.Reward.XP["combat"]; ok {
			q.Reward.XP["melee"] += xp
			delete(q.Reward.XP, "combat")
		}
		// Every quest gives adventure points: 1 unless it says otherwise.
		if q.Reward.AP == 0 {
			q.Reward.AP = 1
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
	if len(c.Skills) == 0 {
		return fmt.Errorf("no skills defined")
	}
	for _, s := range c.Skills {
		if !skillIDRe.MatchString(s.ID) || s.Name == "" || !slices.Contains(SkillGroups, s.Group) {
			return fmt.Errorf("skill %q: bad id, name or group %q", s.ID, s.Group)
		}
	}
	for _, wd := range c.Wilds {
		if c.Mobs[wd.Mob] == nil || wd.MinTier < 1 || wd.MaxTier < wd.MinTier || wd.MaxTier > 10 || wd.Weight < 0 {
			return fmt.Errorf("wild %s/%s: bad mob or tiers", wd.Region, wd.Mob)
		}
	}
	for _, q := range c.Hunting {
		if c.Mobs[q.Mob] == nil || q.MinTier < 1 || q.MinTier > 10 {
			return fmt.Errorf("hunting %s: bad mob or tier", q.Mob)
		}
	}
	for _, b := range c.Bounties {
		if c.NPCs[b.Giver] == nil || c.Mobs[b.Mob] == nil || b.Level < 1 || b.Level > 100 || b.Min < 1 || b.Max < b.Min || b.Max > 50 || b.XP < 0 || b.Coins < 0 {
			return fmt.Errorf("bounty %s/%s: bad giver, mob or numbers", b.Giver, b.Mob)
		}
	}
	for _, co := range c.Courses {
		if !skillIDRe.MatchString(co.ID) || co.Name == "" || co.Level < 1 || co.Level > 100 || co.XP < 0 || co.LimitSec < 5 || len(co.Offsets) < 2 || len(co.Offsets) > 20 {
			return fmt.Errorf("course %q: bad id, name, level, limit or checkpoints", co.ID)
		}
	}
	for _, ch := range c.Charms {
		if !skillIDRe.MatchString(ch.ID) || ch.Name == "" || !slices.Contains(CharmEffects, ch.Effect) || ch.Level < 1 || ch.Level > 100 || ch.Power < 0 || ch.Sec < 0 || ch.Sec > 600 || ch.CooldownMs < 0 {
			return fmt.Errorf("charm %q: bad id, name, effect or numbers", ch.ID)
		}
		if len(ch.Reagents) == 0 {
			return fmt.Errorf("charm %s: needs reagents", ch.ID)
		}
		for it, n := range ch.Reagents {
			if c.Items[it] == nil || n < 1 || n > 64 {
				return fmt.Errorf("charm %s: bad reagent %q", ch.ID, it)
			}
		}
		if ch.Effect == "bane" && c.Mobs[ch.Against] == nil {
			return fmt.Errorf("charm %s: unknown creature %q", ch.ID, ch.Against)
		}
	}
	for _, st := range CombatStyles {
		if !c.validSkill(st) {
			return fmt.Errorf("combat style %q has no skill", st)
		}
	}
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
		if it.Kind == KindWeapon && !slices.Contains(CombatStyles, it.Style) {
			return fmt.Errorf("item %s: unknown weapon style %q", id, it.Style)
		}
		if it.Kind == KindWeapon && it.Style != "melee" {
			if it.Reach < 2 || it.Reach > 30 || it.AmmoKind == "" {
				return fmt.Errorf("item %s: %s weapons need a reach (2-30) and an ammoKind", id, it.Style)
			}
			found := false
			for _, a := range c.Items {
				found = found || (a.Ammo != nil && a.Ammo.Kind == it.AmmoKind)
			}
			if !found {
				return fmt.Errorf("item %s: no ammunition of kind %q", id, it.AmmoKind)
			}
		}
		if tr := it.Trap; tr != nil {
			if tr.Level < 1 || tr.Level > 100 || tr.Sec < 5 || tr.Sec > 3600 || len(tr.Catches) == 0 {
				return fmt.Errorf("item %s: bad trap", id)
			}
			if err := c.checkCatches("item "+id, tr.Catches); err != nil {
				return err
			}
		}
		if cr := it.Crop; cr != nil && (cr.Level < 1 || cr.Level > 100 || cr.Sec < 5 || cr.Sec > 86400 || c.Items[cr.Yield] == nil || cr.Min < 1 || cr.Max < cr.Min || cr.Max > 64 || cr.Ripe == 0) {
			return fmt.Errorf("item %s: bad crop", id)
		}
		if a := it.Ammo; a != nil && (!skillIDRe.MatchString(a.Kind) || a.Damage < 0 || a.Damage > 50) {
			return fmt.Errorf("item %s: bad ammo", id)
		}
		if err := c.checkNeeds("item "+id, it.Needs); err != nil {
			return err
		}
		if it.Model != "" && !Models[it.Model] {
			return fmt.Errorf("item %s: unknown model %q", id, it.Model)
		}
		if b := it.Burn; b != nil && (b.Level < 1 || b.Level > 100 || b.Sec < 5 || b.Sec > 3600 || b.XP < 0) {
			return fmt.Errorf("item %s: bad burn stats", id)
		}
		if it.Kind == KindWeapon && (it.Cooldown < 200 || it.Cooldown > 5000) {
			return fmt.Errorf("item %s: weapon cooldown out of range", id)
		}
	}
	for id, r := range c.Recipes {
		if c.Items[r.Output] == nil {
			return fmt.Errorf("recipe %s: unknown output %q", id, r.Output)
		}
		if r.Count < 1 || r.Count > 64 || len(r.Inputs) == 0 || r.Level < 1 || r.Level > 100 {
			return fmt.Errorf("recipe %s: bad count/inputs/level", id)
		}
		if !c.validSkill(r.Skill) {
			return fmt.Errorf("recipe %s: unknown skill %q", id, r.Skill)
		}
		if err := c.checkNeeds("recipe "+id, r.Needs); err != nil {
			return err
		}
		for in, n := range r.Inputs {
			if c.Items[in] == nil || n < 1 || n > 64 {
				return fmt.Errorf("recipe %s: bad input %q", id, in)
			}
		}
		if r.Station != "" && !slices.Contains(Stations, r.Station) {
			return fmt.Errorf("recipe %s: unknown station %q", id, r.Station)
		}
		if r.Burnt != "" && c.Items[r.Burnt] == nil {
			return fmt.Errorf("recipe %s: unknown burnt item %q", id, r.Burnt)
		}
		if r.SafeAt != 0 && (r.Burnt == "" || r.SafeAt <= r.Level) {
			return fmt.Errorf("recipe %s: safeAt needs a burnt item and must be above the level", id)
		}
	}
	for id, m := range c.Mobs {
		if m.HP < 1 || m.Damage < 0 || m.Speed <= 0 || m.CooldownMs < 200 {
			return fmt.Errorf("mob %s: bad stats", id)
		}
		if m.Skill != "" && !c.validSkill(m.Skill) {
			return fmt.Errorf("mob %s: unknown skill %q", id, m.Skill)
		}
		if err := c.checkNeeds("mob "+id, m.Needs); err != nil {
			return err
		}
		if !MobModels[m.Model] {
			return fmt.Errorf("mob %s: unknown model %q", id, m.Model)
		}
		for _, d := range m.Drops {
			if c.Items[d.Item] == nil || d.Min < 0 || d.Max < d.Min || d.Chance < 0 || d.Chance > 1 {
				return fmt.Errorf("mob %s: bad drop %q", id, d.Item)
			}
		}
	}
	for id, n := range c.NPCs {
		if !NPCHats[n.Hat] {
			return fmt.Errorf("npc %s: unknown hat %q", id, n.Hat)
		}
		if pk := n.Pocket; pk != nil {
			if pk.Level < 1 || pk.Level > 100 || pk.XP < 0 || pk.Coins[0] < 0 || pk.Coins[1] < pk.Coins[0] || pk.Fine < 0 {
				return fmt.Errorf("npc %s: bad pocket", id)
			}
			for _, d := range pk.Loot {
				if c.Items[d.Item] == nil || d.Min < 0 || d.Max < d.Min || d.Chance < 0 || d.Chance > 1 {
					return fmt.Errorf("npc %s: bad pocket loot %q", id, d.Item)
				}
			}
		}
	}
	for _, s := range c.Spawns {
		if c.Mobs[s.Mob] == nil || s.Count < 0 || s.Count > 50 {
			return fmt.Errorf("spawn: bad mob %q", s.Mob)
		}
	}
	for b, n := range c.Nodes {
		if (c.Items[n.Item] == nil && len(n.Catches) == 0) || n.Hits < 1 || !c.validSkill(n.Skill) {
			return fmt.Errorf("node %d: bad item/hits/skill", b)
		}
		if err := c.checkNeeds("node "+n.Name, n.Needs); err != nil {
			return err
		}
		if n.Tool != "" && c.Items[n.Tool] == nil {
			return fmt.Errorf("node %s: unknown tool %q", n.Name, n.Tool)
		}
		if err := c.checkCatches("node "+n.Name, n.Catches); err != nil {
			return err
		}
	}
	for id, q := range c.Quests {
		if c.NPCs[q.Giver] == nil {
			return fmt.Errorf("quest %s: unknown giver %q", id, q.Giver)
		}
		if q.Requires != "" && c.Quests[q.Requires] == nil {
			return fmt.Errorf("quest %s: unknown prerequisite %q", id, q.Requires)
		}
		if err := c.checkNeeds("quest "+id, q.Needs); err != nil {
			return err
		}
		if len(q.Steps) == 0 || len(q.Steps) > 8 {
			return fmt.Errorf("quest %s: needs 1-8 steps", id)
		}
		for _, s := range q.Steps {
			if s.Count < 1 || s.Count > 100 {
				return fmt.Errorf("quest %s: bad step count", id)
			}
			switch s.Kind {
			case StepGather, StepCraft, StepDeliver, StepBurn:
				if c.Items[s.Target] == nil {
					return fmt.Errorf("quest %s: unknown item %q", id, s.Target)
				}
			case StepCast:
				if c.Charm(s.Target) == nil {
					return fmt.Errorf("quest %s: unknown charm %q", id, s.Target)
				}
			case StepKill:
				if c.Mobs[s.Target] == nil {
					return fmt.Errorf("quest %s: unknown mob %q", id, s.Target)
				}
			default:
				return fmt.Errorf("quest %s: bad step kind %q", id, s.Kind)
			}
		}
		if q.Reward.AP < 1 || q.Reward.AP > 100 || q.NeedsAP < 0 || q.NeedsAP > 10000 {
			return fmt.Errorf("quest %s: every quest gives 1-100 adventure points (ap), and needsAP must be 0-10000", id)
		}
		for it, n := range q.Start {
			if c.Items[it] == nil || n < 1 || n > 64 {
				return fmt.Errorf("quest %s: bad start item %q", id, it)
			}
		}
		for it := range q.Reward.Items {
			if c.Items[it] == nil {
				return fmt.Errorf("quest %s: unknown reward item %q", id, it)
			}
		}
		for sk := range q.Reward.XP {
			if !c.validSkill(sk) {
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

// Skill is a trainable skill. Skills are content (base.json), so adding one
// is data plus whatever new actions train it; mods can't add skills in API 1.
type Skill struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Group string `json:"group"` // combat | gathering | production | support
	Desc  string `json:"desc"`
}

var skillIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,23}$`)

// SkillGroups lists the groups in display order.
var SkillGroups = []string{"combat", "gathering", "production", "support"}

// Stations are the work stations recipes can require (each is a block in the
// world; see game.stationBlocks).
var Stations = []string{"anvil", "fire"}

// CombatStyles are the skills a weapon can train (Item.Style).
var CombatStyles = []string{"melee", "range", "magic"}

// Skill looks up a skill by id.
func (c *Content) Skill(id string) *Skill {
	for _, s := range c.Skills {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (c *Content) validSkill(s string) bool { return c.Skill(s) != nil }

// checkNeeds validates a needs map.
func (c *Content) checkNeeds(what string, needs map[string]int) error {
	for sk, lv := range needs {
		if !c.validSkill(sk) || lv < 1 || lv > 100 {
			return fmt.Errorf("%s: bad need %s %d", what, sk, lv)
		}
	}
	return nil
}

func (c *Content) rehash() {
	h := sha256.New()
	ids := make([]string, 0, len(c.Items)+len(c.Recipes)+len(c.Quests))
	for _, s := range c.Skills {
		ids = append(ids, "s:"+s.ID)
	}
	for _, ch := range c.Charms {
		ids = append(ids, "c:"+ch.ID)
	}
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
	Skills   []*Skill    `json:"skills"`
	Charms   []*Charm    `json:"charms"`
	Courses  []*Course   `json:"courses"`
	Bounties []Bounty    `json:"bounties"`
	Items    []*Item     `json:"items"`
	Recipes  []*Recipe   `json:"recipes"`
	Mobs     []ClientMob `json:"mobs"`
	Quests   []*Quest    `json:"quests"`
	NPCs     []*NPC      `json:"npcs"`
	Shop     []ShopEntry `json:"shop"`
	Nodes    []*Node     `json:"nodes"`
}

type ClientMob struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
	HP    int    `json:"hp"`
}

func (c *Content) View() ClientView {
	v := ClientView{Shop: c.Shop, Skills: c.Skills, Charms: c.Charms, Courses: c.Courses, Bounties: c.Bounties}
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

// Locator finds a named place, zone or point in the world: its x, z and the
// y someone standing there would have.
type Locator func(name string) (x, y, z float64, ok bool)

// Resolve returns a copy of the content with NPC and creature-spawn positions
// worked out for one world level. Unknown "at" names are an error.
func (c *Content) Resolve(at Locator, wrapX func(float64) float64) (*Content, error) {
	cp := *c
	cp.NPCs = make(map[string]*NPC, len(c.NPCs))
	for id, n := range c.NPCs {
		x, y, z, ok := at(n.At)
		if !ok {
			return nil, fmt.Errorf("npc %s: unknown place %q", id, n.At)
		}
		nn := *n
		nn.X, nn.Y, nn.Z = wrapX(x+n.DX), y, z+n.DZ
		cp.NPCs[id] = &nn
	}
	cp.Courses = make([]*Course, len(c.Courses))
	for i, co := range c.Courses {
		x, _, z, ok := at(co.At)
		if !ok {
			return nil, fmt.Errorf("course %s: unknown place %q", co.ID, co.At)
		}
		cc := *co
		cc.Points = make([][2]float64, len(co.Offsets))
		for j, o := range co.Offsets {
			cc.Points[j] = [2]float64{wrapX(x + o[0]), z + o[1]}
		}
		cp.Courses[i] = &cc
	}
	cp.Spawns = make([]Spawn, len(c.Spawns))
	for i, s := range c.Spawns {
		x, _, z, ok := at(s.At)
		if !ok {
			return nil, fmt.Errorf("spawn %s: unknown place %q", s.Mob, s.At)
		}
		s.X, s.Z = wrapX(x+s.DX), z+s.DZ
		cp.Spawns[i] = s
	}
	return &cp, nil
}

// Charm is a cunning-folk charm: cast with reagents, it trains Magic.
// Effects: "ward" cuts damage taken by Power percent for Sec seconds; "heal"
// restores Power health; "bane" doubles damage against the creature Against
// for Sec seconds. Attacking with magic is done with a wand (a magic weapon).
type Charm struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Icon       string         `json:"icon"`
	Desc       string         `json:"desc"`
	Words      string         `json:"words,omitempty"` // spoken when cast
	Level      int            `json:"level"`
	XP         int            `json:"xp"`
	Reagents   map[string]int `json:"reagents"`
	Effect     string         `json:"effect"`
	Power      int            `json:"power,omitempty"`
	Sec        int            `json:"sec,omitempty"`
	Against    string         `json:"against,omitempty"`
	CooldownMs int            `json:"cooldownMs,omitempty"`
}

// CharmEffects are the effects a charm can have.
var CharmEffects = []string{"ward", "heal", "bane"}

// Charm looks up a charm by id.
func (c *Content) Charm(id string) *Charm {
	for _, ch := range c.Charms {
		if ch.ID == id {
			return ch
		}
	}
	return nil
}

func (c *Content) checkCatches(what string, cs []Catch) error {
	for _, ct := range cs {
		if c.Items[ct.Item] == nil || ct.Level < 1 || ct.Level > 100 || ct.XP < 0 {
			return fmt.Errorf("%s: bad catch %q", what, ct.Item)
		}
		for it, n := range ct.Extra {
			if c.Items[it] == nil || n < 1 || n > 64 {
				return fmt.Errorf("%s: bad extra %q", what, it)
			}
		}
	}
	return nil
}

// Course is an agility course: run through its checkpoints in order within
// LimitSec seconds for Agility XP. Offsets are from the place At; Points are
// the resolved positions (filled in by Resolve).
type Course struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Level    int          `json:"level"`
	XP       int          `json:"xp"`
	LimitSec int          `json:"limitSec"`
	At       string       `json:"at"`
	Offsets  [][2]float64 `json:"offsets"`
	Points   [][2]float64 `json:"points,omitempty"`
}

// Wild is a creature that roams the countryside of a region (a continent's
// name from the map's regions, or "" for anywhere) between two danger tiers
// (1-10). Roaming creatures are as strong as the danger level where they
// appear. Weight is how common it is (default 1).
type Wild struct {
	Region  string `json:"region,omitempty"`
	Mob     string `json:"mob"`
	MinTier int    `json:"minTier"`
	MaxTier int    `json:"maxTier"`
	Weight  int    `json:"weight,omitempty"`
}

// Quarry is a game animal found at hunting grounds from a danger tier up.
type Quarry struct {
	Mob     string `json:"mob"`
	MinTier int    `json:"minTier"`
}
