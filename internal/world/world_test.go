package world

import (
	"bytes"
	"testing"
)

func TestBuildDeterministic(t *testing.T) {
	a, b := Build(1), Build(1)
	if !bytes.Equal(a.Blocks, b.Blocks) {
		t.Fatal("world build must be deterministic")
	}
}

func TestChunkRoundTrip(t *testing.T) {
	w := Build(1)
	total := 0
	for cz := 0; cz < ChunksZ; cz++ {
		for cx := 0; cx < ChunksX; cx++ {
			enc := w.EncodeChunk(cx, cz)
			total += len(enc)
			dec, ok := DecodeChunk(enc)
			if !ok {
				t.Fatalf("chunk %d,%d decode failed", cx, cz)
			}
			i := 0
			for y := 0; y < SizeY; y++ {
				for z := 0; z < Chunk; z++ {
					for x := 0; x < Chunk; x++ {
						if dec[i] != w.Get(cx*Chunk+x, y, cz*Chunk+z) {
							t.Fatalf("mismatch at chunk %d,%d", cx, cz)
						}
						i++
					}
				}
			}
		}
	}
	t.Logf("whole world encodes to %d KB in RLE (raw %d KB)", total/1024, len(w.Blocks)/1024)
	if total > 300*1024 {
		t.Errorf("world encoding unexpectedly large: %d bytes", total)
	}
}

func clear(w *World, x, y, z float64) bool {
	return !w.SolidAt(x, y+0.1, z) && !w.SolidAt(x, y+1.5, z) && w.SolidAt(x, y-0.1, z)
}

func TestKeyLocationsArePlayable(t *testing.T) {
	w := Build(20261125)
	if !clear(w, SpawnX, SpawnY, SpawnZ) {
		t.Fatal("spawn point is blocked or floating")
	}
	// NPC standing spots (must match base.json).
	for _, p := range [][3]float64{{60.5, 13, 61.5}, {69.5, 13, 67.5}, {95.5, 13, 61.5}} {
		if !clear(w, p[0], p[1], p[2]) {
			t.Errorf("NPC spot %v blocked", p)
		}
	}
	// The Gloam gate is passable.
	for x := 96.5; x < 103; x++ {
		if w.SolidAt(x, 13.1, 63.5) || w.SolidAt(x, 14.5, 63.5) {
			t.Fatalf("gate blocked at x=%.1f", x)
		}
	}
	if Get := w.Get(AnvilX, AnvilY, AnvilZ); Get != Anvil {
		t.Fatal("anvil missing")
	}
}

func TestZones(t *testing.T) {
	if ZoneAt(64, 64) != ZoneVillage || ZoneAt(110, 64) != ZoneGloam || ZoneAt(30, 30) != ZoneWild {
		t.Fatal("zones wrong")
	}
}

func TestResourceCounts(t *testing.T) {
	w := Build(20261125)
	count := map[uint8]int{}
	for _, b := range w.Blocks {
		count[b]++
	}
	t.Logf("logs=%d copper=%d glimmer=%d berries=%d deadwood=%d", count[Log], count[CopperOre], count[Glimmer], count[BerryBush], count[Deadwood])
	if count[Log] < 100 || count[CopperOre] < 20 || count[Glimmer] < 20 || count[BerryBush] < 5 {
		t.Fatal("not enough resources for the progression loop")
	}
}
