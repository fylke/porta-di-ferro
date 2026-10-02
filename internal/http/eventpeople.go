package httpapi

import (
	"github.com/fylke/porta-di-ferro/internal/store"
)

// A discipline inside an event that knows its people (phase 3): every competitor entry
// points at a person in the event's registry, so somebody entered in two disciplines is
// one person with one page.

// EventPeople is the event's registry, as a discipline needs it.
type EventPeople interface {
	// PersonFor is who a new entry is: the person asked for if there is one, else the one
	// who came in on the same signup, else somebody new.
	PersonFor(name, club, submission, want string) (string, error)
}

// UsePeople links this discipline's entries to the event's people. Called once.
func (s *Server) UsePeople(p EventPeople) { s.people = p }

// personFor is PersonFor when there is an event, and nobody otherwise.
func (s *Server) personFor(name, club, submission, want string) (string, error) {
	if s.people == nil {
		return "", nil
	}
	return s.people.PersonFor(name, club, submission, want)
}

// Competitors is this discipline's entries, for the event's registry.
func (s *Server) Competitors() ([]store.Competitor, error) { return s.store.Competitors() }

// LinkPeople points entries at people: resolve says who each entry should be, and
// whether it changes. Written once, and every page told, only when something did.
func (s *Server) LinkPeople(resolve func(store.Competitor) (string, bool)) error {
	s.writeMu.Lock()
	competitors, err := s.store.Competitors()
	if err != nil {
		s.writeMu.Unlock()
		return err
	}
	changed := false
	for i, c := range competitors {
		if id, ok := resolve(c); ok && id != c.Person {
			competitors[i].Person = id
			changed = true
		}
	}
	if changed {
		err = s.store.SaveCompetitors(competitors)
	}
	s.writeMu.Unlock()
	if err == nil && changed {
		s.publishState()
	}
	return err
}
