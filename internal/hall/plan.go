package hall

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// Work items and the event's mats (docs/proposals/one-event-many-disciplines.md §9,
// phase 2).
//
// Mats belong to the event. Every mat has one queue, and its entries are work items from
// any discipline: Longsword pool 3, then Sabre pool 1, then the Longsword final. The item
// at the head is what the mat is running, so a mat "belongs" to a discipline exactly as
// long as one of its items is at the head -- there is no lease to hand back.
//
// The boundary is this. A discipline says what its items are and how the matches inside
// each are ordered: the pool's running order, back-to-back avoidance and the bracket are
// untouched. The plan says which mat runs each item and in what order. Everything here is
// a pure function over a discipline's stored tournament or its snapshot, so the
// coordinator and the browser demo run the same code.

// MaxMats bounds the hall. Four mats per discipline is the draw's own limit; an event of
// several shares more.
const MaxMats = 8

// The kinds of work item.
const (
	ItemPool         = "pool"
	ItemEliminations = "eliminations"
	ItemBronze       = "bronze"
	ItemFinal        = "final"
)

// WorkItem is one unit of mat time.
type WorkItem struct {
	Discipline string `json:"discipline"`
	// Key names the item within its discipline: "pool-3", "elim-1", "bronze", "final".
	Key  string `json:"key"`
	Kind string `json:"kind"`
	// Number is the pool's number, or the lane of an eliminations item.
	Number int `json:"number,omitempty"`
	// Lane is the discipline's own mat for the item -- where it would run on its own,
	// and where it lands on the event's mats until somebody moves it.
	Lane  int    `json:"lane"`
	Stamp string `json:"stamp,omitempty"`
	// Matches are the item's match ids, in the order its mat runs them.
	Matches []string `json:"matches"`
	// Projected says the item is not drawn yet: what the draw would make (projected.go).
	Projected bool `json:"projected,omitempty"`
	// Feeders are, for each bracket match, the matches that decide who fences in it, by
	// id: what the forecast waits for (#120).
	Feeders map[string][]string `json:"-"`
}

// ID is the item's name across the event: "open-sabre/pool-3".
func (w WorkItem) ID() string { return w.Discipline + "/" + w.Key }

// ItemKeyFor is the item a match belongs to. Pools are one item each. The bracket is the
// eliminations on each of the discipline's lanes, and then the bronze match and the final
// as items of their own: finals are normally held back and moved on their own (§15).
func ItemKeyFor(m store.Match) string {
	switch {
	case m.Pool > 0:
		return fmt.Sprintf("pool-%d", m.Pool)
	case m.Round == tournament.RoundBronze:
		return ItemBronze
	case m.Round == tournament.RoundFinal:
		return ItemFinal
	default:
		lane := m.Mat
		if lane < 1 {
			lane = 1
		}
		return fmt.Sprintf("elim-%d", lane)
	}
}

// ItemsOf is a discipline's work items, read off its stored tournament: the pools in
// their running order, then the bracket's.
func ItemsOf(slug string, t store.Tournament) []WorkItem {
	var out []WorkItem
	for _, p := range tournament.RunOrder(t) {
		ids := make([]string, 0, len(p.Matches))
		for _, m := range p.Matches {
			ids = append(ids, m.ID)
		}
		out = append(out, WorkItem{
			Discipline: slug, Key: fmt.Sprintf("pool-%d", p.Number), Kind: ItemPool,
			Number: p.Number, Lane: p.Mat, Stamp: t.GeneratedAt, Matches: ids,
		})
	}
	var bracket []*WorkItem
	byKey := map[string]*WorkItem{}
	for _, m := range t.Bracket {
		key := ItemKeyFor(m)
		it, ok := byKey[key]
		if !ok {
			it = &WorkItem{Discipline: slug, Key: key, Lane: max(m.Mat, 1), Stamp: t.BracketAt}
			switch key {
			case ItemBronze:
				it.Kind = ItemBronze
			case ItemFinal:
				it.Kind = ItemFinal
			default:
				it.Kind, it.Number = ItemEliminations, max(m.Mat, 1)
			}
			byKey[key] = it
			bracket = append(bracket, it)
		}
		it.Matches = append(it.Matches, m.ID)
		for _, feed := range []string{m.FeedRed, m.FeedBlue} {
			if _, from, ok := strings.Cut(feed, ":"); ok && from != "" {
				if it.Feeders == nil {
					it.Feeders = map[string][]string{}
				}
				it.Feeders[m.ID] = append(it.Feeders[m.ID], from)
			}
		}
	}
	// The eliminations first, in lane order, then the bronze match before the final.
	rank := func(it *WorkItem) int {
		switch it.Kind {
		case ItemBronze:
			return 1000
		case ItemFinal:
			return 1001
		}
		return it.Number
	}
	sort.SliceStable(bracket, func(i, j int) bool { return rank(bracket[i]) < rank(bracket[j]) })
	for _, it := range bracket {
		out = append(out, *it)
	}
	return out
}

// MatCount is how many mats the event has: what the organizer said, or as many as the
// disciplines ask for, so an event of one discipline has exactly that discipline's mats.
func MatCount(plan store.Plan, tournaments []store.Tournament) int {
	if plan.Mats > 0 {
		return plan.Mats
	}
	n := 1
	for _, t := range tournaments {
		n = max(n, t.Mats)
	}
	return n
}
