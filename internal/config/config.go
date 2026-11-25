// Package config loads process settings from environment variables and the
// per-world, owner-editable world.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"lanternfall/internal/content"
	"lanternfall/internal/store"
)

const (
	ModeOfficial = "official"
	ModePrivate  = "private"
)

// Env holds process-level settings. Every field maps to an LF_* variable.
type Env struct {
	Addr       string // LF_ADDR: HTTP listen address (default 127.0.0.1:8080)
	DataDir    string // LF_DATA_DIR: world data directory (default ./data/<mode>)
	Mode       string // LF_MODE: official | private
	ModsDir    string // LF_MODS_DIR: mods directory (default <data>/mods)
	WorldList  string // LF_WORLD_LIST: JSON file listing official worlds for /api/worlds
	DERPMapURL string // LF_DERP_MAP_URL: custom Tailcat DERP map (bring your own relay)
	PublicURL  string // LF_PUBLIC_URL: allowed WebSocket Origin for official deployments
	Autosave   int    // LF_AUTOSAVE_SEC (default 30)
}

func FromEnv(mode string) Env {
	e := Env{
		Addr: getenv("LF_ADDR", defaultAddr()), Mode: getenv("LF_MODE", mode),
		WorldList: os.Getenv("LF_WORLD_LIST"), DERPMapURL: os.Getenv("LF_DERP_MAP_URL"),
		PublicURL: os.Getenv("LF_PUBLIC_URL"),
	}
	e.DataDir = getenv("LF_DATA_DIR", filepath.Join("data", e.Mode))
	e.ModsDir = getenv("LF_MODS_DIR", filepath.Join(e.DataDir, "mods"))
	e.Autosave, _ = strconv.Atoi(getenv("LF_AUTOSAVE_SEC", "30"))
	if e.Autosave < 5 {
		e.Autosave = 30
	}
	return e
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// World is the owner-editable configuration of one world (world.json).
type World struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MaxPlayers  int      `json:"maxPlayers"`
	PvP         bool     `json:"pvp"`
	Difficulty  string   `json:"difficulty"` // easy | normal | hard
	Resources   string   `json:"resources"`  // scarce | normal | abundant
	Trading     string   `json:"trading"`    // on | off
	QuestXP     float64  `json:"questXp"`    // quest reward multiplier 0.5..3
	Access      string   `json:"access"`     // invite | open
	AllowMods   bool     `json:"allowMods"`
	EnabledMods []string `json:"enabledMods"`
}

func DefaultWorld(mode string) World {
	w := World{
		Name: "My Lanternfall World", Description: "A cozy world for friends.",
		MaxPlayers: 8, Difficulty: "normal", Resources: "normal", Trading: "on",
		QuestXP: 1, Access: "invite",
	}
	if mode == ModeOfficial {
		w.Name, w.Description, w.MaxPlayers, w.Access = "Wickhollow (Official)", "An official Lanternfall world.", 64, "open"
	}
	return w
}

// Normalize clamps values and enforces mode policy. It returns an error only
// for values that cannot be sensibly fixed.
func (w *World) Normalize(mode string) error {
	w.Name = strings.TrimSpace(w.Name)
	if w.Name == "" || utf8.RuneCountInString(w.Name) > 40 {
		return errors.New("name must be 1-40 characters")
	}
	if utf8.RuneCountInString(w.Description) > 200 {
		return errors.New("description must be at most 200 characters")
	}
	if w.MaxPlayers < 1 || w.MaxPlayers > 64 {
		return errors.New("max players must be 1-64")
	}
	if !oneOf(w.Difficulty, "easy", "normal", "hard") {
		return errors.New("difficulty must be easy, normal or hard")
	}
	if !oneOf(w.Resources, "scarce", "normal", "abundant") {
		return errors.New("resources must be scarce, normal or abundant")
	}
	if !oneOf(w.Trading, "on", "off") {
		return errors.New("trading must be on or off")
	}
	if !oneOf(w.Access, "invite", "open") {
		return errors.New("access must be invite or open")
	}
	if w.QuestXP < 0.5 || w.QuestXP > 3 {
		return errors.New("quest reward multiplier must be 0.5-3")
	}
	if len(w.EnabledMods) > 32 {
		return errors.New("at most 32 mods")
	}
	if mode == ModeOfficial {
		// Official worlds never load third-party mods and keep PvP off.
		w.AllowMods, w.EnabledMods, w.PvP = false, nil, false
	}
	return nil
}

func oneOf(v string, opts ...string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}

// Rules derives gameplay multipliers from config (before mods are applied).
func (w World) Rules() content.Rules {
	r := content.DefaultRules()
	switch w.Difficulty {
	case "easy":
		r.MobHPMultiplier, r.MobDamageMult = 0.75, 0.6
	case "hard":
		r.MobHPMultiplier, r.MobDamageMult = 1.4, 1.5
	}
	switch w.Resources {
	case "scarce":
		r.GatherYield, r.RegrowMultiplier = 1, 1.6
	case "abundant":
		r.GatherYield, r.RegrowMultiplier = 2, 0.5
	}
	r.QuestRewardMult = w.QuestXP
	return r.Clamp()
}

func worldPath(dir string) string { return filepath.Join(dir, "world.json") }

// LoadWorld reads world.json, creating it with defaults if missing.
func LoadWorld(dir, mode string) (World, error) {
	b, err := os.ReadFile(worldPath(dir))
	if errors.Is(err, fs.ErrNotExist) {
		w := DefaultWorld(mode)
		return w, SaveWorld(dir, w)
	}
	if err != nil {
		return World{}, err
	}
	w := DefaultWorld(mode)
	if err := json.Unmarshal(b, &w); err != nil {
		return World{}, fmt.Errorf("world.json: %w", err)
	}
	if err := w.Normalize(mode); err != nil {
		return World{}, fmt.Errorf("world.json: %w", err)
	}
	return w, nil
}

func SaveWorld(dir string, w World) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(w, "", "  ")
	return store.WriteFileAtomic(worldPath(dir), b, 0o600)
}

// defaultAddr honours PORT (set by many hosts and dev tools) on loopback.
func defaultAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		return "127.0.0.1:" + p
	}
	return "127.0.0.1:8080"
}
