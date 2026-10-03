package httpapi

import (
	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// A discipline inside an event with staff of its own (phase 5): its staff list is the
// event's members who work it, an import puts offers to work on the event's staff, and a
// discipline's Remove takes the member off this discipline.

// EventStaff is the event's staff, as a discipline needs it.
type EventStaff interface {
	// StaffFor is the members who work a discipline.
	StaffFor(slug string) ([]store.StaffMember, error)
	// AddStaff takes the rows an import calls staff, and says how many it took.
	AddStaff(slug string, rows []signup.Row) (int, error)
	// RemoveStaff takes a member off a discipline; false when they did not work it.
	RemoveStaff(slug, id string) (bool, error)
}

// UseStaff puts this discipline's staff on the event's. Called once, before it serves.
func (s *Server) UseStaff(st EventStaff) { s.staff = st }
