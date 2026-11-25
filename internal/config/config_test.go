package config

import "testing"

func TestDefaultAddr(t *testing.T) {
	t.Setenv("LF_ADDR", "")
	t.Setenv("PORT", "")
	if got := FromEnv(ModePrivate).Addr; got != "127.0.0.1:23710" {
		t.Fatalf("default addr = %s", got)
	}
	t.Setenv("PORT", "9999")
	if got := FromEnv(ModePrivate).Addr; got != "127.0.0.1:9999" {
		t.Fatalf("PORT not honoured: %s", got)
	}
	t.Setenv("LF_ADDR", "127.0.0.1:1234")
	if got := FromEnv(ModePrivate).Addr; got != "127.0.0.1:1234" {
		t.Fatalf("LF_ADDR not honoured: %s", got)
	}
}

func TestPortsAreUncommonAndBelowEphemeralRange(t *testing.T) {
	for _, p := range []int{DefaultPort, DefaultJoinPort} {
		if p < 10000 || p >= 32768 {
			t.Fatalf("port %d should be in 10000-32767", p)
		}
	}
}
