package game

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/world"
)

func TestDropAndPickupConservesItems(t *testing.T) {
	f := newFixture(t)
	a := f.join("Dropper")
	f.at(a, world.SpawnX, world.SpawnY, world.SpawnZ)
	a.Char.Inv = map[string]int{"log": 5, "copper_sword": 1}
	before := f.g.TotalItems()

	f.mustOK(a, protocol.In{T: "drop", Item: "log", Count: 3})
	if a.Char.Inv["log"] != 2 || len(f.g.Drops) != 1 {
		t.Fatalf("drop: inv %v drops %d", a.Char.Inv, len(f.g.Drops))
	}
	if !reflect.DeepEqual(before, f.g.TotalItems()) {
		t.Fatalf("items not conserved after drop: %v -> %v", before, f.g.TotalItems())
	}
	// Retrying the same request (same rid) must not drop twice.
	f.g.Handle(a.ID, &protocol.In{T: "drop", Rid: f.rid, Item: "log", Count: 3})
	if a.Char.Inv["log"] != 2 {
		t.Fatal("duplicate request dropped again")
	}
	// Dropping a weapon unequips it.
	a.Char.Equipped = "copper_sword"
	f.mustOK(a, protocol.In{T: "drop", Item: "copper_sword", Count: 1})
	if a.Char.Equipped != "" {
		t.Fatal("dropped weapon still equipped")
	}

	// Standing on your own drop doesn't scoop it straight back up, even later…
	for _, d := range f.g.Drops {
		f.at(a, d.X, d.Y, d.Z)
	}
	f.g.Tick(f.g.Now)
	f.advance(10 * time.Second)
	f.g.Tick(f.g.Now)
	if len(f.g.Drops) != 2 {
		t.Fatalf("picked up without stepping away: %d drops left", len(f.g.Drops))
	}
	// …but stepping away and walking back over it does.
	f.stepAwayAndBack(a)
	if len(f.g.Drops) != 0 || a.Char.Inv["log"] != 5 || a.Char.Inv["copper_sword"] != 1 {
		t.Fatalf("pickup: inv %v drops %d", a.Char.Inv, len(f.g.Drops))
	}
	if len(f.g.Drops) != 0 || a.Char.Inv["log"] != 5 || a.Char.Inv["copper_sword"] != 1 {
		t.Fatalf("pickup: inv %v drops %d", a.Char.Inv, len(f.g.Drops))
	}
	if !reflect.DeepEqual(before, f.g.TotalItems()) {
		t.Fatalf("items not conserved after pickup: %v -> %v", before, f.g.TotalItems())
	}
	if f.out.last(a.ID, "pickup") == nil {
		t.Fatal("no pickup message")
	}
}

// stepAwayAndBack walks p clear of every drop, then onto each one in turn.
func (f *fixture) stepAwayAndBack(p *Player) {
	f.at(p, p.Char.X+10, p.Char.Y, p.Char.Z)
	f.g.Tick(f.g.Now)
	f.advance(DropPickupDelay)
	for _, d := range f.g.Drops {
		f.at(p, d.X, d.Y, d.Z)
		f.g.Tick(f.g.Now)
	}
}

func TestDropRules(t *testing.T) {
	f := newFixture(t)
	a := f.join("Rules")
	f.at(a, world.SpawnX, world.SpawnY, world.SpawnZ)
	a.Char.Inv = map[string]int{"log": 2, "warden_token": 1}
	if e := f.mustFail(a, protocol.In{T: "drop", Item: "warden_token", Count: 1}); !strings.Contains(e, "can't be dropped") {
		t.Fatalf("non-tradeable: %s", e)
	}
	f.mustFail(a, protocol.In{T: "drop", Item: "log", Count: 3})
	f.mustFail(a, protocol.In{T: "drop", Item: "glimmer_blade", Count: 1})
	if err := protocol.Validate(&protocol.In{T: "drop", Rid: 1, Item: "log", Count: 0}); err == nil {
		t.Fatal("zero count accepted")
	}
	// Drops expire.
	f.mustOK(a, protocol.In{T: "drop", Item: "log", Count: 1})
	f.at(a, world.SpawnX+20, world.SpawnY, world.SpawnZ)
	f.advance(DropLifetime)
	f.g.Tick(f.g.Now)
	if len(f.g.Drops) != 0 {
		t.Fatal("drop did not expire")
	}
}

