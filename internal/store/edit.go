package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// Full history editing (design §7 item 1) is the one deliberate exception to the log
// being append-only. The score keeper's undo appends a correction; the organizer's
// editor rewrites the log, because a log the organizer has corrected should read as the
// match that happened, not as the match plus a trail of what was thought to have
// happened. What makes that safe is that nothing is lost: the version being replaced is
// kept beside the log as a dated backup, every time, and the organizer can put it back by
// hand.

// ReplaceEvents rewrites a match log wholesale, keeping the previous version as
// <id>.ndjson.<timestamp>.bak. Returns the backup's file name, or "" if there was
// nothing to back up.
func (s *Store) ReplaceEvents(id string, events []match.Event) (string, error) {
	rel, err := matchFile(id)
	if err != nil {
		return "", err
	}
	seqs := map[int]bool{}
	for _, e := range events {
		if e.Seq <= 0 {
			return "", fmt.Errorf("event sequence numbers must be positive, got %d", e.Seq)
		}
		if seqs[e.Seq] {
			return "", fmt.Errorf("sequence number %d appears twice", e.Seq)
		}
		seqs[e.Seq] = true
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })

	s.mu.Lock()
	defer s.mu.Unlock()

	backup := ""
	final := s.path(rel)
	if _, err := os.Stat(final); err == nil {
		backup = filepath.Base(final) + "." + time.Now().Format("20060102-150405.000") + ".bak"
		if err := copyFile(final, filepath.Join(filepath.Dir(final), backup)); err != nil {
			return "", err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	// Written whole to a temporary file and renamed into place, like the JSON files: a
	// crash mid-edit leaves either the old log or the new one, never half of one.
	tmp, err := os.CreateTemp(filepath.Dir(final), "."+filepath.Base(final)+".*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	w := bufio.NewWriter(tmp)
	for _, e := range events {
		e.Match = id
		if e.At == "" {
			e.At = time.Now().Format(time.RFC3339Nano)
		}
		b, err := json.Marshal(e)
		if err != nil {
			tmp.Close()
			return "", err
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, final); err != nil {
		return "", err
	}
	// The idempotency index was built from the old log. Rebuilt on the next append.
	delete(s.seen, id)
	return backup, nil
}

// Backups lists the dated backups kept for a match, newest first.
func (s *Store) Backups(id string) ([]string, error) {
	rel, err := matchFile(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Dir(s.path(rel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	prefix := filepath.Base(rel) + "."
	out := []string{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".bak") {
			out = append(out, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}
