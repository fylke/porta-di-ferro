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
	// CurrentMats is, for every mat of the event, the match it is on if that match is
	// this discipline's, and "" if it is another's or nobody's.
	CurrentMats(slug string) map[int]string
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

// PlacedSnapshot is the snapshot with its pools and matches on the event's mats, and
// without Mats -- which match each mat is on, which only the whole hall can say. The
// coordinator builds the hall from these, so it never asks a discipline for what the
// discipline would have to ask the hall for.
func (s *Server) PlacedSnapshot() (Snapshot, error) {
	snap, err := BuildSnapshot(source{s.store, s}, s.rules, s.self(), func(mat int) string {
		if sk := s.presence.scorekeeperOn(mat); sk != nil {
			return sk.Match
		}
		return ""
	})
	if err == nil && s.mats != nil {
		placed, mats := s.mats.Placements()
		OnEventMats(&snap, s.self().Slug, placed, mats)
	}
	return snap, err
}

// movePoolOnEventMats is the pool override from a discipline's own admin page, in an
// event: it moves the pool's work item on the event's mats instead of the pool's lane.
func (s *Server) movePoolOnEventMats(number, mat int, move string) error {
	id := fmt.Sprintf("%s/pool-%d", s.self().Slug, number)
	return s.mats.MoveItem(id, mat, -1, move)
}
