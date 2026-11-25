// Command lanternfall runs Lanternfall worlds and the friend join companion.
//
//	lanternfall serve [--mode official|private]   run a world (official mode for hosted shards)
//	lanternfall host  [--keep-address]            run a private world and share it via Tailcat
//	lanternfall join  [invite]                     join a friend's world (opens your browser)
//	lanternfall backups | restore <name>           list / restore world backups
//	lanternfall mods check <dir>                   validate a mod folder
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/p2p"
	"lanternfall/internal/protocol"
	"lanternfall/internal/server"
	"lanternfall/internal/store"
	"lanternfall/web"
)

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe(os.Args[2:])
	case "host":
		err = cmdHost(os.Args[2:])
	case "join":
		err = cmdJoin(os.Args[2:])
	case "backups":
		err = cmdBackups(os.Args[2:])
	case "restore":
		err = cmdRestore(os.Args[2:])
	case "mods":
		err = cmdMods(os.Args[2:])
	case "version":
		fmt.Printf("lanternfall %s (protocol %d-%d, mod API %d)\n", server.Version, protocol.ProtoMin, protocol.ProtoMax, content.ModAPI)
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n✗", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Lanternfall `+server.Version+`

  lanternfall serve [--mode private|official]  Run a world on this machine (LF_ADDR, default 127.0.0.1:8080)
  lanternfall host  [--keep-address]           Run a private world and let friends join over Tailcat
  lanternfall join  [invite]                    Join a friend's private world (opens your browser)
  lanternfall backups                           List backups of the world in LF_DATA_DIR
  lanternfall restore <backup-name>             Restore a backup (stop the world first)
  lanternfall mods check <mod-folder>           Validate a mod
  lanternfall version

