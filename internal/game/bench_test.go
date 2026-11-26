package game

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
)

// nullOut discards messages but still forces them to be built.
type nullOut struct{ n int }

func (o *nullOut) Send(pid uint32, msg any)          { o.n++ }
func (o *nullOut) SendBin(pid uint32, b []byte)      { o.n++ }
func (o *nullOut) Kick(pid uint32, code, msg string) {}

// jsonOut encodes every message like the real server does.
type jsonOut struct{ bytes int }

func (o *jsonOut) Send(pid uint32, msg any) {
	b, _ := json.Marshal(msg)
	o.bytes += len(b)
}
func (o *jsonOut) SendBin(pid uint32, b []byte)      {}
func (o *jsonOut) Kick(pid uint32, code, msg string) {}

// BenchmarkTick measures one simulation tick (including the 10 Hz snapshot
// fan-out) with N players spread around the village. Run with:
//
//	go test ./internal/game -bench Tick -run ^$
func BenchmarkTick(b *testing.B) {
	for _, enc := range []bool{false, true} {
		for _, n := range []int{8, 64, 256} {
			benchTick(b, n, enc)
		}
	}
}

func benchTick(b *testing.B, n int, encode bool) {
	name := fmt.Sprintf("players=%d", n)
	if encode {
		name += "/json"
	}
	b.Run(name, func(b *testing.B) {
		var out Outbox = &nullOut{}
		if encode {
			out = &jsonOut{}
		}
		cfg := config.DefaultWorld(config.ModeOfficial)
		cfg.MaxPlayers = n
		cfg.Access = "open"
		g := New(content.MustBase(), cfg, config.ModeOfficial, out)
		for i := 0; i < n; i++ {
			pid, _, err := g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Name: fmt.Sprintf("P%04d", i)})
			if err != nil {
				b.Fatal(err)
			}
			c := g.Players[pid].Char
			sp := g.W.Spawn() // spread players over a 96x96 area around town
			c.X, c.Z = g.W.WrapX(sp.X-48+float64(i%96)), sp.Z-48+float64((i*7)%96)
			c.Y = float64(max(1, g.W.SurfaceY(int(c.X), int(c.Z))))
		}
		for i := 0; i < 40; i++ {
			sp := g.W.Spawn()
			g.Drops[g.allocID()] = &Drop{Item: "log", Count: 1, X: sp.X - 40 + float64(i*2), Y: sp.Y, Z: sp.Z, Expires: g.Now.Add(time.Hour)}
		}
		if jo, ok := out.(*jsonOut); ok {
			jo.bytes = 0 // ignore the join messages
		}
		now := g.Now
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			now = now.Add(time.Second / TickRate)
			g.Tick(now)
		}
		b.StopTimer()
		b.ReportMetric(float64(b.Elapsed().Microseconds())/float64(b.N), "µs/tick")
		if jo, ok := out.(*jsonOut); ok {
			b.ReportMetric(float64(jo.bytes)/float64(b.N)/float64(n)*TickRate/1000, "KB/s/player")
		}
	})
}
