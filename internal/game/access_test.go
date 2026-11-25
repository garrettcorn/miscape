package game

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lanternfall/internal/config"
	"lanternfall/internal/content"
	"lanternfall/internal/protocol"
	"lanternfall/internal/store"
	"lanternfall/internal/world"
)

func joinErr(g *Game, s Session, h protocol.In) string {
	h.T, h.Proto = "hello", 1
	_, _, err := g.Join(s, &h)
	var je *JoinError
	if errors.As(err, &je) {
		return je.Code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func TestInviteOnlyAccess(t *testing.T) {
	f := newFixture(t)
	f.g.Cfg.Access = "invite"
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Stranger"}); c != protocol.ErrInviteRequired {
		t.Fatal(c)
	}
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Stranger", Invite: "MOSS-NOPE123"}); c != protocol.ErrInviteInvalid {
		t.Fatal(c)
	}
	inv := f.g.CreateInvite("for Sam", 1, time.Hour)
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Sam", Invite: inv.Code}); c != "" {
		t.Fatal(c)
	}
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Sam2", Invite: inv.Code}); c != protocol.ErrInviteInvalid {
		t.Fatal("single-use invite reused: ", c)
	}
	// Host's own loopback session never needs an invite.
	if c := joinErr(f.g, Session{Local: true}, protocol.In{Name: "Host"}); c != "" {
		t.Fatal(c)
	}
	// Expiry.
	inv2 := f.g.CreateInvite("", 0, time.Hour)
	f.advance(2 * time.Hour)
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Late", Invite: inv2.Code}); c != protocol.ErrInviteInvalid {
		t.Fatal("expired invite accepted")
	}
	// Revoke.
	inv3 := f.g.CreateInvite("", 0, 0)
	f.g.RevokeInvite(inv3.Code)
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Rev", Invite: inv3.Code}); c != protocol.ErrInviteInvalid {
		t.Fatal("revoked invite accepted")
	}
}

func TestTokensBansNamesAndCapacity(t *testing.T) {
	f := newFixture(t)
	pid, tok, _ := f.g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Name: "Quinn"})
	if c := joinErr(f.g, Session{}, protocol.In{Name: "quinn"}); c != protocol.ErrNameTaken {
		t.Fatal("names must be unique case-insensitively")
	}
	if c := joinErr(f.g, Session{}, protocol.In{Token: NewToken()}); c != protocol.ErrBadToken {
		t.Fatal(c)
	}
	// Reconnect with the token replaces the old session.
	pid2, tok2, err := f.g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Token: tok})
	if err != nil || tok2 != "" || pid2 == pid || f.out.kicked[pid] != protocol.ErrReplaced {
		t.Fatalf("reconnect: %v %q %v", err, tok2, f.out.kicked)
	}
	if len(f.g.Players) != 1 {
		t.Fatal("old session should be gone")
	}
	if err := f.g.BanByName("QUINN", "griefing"); err != nil {
		t.Fatal(err)
	}
	if len(f.g.Players) != 0 {
		t.Fatal("ban should disconnect")
	}
	if c := joinErr(f.g, Session{}, protocol.In{Token: tok}); c != protocol.ErrBanned {
		t.Fatal(c)
	}
	f.g.Unban("quinn")
	if c := joinErr(f.g, Session{}, protocol.In{Token: tok}); c != "" {
		t.Fatal(c)
	}
	f.g.Cfg.MaxPlayers = 1
	if c := joinErr(f.g, Session{}, protocol.In{Name: "Extra"}); c != protocol.ErrFull {
		t.Fatal(c)
	}
}

func TestTokenIsNotStoredInPlaintext(t *testing.T) {
	f := newFixture(t)
	_, tok, _ := f.g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Name: "Rho"})
	for _, c := range f.g.Snapshot().Characters {
		if c.TokenHash == tok || c.TokenHash == "" {
			t.Fatal("token must be stored hashed")
		}
	}
}

func saveLoad(t *testing.T, g *Game) *Game {
	t.Helper()
	dir := t.TempDir()
	st, _ := store.New(dir)
	if err := st.Save(g.Snapshot()); err != nil {
		t.Fatal(err)
	}
	var d SaveData
	if _, err := st.Load(&d); err != nil {
		t.Fatal(err)
	}
	g2, err := Load(content.MustBase(), g.Cfg, &d, newRec())
	if err != nil {
		t.Fatal(err)
	}
	return g2
}

