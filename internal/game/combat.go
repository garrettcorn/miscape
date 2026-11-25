package game

import (
	"fmt"
	"math"
	"time"

	"lanternfall/internal/world"
)

const (
	FistDamage    = 1
	FistCooldown  = 700 * time.Millisecond
	MobRespawn    = 20 * time.Second
	MobMeleeRange = 1.6
	MobLeash      = 16.0
)

func (g *Game) weapon(c *Character) (dmg int, cd time.Duration) {
	if it := g.C.Items[c.Equipped]; it != nil && it.Kind == "weapon" && c.Inv[c.Equipped] > 0 {
		return it.Damage, time.Duration(it.Cooldown) * time.Millisecond
	}
	return FistDamage, FistCooldown
}

// PlayerDamage is the damage a character deals per hit.
func (g *Game) PlayerDamage(c *Character) int {
	d, _ := g.weapon(c)
	return d + c.Level("combat")/2
}

func (g *Game) broadcastNear(x, z float64, msg M) {
	for id, p := range g.Players {
		if dist2(p.Char.X, p.Char.Z, x, z) < InterestRadius*InterestRadius {
			g.Out.Send(id, msg)
		}
	}
}

// Attack hits a mob, or another player when PvP is enabled.
func (g *Game) Attack(p *Player, target uint32) error {
	if err := g.alive(p); err != nil {
		return err
	}
	if g.Now.Before(p.nextAttack) {
		return fail("Too soon.")
	}
	c := p.Char
	dmg, cd := g.weapon(c)
	dmg += c.Level("combat") / 2
	if m := g.Mobs[target]; m != nil {
		if m.HP <= 0 {
			return fail("It's already gone.")
		}
		if math.Hypot(m.X-c.X, m.Z-c.Z) > MeleeRange || math.Abs(m.Y-c.Y) > 2.5 {
			return fail("Out of reach.")
		}
		p.nextAttack = g.Now.Add(cd)
		p.hearthStart = time.Time{}
		m.HP -= dmg
		m.lastHitBy = p.ID
		m.target = p.ID
		m.hitFlash = 3
		g.broadcastNear(m.X, m.Z, M{"t": "hit", "src": p.ID, "dst": m.ID, "dmg": dmg, "mob": true})
		if m.HP <= 0 {
			g.killMob(m, p)
		}
		return nil
	}
	o := g.Players[target]
	if o == nil || o == p {
		return fail("No target.")
	}
	if !g.Cfg.PvP {
		return fail("PvP is off in this world.")
	}
	if world.ZoneAt(c.X, c.Z) == world.ZoneVillage || world.ZoneAt(o.Char.X, o.Char.Z) == world.ZoneVillage {
		return fail("Wickhollow is a safe zone.")
	}
	if o.dead || o.Char.HP <= 0 {
		return fail("They're already down.")
	}
	if math.Hypot(o.Char.X-c.X, o.Char.Z-c.Z) > MeleeRange || math.Abs(o.Char.Y-c.Y) > 2.5 {
		return fail("Out of reach.")
	}
	p.nextAttack = g.Now.Add(cd)
	g.damagePlayer(o, dmg, p.ID)
	return nil
}

func (g *Game) killMob(m *Mob, killer *Player) {
	m.HP = 0
	m.deadUntil = g.Now.Add(MobRespawn)
	m.target = 0
	g.broadcastNear(m.X, m.Z, M{"t": "die", "id": m.ID, "mob": true})
	g.addXP(killer, "combat", m.Def.XP)
	g.progress(killer, "kill", m.Def.ID, 1)
	loot := map[string]int{}
	for _, d := range m.Def.Drops {
		if g.rng.Float64() <= d.Chance {
			n := d.Min
			if d.Max > d.Min {
				n += g.rng.IntN(d.Max - d.Min + 1)
			}
			if n > 0 {
				loot[d.Item] += n
			}
		}
	}
	for id, n := range loot {
		if g.grant(killer, map[string]int{id: n}) {
			g.toast(killer, fmt.Sprintf("+%d %s", n, g.itemName(id)), "item")
		} else {
			g.toast(killer, "Your pack is full — some loot was lost.", "warn")
		}
	}
}

func (g *Game) damagePlayer(p *Player, dmg int, src uint32) {
	c := p.Char
	c.HP -= dmg
	p.lastDamage = g.Now
	p.dirtyMe = true
	g.Dirty = true
	if !p.hearthStart.IsZero() {
		p.hearthStart = time.Time{}
		g.toast(p, "Hearth Lantern interrupted!", "warn")
	}
	g.broadcastNear(c.X, c.Z, M{"t": "hit", "src": src, "dst": p.ID, "dmg": dmg})
	if c.HP <= 0 {
		g.playerDied(p)
	}
}

