package game

import (
	"time"

	"lanternfall/internal/content"
	"lanternfall/internal/world"
)

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

	levelCap int // the world's skill level cap (set when added to a game)
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
	fullWarned       time.Time
	place            string               // last place name sent to the client (see placeKey)
	effects          map[string]effect    // active charm effects by name ("ward", "bane:<mob>")
	charmReady       map[string]time.Time // charm cooldowns
	run              *courseRun           // the agility course being run, if any
	runCooldown      time.Time
	runWarned        string
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
	P     world.Pos `json:"p"`
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
