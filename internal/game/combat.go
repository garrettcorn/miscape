package game

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"lanternfall/internal/content"
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

// weaponStyle is the combat skill the equipped weapon uses (melee for fists).
func (g *Game) weaponStyle(c *Character) string {
	if it := g.C.Items[c.Equipped]; it != nil && it.Kind == "weapon" && c.Inv[c.Equipped] > 0 {
		return styleOf(it)
	}
	return "melee"
}

// PlayerDamage is the damage a character deals per hit: the weapon, plus
// half the level of the skill it uses.
func (g *Game) PlayerDamage(c *Character) int {
	d, _ := g.weapon(c)
	return d + c.Level(g.weaponStyle(c))/2
}

func (g *Game) broadcastNear(x, z float64, msg M) {
	for id, p := range g.Players {
		if g.W.Dist2(p.Char.X, p.Char.Z, x, z) < InterestRadius*InterestRadius {
			g.Out.Send(id, msg)
		}
	}
}

// Attack hits a mob, or another player when PvP is enabled. Melee weapons
// reach MeleeRange; range and magic weapons shoot up to their reach with a
// clear line of sight, using up one piece of ammunition per shot.
func (g *Game) Attack(p *Player, target uint32) error {
	if err := g.alive(p); err != nil {
		return err
	}
	if g.Now.Before(p.nextAttack) {
		return fail("Too soon.")
	}
	c := p.Char
	it := g.weaponItem(c)
	dmg, cd := g.weapon(c)
	dmg += c.Level(styleOf(it)) / 2
	ranged := it != nil && styleOf(it) != "melee" && it.Reach > 0

	var tx, ty, tz float64
	m := g.Mobs[target]
	var o *Player
	if m != nil {
		if m.HP <= 0 {
			return fail("It's already gone.")
		}
		if err := g.meets(c, m.Def.Needs); err != nil {
			return err
		}
		tx, ty, tz = m.X, m.Y, m.Z
	} else {
		o = g.Players[target]
		if o == nil || o == p {
			return fail("No target.")
		}
		if !g.Cfg.PvP {
			return fail("PvP is off in this world.")
		}
		if g.W.ZoneAt(o.Char.X, o.Char.Z) == world.ZoneTown {
			return fail("Towns are safe: no fighting here.")
		}
		if o.dead || o.Char.HP <= 0 {
			return fail("They're already down.")
		}
		tx, ty, tz = o.Char.X, o.Char.Y, o.Char.Z
	}
	if (ranged || o != nil) && g.W.ZoneAt(c.X, c.Z) == world.ZoneTown {
		if o == nil {
			return fail("No shooting in town: step out into the countryside first.")
		}
		return fail("Towns are safe: no fighting here.")
	}
	ammo := ""
	if ranged {
		if math.Hypot(g.W.Dist(tx, tz, c.X, c.Z), ty-c.Y) > it.Reach {
			return fail("Out of range.")
		}
		if !g.lineClear(c.X, c.Y+1.5, c.Z, tx, ty+0.8, tz) {
			return fail("You don't have a clear shot.")
		}
		if ammo = g.pickAmmo(c, it.AmmoKind); ammo == "" {
			return fail("You've nothing to shoot: the %s needs %s.", it.Name, g.ammoNames(it.AmmoKind))
		}
	} else if g.W.Dist(tx, tz, c.X, c.Z) > MeleeRange || math.Abs(ty-c.Y) > 2.5 {
		return fail("Out of reach.")
	}

	p.nextAttack = g.Now.Add(cd)
	p.hearthStart = time.Time{}
	if ranged {
		c.Inv[ammo]--
		if c.Inv[ammo] <= 0 {
			delete(c.Inv, ammo)
		}
		p.dirtyMe, g.Dirty = true, true
		dmg += g.C.Items[ammo].Ammo.Damage
		g.broadcastNear(c.X, c.Z, M{"t": "shot", "src": p.ID, "dst": target, "kind": it.AmmoKind, "color": g.C.Items[ammo].Color})
	}
	if o != nil {
		g.damagePlayer(o, dmg, p.ID)
		return nil
	}
	if g.hasEffect(p, "bane:"+m.Def.ID) {
		dmg *= 2
	}
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

// weaponItem is the equipped weapon, if the character still has it.
func (g *Game) weaponItem(c *Character) *content.Item {
	if it := g.C.Items[c.Equipped]; it != nil && it.Kind == "weapon" && c.Inv[c.Equipped] > 0 {
		return it
	}
	return nil
}

// pickAmmo chooses the strongest ammunition of a kind the character carries.
func (g *Game) pickAmmo(c *Character, kind string) string {
	best, bd := "", -1
	for id, n := range c.Inv {
		it := g.C.Items[id]
		if n < 1 || it == nil || it.Ammo == nil || it.Ammo.Kind != kind {
			continue
		}
		if it.Ammo.Damage > bd || (it.Ammo.Damage == bd && id < best) {
			best, bd = id, it.Ammo.Damage
		}
	}
	return best
}

// ammoNames lists the items that serve as a kind of ammunition, for messages.
func (g *Game) ammoNames(kind string) string {
	var names []string
	for _, id := range sortedItemIDs(g.C.Items) {
		if a := g.C.Items[id].Ammo; a != nil && a.Kind == kind {
			names = append(names, g.C.Items[id].Name)
		}
	}
	return strings.Join(names, " or ")
}

func sortedItemIDs(m map[string]*content.Item) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// lineClear reports whether nothing solid lies between two points (the
// ends themselves aside), so shots can't pass through walls or hills.
func (g *Game) lineClear(ax, ay, az, bx, by, bz float64) bool {
	dx, dy, dz := g.W.DX(ax, bx), by-ay, bz-az
	d := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if d < 0.5 {
		return true
	}
	n := int(d / 0.2)
	for i := 2; i < n-1; i++ {
		t := float64(i) / float64(n)
		x, y, z := int(math.Floor(ax+dx*t)), int(math.Floor(ay+dy*t)), int(math.Floor(az+dz*t))
		if b := g.W.Get(x, y, z); world.Solid(b) && b != world.Leaves {
			return false
		}
	}
	return true
}

func (g *Game) killMob(m *Mob, killer *Player) {
	m.HP = 0
	m.deadUntil = g.Now.Add(MobRespawn)
	m.target = 0
	g.broadcastNear(m.X, m.Z, M{"t": "die", "id": m.ID, "mob": true})
	skill := m.Def.Skill
	if skill == "" {
		skill = g.weaponStyle(killer.Char)
	}
	g.addXP(killer, skill, int(math.Round(float64(m.Def.XP)*levelScale(m.Level, 0.5))))
	g.progress(killer, "kill", m.Def.ID, 1)
	g.bountyKill(killer, m)
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
			g.progress(killer, "gather", id, n)
			g.toast(killer, fmt.Sprintf("+%d %s", n, g.itemName(id)), "item")
		} else {
			g.toast(killer, "Your pack is full — some loot was lost.", "warn")
		}
	}
}

