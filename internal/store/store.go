package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// Store is one tournament's directory.
type Store struct {
	dir string

	mu sync.RWMutex
	// seen indexes the sequence numbers already on disk per match, so a retried push is
	// a no-op without a read of the whole log. The primary key is (match, seq), which is
	// what makes retries idempotent with no deduplication logic anywhere (design §3).
	seen map[string]map[int]bool
}

// Open prepares a tournament directory, creating it if it is not there yet.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "matches"), 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir, seen: map[string]map[int]bool{}}, nil
}

// Dir is the tournament directory, shown to the organizer so the hand-edit escape hatch
// is findable rather than theoretical.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

// writeAtomic replaces a file rather than writing into it, so a crash mid-write leaves
// the previous version intact instead of a half-file.
func (s *Store) writeAtomic(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	final := s.path(name)
	tmp, err := os.CreateTemp(s.dir, "."+name+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, final)
}

func (s *Store) readJSON(name string, v any) error {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Competitors reads the register. A missing file is an empty register, not an error:
// a fresh tournament directory is a valid one.
func (s *Store) Competitors() ([]Competitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Competitor
	if err := s.readJSON("competitors.json", &out); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Competitor{}, nil
		}
		return nil, err
	}
	if out == nil {
		out = []Competitor{}
	}
	return out, nil
}

// SaveCompetitors replaces the register.
func (s *Store) SaveCompetitors(c []Competitor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c == nil {
		c = []Competitor{}
	}
	return s.writeAtomic("competitors.json", c)
}

// Tournament reads the setup and the draw, falling back to the MVP defaults.
func (s *Store) Tournament() (Tournament, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := Defaults()
	if err := s.readJSON("tournament.json", &out); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Defaults(), nil
		}
		return Tournament{}, err
	}
	return out, nil
}

// SaveTournament replaces the setup and the draw.
func (s *Store) SaveTournament(t Tournament) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAtomic("tournament.json", t)
}

func matchFile(id string) (string, error) {
	// Match ids reach this from the network, and they become file names.
	if id == "" || strings.ContainsAny(id, `/\.:`) {
		return "", fmt.Errorf("invalid match id %q", id)
	}
	return filepath.Join("matches", id+".ndjson"), nil
}

// Events reads a match log, returning only the records after seq. "Everything after
// sequence N" is how a client resumes, and it is the same call the Milestone 3 mirror
// will replicate over.
func (s *Store) Events(id string, after int) ([]match.Event, error) {
	rel, err := matchFile(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Open(s.path(rel))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []match.Event{}, nil
		}
		return nil, err
	}
	defer f.Close()

	out := []match.Event{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e match.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			// A truncated final line after a crash costs one event rather than the
			// file, which is the whole reason the log is newline-delimited.
			continue
		}
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// Append writes events to a match log, skipping any sequence number already there.
// Returns the events actually written, so a caller can push exactly those to subscribers.
func (s *Store) Append(id string, events []match.Event) ([]match.Event, error) {
	rel, err := matchFile(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.seen[id]; !ok {
		index, err := s.indexLocked(rel)
		if err != nil {
			return nil, err
		}
		s.seen[id] = index
	}
	index := s.seen[id]

	fresh := make([]match.Event, 0, len(events))
	for _, e := range events {
		if e.Seq <= 0 || index[e.Seq] {
			continue
		}
		e.Match = id
		if e.At == "" {
			// design §3 records a timestamp on every event. A client always sends one;
			// this is the safety net for anything that does not, so the log is never
			// missing the data post-event statistics will want.
			e.At = time.Now().Format(time.RFC3339Nano)
		}
		fresh = append(fresh, e)
		index[e.Seq] = true
	}
	if len(fresh) == 0 {
		return fresh, nil
	}

	f, err := os.OpenFile(s.path(rel), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, e := range fresh {
		b, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return fresh, f.Sync()
}

func (s *Store) indexLocked(rel string) (map[int]bool, error) {
	index := map[int]bool{}
	f, err := os.Open(s.path(rel))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return index, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e match.Event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Seq > 0 {
			index[e.Seq] = true
		}
	}
	return index, sc.Err()
}
