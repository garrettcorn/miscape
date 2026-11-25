// Package server wires the game loop to HTTP/WebSocket clients, autosave,
// and the host-only admin API.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/game"
	"lanternfall/internal/protocol"
	"lanternfall/internal/store"
)

// Version is overridden at release build time with -ldflags "-X lanternfall/internal/server.Version=..."
var Version = "0.1.0"

// Server owns the game loop. All game access happens via s.do on the loop.
type Server struct {
	Env        config.Env
	Mode       string
	Cfg        config.World
	G          *game.Game
	Store      *store.Store
	ModStatus  []content.ModStatus
	LoadInfo   store.LoadResult
	AdminToken string
	Static     fs.FS
	AdminPage  []byte

	// P2P is set by the host command to expose tunnel status to the admin screen.
	P2PStatus func() map[string]any

	cmds     chan func()
	sessions map[uint32]*session // loop-owned
	saveMu   sync.Mutex
	saveErr  atomic.Value // string
	lastSave atomic.Int64
	saving   chan *game.SaveData
	bytesOut atomic.Int64
	bytesIn  atomic.Int64
	conns    atomic.Int32
	stop     chan struct{}
	stopped  chan struct{}
}

type session struct {
	pid   uint32
	out   chan frame
	done  chan struct{}
	once  sync.Once
	close func(code, msg string)
}

type frame struct {
	bin  bool
	data []byte
	kick *[2]string
}

// Open loads (or creates) a world and returns a ready server.
func Open(env config.Env, static fs.FS, adminPage []byte) (*Server, error) {
	if err := os.MkdirAll(env.DataDir, 0o700); err != nil {
		return nil, err
	}
	cfg, err := config.LoadWorld(env.DataDir, env.Mode)
	if err != nil {
		return nil, err
	}
	c, err := content.Base()
	if err != nil {
		return nil, err
	}
	c.Rules = cfg.Rules()
	mods := c.LoadMods(env.ModsDir, cfg.EnabledMods, cfg.AllowMods && env.Mode == config.ModePrivate)
	for _, m := range mods {
		if m.Error != "" {
			log.Printf("mod %s (%s): %s", m.ID, m.Dir, m.Error)
		} else if m.Loaded {
			log.Printf("mod %s@%s loaded", m.ID, m.Version)
		}
	}
	st, err := store.New(env.DataDir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Env: env, Mode: env.Mode, Cfg: cfg, Store: st, ModStatus: mods, Static: static, AdminPage: adminPage,
		cmds: make(chan func(), 1024), sessions: map[uint32]*session{}, saving: make(chan *game.SaveData, 1),
		stop: make(chan struct{}), stopped: make(chan struct{}),
	}
	var data game.SaveData
	res, err := st.Load(&data)
	switch {
	case errors.Is(err, store.ErrNoState):
		s.G = game.New(c, cfg, env.Mode, s)
		log.Printf("created new %s world %q (realm %s)", env.Mode, cfg.Name, s.G.Realm.ID)
	case err != nil:
		return nil, fmt.Errorf("cannot load world: %w — see docs/HOSTING.md#recovery", err)
	default:
		s.G, err = game.Load(c, cfg, &data, s)
		if err != nil {
			return nil, err
		}
		if data.Realm.Kind != env.Mode {
			return nil, fmt.Errorf("this data directory holds a %s world; refusing to run it as %s", data.Realm.Kind, env.Mode)
		}
		if res.Recovered {
			log.Printf("WARNING: state.json was unusable (%s); recovered from %s", res.Problem, res.Source)
		}
		log.Printf("loaded world %q: %d characters (from %s)", cfg.Name, len(s.G.Chars), res.Source)
	}
	s.LoadInfo = res
	if env.Mode == config.ModePrivate {
		s.AdminToken, err = loadOrCreateSecret(filepath.Join(env.DataDir, "admin-token"))
		if err != nil {
			return nil, err
		}
	}
	s.saveErr.Store("")
	// Save immediately so a brand-new realm id is durable before anyone joins.
	if err := s.Store.Save(s.G.Snapshot()); err != nil {
		return nil, fmt.Errorf("initial save failed (is the data directory writable?): %w", err)
	}
	s.lastSave.Store(time.Now().Unix())
	return s, nil
}

func loadOrCreateSecret(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) >= 20 {
		return strings.TrimSpace(string(b)), nil
	}
	t := game.NewToken()
	return t, store.WriteFileAtomic(path, []byte(t+"\n"), 0o600)
}