func (g *Game) playerDied(p *Player) {
	c := p.Char
	zone := world.ZoneAt(c.X, c.Z)
	lost := 0
	if zone == world.ZoneGloam {
		lost = int(math.Floor(float64(c.Inv["coin"]) * g.C.Rules.GloamDeathCoinLoss))
		if lost > 0 {
			c.Inv["coin"] -= lost
		}
	}
	if p.trade != nil {
		g.cancelTrade(p.trade, c.Name+" was knocked out.")
	}
	for _, m := range g.Mobs {
		if m.target == p.ID {
			m.target = 0
		}
	}
	c.HP = c.MaxHP()
	p.hearthStart = time.Time{}
	g.broadcastNear(c.X, c.Z, M{"t": "die", "id": p.ID})
	g.teleport(p, world.SpawnX, world.SpawnY, world.SpawnZ)
	msg := "You were knocked out and woke up in Wickhollow."
	if lost > 0 {
		msg += fmt.Sprintf(" You dropped %d coins in the Gloam.", lost)
	}
	g.Out.Send(p.ID, M{"t": "ko", "text": msg, "lost": lost})
	g.SaveSoon = true
}

func (g *Game) tickMobs() {
	dt := 1.0 / TickRate
	for _, m := range g.Mobs {
		if m.HP <= 0 {
			if !m.deadUntil.IsZero() && g.Now.After(m.deadUntil) {
				g.respawnMob(m)
			}
			continue
		}
		if m.hitFlash > 0 {
			m.hitFlash--
		}
		// Acquire / drop targets.
		var tp *Player
		if m.target != 0 {
			tp = g.Players[m.target]
			if tp == nil || tp.dead || world.ZoneAt(tp.Char.X, tp.Char.Z) == world.ZoneVillage ||
				math.Hypot(tp.Char.X-m.Spawn.X, tp.Char.Z-m.Spawn.Z) > MobLeash+m.Spawn.R {
				m.target, tp = 0, nil
			}
		}
		if tp == nil {
			best := m.Def.Aggro
			for id, p := range g.Players {
				if p.dead || world.ZoneAt(p.Char.X, p.Char.Z) == world.ZoneVillage {
					continue
				}
				if d := math.Hypot(p.Char.X-m.X, p.Char.Z-m.Z); d < best && math.Abs(p.Char.Y-m.Y) < 4 {
					best, tp, m.target = d, p, id
				}
			}
		}
		var gx, gz float64
		speed := m.Def.Speed
		if tp != nil {
			gx, gz = tp.Char.X, tp.Char.Z
			d := math.Hypot(gx-m.X, gz-m.Z)
			if d <= MobMeleeRange && math.Abs(tp.Char.Y-m.Y) < 2 {
				speed = 0
				if g.Now.After(m.nextAtk) {
					m.nextAtk = g.Now.Add(time.Duration(m.Def.CooldownMs) * time.Millisecond)
					dmg := int(math.Round(float64(m.Def.Damage) * g.C.Rules.MobDamageMult))
					if dmg > 0 {
						g.damagePlayer(tp, dmg, m.ID)
					}
				}
			}
		} else {
			if g.Now.After(m.nextThink) {
				a := g.rng.Float64() * 2 * math.Pi
				r := g.rng.Float64() * m.Spawn.R
				m.wanderTo = [2]float64{m.Spawn.X + math.Cos(a)*r + 0.5, m.Spawn.Z + math.Sin(a)*r + 0.5}
				m.nextThink = g.Now.Add(time.Duration(2000+g.rng.IntN(4000)) * time.Millisecond)
			}
			gx, gz = m.wanderTo[0], m.wanderTo[1]
			speed *= 0.4
		}
		dx, dz := gx-m.X, gz-m.Z
		d := math.Hypot(dx, dz)
		if d > 0.2 && speed > 0 {
			step := math.Min(d, speed*dt)
			nx, nz := m.X+dx/d*step, m.Z+dz/d*step
			m.Yaw = math.Atan2(dx, dz)
			if world.ZoneAt(nx, nz) == world.ZoneVillage {
				continue // creatures never enter the village
			}
			// Gloam creatures stay in the Gloam and vice versa.
			if (world.ZoneAt(nx, nz) == world.ZoneGloam) != (world.ZoneAt(m.Spawn.X, m.Spawn.Z) == world.ZoneGloam) {
				continue
			}
			ny := float64(g.W.GroundY(int(math.Floor(nx)), int(math.Floor(nz))))
			if ny >= 0 && ny-m.Y <= 1.01 && ny-m.Y >= -3 {
				m.X, m.Z, m.Y = nx, nz, ny
			} else if tp == nil {
				m.nextThink = g.Now // pick another wander target
			}
		}
	}
}
