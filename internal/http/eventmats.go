package httpapi

import (
	"fmt"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// A discipline inside an event with shared mats (phase 2): its pools and bracket run on
// the event's mats, where the plan puts them, and the devices at those mats belong to the
// event rather than to any one discipline.

// EventMats is the event's plan, as a discipline needs it.
type EventMats interface {
	// Placements is where every item in the event runs, and how many mats there are.
	Placements() (map[string]store.Placement, int)
	// MoveItem moves one item: to a mat at an index, or a step "up" or "down".
	MoveItem(id string, mat, index int, move string) error
}

// UseMats puts this discipline's work on the event's mats. Called once, before it serves.
func (s *Server) UseMats(m EventMats) { s.mats = m }

// ReleaseClaims lets go of every match a device holds in this discipline: the device left
// on purpose, from the event's mats.
func (s *Server) ReleaseClaims(client string) {
	s.writeMu.Lock()
	s.releaseAllClaims(client)
	s.writeMu.Unlock()
}

// onEventMats is the snapshot speaking the event's mats, when there is an event.
func (s *Server) onEventMats(snap *Snapshot) {
	if s.mats == nil {
		return
	}
	placed, mats := s.mats.Placements()
	OnEventMats(snap, s.self().Slug, placed, mats)
}

// movePoolOnEventMats is the pool override from a discipline's own admin page, in an
// event: it moves the pool's work item on the event's mats instead of the pool's lane.
func (s *Server) movePoolOnEventMats(number, mat int, move string) error {
	id := fmt.Sprintf("%s/pool-%d", s.self().Slug, number)
	return s.mats.MoveItem(id, mat, -1, move)
}
