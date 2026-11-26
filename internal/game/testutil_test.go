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

// The tests run on the real level-1 map. These helpers find reference spots
// there (relative to the spawn town), so tests don't hard-code coordinates.

// town returns the spawn town's center, offset by (dx,dz).
func (f *fixture) town(dx, dz float64) (x, z float64) {
	x, _, z, _ = f.g.W.Locate(f.g.W.Def.Spawn)
	return f.g.W.WrapX(x + dx), z + dz
}

// wildSpot is the center of a clear, dry, flat patch of countryside near
// town (at least 3x3 open ground), plus (dx, dz).
func (f *fixture) wildSpot(dx, dz float64) (x, y, z float64) {
	w := f.g.W
	cx, cz := f.town(0, 0)
	stand := w.Def.GroundY + 1
	clear := func(x, z int) bool {
		return w.ZoneAt(float64(x)+0.5, float64(z)+0.5) == world.ZoneWild && !f.g.wet(x, z) &&
			w.GroundY(x, z) == stand && w.SurfaceY(x, z) == stand && w.Get(x, stand, z) == world.Air
	}
	for r := 22; r < 90; r++ {
		for _, d := range [][2]int{{0, r}, {r, 0}, {-r, 0}, {0, -r}, {r, r}, {-r, r}} {
			x, z := int(cx)+d[0], int(cz)+d[1]
			ok := true
			for ox := -1; ox <= 1 && ok; ox++ {
				for oz := -1; oz <= 1 && ok; oz++ {
					ok = clear(x+ox, z+oz)
				}
			}
			if ok {
				return w.WrapX(float64(x) + 0.5 + dx), float64(stand), float64(z) + 0.5 + dz
			}
		}
	}
	f.t.Fatal("no clear wild spot near town")
	return
}

// atWild puts p on the clear wild patch, offset by (dx, dz).
func (f *fixture) atWild(p *Player, dx, dz float64) {
	x, y, z := f.wildSpot(dx, dz)
	f.at(p, x, y, z)
}

// atNPC puts p next to an NPC.
func (f *fixture) atNPC(p *Player, id string) {
	n := f.g.C.NPCs[id]
	f.at(p, n.X, n.Y, n.Z+1)
}

// danger returns the center of the first danger zone.
func (f *fixture) danger() (x, y, z float64) {
	for _, zd := range f.g.W.Def.Zones {
		if zd.Kind == world.ZoneDanger {
			return zd.X, float64(f.g.W.Def.GroundY + 1), zd.Z
		}
	}
	f.t.Fatal("no danger zone")
	return
}

// findBlock returns the nearest block of type b to town satisfying pred,
// searching up to 120 blocks out.
func (f *fixture) findBlock(b uint8, pred func(x, y, z int) bool) (int, int, int) {
	cx, cz := f.town(0, 0)
	for r := 0; r < 120; r++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dz)) != r {
					continue
				}
				x, z := int(cx)+dx, int(cz)+dz
				for y := 1; y < world.SizeY; y++ {
					if f.g.W.Get(x, y, z) == b && (pred == nil || pred(x, y, z)) {
						return int(f.g.W.WrapX(float64(x))), y, z
					}
				}
			}
		}
	}
	f.t.Fatalf("no block %d found near town", b)
	return 0, 0, 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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

// wild reports whether block (x,y,z) is in the countryside (not a town or danger zone).
func (f *fixture) wild(x, _, z int) bool {
	return f.g.W.ZoneAt(float64(x)+0.5, float64(z)+0.5) == world.ZoneWild
}

// atSpawn puts p at the spawn point in town.
func (f *fixture) atSpawn(p *Player) {
	sp := f.g.W.Spawn()
	f.at(p, sp.X, sp.Y, sp.Z)
}