func TestSaveLoadRoundTrip(t *testing.T) {
	f := newFixture(t)
	p := f.join("Sol")
	p.Char.Inv["log"] = 7
	p.Char.XP["mining"] = 123
	f.at(p, 30, 14, 30)
	f.g.Join(Session{Local: true}, &protocol.In{T: "hello", Proto: 1, Name: "Tam"})
	p.Char.Inv["plank"] = 1
	x, z := 20, 30
	y := f.g.W.SurfaceY(x, z)
	f.g.placed[world.Index(x, y, z)] = p.Char.ID
	f.g.W.Set(x, y, z, world.Plank)
	inv := f.g.CreateInvite("x", 3, 0)
	f.g.Bans["someone"] = &Ban{CharID: "someone", Name: "Bad"}
	g2 := saveLoad(t, f.g)
	c := g2.Chars[p.Char.ID]
	if c == nil || c.Inv["log"] != 7 || c.XP["mining"] != 123 || c.X != 30 {
		t.Fatalf("character not restored: %+v", c)
	}
	if g2.W.Get(x, y, z) != world.Plank || g2.placed[world.Index(x, y, z)] != p.Char.ID {
		t.Fatal("world edits not restored")
	}
	if g2.Invites[inv.Code] == nil || g2.Bans["someone"] == nil || g2.Realm.ID != f.g.Realm.ID {
		t.Fatal("invites/bans/realm not restored")
	}
	if len(g2.Players) != 0 {
		t.Fatal("nobody is online after a restart")
	}
	if _, ok := g2.names["sol"]; !ok {
		t.Fatal("name index not rebuilt")
	}
}

func TestRealmSeparation(t *testing.T) {
	f := newFixture(t)
	f.join("Uma")
	d := f.g.Snapshot()
	// A character smuggled in from another world must be rejected.
	for _, c := range d.Characters {
		c.Realm = "some-official-realm"
	}
	if _, err := Load(content.MustBase(), f.g.Cfg, d, newRec()); err == nil {
		t.Fatal("cross-realm character accepted")
	}
	// Two private worlds get different realms; tokens don't work across them.
	other := New(content.MustBase(), f.g.Cfg, config.ModePrivate, newRec())
	if other.Realm.ID == f.g.Realm.ID {
		t.Fatal("realm ids must be unique")
	}
	_, tok, _ := f.g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Name: "Vic"})
	if c := joinErr(other, Session{}, protocol.In{Token: tok}); c != protocol.ErrBadToken {
		t.Fatal("token from another world accepted")
	}
}

func TestModdedFlagAndMissingItemsPreserved(t *testing.T) {
	c := content.MustBase()
	m, err := content.ParseManifest([]byte(`{"id":"tea","name":"Tea","version":"1.0.0","api":1,"description":"","content":{"items":[{"id":"tea:cup","name":"Cup of Tea","kind":"food","heal":3,"tradeable":true,"stack":8,"color":"#aa7744"}],"recipes":[],"quests":[],"shop":[]},"rules":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Apply(m); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultWorld(config.ModePrivate)
	cfg.Access = "open"
	g := New(c, cfg, config.ModePrivate, newRec())
	if !g.Realm.EverModded {
		t.Fatal("world should be flagged modded")
	}
	pid, _, _ := g.Join(Session{}, &protocol.In{T: "hello", Proto: 1, Name: "Wes"})
	g.Players[pid].Char.Inv["tea:cup"] = 2
	d := g.Snapshot()
	// Restart without the mod: item kept, flag kept, not tradeable.
	g2, err := Load(content.MustBase(), cfg, d, newRec())
	if err != nil {
		t.Fatal(err)
	}
	if !g2.Realm.EverModded {
		t.Fatal("modded flag must persist")
	}
	var ch *Character
	for _, x := range g2.Chars {
		ch = x
	}
	if ch.Inv["tea:cup"] != 2 {
		t.Fatal("items from disabled mods must not be deleted")
	}
}

func TestCorruptStateRecoversFromBackup(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)
	f := newFixture(t)
	f.join("Xan")
	if err := st.Save(f.g.Snapshot()); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"format":1,"sha256":"00","payload":{}}`), 0o600)
	var d SaveData
	res, err := st.Load(&d)
	if err != nil || !res.Recovered || len(d.Characters) != 1 {
		t.Fatalf("recovery failed: %v %+v", err, res)
	}
}
