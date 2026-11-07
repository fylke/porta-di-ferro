package hall

import (
	"fmt"
	"maps"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Moving a work item by hand, as the mat board and a pool's controls do.

// ErrNotMovable is a move of an item under way or finished.
type ErrNotMovable struct{ Status string }

func (e ErrNotMovable) Error() string {
	if e.Status == "done" {
		return "that item is already fenced"
	}
	if e.Status == "follows" {
		return "only pools are moved by hand: the eliminations and the finals follow them"
	}
	return "that item is under way; it can be moved once it is done, or what comes after it can"
}

// Step moves an item one place earlier or later on its own mat, as the arrows on a card
// and on a pool's header do.
func Step(view MatsView, placed map[string]store.Placement, id string, earlier bool, mats int) (map[string]store.Placement, bool, error) {
	for _, it := range view.Items {
		if it.ID != id {
			continue
		}
		index := it.Position // the position from 1 is the index after it, from 0
		if earlier {
			index = it.Position - 2
		}
		if index < 0 {
			return placed, false, nil
		}
		return Move(view, placed, id, it.Mat, index, mats)
	}
	return placed, false, fmt.Errorf("no work item %q", id)
}

// Move puts an item at index (from 0) in a mat's queue, and returns the new placements and
// whether anything changed. The index is clamped to the queue, and to after any item there
// that is under way or finished: nothing jumps in front of what a mat is already running.
// Dropping an item where it already is changes nothing.
func Move(view MatsView, placed map[string]store.Placement, id string, mat, index, mats int) (map[string]store.Placement, bool, error) {
	if mat < 1 || mat > mats {
		return placed, false, fmt.Errorf("there is no mat %d", mat)
	}
	var moving *ItemView
	for i := range view.Items {
		if view.Items[i].ID == id {
			moving = &view.Items[i]
		}
	}
	if moving == nil {
		return placed, false, fmt.Errorf("no work item %q", id)
	}
	if !moving.Movable {
		return placed, false, ErrNotMovable{Status: moving.Status}
	}
	// Only pools are moved by hand (#149): the eliminations follow the pools, and the
	// bronze match and the final follow the eliminations, wherever the plan puts them.
	if moving.Kind != ItemPool {
		return placed, false, ErrNotMovable{Status: "follows"}
	}

	var queue []ItemView
	for _, it := range view.Items {
		if it.Mat == mat && it.ID != id {
			queue = append(queue, it)
		}
	}
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].Position < queue[j].Position })
	index = clampIndex(queue, *moving, index)

	order := make([]string, 0, len(queue)+1)
	for _, it := range queue[:index] {
		order = append(order, it.ID)
	}
	order = append(order, id)
	for _, it := range queue[index:] {
		order = append(order, it.ID)
	}

	next := maps.Clone(placed)
	if next == nil {
		next = map[string]store.Placement{}
	}
	changed := false
	for i, itemID := range order {
		p := next[itemID]
		if p.Mat != mat || p.Seq != i+1 {
			changed = true
		}
		p.Mat, p.Seq = mat, i+1
		next[itemID] = p
	}
	if !changed {
		return placed, false, nil
	}
	// Put there by hand, so a suggested plan keeps it on this mat (phase 4).
	p := next[id]
	p.Pinned = true
	next[id] = p
	return next, true, nil
}

// clampIndex keeps a place in a queue behind everything under way or finished, and in front
// of the eliminations or finals of the item's own discipline: they wait for it, and the mat
// would stand idle until somebody noticed (#149).
func clampIndex(queue []ItemView, moving ItemView, index int) int {
	floor := 0
	for i, it := range queue {
		if !it.Movable {
			floor = i + 1
		}
	}
	ceiling := len(queue)
	for i := len(queue) - 1; i >= 0; i-- {
		if queue[i].Discipline == moving.Discipline && queue[i].Kind != ItemPool {
			ceiling = i
		}
	}
	return min(max(index, floor), max(ceiling, floor))
}

// MoveOrStep is a move from the mat board or a pool's controls: one step "up" or "down" the
// item's own mat, or to a mat at an index, -1 for the end.
func MoveOrStep(view MatsView, placed map[string]store.Placement, id string, mat, index int, move string, mats int) (map[string]store.Placement, bool, error) {
	if move == "up" || move == "down" {
		return Step(view, placed, id, move == "up", mats)
	}
	if index < 0 {
		index = len(view.Items)
	}
	return Move(view, placed, id, mat, index, mats)
}