// ---------- Outbox (called on the loop goroutine) ----------

func (s *Server) Send(pid uint32, msg any) {
	ss := s.sessions[pid]
	if ss == nil {
		return
	}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal: %v", err)
		return
	}
	s.push(ss, frame{data: b})
}

func (s *Server) SendBin(pid uint32, b []byte) {
	if ss := s.sessions[pid]; ss != nil {
		s.push(ss, frame{bin: true, data: b})
	}
}

func (s *Server) Kick(pid uint32, code, msg string) {
	if ss := s.sessions[pid]; ss != nil {
		delete(s.sessions, pid)
		ss.close(code, msg)
	}
}

func (s *Server) push(ss *session, f frame) {
	select {
	case ss.out <- f:
	default:
		// Client can't keep up; drop it rather than stall the world. The
		// reader goroutine removes the player when the socket closes.
		delete(s.sessions, ss.pid)
		ss.close("slow_client", "Connection too slow.")
	}
}

// do runs f on the loop goroutine and waits for it.
func (s *Server) do(f func()) {
	done := make(chan struct{})
	select {
	case s.cmds <- func() { f(); close(done) }:
	case <-s.stopped:
		return
	}
	select {
	case <-done:
	case <-s.stopped:
	}
}

// ---------- loop ----------

// Run executes the game loop until Shutdown.
func (s *Server) Run() {
	go s.saver()
	t := time.NewTicker(time.Second / game.TickRate)
	defer t.Stop()
	autosave := time.Duration(s.Env.Autosave) * time.Second
	lastSnap := time.Now()
	for {
		select {
		case f := <-s.cmds:
			f()
		case now := <-t.C:
			s.G.Tick(now)
			if s.G.SaveSoon || (s.G.Dirty && now.Sub(lastSnap) >= autosave) {
				s.queueSave(s.G.Snapshot())
				s.G.SaveSoon, s.G.Dirty = false, false
				lastSnap = now
			}
		case <-s.stop:
			// Final save, synchronously.
			for pid, ss := range s.sessions {
				ss.close("server_stopping", "The world is shutting down. Your progress was saved.")
				s.G.Leave(pid)
			}
			snap := s.G.Snapshot()
			s.saveMu.Lock()
			if err := s.Store.Save(snap); err != nil {
				log.Printf("FINAL SAVE FAILED: %v", err)
			} else {
				log.Printf("world saved")
			}
			s.saveMu.Unlock()
			close(s.stopped)
			return
		}
	}
}

func (s *Server) queueSave(d *game.SaveData) {
	select {
	case s.saving <- d:
	default:
		// A save is pending; replace it with the newer snapshot.
		select {
		case <-s.saving:
		default:
		}
		s.saving <- d
	}
}

// saver writes snapshots off the loop goroutine, retrying failures with backoff.
func (s *Server) saver() {
	backoff := time.Second
	for {
		var d *game.SaveData
		select {
		case d = <-s.saving:
		case <-s.stopped:
			return
		}
		for {
			s.saveMu.Lock()
			err := s.Store.Save(d)
			s.saveMu.Unlock()
			if err == nil {
				if prev, _ := s.saveErr.Load().(string); prev != "" {
					log.Printf("saving recovered")
					s.notifyAll("Saving works again. All progress is safe.", "good")
				}
				s.saveErr.Store("")
				s.lastSave.Store(time.Now().Unix())
				backoff = time.Second
				break
			}
			log.Printf("SAVE FAILED: %v (retrying in %s)", err, backoff)
			if prev, _ := s.saveErr.Load().(string); prev == "" {
				s.notifyAll("The host's world couldn't save. Retrying — progress is kept in memory.", "warn")
			}
			s.saveErr.Store(err.Error())
			select {
			case newer := <-s.saving:
				d = newer
			case <-time.After(backoff):
			case <-s.stopped:
				return
			}
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

func (s *Server) notifyAll(text, kind string) {
	select {
	case s.cmds <- func() {
		for pid := range s.sessions {
			s.Send(pid, game.M{"t": "toast", "text": text, "kind": kind})
		}
	}:
	default:
	}
}

// Shutdown stops the loop after a final save.
func (s *Server) Shutdown() {
	close(s.stop)
	<-s.stopped
}

// ---------- HTTP ----------

// Handler returns the HTTP handler. local=true is the host's loopback
// listener: it serves the admin API and is trusted for access control.
func (s *Server) Handler(local bool) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", s.staticHandler())
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/worlds", s.handleWorlds)
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) { s.handleWS(w, r, local) })
	if local && s.Mode == config.ModePrivate {
		s.adminRoutes(mux)
	}
	return securityHeaders(mux)
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self' 'unsafe-inline'")
		h.ServeHTTP(w, r)
	})
}