func TestOthersCanOnlyTakeDropsWhenAllowed(t *testing.T) {
	for _, trading := range []string{"on", "off"} {
		t.Run("trading "+trading, func(t *testing.T) {
			f := newFixture(t)
			f.g.Cfg.Trading = trading
			a, b := f.join("Giver"), f.join("Taker")
			f.at(a, world.SpawnX, world.SpawnY, world.SpawnZ)
			a.Char.Inv = map[string]int{"berry": 4}
			f.mustOK(a, protocol.In{T: "drop", Item: "berry", Count: 4})
			var d *Drop
			for _, x := range f.g.Drops {
				d = x
			}
			f.at(a, world.SpawnX+20, world.SpawnY, world.SpawnZ)
			f.at(b, d.X, d.Y, d.Z)
			f.advance(5 * time.Second)
			f.g.Tick(f.g.Now)
			if b.Char.Inv["berry"] != 0 {
				t.Fatal("someone else took a fresh drop")
			}
			f.advance(DropPrivateFor)
			f.g.Tick(f.g.Now)
			want := map[string]int{"on": 4, "off": 0}[trading]
			if b.Char.Inv["berry"] != want {
				t.Fatalf("taker has %d berries, want %d", b.Char.Inv["berry"], want)
			}
		})
	}
}

func TestPickupOnlyWhatFits(t *testing.T) {
	f := newFixture(t)
	a := f.join("Packrat")
	f.at(a, world.SpawnX, world.SpawnY, world.SpawnZ)
	a.Char.Inv = map[string]int{"berry": 10}
	f.mustOK(a, protocol.In{T: "drop", Item: "berry", Count: 10})
	// Fill every slot except a partial berry stack of 30 (stack size 32).
	// (Swords stack to 1, so each uses a slot.)
	a.Char.Inv = map[string]int{"berry": 30, "wooden_sword": InventorySlots - 1}
	if f.g.slotsUsed(a.Char.Inv) != InventorySlots {
		t.Fatalf("setup: %d slots used", f.g.slotsUsed(a.Char.Inv))
	}
	before := f.g.TotalItems()
	f.stepAwayAndBack(a)
	if a.Char.Inv["berry"] != 32 {
		t.Fatalf("berries %d, want 32 (topped up the partial stack)", a.Char.Inv["berry"])
	}
	for _, d := range f.g.Drops {
		if d.Count != 8 {
			t.Fatalf("remaining drop %d, want 8", d.Count)
		}
	}
	if !reflect.DeepEqual(before, f.g.TotalItems()) {
		t.Fatalf("items not conserved: %v -> %v", before, f.g.TotalItems())
	}
}

func TestDropsSurviveSaveAndLoad(t *testing.T) {
	f := newFixture(t)
	a := f.join("Saver")
	f.at(a, world.SpawnX, world.SpawnY, world.SpawnZ)
	a.Char.Inv = map[string]int{"plank": 7}
	f.mustOK(a, protocol.In{T: "drop", Item: "plank", Count: 7})
	g2, err := Load(content.MustBase(), config.DefaultWorld(config.ModePrivate), f.g.Snapshot(), newRec())
	if err != nil {
		t.Fatal(err)
	}
	if got := g2.TotalItems()["plank"]; got != 7 || len(g2.Drops) != 1 {
		t.Fatalf("after load: %d planks on the ground, %d drops", got, len(g2.Drops))
	}
}

func TestUnknownModelRejected(t *testing.T) {
	c := content.MustBase()
	c.Items["wooden_sword"].Model = "laser"
	if err := c.Validate(); err == nil {
		t.Fatal("unknown model accepted")
	}
	c.Items["wooden_sword"].Model = "sword"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
