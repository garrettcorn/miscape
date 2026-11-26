package protocol

import (
	"strings"
	"testing"
)

func TestDecodeValid(t *testing.T) {
	good := []string{
		`{"t":"hello","proto":1,"name":"Wren","color":"#aabbcc"}`,
		`{"t":"hello","proto":1,"token":"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ"}`,
		`{"t":"move","x":1.5,"y":13,"z":2,"yaw":0.3}`,
		`{"t":"gather","rid":1,"bx":3,"by":12,"bz":4}`,
		`{"t":"craft","rid":2,"recipe":"wooden_sword"}`,
		`{"t":"trade_offer","rid":3,"trade":9,"offer":{"log":3,"mymod:thing":1}}`,
		`{"t":"chunks","chunks":[[0,0],[1,2]]}`,
		`{"t":"chat","rid":4,"text":"hello there 👋"}`,
		`{"t":"equip","rid":5,"item":""}`,
	}
	for _, s := range good {
		if _, err := Decode([]byte(s)); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	bad := map[string]string{
		"unknown type":       `{"t":"teleport","x":1}`,
		"unknown field":      `{"t":"move","x":1,"admin":true}`,
		"missing rid":        `{"t":"craft","recipe":"planks"}`,
		"bad name":           `{"t":"hello","proto":1,"name":"<script>"}`,
		"short name":         `{"t":"hello","proto":1,"name":"ab"}`,
		"bad token":          `{"t":"hello","proto":1,"token":"short"}`,
		"negative block":     `{"t":"gather","rid":1,"bx":1,"by":2,"bz":-3}`,
		"bad item id":        `{"t":"place","rid":1,"bx":1,"by":2,"bz":3,"item":"../../etc"}`,
		"huge count":         `{"t":"buy","rid":1,"item":"berry","count":100000}`,
		"zero count":         `{"t":"buy","rid":1,"item":"berry","count":0}`,
		"negative offer":     `{"t":"trade_offer","rid":1,"trade":1,"offer":{"coin":-5}}`,
		"too many offers":    `{"t":"trade_offer","rid":1,"trade":1,"offer":{"a":1,"b":1,"c":1,"d":1,"e":1,"f":1,"g":1,"h":1,"i":1}}`,
		"control chars":      `{"t":"chat","rid":1,"text":"hi\u0007"}`,
		"empty chat":         `{"t":"chat","rid":1,"text":"   "}`,
		"too many chunks":    `{"t":"chunks","chunks":[` + strings.Repeat(`[0,0],`, 16) + `[0,0]]}`,
		"trailing data":      `{"t":"ping"}{"t":"ping"}`,
		"not json":           `hello`,
		"huge number":        `{"t":"move","x":1e300}`,
		"proto out of range": `{"t":"hello","proto":0,"name":"Wren"}`,
	}
	for name, s := range bad {
		if _, err := Decode([]byte(s)); err == nil {
			t.Errorf("%s: accepted %s", name, s)
		}
	}
	big := `{"t":"chat","rid":1,"text":"` + strings.Repeat("a", MaxMessageBytes) + `"}`
	if _, err := Decode([]byte(big)); err == nil {
		t.Error("oversize message accepted")
	}
	long := `{"t":"chat","rid":1,"text":"` + strings.Repeat("é", MaxChatLen+1) + `"}`
	if _, err := Decode([]byte(long)); err == nil {
		t.Error("over-long chat accepted")
	}
}

func TestCompatible(t *testing.T) {
	if !Compatible(Current) || Compatible(ProtoMax+1) || Compatible(ProtoMin-1) {
		t.Fatal("compat range wrong")
	}
}

func TestRidRequiredForStateChanges(t *testing.T) {
	for typ, need := range types {
		if need != NeedsRid(typ) {
			t.Fatal(typ)
		}
	}
	for _, typ := range []string{"gather", "attack", "craft", "trade_confirm", "buy", "sell", "place", "break"} {
		if !NeedsRid(typ) {
			t.Errorf("%s must require a rid", typ)
		}
	}
}

func TestChunkFrame(t *testing.T) {
	// Chunk coordinates beyond 255 (level 1 is 192x96 chunks; later levels are larger).
	f := ChunkFrame(300, 1000, []byte{9, 1})
	cx, cz := int(f[1])|int(f[2])<<8, int(f[3])|int(f[4])<<8
	if f[0] != BinChunk || cx != 300 || cz != 1000 || len(f) != 7 || f[5] != 9 {
		t.Fatal(f)
	}
	if _, err := Decode([]byte(`{"t":"chunks","chunks":[[191,95],[12000,6000]]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode([]byte(`{"t":"gather","rid":1,"bx":-1,"by":13,"bz":700}`)); err != nil {
		t.Fatal("a block just west of the date line should be accepted:", err)
	}
}
