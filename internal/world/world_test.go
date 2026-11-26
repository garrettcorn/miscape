package world_test

import (
	"bytes"
	"math"
	"testing"

	"lanternfall/internal/world"
	"lanternfall/maps"
)

func level(t testing.TB, name string) *world.World {
	t.Helper()
	d, err := world.LoadDef(maps.FS, "earth", name)
	if err != nil {
		t.Fatal(err)
	}
	return world.New(d)
}

func TestLevel1IsEarthSized(t *testing.T) {
	w := level(t, "level1")
	if w.W != 3072 || w.H != 1536 || w.CX != 192 || w.CZ != 96 {
		t.Fatalf("size %dx%d (%dx%d chunks)", w.W, w.H, w.CX, w.CZ)
	}
	m := w.Def.Place("manchester")
	// 53.48N 2.24W on a 3072x1536 equirectangular grid.
	if math.Abs(m.X-1517) > 1 || math.Abs(m.Z-312) > 1 {
		t.Fatalf("Manchester at %.1f,%.1f", m.X, m.Z)
	}
	if w.ZoneAt(m.X, m.Z) != world.ZoneTown || w.ZoneName(m.X, m.Z) != "Manchester" {
		t.Fatal("Manchester isn't a town zone")
	}
	moors := w.Def.Zones[0]
	if w.ZoneAt(moors.X, moors.Z) != world.ZoneDanger {
		t.Fatal("the moors aren't dangerous")
	}
	if w.ZoneAt(100, 700) != world.ZoneWild {
		t.Fatal("the Pacific should be wild")
	}
	// Spot-check the map: the middle of the Pacific is ocean, Greenland is ice,
	// the Sahara is desert.
	for _, c := range []struct {
		lat, lon float64
		want     world.Terrain
	}{{0, -150, world.TOcean}, {75, -40, world.TIce}, {24, 15, world.TDesert}} {
		x, z := w.Def.LatLon(c.lat, c.lon)
		if got := w.Terrain(int(x), int(z)); got != c.want {
			t.Errorf("%v,%v: %s, want %s", c.lat, c.lon, got, c.want)
		}
	}
}

func TestTownStructure(t *testing.T) {
	w := level(t, "level1")
	sp := w.Spawn()
	if sp.Y != 13 || w.ZoneAt(sp.X, sp.Z) != world.ZoneTown {
		t.Fatalf("spawn %+v", sp)
	}
	if w.SolidAt(sp.X, sp.Y, sp.Z) || !w.SolidAt(sp.X, sp.Y-0.5, sp.Z) {
		t.Fatal("spawn should be standing room on solid ground")
	}
	a, err := w.Point("anvil")
	if err != nil {
		t.Fatal(err)
	}
	if w.Get(int(a.X), int(a.Y), int(a.Z)) != world.Anvil {
		t.Fatal("no anvil at the anvil point")
	}
	// The gate faces the moors, which are north of Manchester.
	g, _ := w.Point("town:manchester:gate")
	m := w.Def.Place("manchester")
	if !(g.Z < m.Z-15 && math.Abs(w.DX(m.X, g.X)) < 2) {
		t.Fatalf("gate at %+v, town at %.1f,%.1f", g, m.X, m.Z)
	}
	if w.ZoneAt(g.X, g.Z) == world.ZoneDanger {
		t.Fatal("the gate is inside the danger zone")
	}
}

func TestWrapsEastWest(t *testing.T) {
	w := level(t, "level1")
	if d := w.DX(3070, 2); d != 4 {
		t.Fatalf("DX across the date line = %v", d)
	}
	if d := w.DX(2, 3070); d != -4 {
		t.Fatalf("DX back = %v", d)
	}
	if a, b := w.Dist2(3071.5, 500, 0.5, 503), w.Dist2(0.5, 503, 3071.5, 500); a != b || a != 1+9 {
		t.Fatalf("Dist2 %v %v", a, b)
	}
	if x := w.WrapX(-0.25); x != 3071.75 {
		t.Fatalf("WrapX = %v", x)
	}
	for y := 0; y < world.SizeY; y++ {
		for z := 400; z < 420; z++ {
			if w.Get(-1, y, z) != w.Get(w.W-1, y, z) || w.Get(w.W, y, z) != w.Get(0, y, z) {
				t.Fatalf("x doesn't wrap at y=%d z=%d", y, z)
			}
		}
	}
	// Chunk -1 is the last chunk.
	if !bytes.Equal(w.EncodeChunk(-1, 30), w.EncodeChunk(w.CX-1, 30)) {
		t.Fatal("chunk x doesn't wrap")
	}
}