func (g *Game) damagePlayer(p *Player, dmg int, src uint32) {
	c := p.Char
	if w := p.effects["ward"]; g.Now.Before(w.until) {
		dmg = max(1, int(math.Round(float64(dmg)*(1-float64(w.power)/100))))
	}
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
	zone, zoneName := g.W.ZoneAt(c.X, c.Z), g.W.ZoneName(c.X, c.Z)
	lost := 0
	if zone == world.ZoneDanger {
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
	sp := g.W.Spawn()
	if tp, ok := g.townSpawn(g.nearestVisited(c, c.X, c.Z)); ok {
		sp = tp
	}
	g.teleport(p, sp.X, sp.Y, sp.Z)
	msg := fmt.Sprintf("You were knocked out and woke up in %s.", g.W.ZoneName(sp.X, sp.Z))
	if lost > 0 {
		msg += fmt.Sprintf(" You dropped %d coins on %s.", lost, zoneName)
	}
	g.Out.Send(p.ID, M{"t": "ko", "text": msg, "lost": lost})
	g.SaveSoon = true
}

func (g *Game) tickMobs() {
	dt := 1.0 / TickRate
	for _, m := range g.Mobs {
		if m.HP <= 0 {
			if !m.deadUntil.IsZero() && g.Now.After(m.deadUntil) {
				if m.Wild {
					delete(g.Mobs, m.ID) // roaming creatures don't come back; others will appear
				} else {
					g.respawnMob(m)
				}
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
			if tp == nil || tp.dead || g.W.ZoneAt(tp.Char.X, tp.Char.Z) == world.ZoneTown ||
				g.W.Dist(tp.Char.X, tp.Char.Z, m.Spawn.X, m.Spawn.Z) > MobLeash+m.Spawn.R ||
				g.warded(m, tp.Char.X, tp.Char.Y, tp.Char.Z, FireWard) ||
				(m.Def.Flees && g.W.Dist(tp.Char.X, tp.Char.Z, m.X, m.Z) > m.Def.Aggro+4) {
				m.target, tp = 0, nil
			}
		}
		if tp == nil {
			best := m.Def.Aggro
			for id, p := range g.Players {
				if p.dead || g.W.ZoneAt(p.Char.X, p.Char.Z) == world.ZoneTown || g.warded(m, p.Char.X, p.Char.Y, p.Char.Z, FireWard) {
					continue
				}
				if d := g.W.Dist(m.X, m.Z, p.Char.X, p.Char.Z); d < best && math.Abs(p.Char.Y-m.Y) < 4 {
					best, tp, m.target = d, p, id
				}
			}
		}
		var gx, gz float64
		speed := m.Def.Speed
		if tp != nil && m.Def.Flees {
			// Game animals bolt directly away from whoever comes near.
			ax, az := g.W.DX(tp.Char.X, m.X), m.Z-tp.Char.Z
			if d := math.Hypot(ax, az); d > 0.01 {
				ax, az = ax/d, az/d
			} else {
				ax, az = 1, 0
			}
			gx, gz = m.X+ax*4, m.Z+az*4
			if g.W.Dist(gx, gz, m.Spawn.X, m.Spawn.Z) > MobLeash+m.Spawn.R {
				gx, gz = m.X-az*4, m.Z+ax*4 // cornered at the edge of its range: dodge sideways
			}
		} else if tp != nil {
			gx, gz = tp.Char.X, tp.Char.Z
			d := g.W.Dist(m.X, m.Z, gx, gz)
			if d <= MobMeleeRange && math.Abs(tp.Char.Y-m.Y) < 2 {
				speed = 0
				if g.Now.After(m.nextAtk) {
					m.nextAtk = g.Now.Add(time.Duration(m.Def.CooldownMs) * time.Millisecond)
					dmg := int(math.Round(float64(m.Def.Damage) * g.C.Rules.MobDamageMult * levelScale(m.Level, 0.35)))
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
		dx, dz := g.W.DX(m.X, gx), gz-m.Z
		d := math.Hypot(dx, dz)
		if d > 0.2 && speed > 0 {
			step := math.Min(d, speed*dt)
			nx, nz := m.X+dx/d*step, m.Z+dz/d*step
			m.Yaw = math.Atan2(dx, dz)
			nx = g.W.WrapX(nx)
			if g.W.ZoneAt(nx, nz) == world.ZoneTown {
				continue // creatures never enter towns
			}
			if g.warded(m, nx, m.Y, nz, FireWard-1) {
				continue // and fire-fearing ones keep clear of campfires
			}
			// Creatures of a danger zone stay in it, and others stay out.
			if (g.W.ZoneAt(nx, nz) == world.ZoneDanger) != (g.W.ZoneAt(m.Spawn.X, m.Spawn.Z) == world.ZoneDanger) {
				continue
			}
			ix, iz := int(math.Floor(nx)), int(math.Floor(nz))
			ny := float64(g.W.GroundY(ix, iz))
			if ny >= 0 && ny-m.Y <= 1.01 && ny-m.Y >= -3 && !g.wet(ix, iz) {
				m.X, m.Z, m.Y = nx, nz, ny
			} else if tp == nil {
				m.nextThink = g.Now // pick another wander target
			}
		}
	}
}

// wet reports whether column (x,z) is open water (creatures don't swim).
func (g *Game) wet(x, z int) bool {
	return world.IsWater(g.W.Get(x, g.W.Def.GroundY, z))
}
