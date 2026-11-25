package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tailscale/tailcat"
)

// Invite is what a host shares with friends: "LF1.<code>.<tailcat address>".
// The code is the game-level invite; the address is the tunnel secret.
type Invite struct {
	Code string
	Addr tailcat.Addr
}

func ParseInvite(s string) (Invite, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'<> \t\r\n")
	if strings.HasPrefix(s, "LF1.") {
		parts := strings.SplitN(s, ".", 3)
		if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
			return Invite{}, errors.New("that invite looks cut off — copy the whole line starting with LF1.")
		}
		inv := Invite{Code: parts[1], Addr: tailcat.Addr(parts[2])}
		if _, err := tailcat.ParseAddr(inv.Addr); err != nil {
			return Invite{}, errors.New("that invite's connection address is damaged — ask the host to copy it again")
		}
		return inv, nil
	}
	if strings.HasPrefix(s, "tc") {
		if _, err := tailcat.ParseAddr(tailcat.Addr(s)); err != nil {
			return Invite{}, errors.New("that doesn't look like a valid Tailcat address")
		}
		return Invite{Addr: tailcat.Addr(s)}, nil
	}
	return Invite{}, errors.New("that isn't a Lanternfall invite (they start with LF1.)")
}

// Diagnostics is reported to the console and to the browser via /companion/status.
type Diagnostics struct {
	Stage       string    `json:"stage"` // connecting | connected | failed
	Path        string    `json:"path"`  // direct | relay | unknown
	RelayRegion string    `json:"relayRegion,omitempty"`
	LatencyMs   float64   `json:"latencyMs"`
	ServerName  string    `json:"serverName,omitempty"`
	ServerVer   string    `json:"serverVersion,omitempty"`
	Problem     string    `json:"problem,omitempty"`
	Advice      string    `json:"advice,omitempty"`
	Updated     time.Time `json:"updated"`
}

type Companion struct {
	inv      Invite
	cl       *tailcat.Client
	mu       sync.Mutex
	diag     Diagnostics
	codeUsed bool
	Logf     func(string, ...any)
}

func NewCompanion(inv Invite, derpMapURL string, verbose bool) *Companion {
	cl := &tailcat.Client{Server: inv.Addr, Logf: discard, DERPMapURL: derpMapURL}
	if verbose {
		cl.Logf = nil
	}
	return &Companion{inv: inv, cl: cl, diag: Diagnostics{Stage: "connecting", Path: "unknown"}, Logf: func(string, ...any) {}}
}

func (c *Companion) setDiag(f func(d *Diagnostics)) {
	c.mu.Lock()
	f(&c.diag)
	c.diag.Updated = time.Now()
	c.mu.Unlock()
}

func (c *Companion) Diag() Diagnostics {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.diag
}

// GameInfo mirrors server.Info (only fields the companion needs).
type GameInfo struct {
	Game    string `json:"game"`
	Version string `json:"version"`
	Proto   [2]int `json:"proto"`
	Name    string `json:"name"`
	Players int    `json:"players"`
	Max     int    `json:"max"`
}

func (c *Companion) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	return c.cl.DialTCPPort(ctx, GamePort)
}