func TestPolesAreWalls(t *testing.T) {
	w := level(t, "level1")
	if !w.SolidAt(100, 20, -0.5) || !w.SolidAt(100, 20, float64(w.H)+0.5) {
		t.Fatal("past the poles must be solid")
	}
	if w.Get(100, 20, 1) != world.Ice {
		t.Fatal("expected an ice cliff at the north pole")
	}
}

func TestGenerationIsDeterministicAcrossChunkSeams(t *testing.T) {
	a, b := level(t, "level1"), level(t, "level1")
	m := a.Def.Place("manchester")
	cx, cz := int(m.X)/world.Chunk, int(m.Z)/world.Chunk
	for dz := -3; dz <= 3; dz++ {
		for dx := -3; dx <= 3; dx++ {
			if !bytes.Equal(a.EncodeChunk(cx+dx, cz+dz), b.EncodeChunk(cx+dx, cz+dz)) {
				t.Fatalf("chunk %d,%d differs between builds", cx+dx, cz+dz)
			}
		}
	}
	// A block reads the same whether its chunk was generated first or a
	// neighbor was (features straddle chunk edges).
	c := level(t, "level1")
	for z := int(m.Z) - 40; z < int(m.Z)+40; z += 3 {
		for x := int(m.X) - 60; x < int(m.X)+60; x += 3 {
			for y := 12; y < 20; y++ {
				if a.Get(x, y, z) != c.Get(x, y, z) {
					t.Fatalf("block %d,%d,%d depends on generation order", x, y, z)
				}
			}
		}
	}
	// Somewhere in the forests there are trees.
	x, z := a.Def.LatLon(-3, -60) // Amazon
	logs := 0
	for dz := 0; dz < 40; dz++ {
		for dx := 0; dx < 40; dx++ {
			if a.Get(int(x)+dx, 13, int(z)+dz) == world.Log {
				logs++
			}
		}
	}
	if logs < 20 {
		t.Fatalf("only %d tree trunks in a patch of rainforest", logs)
	}
}

func TestEditsAreSparseAndReapply(t *testing.T) {
	w := level(t, "level1")
	sp := w.Spawn()
	x, z := int(sp.X)+3, int(sp.Z)+3
	before := w.Get(x, 13, z)
	w.Set(x, 13, z, world.Plank)
	w.Set(-5, 13, 700, world.Cobble) // across the date line: stored canonically
	if w.NumEdits() != 2 || w.Get(x, 13, z) != world.Plank || w.Get(w.W-5, 13, 700) != world.Cobble {
		t.Fatal("edit not applied")
	}
	edits := w.Edits()
	// A fresh world plus the saved edits is identical.
	w2 := level(t, "level1")
	for _, e := range edits {
		w2.Set(e[0], e[1], e[2], uint8(e[3]))
	}
	cx, cz := x/world.Chunk, z/world.Chunk
	if !bytes.Equal(w.EncodeChunk(cx, cz), w2.EncodeChunk(cx, cz)) {
		t.Fatal("edits don't reproduce the chunk")
	}
	// Restoring the generated block removes the edit.
	w.Set(x, 13, z, before)
	if w.NumEdits() != 1 {
		t.Fatalf("edits = %d", w.NumEdits())
	}
}

func TestChunkCacheIsBounded(t *testing.T) {
	w := level(t, "level1")
	w.Set(5, 13, 700, world.Plank)
	for i := 0; i < world.MaxCachedChunks+500; i++ {
		w.EncodeChunk(i%w.CX, 20+i/w.CX)
	}
	if n := w.CachedChunks(); n > world.MaxCachedChunks {
		t.Fatalf("%d chunks cached", n)
	}
	if w.Get(5, 13, 700) != world.Plank {
		t.Fatal("an edit was lost when its chunk was evicted")
	}
}

