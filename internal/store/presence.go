package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// The files behind score keeper handover and server-assigned displays (design §7 items
// 4 and 10).
//
//	writers.json                     which device is writing which match, and the epoch
//	displays.json                    what each server-assigned screen has been told to show
//	matches/<match_id>.quarantine.ndjson  late events from a device that lost its match
//
// The connected-client registry itself is not here: who is alive is a fact about now,
// and a restart forgets it and lets everyone register again. What has to survive a
// restart is who is allowed to write, so a stale device cannot slip its backlog in while
// the server is coming up, and what each screen shows, so a hall of displays does not
// need reassigning after the organizer's laptop reboots.

// Writer is the one device allowed to write a match, and the epoch of its tenure. A
// handover -- graceful or not -- bumps the epoch, and events stamped with an older one
// are quarantined rather than appended (design §7 item 10).
type Writer struct {
	Client string `json:"client"`
	Epoch  int    `json:"epoch"`
}

// Writers reads the claims. A missing file is no claims.
func (s *Store) Writers() (map[string]Writer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]Writer{}
	if err := s.readJSON("writers.json", &out); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if out == nil {
		out = map[string]Writer{}
	}
	return out, nil
}

// SaveWriters replaces the claims.
func (s *Store) SaveWriters(w map[string]Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAtomic("writers.json", w)
}

// Displays reads what each server-assigned screen shows, by client id.
func (s *Store) Displays() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]string{}
	if err := s.readJSON("displays.json", &out); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

// SaveDisplays replaces the screen assignments.
func (s *Store) SaveDisplays(d map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAtomic("displays.json", d)
}

// Quarantined is one event a device wrote after its match had been handed to another.
// Kept, with who wrote it and when it arrived, because "shown to the organizer" is the
// whole point: silently dropping a score keeper's work is the failure this exists to
// prevent.
type Quarantined struct {
	Match      string      `json:"match"`
	Client     string      `json:"client"`
	ClientName string      `json:"clientName"`
	ReceivedAt string      `json:"receivedAt"`
	Event      match.Event `json:"event"`
}

func quarantineFile(id string) (string, error) {
	rel, err := matchFile(id)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(rel, ".ndjson") + ".quarantine.ndjson", nil
}

// Quarantine appends late events to a match's quarantine log.
func (s *Store) Quarantine(id, client, clientName string, events []match.Event) error {
	rel, err := quarantineFile(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path(rel), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	now := time.Now().Format(time.RFC3339)
	for _, e := range events {
		e.Match = id
		b, err := json.Marshal(Quarantined{Match: id, Client: client, ClientName: clientName, ReceivedAt: now, Event: e})
		if err != nil {
			return err
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// Quarantined lists every quarantined event in the tournament, oldest first.
func (s *Store) Quarantined() ([]Quarantined, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.path("matches"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Quarantined{}, nil
		}
		return nil, err
	}
	out := []Quarantined{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".quarantine.ndjson") {
			continue
		}
		f, err := os.Open(s.path("matches/" + entry.Name()))
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			var q Quarantined
			if json.Unmarshal(sc.Bytes(), &q) == nil {
				out = append(out, q)
			}
		}
		f.Close()
	}
	return out, nil
}

// DiscardQuarantine removes a match's quarantine log once the organizer has dealt with it.
func (s *Store) DiscardQuarantine(id string) error {
	rel, err := quarantineFile(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
