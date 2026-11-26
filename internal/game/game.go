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
	"strings"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

const (
	TickRate              = 20
	SnapshotEvery         = 2 // ticks (10 Hz)
	InventorySlots        = 24
	InterestRadius        = 56.0
	ReachBlocks           = 5.2
	MeleeRange            = 3.3
	NPCRange              = 4.5
	AnvilRange            = 4.5
	TradeRange            = 10.0
	HearthSeconds         = 5.0
	MaxSkillLevel         = 10
	WorldSeed             = 20261125
	MaxPlacedPerCharacter = 500
)

// Outbox is how the game talks to connected clients.
type Outbox interface {
	Send(pid uint32, msg any)
	SendBin(pid uint32, b []byte)
	Kick(pid uint32, code, msg string)
}

type QuestState struct {
	Status   string `json:"status"` // active | done
	Progress []int  `json:"progress"`
}

// Character is persistent per-realm progression. It never leaves its realm.
type Character struct {
	ID        string                 `json:"id"`
	Realm     string                 `json:"realm"`
	Name      string                 `json:"name"`
	TokenHash string                 `json:"tokenHash"`
	Color     string                 `json:"color"`
	X         float64                `json:"x"`
	Y         float64                `json:"y"`
	Z         float64                `json:"z"`
	Yaw       float64                `json:"yaw"`
	HP        int                    `json:"hp"`
	Inv       map[string]int         `json:"inv"`
	Equipped  string                 `json:"equipped"`
	XP        map[string]int         `json:"xp"`
	Quests    map[string]*QuestState `json:"quests"`
	Placed    int                    `json:"placed"`
	Created   time.Time              `json:"created"`
	LastSeen  time.Time              `json:"lastSeen"`
}

// Player is the runtime state of an online character.
type Player struct {
	ID    uint32
	Char  *Character
	Local bool // connected via the host's loopback listener

	lastMove         time.Time
	groundY          float64
	airSince         time.Time
	violations       int
	nextGather       time.Time
	nextAttack       time.Time
	nextEat          time.Time
	lastDamage       time.Time
	lastRegen        time.Time
	hearthStart      time.Time
	hearthX, hearthZ float64
	trade            *Trade
	dead             bool
	dirtyMe          bool
	sentChunks       map[int]bool
	fullWarned       time.Time
}

type Mob struct {
	ID        uint32
	Def       *content.Mob
	Spawn     content.Spawn
	X, Y, Z   float64
	Yaw       float64
	HP, MaxHP int
	target    uint32
	nextAtk   time.Time
	wanderTo  [2]float64
	nextThink time.Time
	deadUntil time.Time
	lastHitBy uint32
	hitFlash  int
}

type regen struct {
	Idx   int       `json:"i"`
	Block uint8     `json:"b"`
	At    time.Time `json:"at"`
}

type Invite struct {
	Code     string    `json:"code"`
	Label    string    `json:"label"`
	UsesLeft int       `json:"usesLeft"` // -1 = unlimited
	Expires  time.Time `json:"expires,omitempty"`
	Created  time.Time `json:"created"`
}

