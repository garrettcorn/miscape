package game

import (
	"math"
	"time"

	"lanternfall/internal/world"
)

// ---------- tick ----------

// Tick advances simulation time by dt.
func (g *Game) Tick(now time.Time) {
	g.Now = now
	g.tick++
	g.tickMobs()
	g.tickRegen()
	g.tickFires()
	g.tickPlayers()
	g.tickDrops()
	g.tickTrades()
	if g.tick%SnapshotEvery == 0 {
		g.sendSnapshots()
	}
	for id, p := range g.Players {
		if p.dirtyMe {
			g.Out.Send(id, g.meMsg(p))
			p.dirtyMe = false
		}
	}
}

func (g *Game) tickRegen() {
	keep := g.regens[:0]
	for _, r := range g.regens {
		if g.Now.Before(r.At) {
			keep = append(keep, r)
			continue
		}
		x, y, z := int(r.P.X), int(r.P.Y), int(r.P.Z)
		if g.W.Get(x, y, z) != world.Air || g.entityInCell(x, y, z) {
			r.At = g.Now.Add(5 * time.Second)
			keep = append(keep, r)
			continue
		}
		g.setBlock(x, y, z, r.Block)
	}
	g.regens = keep
}

func (g *Game) tickPlayers() {
	for _, p := range g.Players {
		c := p.Char
		if c.HP < c.MaxHP() && g.Now.Sub(p.lastDamage) > 6*time.Second && g.Now.Sub(p.lastRegen) > 3*time.Second {
			c.HP++
			p.lastRegen = g.Now
			p.dirtyMe = true
		}
		if !p.hearthStart.IsZero() {
			if g.W.Dist(c.X, c.Z, p.hearthX, p.hearthZ) > 0.8 {
				p.hearthStart = time.Time{}
				g.toast(p, "Hearth Lantern interrupted — you moved.", "warn")
				p.dirtyMe = true
			} else if g.Now.Sub(p.hearthStart).Seconds() >= HearthSeconds {
				p.hearthStart = time.Time{}
				sp := g.W.Spawn()
				g.teleport(p, sp.X, sp.Y, sp.Z)
				g.toast(p, "The lantern's glow carries you home.", "good")
			}
		}
	}
}

func (g *Game) teleport(p *Player, x, y, z float64) {
	p.Char.X, p.Char.Y, p.Char.Z = x, y, z
	p.groundY = y
	p.lastMove = g.Now
	p.dirtyMe = true
	g.Out.Send(p.ID, M{"t": "correct", "x": x, "y": y, "z": z, "teleport": true})
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

// Snapshot entries. Each entity's entry is built once per snapshot and shared
// by every viewer that can see it (viewers only differ in what's in range and
// in the small per-viewer flags), so the cost grows with the number of
// entities rather than with players × entities.
type snapPlayer struct {
	ID    uint32  `json:"id"`
	Name  string  `json:"n"`
	Color string  `json:"c"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Yaw   float64 `json:"r"`
	HP    int     `json:"hp"`
	MaxHP int     `json:"mhp"`
	Held  string  `json:"w"`
	Light bool    `json:"l"`
}

type snapMob struct {
	ID     uint32  `json:"id"`
	Kind   string  `json:"k"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Yaw    float64 `json:"r"`
	HP     int     `json:"hp"`
	MaxHP  int     `json:"mhp"`
	Aggro  bool    `json:"a"` // targeting this viewer
	target uint32
}

type snapDrop struct {
	ID    uint32  `json:"id"`
	Item  string  `json:"i"`
	Count int     `json:"n"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Own   bool    `json:"own"` // dropped by this viewer
	owner string
}

type snapMsg struct {
	T       string       `json:"t"`
	Players []snapPlayer `json:"p"`
	Mobs    []snapMob    `json:"m"`
	Drops   []snapDrop   `json:"d"`
}

func (g *Game) sendSnapshots() {
	if len(g.Players) == 0 {
		return
	}
	players := make([]snapPlayer, 0, len(g.Players))
	for id, o := range g.Players {
		c := o.Char
		players = append(players, snapPlayer{ID: id, Name: c.Name, Color: c.Color, X: r2(c.X), Y: r2(c.Y), Z: r2(c.Z),
			Yaw: r2(c.Yaw), HP: c.HP, MaxHP: c.MaxHP(), Held: c.Equipped, Light: g.carriesLight(c)})
	}
	mobs := make([]snapMob, 0, len(g.Mobs))
	for id, m := range g.Mobs {
		if m.HP > 0 {
			mobs = append(mobs, snapMob{ID: id, Kind: m.Def.ID, X: r2(m.X), Y: r2(m.Y), Z: r2(m.Z), Yaw: r2(m.Yaw), HP: m.HP, MaxHP: m.MaxHP, target: m.target})
		}
	}
	drops := make([]snapDrop, 0, len(g.Drops))
	for id, d := range g.Drops {
		drops = append(drops, snapDrop{ID: id, Item: d.Item, Count: d.Count, X: r2(d.X), Y: r2(d.Y), Z: r2(d.Z), owner: d.Owner})
	}
	const r2max = InterestRadius * InterestRadius
	for id, p := range g.Players {
		px, pz := p.Char.X, p.Char.Z
		msg := snapMsg{T: "snap"}
		for _, o := range players {
			if o.ID != id && g.W.Dist2(o.X, o.Z, px, pz) < r2max {
				msg.Players = append(msg.Players, o)
			}
		}
		for _, m := range mobs {
			if g.W.Dist2(m.X, m.Z, px, pz) < r2max {
				m.Aggro = m.target == id
				msg.Mobs = append(msg.Mobs, m)
			}
		}
		for _, d := range drops {
			if g.W.Dist2(d.X, d.Z, px, pz) < r2max {
				d.Own = d.owner == p.Char.ID
				msg.Drops = append(msg.Drops, d)
			}
		}
		g.Out.Send(id, msg)
	}
}