func TestLevel2InheritsAndDoubles(t *testing.T) {
	l1, l2 := level(t, "level1"), level(t, "level2")
	if l2.W != 2*l1.W || l2.H != 2*l1.H || l2.Def.Level != 2 {
		t.Fatalf("level 2 is %dx%d", l2.W, l2.H)
	}
	m1, m2 := l1.Def.Place("manchester"), l2.Def.Place("manchester")
	if m2 == nil || math.Abs(m2.X-2*m1.X) > 2 || math.Abs(m2.Z-2*m1.Z) > 2 {
		t.Fatalf("Manchester at level 2: %+v", m2)
	}
	if a, err := l2.Point("anvil"); err != nil || l2.Get(int(a.X), int(a.Y), int(a.Z)) != world.Anvil {
		t.Fatal("level 2 should have Manchester's town too")
	}
	// The same Earth, twice as detailed: terrain mostly agrees at 2x.
	same, n := 0, 0
	for z := 100; z < l1.H-100; z += 37 {
		for x := 0; x < l1.W; x += 41 {
			n++
			if l1.Terrain(x, z) == l2.Terrain(2*x+1, 2*z+1) {
				same++
			}
		}
	}
	if float64(same)/float64(n) < 0.9 {
		t.Fatalf("only %d/%d samples agree between levels", same, n)
	}
}

func TestChunkRoundTrip(t *testing.T) {
	w := level(t, "level1")
	m := w.Def.Place("manchester")
	data := w.EncodeChunk(int(m.X)/world.Chunk, int(m.Z)/world.Chunk)
	blocks, ok := world.DecodeChunk(data)
	if !ok {
		t.Fatal("decode failed")
	}
	x0, z0 := int(m.X)/world.Chunk*world.Chunk, int(m.Z)/world.Chunk*world.Chunk
	for y := 0; y < world.SizeY; y++ {
		for z := 0; z < world.Chunk; z++ {
			for x := 0; x < world.Chunk; x++ {
				if blocks[(y*world.Chunk+z)*world.Chunk+x] != w.Get(x0+x, y, z0+z) {
					t.Fatal("round trip mismatch")
				}
			}
		}
	}
	if len(data) > 1500 {
		t.Fatalf("town chunk encodes to %d bytes", len(data))
	}
}

func BenchmarkGenChunk(b *testing.B) {
	w := level(b, "level1")
	m := w.Def.Place("manchester")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Distinct chunks so the cache doesn't help.
		w.EncodeChunk(int(m.X)/world.Chunk+i%150, (int(m.Z)/world.Chunk+i/150)%(w.CZ-2)+1)
	}
}

func TestRegionsNameTheCountryside(t *testing.T) {
	for _, lvl := range []string{"level1", "level2"} {
		w := level(t, lvl)
		for _, c := range []struct {
			lat, lon float64
			want     string
		}{
			{48.8, 2.3, "the European countryside"},          // Paris
			{55.7, 37.6, "the European countryside"},         // Moscow
			{40.0, -77.0, "the North American countryside"},  // Pennsylvania
			{-15.8, -47.9, "the South American countryside"}, // Brasília
			{24, 15, "the African countryside"},              // Sahara
			{30.0, 31.2, "the African countryside"},          // Cairo
			{24.7, 46.7, "the Asian countryside"},            // Riyadh
			{39.9, 116.4, "the Asian countryside"},           // Beijing
			{-25, 134, "the Australian countryside"},         // Alice Springs
			{72, -40, "the North American countryside"},      // Greenland
			{30, -40, "the Atlantic Ocean"},
			{0, -150, "the Pacific Ocean"},
			{-20, 75, "the Indian Ocean"},
			{35, 18, "the Mediterranean Sea"},
		} {
			x, z := w.Def.LatLon(c.lat, c.lon)
			if got := w.ZoneName(x, z); got != c.want {
				t.Errorf("%s %v,%v: %q, want %q", lvl, c.lat, c.lon, got, c.want)
			}
		}
		// Ground a town keeps dry, where the map has sea, is still on land.
		g, _ := w.Point("town:manchester:gate")
		if got := w.ZoneName(g.X, g.Z-1); got != "the European countryside" {
			t.Errorf("%s: just outside the north gate is %q", lvl, got)
		}
	}
}
