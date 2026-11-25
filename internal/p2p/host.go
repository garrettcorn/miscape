// Package p2p integrates Tailcat (github.com/tailscale/tailcat) as the
// connectivity layer for player-hosted worlds. Tailcat is only a transport:
// it provides an encrypted tunnel (direct UDP when NAT traversal succeeds,
// DERP relay otherwise). Identity, invitations and authorization are handled
// by the game server, never by Tailcat.
package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"

	"lanternfall/internal/store"
)

// GamePort is the tunnel port the game's HTTP/WebSocket handler listens on.
const GamePort = 80

func discard(string, ...any) {}

// savedKey is the persisted host identity for --keep-address.
type savedKey struct {
	Key      key.NodePrivate      `json:"key"`
	PSK      tailcat.PresharedKey `json:"psk"`
	RegionID tailcfg.DERPRegionID `json:"regionId"`
}

// Host runs a Tailcat listener that serves the game handler to friends.
type Host struct {
	srv     *tailcat.Server
	addr    string
	started time.Time
	mu      sync.Mutex
	httpSrv *http.Server
	err     error
}

// HostOptions configures StartHost.
type HostOptions struct {
	KeyFile    string // if set, persist the identity so the address survives restarts
	DERPMapURL string // optional custom DERP map (bring your own relay)
	Verbose    bool
}

// StartHost brings up the Tailcat listener and serves h on it.
func StartHost(ctx context.Context, h http.Handler, opt HostOptions) (*Host, error) {
	s := &tailcat.Server{Logf: discard, DERPMapURL: opt.DERPMapURL}
	if opt.Verbose {
		s.Logf = nil // tailcat default: log.Printf
	}
	var sk savedKey
	persist := false
	if opt.KeyFile != "" {
		if b, err := os.ReadFile(opt.KeyFile); err == nil {
			if err := json.Unmarshal(b, &sk); err != nil {
				return nil, fmt.Errorf("host key file is corrupt (%v); delete it to get a new address", err)
			}
			s.Key, s.PresharedKey, s.RegionID = sk.Key, sk.PSK, sk.RegionID
		} else {
			sk = savedKey{Key: key.NewNode(), PSK: tailcat.NewPresharedKey()}
			s.Key, s.PresharedKey = sk.Key, sk.PSK
			persist = true
		}
	}
	// Only the game port is reachable through the tunnel (defense in depth);
	// no proxying, exit-node, or other services are enabled.
	s.AllowProxy = func(netip.AddrPort) bool { return false }
	sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	ln, err := s.Listen(sctx, "tcp", fmt.Sprintf(":%d", GamePort))
	if err != nil {
		return nil, Explain(err)
	}
	addr := string(s.TailcatAddr())
	if persist {
		if ci, err := tailcat.ParseAddr(tailcat.Addr(addr)); err == nil {
			sk.RegionID = ci.RegionID
		}
		b, _ := json.Marshal(sk)
		if err := store.WriteFileAtomic(opt.KeyFile, b, 0o600); err != nil {
			s.Close()
			return nil, fmt.Errorf("could not save host key: %w", err)
		}
	}
	hst := &Host{srv: s, addr: addr, started: time.Now()}
	hst.httpSrv = &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		err := hst.httpSrv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			hst.mu.Lock()
			hst.err = err
			hst.mu.Unlock()
		}
	}()
	return hst, nil
}

// Address returns the secret tailcat address. Never log it.
func (h *Host) Address() string { return h.addr }

// Status summarizes tunnel peers for the admin screen. It includes the
// address because the admin screen is loopback-only and token-protected.
func (h *Host) Status() map[string]any {
	st := h.srv.Status()
	var peers []map[string]any
	if st != nil {
		for _, p := range st.Peer {
			path := "relay"
			if p.CurAddr != "" {
				path = "direct"
			}
			peers = append(peers, map[string]any{
				"path": path, "relay": p.Relay, "rx": p.RxBytes, "tx": p.TxBytes,
				"lastHandshake": p.LastHandshake,
			})
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	errStr := ""
	if h.err != nil {
		errStr = h.err.Error()
	}
	return map[string]any{"address": h.addr, "since": h.started, "peers": peers, "error": errStr}
}

func (h *Host) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h.httpSrv.Shutdown(ctx)
	return h.srv.Close()
}

// Explain converts tailcat/network errors into player-friendly text.
func Explain(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	var ne net.Error
	switch {
	case strings.Contains(s, "derpmap") || strings.Contains(s, "DERP map") || strings.Contains(s, "derp map"):
		return fmt.Errorf("can't reach the Tailcat relay directory — check your internet connection or firewall (details: %v)", err)
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()):
		return fmt.Errorf("timed out reaching the relay network — check your internet connection (details: %v)", err)
	}
	return err
}