func (s *Server) staticHandler() http.Handler {
	fsrv := http.FileServerFS(s.Static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fsrv.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// Info is the public world description (no secrets).
type Info struct {
	Game    string   `json:"game"`
	Version string   `json:"version"`
	Proto   [2]int   `json:"proto"`
	Realm   string   `json:"realm"`
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Desc    string   `json:"desc"`
	Modded  bool     `json:"modded"`
	Mods    []string `json:"mods"`
	Players int      `json:"players"`
	Max     int      `json:"max"`
	Access  string   `json:"access"`
	PvP     bool     `json:"pvp"`
	Trading bool     `json:"trading"`
	Source  string   `json:"source"`
	License string   `json:"license"`
}

func (s *Server) info() Info {
	var inf Info
	s.do(func() {
		inf = Info{Game: "lanternfall", Version: Version, Proto: [2]int{protocol.ProtoMin, protocol.ProtoMax},
			Realm: s.G.Realm.ID, Kind: s.G.Realm.Kind, Name: s.G.Cfg.Name, Desc: s.G.Cfg.Description,
			Modded: s.G.Realm.EverModded, Mods: s.G.C.Mods, Players: len(s.G.Players), Max: s.G.Cfg.MaxPlayers,
			Access: s.G.Cfg.Access, PvP: s.G.Cfg.PvP, Trading: s.G.Cfg.Trading == "on", Source: s.Env.SourceURL, License: "AGPL-3.0-or-later"}
	})
	return inf
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.info()) }

func (s *Server) handleWorlds(w http.ResponseWriter, r *http.Request) {
	if s.Env.WorldList == "" {
		inf := s.info()
		if inf.Kind != config.ModeOfficial {
			writeJSON(w, 200, []any{})
			return
		}
		writeJSON(w, 200, []any{map[string]any{"name": inf.Name, "url": "", "players": inf.Players, "max": inf.Max}})
		return
	}
	b, err := os.ReadFile(s.Env.WorldList)
	if err != nil || !json.Valid(b) {
		writeJSON(w, 500, map[string]string{"error": "world list unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// ---------- WebSocket sessions ----------

type bucket struct {
	tokens float64
	last   time.Time
}

func (b *bucket) take(rate, burst float64) bool {
	now := time.Now()
	b.tokens = min(burst, b.tokens+now.Sub(b.last).Seconds()*rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request, local bool) {
	opts := &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover}
	if s.Env.PublicURL != "" {
		if host := strings.TrimPrefix(strings.TrimPrefix(s.Env.PublicURL, "https://"), "http://"); host != "" {
			opts.OriginPatterns = []string{host}
		}
	}
	c, err := websocket.Accept(w, r, opts)
	if err != nil {
		return
	}
	c.SetReadLimit(protocol.MaxMessageBytes)
	s.conns.Add(1)
	defer s.conns.Add(-1)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sendErr := func(code, msg string, extra map[string]any) {
		m := map[string]any{"t": "error", "code": code, "text": msg}
		for k, v := range extra {
			m[k] = v
		}
		b, _ := json.Marshal(m)
		wctx, wc := context.WithTimeout(ctx, 2*time.Second)
		c.Write(wctx, websocket.MessageText, b)
		wc()
		c.Close(websocket.StatusPolicyViolation, code)
	}

	// The first message must be hello, within 10 seconds.
	hctx, hcancel := context.WithTimeout(ctx, 10*time.Second)
	typ, data, err := c.Read(hctx)
	hcancel()
	if err != nil || typ != websocket.MessageText {
		c.Close(websocket.StatusPolicyViolation, protocol.ErrNeedHello)
		return
	}
	hello, err := protocol.Decode(data)
	if err != nil || hello.T != "hello" {
		// Check protocol version first so old clients get a useful message.
		var probe struct{ Proto int }
		json.Unmarshal(data, &probe)
		if probe.Proto != 0 && !protocol.Compatible(probe.Proto) {
			sendErr(protocol.ErrIncompatible, incompatibleText(probe.Proto), map[string]any{"server": Version, "proto": []int{protocol.ProtoMin, protocol.ProtoMax}})
			return
		}
		sendErr(protocol.ErrNeedHello, "Expected a hello message.", nil)
		return
	}
	if !protocol.Compatible(hello.Proto) {
		sendErr(protocol.ErrIncompatible, incompatibleText(hello.Proto), map[string]any{"server": Version, "proto": []int{protocol.ProtoMin, protocol.ProtoMax}})
		return
	}

	ss := &session{out: make(chan frame, 512), done: make(chan struct{})}
	ss.close = func(code, msg string) {
		ss.once.Do(func() {
			k := [2]string{code, msg}
			select {
			case ss.out <- frame{kick: &k}:
			default:
				cancel()
			}
			close(ss.done)
		})
	}
	var joinErr error
	var welcome game.M
	s.do(func() {
		pid, token, err := s.G.Join(game.Session{Local: local && s.Mode == config.ModePrivate}, hello)
		if err != nil {
			joinErr = err
			return
		}
		ss.pid = pid
		s.sessions[pid] = ss
		welcome = s.G.Welcome(pid, token)
		s.Send(pid, welcome)
	})
	if joinErr != nil {
		var je *game.JoinError
		if errors.As(joinErr, &je) {
			sendErr(je.Code, je.Msg, nil)
		} else {
			sendErr(protocol.ErrServer, "Could not join.", nil)
		}
		return
	}
	pid := ss.pid

	// Writer.
	go func() {
		for {
			select {
			case f := <-ss.out:
				wctx, wc := context.WithTimeout(ctx, 5*time.Second)
				var err error
				switch {
				case f.kick != nil:
					b, _ := json.Marshal(map[string]any{"t": "error", "code": f.kick[0], "text": f.kick[1]})
					c.Write(wctx, websocket.MessageText, b)
					wc()
					c.Close(websocket.StatusNormalClosure, f.kick[0])
					cancel()
					return
				case f.bin:
					err = c.Write(wctx, websocket.MessageBinary, f.data)
				default:
					err = c.Write(wctx, websocket.MessageText, f.data)
				}
				wc()
				s.bytesOut.Add(int64(len(f.data)))
				if err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Reader.
	var rl bucket
	rl.tokens, rl.last = 60, time.Now()
	strikes := 0
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			break
		}
		s.bytesIn.Add(int64(len(data)))
		if typ != websocket.MessageText {
			continue
		}
		if !rl.take(40, 80) {
			strikes++
			if strikes > 50 {
				s.do(func() { s.Kick(pid, protocol.ErrRateLimited, "Too many messages.") })
				break
			}
			continue
		}
		m, err := protocol.Decode(data)
		if err != nil {
			strikes++
			if strikes > 50 {
				s.do(func() { s.Kick(pid, protocol.ErrBadMessage, "Too many invalid messages.") })
				break
			}
			b, _ := json.Marshal(map[string]any{"t": "ack", "rid": 0, "ok": false, "err": "invalid message"})
			select {
			case ss.out <- frame{data: b}:
			default:
			}
			continue
		}
		if m.T == "hello" {
			continue
		}
		select {
		case s.cmds <- func() {
			if s.sessions[pid] == ss {
				s.G.Handle(pid, m)
			}
		}:
		case <-s.stopped:
			return
		}
	}
	s.do(func() {
		if cur := s.sessions[pid]; cur == ss || cur == nil {
			delete(s.sessions, pid)
			if s.G.Players[pid] != nil {
				s.G.Leave(pid)
			}
		}
	})
	c.CloseNow()
}

func incompatibleText(client int) string {
	if client < protocol.ProtoMin {
		return fmt.Sprintf("Your game client is older than this world (client protocol %d, world needs %d–%d). Refresh the page or update the join companion.", client, protocol.ProtoMin, protocol.ProtoMax)
	}
	return fmt.Sprintf("This world runs an older server (protocol %d–%d) than your client (%d). Ask the host to update Lanternfall.", protocol.ProtoMin, protocol.ProtoMax, client)
}

// IsLoopbackHost reports whether an HTTP Host header names this machine.
func IsLoopbackHost(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.Trim(h, "[]")
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) checkAdmin(r *http.Request) bool {
	if !IsLoopbackHost(r.Host) { // DNS-rebinding guard
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return s.AdminToken != "" && subtle.ConstantTimeCompare([]byte(got), []byte(s.AdminToken)) == 1
}
