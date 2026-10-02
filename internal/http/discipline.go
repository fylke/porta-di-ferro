package httpapi

import (
	"errors"
	"regexp"
	"strings"
	"sync"
)

// One discipline, as the rest of the application sees it (design §7 item 9).
//
// A discipline is one tournament under one ruleset, with its own folder, files and lock.
// Several used to be several runs of the application, each on its own port; they are now
// several of these Servers in one process, under one address, each mounted at
// /api/d/{slug}/ by the event coordinator (docs/proposals/one-event-many-disciplines.md
// §5, option D). Nothing in this package below the coordinator knows there is more than
// one.

// Instance describes which discipline a server is, so every page can say it.
type Instance struct {
	// Name is the discipline: "Open steel Longsword". Empty on one not named yet.
	Name string `json:"name"`
	// Slug is the discipline's address in the event: /api/d/{slug}/, /d/{slug}/. It never
	// changes once made, whatever the discipline is renamed to.
	Slug string `json:"slug,omitempty"`
	Dir  string `json:"dir"`
	// URL is where the discipline's own pages are, relative to the event's address.
	URL string `json:"url"`
}

// identity is the server's Instance, read and written under a lock: renaming a
// discipline writes to it while the snapshot builder is reading (issue #80).
type identity struct {
	mu   sync.Mutex
	self Instance
}

var badName = regexp.MustCompile(`[^\p{L}\p{N} _-]+`)

// Disciplines is the list an organizer picks from rather than types out (issue #80). The
// first one is what a discipline that has not been named yet is offered.
//
// Preloaded rather than configurable: these are the four MSL disciplines, they are spelled
// the same way on every entry list, and a volunteer typing "Womens longsword" at one event
// and "Women's and underrepresented genders Longsword" at the next makes two disciplines
// out of one. Nothing stops them typing their own -- the list is a starting point, not a
// closed set.
var Disciplines = []string{
	"Open steel Longsword",
	"Women's and underrepresented genders Longsword",
	"Open Sabre",
	"Open foam Longsword",
}

// CleanName is the shared check on a discipline's name: something readable, and short
// enough to sit in a page header beside the mat number.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a discipline needs a name")
	}
	if len([]rune(name)) > 60 {
		return "", errors.New("that name is too long to fit on the screens")
	}
	if strings.TrimSpace(badName.ReplaceAllString(name, "")) == "" {
		return "", errors.New("the name needs some letters or digits in it")
	}
	return name, nil
}

// Rename changes what this discipline is called. The name is kept in its tournament.json,
// which is what makes it survive a restart (issue #80).
func (s *Server) Rename(name string) error {
	name, err := CleanName(name)
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
func (s *Server) self() Instance {
	s.identity.mu.Lock()
	defer s.identity.mu.Unlock()
	return s.identity.self
}

// Self is which discipline this server is.
func (s *Server) Self() Instance { return s.self() }
