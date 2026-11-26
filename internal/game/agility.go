package game

import (
	"fmt"
	"math"
	"time"

	"lanternfall/internal/content"
)

// ---------- agility ----------
//
// Agility courses are content (base.json "courses"): a run through
// checkpoint flags in order, against the clock. The server follows the
// player's accepted (speed-checked) position, so a lap can't be faked.
// Every Agility level also adds 1% to sprint speed (see sprintSpeed).

const CheckpointRadius = 1.8

type courseRun struct {
	id    string
	next  int // index of the next checkpoint
	start time.Time
}

// sprintSpeed is how fast a character may sprint: 1% faster per Agility level.
func sprintSpeed(c *Character) float64 {
	return SprintSpeed * (1 + 0.01*float64(c.Level("agility")))
}

func (g *Game) course(id string) *content.Course {
	for _, co := range g.C.Courses {
		if co.ID == id {
			return co
		}
	}
	return nil
}

// checkCourse moves a player's course run along after each accepted move.
func (g *Game) checkCourse(p *Player) {
	c := p.Char
	at := func(pt [2]float64) bool { return g.W.Dist(c.X, c.Z, pt[0], pt[1]) <= CheckpointRadius }
	if r := p.run; r != nil {
		co := g.course(r.id)
		if co == nil {
			p.run = nil
			return
		}
		if g.Now.Sub(r.start) > time.Duration(co.LimitSec)*time.Second {
			p.run, p.dirtyMe = nil, true
			p.runCooldown = g.Now.Add(2 * time.Second)
			g.toast(p, fmt.Sprintf("Too slow for the %s. Back to the start flag to try again.", co.Name), "warn")
			return
		}
		if !at(co.Points[r.next]) {
			return
		}
		r.next++
		p.dirtyMe = true
		if r.next < len(co.Points) {
			g.toast(p, fmt.Sprintf("Checkpoint %d of %d", r.next, len(co.Points)-1), "info")
			return
		}
		secs := g.Now.Sub(r.start).Seconds()
		p.run = nil
		p.runCooldown = g.Now.Add(3 * time.Second)
		g.addXP(p, "agility", co.XP)
		g.toast(p, fmt.Sprintf("%s: lap done in %.1f seconds!", co.Name, secs), "good")
		return
	}
	if g.Now.Before(p.runCooldown) {
		return
	}
	for _, co := range g.C.Courses {
		if len(co.Points) < 2 || !at(co.Points[0]) {
			continue
		}
		if lvl := c.Level("agility"); lvl < co.Level {
			if p.runWarned != co.ID {
				p.runWarned = co.ID
				g.toast(p, fmt.Sprintf("The %s needs Agility %d (you have %d).", co.Name, co.Level, lvl), "warn")
			}
			return
		}
		p.run = &courseRun{id: co.ID, next: 1, start: g.Now}
		p.dirtyMe = true
		g.toast(p, fmt.Sprintf("%s: go! Reach every flag within %d seconds.", co.Name, co.LimitSec), "good")
		return
	}
}

// runView is the me-message course state: which course, the next flag, and
// seconds left.
func (g *Game) runView(p *Player) any {
	r := p.run
	if r == nil {
		return nil
	}
	co := g.course(r.id)
	if co == nil {
		return nil
	}
	left := time.Duration(co.LimitSec)*time.Second - g.Now.Sub(r.start)
	return M{"id": r.id, "next": r.next, "left": math.Max(0, math.Round(left.Seconds()*10)/10)}
}
