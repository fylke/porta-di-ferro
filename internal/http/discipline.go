package httpapi

import (
	"sync"

	"github.com/fylke/porta-di-ferro/internal/hall"
)

// One discipline, as the rest of the application sees it (design §7 item 9).
//
// A discipline is one tournament under one ruleset, with its own folder, files and lock.
// Several used to be several runs of the application, each on its own port; they are now
// several of these Servers in one process, under one address, each mounted at
// /api/d/{slug}/ by the event coordinator (docs/proposals/one-event-many-disciplines.md
// §5, option D). Nothing in this package below the coordinator knows there is more than
// one.

// identity is the server's Instance, read and written under a lock: renaming a
// discipline writes to it while the snapshot builder is reading (issue #80).
type identity struct {
	mu   sync.Mutex
	self hall.Instance
}

// Rename changes what this discipline is called. The name is kept in its tournament.json,
// which is what makes it survive a restart (issue #80).
func (s *Server) Rename(name string) error {
	name, err := hall.CleanName(name)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	t, err := s.store.Tournament()
	if err != nil {
		s.writeMu.Unlock()
		return err
	}
	t.Discipline = name
	err = s.store.SaveTournament(t)
	s.writeMu.Unlock()
	if err != nil {
		return err
	}
	s.identity.mu.Lock()
	s.identity.self.Name = name
	s.identity.mu.Unlock()
	// Every page carries the name, so every page has to hear about it.
	s.publishState()
	return nil
}

// self is this discipline, read under the lock.
func (s *Server) self() hall.Instance {
	s.identity.mu.Lock()
	defer s.identity.mu.Unlock()
	return s.identity.self
}

// Self is which discipline this server is.
func (s *Server) Self() hall.Instance { return s.self() }