See docs/HOSTING.md for details. Configuration uses LF_* environment variables (docs/CONFIG.md).
`)
}

func openServer(mode string) (*server.Server, error) {
	env := config.FromEnv(mode)
	if env.Mode != config.ModeOfficial && env.Mode != config.ModePrivate {
		return nil, fmt.Errorf("LF_MODE must be official or private")
	}
	return server.Open(env, web.FS(), web.AdminPage)
}

func waitSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	fmt.Println("\nShutting down and saving…")
}

func listenLocal(s *server.Server, local bool) (*http.Server, string, error) {
	ln, err := net.Listen("tcp", s.Env.Addr)
	if err != nil {
		return nil, "", fmt.Errorf("can't listen on %s (is another world running? set LF_ADDR to change the port): %w", s.Env.Addr, err)
	}
	hs := &http.Server{Handler: s.Handler(local), ReadHeaderTimeout: 10 * time.Second}
	go hs.Serve(ln)
	return hs, ln.Addr().String(), nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	mode := fs.String("mode", "private", "official or private")
	fs.Parse(args)
	os.Setenv("LF_MODE", *mode)
	s, err := openServer(*mode)
	if err != nil {
		return err
	}
	go s.Run()
	// In official mode the listener is public (behind a TLS proxy) and is not
	// trusted; in private mode `serve` is a local-only world.
	hs, addr, err := listenLocal(s, *mode == config.ModePrivate && server.IsLoopbackHost(s.Env.Addr))
	if err != nil {
		return err
	}
	fmt.Printf("🏮 Lanternfall %s world %q running at http://%s/\n", *mode, s.Cfg.Name, addr)
	if *mode == config.ModePrivate {
		fmt.Printf("   Admin screen: http://%s/admin   (admin token is in %s)\n", addr, filepath.Join(s.Env.DataDir, "admin-token"))
	}
	waitSignal()
	hs.Close()
	s.Shutdown()
	return nil
}

func cmdHost(args []string) error {
	fs := flag.NewFlagSet("host", flag.ExitOnError)
	keep := fs.Bool("keep-address", false, "keep the same friend address across restarts (stores a key in the world folder)")
	verbose := fs.Bool("verbose", false, "verbose Tailcat networking logs")
	fs.Parse(args)
	os.Setenv("LF_MODE", config.ModePrivate)
	s, err := openServer(config.ModePrivate)
	if err != nil {
		return err
	}
	go s.Run()
	hs, addr, err := listenLocal(s, true)
	if err != nil {
		return err
	}
	fmt.Printf("🏮 Private world %q is running.\n", s.Cfg.Name)
	fmt.Printf("   Play:   http://%s/\n", addr)
	fmt.Printf("   Admin:  http://%s/admin\n", addr)
	fmt.Printf("   Admin token (paste into the admin screen): %s\n\n", s.AdminToken)
	fmt.Println("   Opening a Tailcat tunnel so friends can join…")
	opt := p2p.HostOptions{DERPMapURL: s.Env.DERPMapURL, Verbose: *verbose}
	if *keep {
		opt.KeyFile = filepath.Join(s.Env.DataDir, "host-key.json")
	}
	h, err := p2p.StartHost(context.Background(), s.Handler(false), opt)
	if err != nil {
		fmt.Printf("\n⚠  Friends can't join right now: %v\n   You can still play locally. Fix the network issue and restart to share.\n", err)
	} else {
		s.P2PStatus = h.Status
		defer h.Close()
		fmt.Println("\n   ✓ Tunnel ready. To invite a friend: open the Admin screen → Invites → Create,")
		fmt.Println("     and send them the invite line it shows (it starts with LF1.).")
		fmt.Println("     Treat invites like a house key — share them privately, not in public channels.")
		if *keep {
			fmt.Println("     Your address stays the same across restarts (--keep-address). Delete host-key.json to rotate it.")
		} else {
			fmt.Println("     Your address changes every time you restart; old invites stop working.")
		}
	}
	waitSignal()
	hs.Close()
	s.Shutdown()
	return nil
}

func cmdJoin(args []string) error {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	port := fs.Int("port", 8090, "local port for your browser")
	noOpen := fs.Bool("no-browser", false, "don't open a browser automatically")
	verbose := fs.Bool("verbose", false, "verbose Tailcat networking logs")
	fs.Parse(args)
	raw := strings.Join(fs.Args(), "")
	if raw == "" {
		raw = os.Getenv("LF_INVITE")
	}
	if raw == "" {
		fmt.Print("Paste the invite your friend sent you (starts with LF1.) and press Enter:\n> ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		raw = line
	}
	inv, err := p2p.ParseInvite(raw)
	if err != nil {
		return err
	}
	fmt.Println("🔌 Connecting to your friend's world (this can take up to 20 seconds)…")
	c := p2p.NewCompanion(inv, os.Getenv("LF_DERP_MAP_URL"), *verbose)
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gi, err := c.Connect(ctx, protocol.ProtoMin, protocol.ProtoMax)
	if err != nil {
		if d := c.Diag(); d.Advice != "" {
			return fmt.Errorf("%v\n  → %s", err, d.Advice)
		}
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
	}
	url := "http://" + ln.Addr().String() + "/"
	d := c.Diag()
	fmt.Printf("✓ Connected to %q (Lanternfall %s, %d/%d players) — %.0f ms\n", gi.Name, gi.Version, gi.Players, gi.Max, d.LatencyMs)
	fmt.Printf("  Open %s in your browser to play. Keep this window open while you play.\n", url)
	go http.Serve(ln, c.Handler())
	if !*noOpen {
		openBrowser(url)
	}
	go func() {
		last := ""
		for range time.Tick(5 * time.Second) {
			d := c.Diag()
			if d.Path != last {
				last = d.Path
				switch d.Path {
				case "direct":
					fmt.Printf("  ↔ Direct peer-to-peer connection (%.0f ms)\n", d.LatencyMs)
				case "relay":
					fmt.Printf("  ↔ Using a relay%s (%.0f ms). NAT traversal couldn't find a direct path; play works with a little extra lag.\n", regionSuffix(d.RelayRegion), d.LatencyMs)
				}
			}
		}
	}()
	waitSignal()
	return nil
}

func regionSuffix(r string) string {
	if r == "" {
		return ""
	}
	return " (" + r + ")"
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func cmdBackups(args []string) error {
	env := config.FromEnv(config.ModePrivate)
	st, err := store.New(env.DataDir)
	if err != nil {
		return err
	}
	list, err := st.Backups()
	if err != nil {
		return err
	}
	fmt.Printf("Backups in %s:\n", filepath.Join(env.DataDir, "backups"))
	for _, b := range list {
		fmt.Println("  ", b)
	}
	return nil
}

func cmdRestore(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: lanternfall restore <backup-name>  (see `lanternfall backups`)")
	}
	env := config.FromEnv(config.ModePrivate)
	st, err := store.New(env.DataDir)
	if err != nil {
		return err
	}
	if err := st.Restore(args[0]); err != nil {
		return err
	}
	fmt.Println("Restored", args[0], "→ state.json. Start the world again.")
	return nil
}

func cmdMods(args []string) error {
	if len(args) != 2 || args[0] != "check" {
		return errors.New("usage: lanternfall mods check <mod-folder>")
	}
	b, err := os.ReadFile(filepath.Join(args[1], "mod.json"))
	if err != nil {
		return err
	}
	m, err := content.ParseManifest(b)
	if err != nil {
		return err
	}
	c, _ := content.Base()
	if err := c.Apply(m); err != nil {
		return err
	}
	fmt.Printf("✓ %s@%s is valid (%d items, %d recipes, %d quests)\n", m.ID, m.Version, len(m.Content.Items), len(m.Content.Recipes), len(m.Content.Quests))
	return nil
}
