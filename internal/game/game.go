// Package game is the authoritative simulation. It is single-threaded by
// design: the server's loop goroutine is the only caller, so every rule
// (including two-party trades) executes atomically without locks.
package game

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"sort"
	"sync"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/world"
	"lanternfall/maps"
)

const (
	TickRate          = 20
	SnapshotEvery     = 2 // ticks (10 Hz)
	InventorySlots    = 24
	InterestRadius    = 56.0
	BlockUpdateRadius = 160.0 // beyond the farthest render distance (8 chunks)
	ReachBlocks       = 5.2
	MeleeRange        = 3.3
	NPCRange          = 4.5
	AnvilRange        = 4.5
	TradeRange        = 10.0
	HearthSeconds     = 5.0

	MaxPlacedPerCharacter = 500
)

// Outbox is how the game talks to connected clients.
type Outbox interface {
	Send(pid uint32, msg any)
	SendBin(pid uint32, b []byte)
	Kick(pid uint32, code, msg string)
}

type Game struct {
	C     *content.Content
	W     *world.World
	Cfg   config.World
	Realm Realm
	Out   Outbox

	Chars   map[string]*Character
	tokens  map[string]string // token hash -> char id
	names   map[string]string // lower name -> char id
	Players map[uint32]*Player
	byChar  map[string]uint32
	Mobs    map[uint32]*Mob
	nextID  uint32

	regens   []regen
	fires    []fire
	nodeHits map[world.Pos]int
	placed   map[world.Pos]string // block -> placer char id
	Trades   map[uint32]*Trade
	Drops    map[uint32]*Drop
	Invites  map[string]*Invite
	Bans     map[string]*Ban

	// rid cache: per character, survives reconnects within a process run.
	rids map[string]*ridCache

	Now      time.Time
	tick     uint64
	rng      *mrand.Rand
	Dirty    bool // something worth saving changed
	SaveSoon bool // save at the next opportunity (trade, quest, rare craft)
	Notices  []string
	chatLog  []map[string]any
}

// New creates a fresh world.
func New(c *content.Content, cfg config.World, kind string, out Outbox) *Game {
	g := &Game{
		C: c, Cfg: cfg, Out: out,
		Chars: map[string]*Character{}, tokens: map[string]string{}, names: map[string]string{},
		Players: map[uint32]*Player{}, byChar: map[string]uint32{}, Mobs: map[uint32]*Mob{},
		nodeHits: map[world.Pos]int{}, placed: map[world.Pos]string{}, Trades: map[uint32]*Trade{}, Drops: map[uint32]*Drop{},
		Invites: map[string]*Invite{}, Bans: map[string]*Ban{}, rids: map[string]*ridCache{},
		Now: time.Now(), rng: mrand.New(mrand.NewPCG(uint64(time.Now().UnixNano()), 7)),
	}
	g.W = world.New(DefaultLevel())
	rc, err := c.Resolve(g.W.Locate, g.W.WrapX)
	if err != nil {
		panic("content doesn't fit the world: " + err.Error())
	}
	g.C = rc
	g.Realm = Realm{ID: newID(), Kind: kind, Created: g.Now}
	g.noteMods()
	g.spawnMobs()
	return g
}

func (g *Game) noteMods() {
	if len(g.C.Mods) > 0 {
		g.Realm.EverModded = true
		for _, m := range g.C.Mods {
			if !contains(g.Realm.ModsSeen, m) {
				g.Realm.ModsSeen = append(g.Realm.ModsSeen, m)
			}
		}
		sort.Strings(g.Realm.ModsSeen)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func newID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// NewToken returns a random 256-bit URL-safe token (43 chars).
func NewToken() string {
	var b [32]byte
	rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func HashToken(t string) string {
	s := sha256.Sum256([]byte("lanternfall-token:" + t))
	return hex.EncodeToString(s[:])
}

func (g *Game) allocID() uint32 {
	g.nextID++
	return g.nextID
}

func (g *Game) spawnMobs() {
	for _, s := range g.C.Spawns {
		def := g.C.Mobs[s.Mob]
		for i := 0; i < s.Count; i++ {
			m := &Mob{ID: g.allocID(), Def: def, Spawn: s}
			g.respawnMob(m)
			g.Mobs[m.ID] = m
		}
	}
}

func (g *Game) mobMaxHP(def *content.Mob) int {
	return max(1, int(math.Round(float64(def.HP)*g.C.Rules.MobHPMultiplier)))
}

func (g *Game) respawnMob(m *Mob) {
	// Pick a spot with headroom (not inside a trunk), falling back to the spawn center.
	m.X, m.Z = g.W.WrapX(m.Spawn.X), m.Spawn.Z
	for try_ := 0; try_ < 20; try_++ {
		a := g.rng.Float64() * 2 * math.Pi
		r := g.rng.Float64() * m.Spawn.R
		x, z := g.W.WrapX(m.Spawn.X+math.Cos(a)*r), m.Spawn.Z+math.Sin(a)*r
		if g.W.GroundY(int(x), int(z)) > 0 && !g.wet(int(x), int(z)) {
			m.X, m.Z = x, z
			break
		}
	}
	m.Y = float64(max(0, g.W.GroundY(int(m.X), int(m.Z))))
	m.MaxHP = g.mobMaxHP(m.Def)
	m.HP = m.MaxHP
	m.target = 0
	m.deadUntil = time.Time{}
	m.wanderTo = [2]float64{m.X, m.Z}
}

// M is a shorthand for outgoing JSON messages.
type M map[string]any

func (g *Game) toast(p *Player, text, kind string) {
	g.Out.Send(p.ID, M{"t": "toast", "text": text, "kind": kind})
}

// ActionError is a rule rejection shown to the player.
type ActionError string

func (e ActionError) Error() string { return string(e) }

func fail(format string, a ...any) error { return ActionError(fmt.Sprintf(format, a...)) }

var (
	defaultLevelOnce sync.Once
	defaultLevel     *world.Def
)

// DefaultLevel is the world every game currently runs: Earth, level 1. (Later
// levels exist in maps/earth; travelling between them is future work.)
func DefaultLevel() *world.Def {
	defaultLevelOnce.Do(func() {
		d, err := world.LoadDef(maps.FS, "earth", "level1")
		if err != nil {
			panic("built-in world map: " + err.Error())
		}
		defaultLevel = d
	})
	return defaultLevel
}
