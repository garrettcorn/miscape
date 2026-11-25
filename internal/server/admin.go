package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/game"
)

// Admin routes exist only on the host's loopback listener, require the
// admin token, and refuse non-loopback Host headers (DNS rebinding).
func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackHost(r.Host) {
			http.Error(w, "admin is only available on this computer", http.StatusForbidden)
			return
		}
		nonce := game.NewToken()[:22]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'nonce-"+nonce+"'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		w.Write(bytes.Replace(s.AdminPage, []byte("<script>"), []byte(`<script nonce="`+nonce+`">`), 1))
	})
	h := func(f func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !s.checkAdmin(r) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Admin token required. It is printed in the host terminal and stored in admin-token in the world folder."})
				return
			}
			f(w, r)
		}
	}
	mux.HandleFunc("GET /admin/api/state", h(s.adminState))
	mux.HandleFunc("PUT /admin/api/config", h(s.adminConfig))
	mux.HandleFunc("POST /admin/api/invites", h(s.adminCreateInvite))
	mux.HandleFunc("DELETE /admin/api/invites/{code}", h(func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		var ok bool
		s.do(func() { ok = s.G.RevokeInvite(code) })
		writeJSON(w, 200, map[string]bool{"ok": ok})
	}))
	mux.HandleFunc("POST /admin/api/players", h(s.adminPlayerAction))
	mux.HandleFunc("POST /admin/api/save", h(func(w http.ResponseWriter, r *http.Request) {
		var snap *game.SaveData
		s.do(func() { snap = s.G.Snapshot() })
		s.saveMu.Lock()
		err := s.Store.Save(snap)
		s.saveMu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		s.lastSave.Store(time.Now().Unix())
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) adminState(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	s.do(func() {
		out["config"] = s.G.Cfg
		out["realm"] = s.G.Realm
		out["invites"] = s.G.InviteList()
		out["characters"] = s.G.Characters()
		out["bans"] = s.G.Bans
		out["online"] = s.G.OnlineNames()
		out["mods"] = s.ModStatus
		out["loadedMods"] = s.G.C.Mods
		out["rules"] = s.G.C.Rules
	})
	backups, _ := s.Store.Backups()
	errStr, _ := s.saveErr.Load().(string)
	out["save"] = map[string]any{"lastSave": time.Unix(s.lastSave.Load(), 0), "error": errStr, "backups": backups,
		"dataDir": s.Env.DataDir, "recoveredFrom": s.LoadInfo.Source, "recovered": s.LoadInfo.Recovered}
	out["stats"] = map[string]any{"connections": s.conns.Load(), "bytesOut": s.bytesOut.Load(), "bytesIn": s.bytesIn.Load()}
	if s.P2PStatus != nil {
		out["p2p"] = s.P2PStatus()
	}
	writeJSON(w, 200, out)
}

func (s *Server) adminConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.World
	if err := readJSON(r, &cfg); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid config: " + err.Error()})
		return
	}
	if err := cfg.Normalize(s.Mode); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := config.SaveWorld(s.Env.DataDir, cfg); err != nil {
		writeJSON(w, 500, map[string]string{"error": "could not save world.json: " + err.Error()})
		return
	}
	restart := false
	s.do(func() {
		old := s.G.Cfg
		restart = old.AllowMods != cfg.AllowMods || !sameStrings(old.EnabledMods, cfg.EnabledMods)
		s.G.Cfg = cfg
		s.Cfg = cfg
		s.G.C.RecomputeRules(cfg.Rules())
		if old.PvP != cfg.PvP || old.Trading != cfg.Trading {
			for pid := range s.sessions {
				s.Send(pid, game.M{"t": "toast", "kind": "info", "text": "The host changed world settings: PvP " + onOff(cfg.PvP) + ", trading " + cfg.Trading + "."})
			}
		}
	})
	writeJSON(w, 200, map[string]any{"ok": true, "restartRequired": restart})
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
		if m[x] < 0 {
			return false
		}
	}
	return true
}

func (s *Server) adminCreateInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
		Uses  int    `json:"uses"`
		Hours int    `json:"hours"`
	}
	if err := readJSON(r, &req); err != nil || req.Uses < 0 || req.Uses > 100 || req.Hours < 0 || req.Hours > 24*90 {
		writeJSON(w, 400, map[string]string{"error": "invalid invite request"})
		return
	}
	var inv game.Invite
	s.do(func() { inv = *s.G.CreateInvite(req.Label, req.Uses, time.Duration(req.Hours)*time.Hour) })
	out := map[string]any{"invite": inv}
	if s.P2PStatus != nil {
		if st := s.P2PStatus(); st["address"] != nil {
			out["shareText"] = "LF1." + inv.Code + "." + st["address"].(string)
		}
	}
	writeJSON(w, 200, out)
}

func (s *Server) adminPlayerAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"` // kick | ban | unban
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	var err error
	ok := true
	s.do(func() {
		switch req.Action {
		case "kick":
			ok = s.G.KickByName(req.Name)
		case "ban":
			err = s.G.BanByName(req.Name, req.Reason)
		case "unban":
			ok = s.G.Unban(req.Name)
		default:
			ok = false
		}
	})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": ok})
}

// keep content imported for ModStatus type in JSON docs
var _ = content.ModStatus{}
