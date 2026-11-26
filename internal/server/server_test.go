package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"lanternfall/internal/config"
)

func testEnv(dir string) config.Env {
	return config.Env{Addr: "127.0.0.1:0", DataDir: dir, Mode: config.ModePrivate, ModsDir: dir + "/mods", Autosave: 30, SourceURL: config.DefaultSourceURL}
}

func start(t *testing.T, dir string, local bool) (*Server, *httptest.Server) {
	t.Helper()
	s, err := Open(testEnv(dir), fstest.MapFS{"index.html": {Data: []byte("hi")}}, []byte("admin"))
	if err != nil {
		t.Fatal(err)
	}
	go s.Run()
	ts := httptest.NewServer(s.Handler(local))
	return s, ts
}

type client struct {
	t *testing.T
	c *websocket.Conn
}

func dial(t *testing.T, ts *httptest.Server) *client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.SetReadLimit(1 << 20)
	return &client{t, c}
}

func (c *client) send(v any) {
	b, _ := json.Marshal(v)
	c.raw(string(b))
}

func (c *client) raw(s string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.c.Write(ctx, websocket.MessageText, []byte(s)); err != nil {
		c.t.Fatal(err)
	}
}

// waitFor reads messages until one has type typ.
func (c *client) waitFor(typ string) map[string]any {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		mt, b, err := c.c.Read(ctx)
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", typ, err)
		}
		if mt != websocket.MessageText {
			continue
		}
		var m map[string]any
		json.Unmarshal(b, &m)
		if m["t"] == typ {
			return m
		}
	}
}

func TestJoinPlayAndPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s, ts := start(t, dir, true)
	c := dial(t, ts)
	c.send(map[string]any{"t": "hello", "proto": 2, "name": "Rowan"})
	w := c.waitFor("welcome")
	token, _ := w["token"].(string)
	if token == "" || w["realm"].(map[string]any)["kind"] != "private" {
		t.Fatalf("welcome: %v", w)
	}
	realm := w["realm"].(map[string]any)["id"]
	c.send(map[string]any{"t": "chunks", "chunks": [][2]int{{4, 4}}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	for {
		mt, b, err := c.c.Read(ctx)
		if err != nil {
			t.Fatal("no chunk:", err)
		}
		if mt == websocket.MessageBinary {
			if b[0] != 1 || b[1] != 4 || b[2] != 0 || b[3] != 4 || b[4] != 0 { // type, then x and z as uint16 LE
				t.Fatal("bad chunk frame")
			}
			break
		}
	}
	cancel()
	c.send(map[string]any{"t": "chat", "rid": 1, "text": "hello world"})
	if a := c.waitFor("ack"); a["ok"] != true {
		t.Fatal(a)
	}
	s.do(func() {
		for _, p := range s.G.Players {
			p.Char.Inv["log"] = 3
		}
	})
	c.c.Close(websocket.StatusNormalClosure, "")
	time.Sleep(150 * time.Millisecond)
	ts.Close()
	s.Shutdown()

	s2, ts2 := start(t, dir, true)
	defer ts2.Close()
	defer s2.Shutdown()
	c2 := dial(t, ts2)
	c2.send(map[string]any{"t": "hello", "proto": 2, "token": token})
	w2 := c2.waitFor("welcome")
	if w2["realm"].(map[string]any)["id"] != realm {
		t.Fatal("realm changed across restart")
	}
	me := c2.waitFor("me")
	if inv := me["inv"].(map[string]any); inv["log"] != float64(3) {
		t.Fatalf("inventory not persisted: %v", inv)
	}
}

func TestIncompatibleClientGetsClearError(t *testing.T) {
	s, ts := start(t, t.TempDir(), true)
	defer ts.Close()
	defer s.Shutdown()
	c := dial(t, ts)
	c.send(map[string]any{"t": "hello", "proto": 99, "name": "Future"})
	e := c.waitFor("error")
	if e["code"] != "incompatible_version" || !strings.Contains(e["text"].(string), "older server") {
		t.Fatal(e)
	}
}

func TestRemoteNeedsInviteLocalDoesNot(t *testing.T) {
	s, remote := start(t, t.TempDir(), false)
	defer remote.Close()
	defer s.Shutdown()
	c := dial(t, remote)
	c.send(map[string]any{"t": "hello", "proto": 2, "name": "Remote"})
	if e := c.waitFor("error"); e["code"] != "invite_required" {
		t.Fatal(e)
	}
	local := httptest.NewServer(s.Handler(true))
	defer local.Close()
	c2 := dial(t, local)
	c2.send(map[string]any{"t": "hello", "proto": 2, "name": "Hosty"})
	c2.waitFor("welcome")
}

func TestBadMessagesAreRejectedWithoutDisconnect(t *testing.T) {
	s, ts := start(t, t.TempDir(), true)
	defer ts.Close()
	defer s.Shutdown()
	c := dial(t, ts)
	c.send(map[string]any{"t": "hello", "proto": 2, "name": "Fuzzy"})
	c.waitFor("welcome")
	for _, raw := range []string{`{"t":"craft"}`, `nonsense`, `{"t":"gather","rid":5,"bx":-9}`, `{"t":"move","x":"NaN"}`} {
		c.raw(raw)
		if a := c.waitFor("ack"); a["ok"] != false {
			t.Fatalf("%s accepted", raw)
		}
	}
	c.send(map[string]any{"t": "ping"})
	c.waitFor("pong")
}

func TestAdminRequiresTokenAndLoopbackHost(t *testing.T) {
	s, ts := start(t, t.TempDir(), true)
	defer ts.Close()
	defer s.Shutdown()
	get := func(base, token, host string) int {
		req, _ := http.NewRequest("GET", base+"/admin/api/state", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if get(ts.URL, "", "") != 401 || get(ts.URL, "wrong", "") != 401 {
		t.Fatal("admin without valid token")
	}
	if get(ts.URL, s.AdminToken, "") != 200 {
		t.Fatal("admin with token failed")
	}
	if get(ts.URL, s.AdminToken, "evil.example.com") != 401 {
		t.Fatal("DNS rebinding host accepted")
	}
	remote := httptest.NewServer(s.Handler(false))
	defer remote.Close()
	if get(remote.URL, s.AdminToken, "") == 200 {
		t.Fatal("admin reachable via remote (tunnel) listener")
	}
	put := func(body string) int {
		req, _ := http.NewRequest("PUT", ts.URL+"/admin/api/config", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+s.AdminToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	good := `{"name":"Fernhollow","description":"","maxPlayers":4,"pvp":true,"difficulty":"hard","resources":"abundant","trading":"off","questXp":2,"access":"invite","allowMods":false,"enabledMods":[]}`
	if put(good) != 200 {
		t.Fatal("valid config rejected")
	}
	var pvp bool
	var yield float64
	s.do(func() { pvp, yield = s.G.Cfg.PvP, s.G.C.Rules.GatherYield })
	if !pvp || yield != 2 {
		t.Fatal("config not applied live")
	}
	if put(strings.Replace(good, `"maxPlayers":4`, `"maxPlayers":1000`, 1)) != 400 {
		t.Fatal("bad config accepted")
	}
}

func TestOfficialModeLocksPolicy(t *testing.T) {
	w := config.DefaultWorld(config.ModeOfficial)
	w.AllowMods, w.PvP, w.EnabledMods = true, true, []string{"x"}
	if err := w.Normalize(config.ModeOfficial); err != nil {
		t.Fatal(err)
	}
	if w.AllowMods || w.PvP || len(w.EnabledMods) != 0 {
		t.Fatal("official worlds must not allow mods or PvP")
	}
	dir := t.TempDir()
	env := testEnv(dir)
	env.Mode = config.ModeOfficial
	s, err := Open(env, fstest.MapFS{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	go s.Run()
	ts := httptest.NewServer(s.Handler(true))
	resp, err := http.Get(ts.URL + "/admin/api/state")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		t.Fatalf("official mode must not expose the private admin API (got %d)", resp.StatusCode)
	}
	ts.Close()
	s.Shutdown()
	env.Mode = config.ModePrivate
	if _, err := Open(env, fstest.MapFS{}, nil); err == nil {
		t.Fatal("official data opened as private world")
	}
}

func TestInfoHasNoSecrets(t *testing.T) {
	s, ts := start(t, t.TempDir(), true)
	defer ts.Close()
	defer s.Shutdown()
	resp, err := http.Get(ts.URL + "/api/info")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(b), s.AdminToken) || strings.Contains(string(b), "token") {
		t.Fatal("info leaks secrets: " + string(b))
	}
}

func TestInfoOffersSourceCode(t *testing.T) {
	s, ts := start(t, t.TempDir(), true)
	defer ts.Close()
	defer s.Shutdown()
	resp, err := http.Get(ts.URL + "/api/info")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var inf Info
	json.NewDecoder(resp.Body).Decode(&inf)
	// AGPL-3.0 section 13: network users must be offered the source.
	if !strings.HasPrefix(inf.Source, "https://") || inf.License != "AGPL-3.0-or-later" {
		t.Fatalf("info must advertise source and license: %+v", inf)
	}
}

// The world map image is served to the client; the level files next to it
// are not (only PNGs).
func TestWorldMapImageIsServed(t *testing.T) {
	_, ts := start(t, t.TempDir(), false)
	r, err := http.Get(ts.URL + "/maps/earth/terrain.png")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("terrain.png: %d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}
	for _, p := range []string{"/maps/earth/level1.json", "/maps/", "/maps/../go.mod"} {
		r, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode == 200 {
			t.Fatalf("%s should not be served", p)
		}
	}
}