type Ban struct {
	CharID string    `json:"charId"`
	Name   string    `json:"name"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// Realm identifies a world's progression namespace.
type Realm struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // official | private
	Created    time.Time `json:"created"`
	EverModded bool      `json:"everModded"`
	ModsSeen   []string  `json:"modsSeen,omitempty"`
}

type Game struct {
	C     *content.Content
	W     *world.World
	base  *world.World // pristine build, for diffing on save
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
	nodeHits map[int]int
	placed   map[int]string // block index -> placer char id
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
		nodeHits: map[int]int{}, placed: map[int]string{}, Trades: map[uint32]*Trade{}, Drops: map[uint32]*Drop{},
		Invites: map[string]*Invite{}, Bans: map[string]*Ban{}, rids: map[string]*ridCache{},
		Now: time.Now(), rng: mrand.New(mrand.NewPCG(uint64(time.Now().UnixNano()), 7)),
	}
	g.W = world.Build(WorldSeed)
	g.base = world.Build(WorldSeed)
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
	m.X, m.Z = m.Spawn.X+0.5, m.Spawn.Z+0.5
	for try_ := 0; try_ < 20; try_++ {
		a := g.rng.Float64() * 2 * math.Pi
		r := g.rng.Float64() * m.Spawn.R
		x, z := m.Spawn.X+math.Cos(a)*r+0.5, m.Spawn.Z+math.Sin(a)*r+0.5
		if g.W.GroundY(int(x), int(z)) > 0 {
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

// ---------- skills ----------

// XPForLevel is the total XP needed to reach level L (L>=1).
func XPForLevel(l int) int { return 25 * (l - 1) * l }

func LevelFor(xp int) int {
	l := 1
	for l < MaxSkillLevel && xp >= XPForLevel(l+1) {
		l++
	}
	return l
}

func (c *Character) Level(skill string) int { return LevelFor(c.XP[skill]) }

func (c *Character) MaxHP() int { return 18 + 2*c.Level("combat") }

func (g *Game) addXP(p *Player, skill string, amount int) {
	if amount <= 0 {
		return
	}
	amt := int(math.Round(float64(amount) * g.C.Rules.XPMultiplier))
	before := p.Char.Level(skill)
	p.Char.XP[skill] += amt
	after := p.Char.Level(skill)
	p.dirtyMe = true
	g.Dirty = true
	if after > before {
		g.Out.Send(p.ID, M{"t": "levelup", "skill": skill, "level": after})
		g.toast(p, fmt.Sprintf("%s reached level %d!", title(skill), after), "good")
	}
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// M is a shorthand for outgoing JSON messages.
type M map[string]any

func (g *Game) toast(p *Player, text, kind string) {
	g.Out.Send(p.ID, M{"t": "toast", "text": text, "kind": kind})
}

// ---------- join / leave ----------

// Session describes an incoming connection to Join.
type Session struct {
	Local bool // host's own loopback connection (trusted for access control)
}

// JoinError is an access failure with a protocol error code.
type JoinError struct{ Code, Msg string }

func (e *JoinError) Error() string { return e.Code + ": " + e.Msg }

// Join authenticates a hello and puts the character in the world. On success
// it returns the new player id and, for new characters, the issued token.
func (g *Game) Join(s Session, h *protocol.In) (pid uint32, token string, err error) {
	var c *Character
	if h.Token != "" {
		id, ok := g.tokens[HashToken(h.Token)]
		if !ok {
			return 0, "", &JoinError{protocol.ErrBadToken, "This world doesn't recognise your character. It may belong to a different world."}
		}
		c = g.Chars[id]
	} else {
		if !protocol.ValidName(h.Name) {
			return 0, "", &JoinError{protocol.ErrBadName, "Names are 3-16 letters, digits or _, starting with a letter."}
		}
		if _, taken := g.names[strings.ToLower(h.Name)]; taken {
			return 0, "", &JoinError{protocol.ErrNameTaken, "That name is already taken in this world."}
		}
		if !s.Local && g.Cfg.Access == "invite" {
			if h.Invite == "" {
				return 0, "", &JoinError{protocol.ErrInviteRequired, "This world is invite-only. Ask the host for an invite code."}
			}
			if !g.useInvite(h.Invite) {
				return 0, "", &JoinError{protocol.ErrInviteInvalid, "That invite code is invalid, used up, or expired."}
			}
		}
	}
	if c != nil {
		if b := g.Bans[c.ID]; b != nil {
			return 0, "", &JoinError{protocol.ErrBanned, "You have been banned from this world."}
		}
		if old, online := g.byChar[c.ID]; online {
			// Reconnect: the newest session wins.
			g.Out.Kick(old, protocol.ErrReplaced, "You connected from somewhere else.")
			g.Leave(old)
		}
	}
	if len(g.Players) >= g.Cfg.MaxPlayers {
		return 0, "", &JoinError{protocol.ErrFull, fmt.Sprintf("This world is full (%d players).", g.Cfg.MaxPlayers)}
	}
	if c == nil {
		token = NewToken()
		color := h.Color
		if color == "" {
			color = "#4a90d9"
		}
		c = &Character{
			ID: newID(), Realm: g.Realm.ID, Name: h.Name, TokenHash: HashToken(token), Color: color,
			X: world.SpawnX, Y: world.SpawnY, Z: world.SpawnZ,
			Inv: map[string]int{}, XP: map[string]int{}, Quests: map[string]*QuestState{},
			Created: g.Now,
		}
		c.HP = c.MaxHP()
		g.addChar(c)
		g.SaveSoon = true
	}
	p := &Player{ID: g.allocID(), Char: c, Local: s.Local, lastMove: g.Now, groundY: c.Y, lastRegen: g.Now, dirtyMe: true, sentChunks: map[int]bool{}}
	if c.HP <= 0 {
		c.HP = c.MaxHP()
	}
	// Never resume inside a block (world may have changed while offline).
	if g.bodyBlocked(c.X, c.Y, c.Z) {
		c.X, c.Y, c.Z = world.SpawnX, world.SpawnY, world.SpawnZ
	}
	g.Players[p.ID] = p
	g.byChar[c.ID] = p.ID
	g.ensureEquip(c)
	g.Dirty = true
	g.broadcastChat("", fmt.Sprintf("%s arrived in Wickhollow.", c.Name), "system")
	return p.ID, token, nil
}

func (g *Game) addChar(c *Character) {
	g.Chars[c.ID] = c
	g.tokens[c.TokenHash] = c.ID
	g.names[strings.ToLower(c.Name)] = c.ID
}

// Leave removes a player (disconnect). Any open trade is cancelled; since
// trades never move items until the atomic commit, nothing is lost.
func (g *Game) Leave(pid uint32) {
	p := g.Players[pid]
	if p == nil {
		return
	}
	if p.trade != nil {
		g.cancelTrade(p.trade, p.Char.Name+" left.")
	}
	p.Char.LastSeen = g.Now
	delete(g.Players, pid)
	delete(g.byChar, p.Char.ID)
	g.Dirty, g.SaveSoon = true, true
	g.broadcastChat("", p.Char.Name+" headed home.", "system")
}

// Welcome is sent after a successful join.
func (g *Game) Welcome(pid uint32, token string) M {
	p := g.Players[pid]
	return M{
		"t": "welcome", "proto": protocol.Current, "you": pid, "token": token,
		"realm": M{"id": g.Realm.ID, "kind": g.Realm.Kind, "name": g.Cfg.Name, "desc": g.Cfg.Description,
			"modded": g.Realm.EverModded, "mods": g.C.Mods, "pvp": g.Cfg.PvP, "trading": g.Cfg.Trading == "on"},
		"world": M{"sx": world.SizeX, "sy": world.SizeY, "sz": world.SizeZ, "chunk": world.Chunk,
			"gloamX": world.GloamMinX, "village": []float64{world.VillageCX, world.VillageCZ, world.VillageR},
			"spawn": []float64{world.SpawnX, world.SpawnY, world.SpawnZ}, "anvil": []int{world.AnvilX, world.AnvilY, world.AnvilZ}},
		"content": g.C.View(), "contentHash": g.C.Hash,
		"pos":  []float64{p.Char.X, p.Char.Y, p.Char.Z, p.Char.Yaw},
		"chat": g.chatLog,
	}
}

// ---------- request handling ----------

type ridCache struct {
	order []uint32
	acks  map[uint32]M
}

func (g *Game) cachedAck(p *Player, rid uint32) (M, bool) {
	rc := g.rids[p.Char.ID]
	if rc == nil {
		return nil, false
	}
	a, ok := rc.acks[rid]
	return a, ok
}

func (g *Game) rememberAck(p *Player, rid uint32, a M) {
	rc := g.rids[p.Char.ID]
	if rc == nil {
		rc = &ridCache{acks: map[uint32]M{}}
		g.rids[p.Char.ID] = rc
	}
	rc.acks[rid] = a
	rc.order = append(rc.order, rid)
	if len(rc.order) > 128 {
		delete(rc.acks, rc.order[0])
		rc.order = rc.order[1:]
	}
}

// ActionError is a rule rejection shown to the player.
type ActionError string

func (e ActionError) Error() string { return string(e) }

func fail(format string, a ...any) error { return ActionError(fmt.Sprintf(format, a...)) }

// Handle processes one validated client message.
func (g *Game) Handle(pid uint32, m *protocol.In) {
	p := g.Players[pid]
	if p == nil {
		return
	}
	if m.Rid != 0 {
		if a, ok := g.cachedAck(p, m.Rid); ok {
			// Duplicate request (retry / reconnect resend): same answer, no re-apply.
			g.Out.Send(pid, a)
			return
		}
	}
	var err error
	switch m.T {
	case "move":
		g.handleMove(p, m)
		return
	case "chunks":
		g.sendChunks(p, m.Chunks)
		return
	case "ping":
		g.Out.Send(pid, M{"t": "pong"})
		return
	case "gather":
		err = g.Gather(p, m.BX, m.BY, m.BZ)
	case "place":
		err = g.Place(p, m.BX, m.BY, m.BZ, m.Item)
	case "break":
		err = g.Break(p, m.BX, m.BY, m.BZ)
	case "attack":
		err = g.Attack(p, m.Target)
	case "craft":
		err = g.Craft(p, m.Recipe)
	case "eat":
		err = g.Eat(p, m.Item)
	case "equip":
		err = g.Equip(p, m.Item)
	case "drop":
		err = g.DropItem(p, m.Item, m.Count)
	case "talk":
		err = g.Talk(p, m.NPC)
	case "quest_accept":
		err = g.AcceptQuest(p, m.Quest)
	case "quest_turnin":
		err = g.TurnIn(p, m.Quest)
	case "buy":
		err = g.Buy(p, m.Item, m.Count)
	case "sell":
		err = g.Sell(p, m.Item, m.Count)
	case "trade_request":
		err = g.TradeRequest(p, m.Target)
	case "trade_respond":
		err = g.TradeRespond(p, m.Trade, m.Accept)
	case "trade_offer":
		err = g.TradeOffer(p, m.Trade, m.Offer)
	case "trade_lock":
		err = g.TradeLock(p, m.Trade)
	case "trade_confirm":
		err = g.TradeConfirm(p, m.Trade)
	case "trade_cancel":
		err = g.TradeCancel(p, m.Trade)
	case "chat":
		g.broadcastChat(p.Char.Name, strings.TrimSpace(m.Text), "say")
	case "hearth":
		err = g.Hearth(p)
	}
	ack := M{"t": "ack", "rid": m.Rid, "ok": err == nil}
	if err != nil {
		ack["err"] = err.Error()
	}
	g.rememberAck(p, m.Rid, ack)
	g.Out.Send(pid, ack)
}

func (g *Game) broadcastChat(from, text, kind string) {
	msg := M{"t": "chat", "from": from, "text": text, "kind": kind}
	g.chatLog = append(g.chatLog, msg)
	if len(g.chatLog) > 30 {
		g.chatLog = g.chatLog[1:]
	}
	for id := range g.Players {
		g.Out.Send(id, msg)
	}
}

func (g *Game) sendChunks(p *Player, cs [][2]int) {
	for _, c := range cs {
		if c[0] >= world.ChunksX || c[1] >= world.ChunksZ {
			continue
		}
		g.Out.SendBin(p.ID, protocol.ChunkFrame(c[0], c[1], g.W.EncodeChunk(c[0], c[1])))
		p.sentChunks[c[1]*world.ChunksX+c[0]] = true
	}
}

func (g *Game) setBlock(x, y, z int, b uint8) {
	g.W.Set(x, y, z, b)
	g.Dirty = true
	msg := M{"t": "block", "x": x, "y": y, "z": z, "b": b}
	for id := range g.Players {
		g.Out.Send(id, msg)
	}
}

// ---------- tick ----------

// Tick advances simulation time by dt.
func (g *Game) Tick(now time.Time) {
	g.Now = now
	g.tick++
	g.tickMobs()
	g.tickRegen()
	g.tickPlayers()
	g.tickDrops()
	g.tickTrades()
	if g.tick%SnapshotEvery == 0 {
		g.sendSnapshots()
	}
	for id, p := range g.Players {
		if p.dirtyMe {
			g.Out.Send(id, g.meMsg(p))
			p.dirtyMe = false
		}
	}
}

func (g *Game) tickRegen() {
	keep := g.regens[:0]
	for _, r := range g.regens {
		if g.Now.Before(r.At) {
			keep = append(keep, r)
			continue
		}
		x, y, z := r.Idx%world.SizeX, r.Idx/(world.SizeX*world.SizeZ), (r.Idx/world.SizeX)%world.SizeZ
		if g.W.Get(x, y, z) != world.Air || g.entityInCell(x, y, z) {
			r.At = g.Now.Add(5 * time.Second)
			keep = append(keep, r)
			continue
		}
		g.setBlock(x, y, z, r.Block)
	}
	g.regens = keep
}

func (g *Game) tickPlayers() {
	for _, p := range g.Players {
		c := p.Char
		if c.HP < c.MaxHP() && g.Now.Sub(p.lastDamage) > 6*time.Second && g.Now.Sub(p.lastRegen) > 3*time.Second {
			c.HP++
			p.lastRegen = g.Now
			p.dirtyMe = true
		}
		if !p.hearthStart.IsZero() {
			if math.Hypot(c.X-p.hearthX, c.Z-p.hearthZ) > 0.8 {
				p.hearthStart = time.Time{}
				g.toast(p, "Hearth Lantern interrupted — you moved.", "warn")
				p.dirtyMe = true
			} else if g.Now.Sub(p.hearthStart).Seconds() >= HearthSeconds {
				p.hearthStart = time.Time{}
				g.teleport(p, world.SpawnX, world.SpawnY, world.SpawnZ)
				g.toast(p, "The lantern's glow carries you home.", "good")
			}
		}
	}
}

func (g *Game) teleport(p *Player, x, y, z float64) {
	p.Char.X, p.Char.Y, p.Char.Z = x, y, z
	p.groundY = y
	p.lastMove = g.Now
	p.dirtyMe = true
	g.Out.Send(p.ID, M{"t": "correct", "x": x, "y": y, "z": z, "teleport": true})
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

func (g *Game) sendSnapshots() {
	for id, p := range g.Players {
		var ps, ms []M
		for oid, o := range g.Players {
			if oid != id && dist2(o.Char.X, o.Char.Z, p.Char.X, p.Char.Z) < InterestRadius*InterestRadius {
				ps = append(ps, M{"id": oid, "n": o.Char.Name, "c": o.Char.Color, "x": r2(o.Char.X), "y": r2(o.Char.Y), "z": r2(o.Char.Z),
					"r": r2(o.Char.Yaw), "hp": o.Char.HP, "mhp": o.Char.MaxHP(), "w": o.Char.Equipped, "l": o.Char.Inv["lantern"] > 0})
			}
		}
		for mid, m := range g.Mobs {
			if m.HP > 0 && dist2(m.X, m.Z, p.Char.X, p.Char.Z) < InterestRadius*InterestRadius {
				ms = append(ms, M{"id": mid, "k": m.Def.ID, "x": r2(m.X), "y": r2(m.Y), "z": r2(m.Z), "r": r2(m.Yaw), "hp": m.HP, "mhp": m.MaxHP, "a": m.target == id})
			}
		}
		g.Out.Send(id, M{"t": "snap", "p": ps, "m": ms, "d": g.dropView(p)})
	}
}

func dist2(ax, az, bx, bz float64) float64 { return (ax-bx)*(ax-bx) + (az-bz)*(az-bz) }

func (g *Game) meMsg(p *Player) M {
	c := p.Char
	skills := M{}
	for _, s := range content.Skills {
		l := c.Level(s)
		next := XPForLevel(min(l+1, MaxSkillLevel))
		skills[s] = M{"xp": c.XP[s], "lvl": l, "cur": XPForLevel(l), "next": next}
	}
	var hearth float64
	if !p.hearthStart.IsZero() {
		hearth = g.Now.Sub(p.hearthStart).Seconds() / HearthSeconds
	}
	return M{"t": "me", "name": c.Name, "color": c.Color, "hp": c.HP, "mhp": c.MaxHP(), "inv": c.Inv, "eq": c.Equipped, "skills": skills,
		"quests": c.Quests, "slots": g.slotsUsed(c.Inv), "maxSlots": InventorySlots,
		"zone": world.ZoneAt(c.X, c.Z), "hearth": hearth, "trade": g.tradeView(p)}
}

// OnlineNames lists online players (for /api/info and admin).
func (g *Game) OnlineNames() []string {
	var out []string
	for _, p := range g.Players {
		out = append(out, p.Char.Name)
	}
	sort.Strings(out)
	return out
}
