package p2p

import (
	"strings"
	"testing"

	"github.com/tailscale/tailcat"
	"tailscale.com/types/key"
)

func fakeAddr(t *testing.T) string {
	// Build a syntactically valid address offline (no network).
	k := key.NewNode()
	ci := tailcat.ConnInfo{ServerPublic: tailcat.NodePublic{NodePublic: k.Public()}, PresharedKey: tailcat.NewPresharedKey(), RegionID: 1}
	return string(ci.Addr())
}

func TestParseInvite(t *testing.T) {
	addr := fakeAddr(t)
	inv, err := ParseInvite("  LF1.MOSS-ABC2345." + addr + "\r\n")
	if err != nil || inv.Code != "MOSS-ABC2345" || string(inv.Addr) != addr {
		t.Fatalf("%v %+v", err, inv)
	}
	if _, err := ParseInvite(addr); err != nil {
		t.Fatal("bare tailcat address should be accepted", err)
	}
	for _, bad := range []string{"", "hello", "LF1.", "LF1.CODE", "LF1.CODE.tcgarbage", "LF1." + "." + addr[:10]} {
		if _, err := ParseInvite(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if _, err := ParseInvite("LF1.CODE." + addr[:len(addr)/2]); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("truncated address: %v", err)
	}
}
