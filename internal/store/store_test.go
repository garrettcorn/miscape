package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type doc struct {
	N    int    `json:"n"`
	Name string `json:"name"`
}

func TestSaveLoad(t *testing.T) {
	s, _ := New(t.TempDir())
	var d doc
	if _, err := s.Load(&d); !errors.Is(err, ErrNoState) {
		t.Fatalf("want ErrNoState, got %v", err)
	}
	if err := s.Save(doc{1, "a"}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Load(&d)
	if err != nil || d.N != 1 || res.Recovered {
		t.Fatal(d, err, res)
	}
}

func TestNoTempFilesLeft(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	for i := 0; i < 5; i++ {
		s.Save(doc{N: i})
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatal("temp file left behind:", e.Name())
		}
	}
}

func TestCorruptPrimaryFallsBackToNewestBackup(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	s.BackupEvery = time.Minute
	s.Save(doc{N: 1})
	clock = clock.Add(2 * time.Minute)
	s.Save(doc{N: 2})
	// Simulate a torn write / disk damage.
	cases := map[string][]byte{
		"truncated":         []byte(`{"format":1,"sha2`),
		"checksum mismatch": []byte(`{"format":1,"sha256":"deadbeef","payload":{"n":999}}`),
		"future format":     []byte(`{"format":99,"sha256":"","payload":{}}`),
	}
	for name, b := range cases {
		os.WriteFile(filepath.Join(dir, "state.json"), b, 0o600)
		var d doc
		res, err := s.Load(&d)
		if err != nil || !res.Recovered || d.N != 2 {
			t.Errorf("%s: %v %+v %+v", name, err, res, d)
		}
	}
	// Missing primary also recovers.
	os.Remove(filepath.Join(dir, "state.json"))
	var d doc
	if res, err := s.Load(&d); err != nil || !res.Recovered {
		t.Fatal(err, res)
	}
}

func TestAllCorruptIsAnError(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	s.Save(doc{N: 1})
	list, _ := s.Backups()
	for _, b := range list {
		os.WriteFile(filepath.Join(dir, "backups", b), []byte("garbage"), 0o600)
	}
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("garbage"), 0o600)
	var d doc
	if _, err := s.Load(&d); err == nil || errors.Is(err, ErrNoState) {
		t.Fatal("expected a hard error, not silently starting fresh:", err)
	}
}

func TestBackupRotation(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	s.KeepBackups, s.BackupEvery = 3, time.Second
	for i := 0; i < 8; i++ {
		clock = clock.Add(2 * time.Second)
		s.Save(doc{N: i})
	}
	list, _ := s.Backups()
	if len(list) != 3 {
		t.Fatalf("want 3 backups, got %d", len(list))
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	s.Save(doc{N: 1})
	list, _ := s.Backups()
	s.BackupEvery = time.Hour
	s.Save(doc{N: 2})
	if err := s.Restore(list[0]); err != nil {
		t.Fatal(err)
	}
	var d doc
	s.Load(&d)
	if d.N != 1 {
		t.Fatal("restore didn't take", d)
	}
	if err := s.Restore("../state.json"); err == nil {
		t.Fatal("path traversal accepted")
	}
}
