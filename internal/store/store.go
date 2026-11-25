// Package store persists world snapshots durably: checksummed envelopes,
// write-temp-fsync-rename atomic replacement, rotating backups, and
// recovery from the newest valid backup when the primary file is damaged.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	FormatVersion = 1
	stateFile     = "state.json"
	backupDir     = "backups"
)

// ErrNoState means no snapshot exists yet (a brand new world).
var ErrNoState = errors.New("no saved state")

type envelope struct {
	Format  int             `json:"format"`
	SavedAt time.Time       `json:"savedAt"`
	SHA256  string          `json:"sha256"`
	Payload json.RawMessage `json:"payload"`
}

type Store struct {
	Dir         string
	KeepBackups int
	// BackupEvery limits how often a save is also copied to backups/.
	BackupEvery time.Duration
	lastBackup  time.Time
	now         func() time.Time
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, backupDir), 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, KeepBackups: 10, BackupEvery: 5 * time.Minute, now: time.Now}, nil
}

// Save atomically replaces state.json with v. The previous file is never
// truncated in place, so a crash mid-save leaves either old or new content.
func (s *Store) Save(v any) error {
	b, err := Encode(v, s.now())
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(filepath.Join(s.Dir, stateFile), b, 0o600); err != nil {
		return err
	}
	if s.lastBackup.IsZero() || s.now().Sub(s.lastBackup) >= s.BackupEvery {
		if err := s.backup(b); err != nil {
			return fmt.Errorf("state saved, but backup failed: %w", err)
		}
		s.lastBackup = s.now()
	}
	return nil
}

func (s *Store) backup(b []byte) error {
	name := "state-" + s.now().UTC().Format("20060102T150405.000Z") + ".json"
	if err := WriteFileAtomic(filepath.Join(s.Dir, backupDir, name), b, 0o600); err != nil {
		return err
	}
	list, _ := s.Backups()
	for len(list) > s.KeepBackups {
		os.Remove(filepath.Join(s.Dir, backupDir, list[0]))
		list = list[1:]
	}
	return nil
}

// Backups lists backup file names, oldest first.
func (s *Store) Backups() ([]string, error) {
	ents, err := os.ReadDir(filepath.Join(s.Dir, backupDir))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "state-") && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// LoadResult describes where state came from.
type LoadResult struct {
	Source    string // "state.json" or "backups/<name>"
	Recovered bool   // true if the primary was missing/corrupt and a backup was used
	Problem   string // what was wrong with the primary, if anything
}

// Load decodes the newest valid snapshot into v.
func (s *Store) Load(v any) (LoadResult, error) {
	primary := filepath.Join(s.Dir, stateFile)
	err := loadFile(primary, v)
	if err == nil {
		return LoadResult{Source: stateFile}, nil
	}
	problem := err.Error()
	primaryMissing := errors.Is(err, fs.ErrNotExist)
	list, _ := s.Backups()
	for i := len(list) - 1; i >= 0; i-- {
		if loadFile(filepath.Join(s.Dir, backupDir, list[i]), v) == nil {
			return LoadResult{Source: backupDir + "/" + list[i], Recovered: true, Problem: problem}, nil
		}
	}
	if primaryMissing && len(list) == 0 {
		return LoadResult{}, ErrNoState
	}
	return LoadResult{Problem: problem}, fmt.Errorf("no valid snapshot found (primary: %s)", problem)
}

func loadFile(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := Decode(b, v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// Encode wraps v in the checksummed snapshot envelope. It does no I/O, so it
// is shared by the file store and the in-browser world (IndexedDB).
func Encode(v any, now time.Time) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode state: %w", err)
	}
	sum := sha256.Sum256(payload)
	return json.Marshal(envelope{Format: FormatVersion, SavedAt: now.UTC(), SHA256: hex.EncodeToString(sum[:]), Payload: payload})
}

// Decode verifies an envelope (format and checksum) and decodes its payload.
func Decode(b []byte, v any) error {
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return fmt.Errorf("corrupt envelope: %w", err)
	}
	if env.Format != FormatVersion {
		return fmt.Errorf("unsupported format %d", env.Format)
	}
	sum := sha256.Sum256(env.Payload)
	if hex.EncodeToString(sum[:]) != env.SHA256 {
		return errors.New("checksum mismatch")
	}
	return json.Unmarshal(env.Payload, v)
}

// Restore copies a named backup over state.json (used by `lanternfall restore`).
func (s *Store) Restore(name string) error {
	if name != filepath.Base(name) || !strings.HasPrefix(name, "state-") {
		return errors.New("invalid backup name")
	}
	b, err := os.ReadFile(filepath.Join(s.Dir, backupDir, name))
	if err != nil {
		return err
	}
	var probe json.RawMessage
	if err := loadFile(filepath.Join(s.Dir, backupDir, name), &probe); err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(s.Dir, stateFile), b, 0o600)
}

// WriteFileAtomic writes data to a temp file in the same directory, fsyncs it,
// and renames it over path.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	if d, err := os.Open(dir); err == nil { // best effort dir fsync (no-op on Windows)
		d.Sync()
		d.Close()
	}
	return nil
}