// Connect establishes the tunnel and verifies the far side is a compatible
// Lanternfall world. It returns friendly errors for common failures.
func (c *Companion) Connect(ctx context.Context, protoMin, protoMax int) (*GameInfo, error) {
	pctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	pr, err := c.cl.Ping(pctx)
	if err != nil {
		e := classifyJoinErr(err)
		c.setDiag(func(d *Diagnostics) { d.Stage, d.Problem, d.Advice = "failed", e.Error(), adviceFor(e) })
		return nil, e
	}
	c.setDiag(func(d *Diagnostics) { d.LatencyMs = float64(pr.Latency.Microseconds()) / 1000; d.Path = "relay" })
	if r := c.cl.DERPRegion(); r != nil {
		c.setDiag(func(d *Diagnostics) { d.RelayRegion = r.RegionName })
	}
	hc := &http.Client{Transport: &http.Transport{DialContext: c.dial}, Timeout: 20 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://lanternfall.host/api/info", nil)
	resp, err := hc.Do(req)
	if err != nil {
		e := fmt.Errorf("reached the host's tunnel, but the game server didn't answer — the host's world may be starting up or stopped (%v)", err)
		c.setDiag(func(d *Diagnostics) { d.Stage, d.Problem = "failed", e.Error() })
		return nil, e
	}
	defer resp.Body.Close()
	var gi GameInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&gi); err != nil || gi.Game != "lanternfall" {
		e := errors.New("the host answered, but it isn't a Lanternfall world")
		c.setDiag(func(d *Diagnostics) { d.Stage, d.Problem = "failed", e.Error() })
		return nil, e
	}
	if gi.Proto[1] < protoMin || gi.Proto[0] > protoMax {
		e := fmt.Errorf("version mismatch: the host runs Lanternfall %s (protocol %d-%d) and this companion supports %d-%d — one of you needs to update", gi.Version, gi.Proto[0], gi.Proto[1], protoMin, protoMax)
		c.setDiag(func(d *Diagnostics) { d.Stage, d.Problem = "failed", e.Error() })
		return nil, e
	}
	c.setDiag(func(d *Diagnostics) { d.Stage, d.ServerName, d.ServerVer = "connected", gi.Name, gi.Version })
	go c.watchPath(ctx)
	return &gi, nil
}

// watchPath periodically checks whether NAT traversal found a direct path.
func (c *Companion) watchPath(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		pctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		res, err := c.cl.DiscoPing(pctx)
		cancel()
		if err == nil && res != nil {
			c.setDiag(func(d *Diagnostics) {
				d.LatencyMs = res.LatencySeconds * 1000
				switch {
				case res.Endpoint != "":
					d.Path, d.Advice = "direct", ""
				case res.DERPRegionID != 0:
					d.Path, d.RelayRegion = "relay", res.DERPRegionCode
					d.Advice = "Connected through a relay (NAT traversal didn't find a direct path). It works, with a bit more lag."
				}
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func classifyJoinErr(err error) error {
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "derp") && (strings.Contains(s, "map") || strings.Contains(s, "fetch")):
		return fmt.Errorf("can't reach the Tailcat relay network — check your internet connection, VPN or firewall (%v)", err)
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(s, "timeout") || strings.Contains(s, "deadline"):
		return errors.New("the host didn't answer. Their world may be offline, or the invite is from an earlier session (addresses change each time the host restarts unless they use --keep-address)")
	}
	return fmt.Errorf("couldn't connect to the host: %v", err)
}

func adviceFor(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "relay network"):
		return "Try another network, or ask the host whether they use a custom relay (LF_DERP_MAP_URL)."
	case strings.Contains(s, "didn't answer"):
		return "Ask the host to check their world is running and to send a fresh invite."
	}
	return ""
}

// Handler serves the local reverse proxy plus companion endpoints.
func (c *Companion) Handler() http.Handler {
	target, _ := url.Parse("http://lanternfall.host")
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			// Keep the browser's Host so the server's WebSocket Origin check
			// (Origin must match Host) passes for http://127.0.0.1:<port>.
			r.Out.Host = r.In.Host
			r.Out.Header.Del("X-Forwarded-For")
		},
		Transport: &http.Transport{DialContext: c.dial, MaxIdleConns: 8, IdleConnTimeout: 60 * time.Second},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Lost connection to the host's world: "+classifyJoinErr(err).Error(), http.StatusBadGateway)
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /companion/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(c.Diag())
	})
	// Hands the game-level invite code to the local game page exactly once,
	// so it never has to appear in a URL.
	mux.HandleFunc("POST /companion/invite", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" || r.Header.Get("X-Lanternfall") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		c.mu.Lock()
		code := ""
		if !c.codeUsed {
			code, c.codeUsed = c.inv.Code, true
		}
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"code": code})
	})
	mux.Handle("/", rp)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, _, _ := net.SplitHostPort(r.Host)
		if h != "127.0.0.1" && h != "localhost" && h != "::1" {
			http.Error(w, "forbidden", http.StatusForbidden) // DNS-rebinding guard
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (c *Companion) Close() error { return c.cl.Close() }
