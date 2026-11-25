package game

import (
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

type rec struct {
	msgs   map[uint32][]M
	bins   map[uint32]int
	kicked map[uint32]string
}

func newRec() *rec {
	return &rec{msgs: map[uint32][]M{}, bins: map[uint32]int{}, kicked: map[uint32]string{}}
}

func (r *rec) Send(pid uint32, msg any) {
	if m, ok := msg.(M); ok {
		r.msgs[pid] = append(r.msgs[pid], m)
	}
}
func (r *rec) SendBin(pid uint32, b []byte)      { r.bins[pid]++ }
func (r *rec) Kick(pid uint32, code, msg string) { r.kicked[pid] = code }

func (r *rec) last(pid uint32, t string) M {
	ms := r.msgs[pid]
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i]["t"] == t {
			return ms[i]
		}
	}
	return nil
}

type fixture struct {
	t   *testing.T
	g   *Game
	out *rec
	rid uint32
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	cfg := config.DefaultWorld(config.ModePrivate)
	cfg.Access = "open"
	out := newRec()
	g := New(content.MustBase(), cfg, config.ModePrivate, out)
	g.Now = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	return &fixture{t: t, g: g, out: out}
}

func (f *fixture) join(name string) *Player {
	f.t.Helper()
	pid, tok, err := f.g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Name: name})
	if err != nil {
		f.t.Fatalf("join %s: %v", name, err)
	}
	if tok == "" {
		f.t.Fatal("expected token for new character")
	}
	return f.g.Players[pid]
}

func (f *fixture) at(p *Player, x, y, z float64) {
	p.Char.X, p.Char.Y, p.Char.Z = x, y, z
	p.groundY = y
}

func (f *fixture) advance(d time.Duration) { f.g.Now = f.g.Now.Add(d) }

// do sends a request through Handle (exercising rid handling) and returns the ack.
func (f *fixture) do(p *Player, m protocol.In) M {
	f.t.Helper()
	f.rid++
	if m.Rid == 0 {
		m.Rid = f.rid
	}
	if err := protocol.Validate(&m); err != nil {
		f.t.Fatalf("invalid test message %+v: %v", m, err)
	}
	f.g.Handle(p.ID, &m)
	return f.out.last(p.ID, "ack")
}

func (f *fixture) mustOK(p *Player, m protocol.In) {
	f.t.Helper()
	if a := f.do(p, m); a == nil || a["ok"] != true {
		f.t.Fatalf("%s failed: %v", m.T, a)
	}
}

func (f *fixture) mustFail(p *Player, m protocol.In) string {
	f.t.Helper()
	a := f.do(p, m)
	if a == nil || a["ok"] != false {
		f.t.Fatalf("%s should fail, got %v", m.T, a)
	}
	return a["err"].(string)
}

// findBlock returns the first block of type b satisfying pred.
func (f *fixture) findBlock(b uint8, pred func(x, y, z int) bool) (int, int, int) {
	for y := 1; y < world.SizeY; y++ {
		for z := 0; z < world.SizeZ; z++ {
			for x := 0; x < world.SizeX; x++ {
				if f.g.W.Get(x, y, z) == b && (pred == nil || pred(x, y, z)) {
					return x, y, z
				}
			}
		}
	}
	f.t.Fatalf("no block %d found", b)
	return 0, 0, 0
}

// standNear puts the player on the ground next to block (x,y,z).
func (f *fixture) standNear(p *Player, x, y, z int) {
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {2, 0}, {0, 2}} {
		sx, sz := x+d[0], z+d[1]
		sy := f.g.W.SurfaceY(sx, sz)
		px, pz := float64(sx)+0.5, float64(sz)+0.5
		if !f.g.bodyBlocked(px, float64(sy), pz) && f.g.eyeDist(&Player{Char: &Character{X: px, Y: float64(sy), Z: pz}}, x, y, z) <= ReachBlocks {
			f.at(p, px, float64(sy), pz)
			return
		}
	}
	// Fall back to hovering right next to it; reach is what matters for these tests.
	f.at(p, float64(x)+1.5, float64(y), float64(z)+0.5)
}

func wild(x, _, z int) bool {
	return world.ZoneAt(float64(x)+0.5, float64(z)+0.5) == world.ZoneWild
}
