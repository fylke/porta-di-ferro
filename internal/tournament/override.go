package tournament

import (
	"fmt"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The organizer's override of the mat assignment (design §7 item 8).
//
// The default mapping is fixed and predictable because confusion at the mat costs more
// than throughput. This is the escape hatch for when reality disagrees: a mat that is
// running long, a mat that lost its score keeper, a pool that has to go early because a
// competitor has a train. Two operations, both small: move a pool to another mat, where
// it queues behind whatever is already there; and move a pool up or down the queue of the
// mat it is on.
//
// A pool's place in its mat's queue is Pool.Sequence. The generator sets it to the pool
// number, so the default order is by number and an untouched tournament reads exactly as
// it did before the field existed.

// DefaultMat is where pool number lands with nobody overriding: ((N-1) mod mats)+1.
func DefaultMat(number, mats int) int {
	if mats < 1 {
		return 1
	}
	return ((number - 1) % mats) + 1
}

// MovePool puts a pool on another mat, at the back of that mat's queue.
func MovePool(t store.Tournament, number, mat int) (store.Tournament, error) {
	if mat < 1 || mat > t.Mats {
		return t, fmt.Errorf("mat must be between 1 and %d", t.Mats)
	}
	i := poolIndex(t, number)
	if i < 0 {
		return t, fmt.Errorf("no pool %d", number)
	}
	normalise(&t)
	last := 0
	for _, p := range t.Pools {
		if p.Mat == mat && p.Sequence > last {
			last = p.Sequence
		}
	}
	t.Pools[i].Mat = mat
	t.Pools[i].Sequence = last + 1
	// A match carries its mat too, so every view of a match agrees with its pool.
	for j := range t.Pools[i].Matches {
		t.Pools[i].Matches[j].Mat = mat
	}
	normalise(&t)
	return t, nil
}

// ReorderPool moves a pool one step up (earlier) or down (later) in its mat's queue. A
// step past either end is not an error: the pool is already where it was asked to go.
func ReorderPool(t store.Tournament, number int, up bool) (store.Tournament, error) {
	i := poolIndex(t, number)
	if i < 0 {
		return t, fmt.Errorf("no pool %d", number)
	}
	normalise(&t)
	queue := onMat(t, t.Pools[i].Mat)
	pos := -1
	for k, idx := range queue {
		if idx == i {
			pos = k
		}
	}
	other := pos + 1
	if up {
		other = pos - 1
	}
	if other < 0 || other >= len(queue) {
		return t, nil
	}
	j := queue[other]
	t.Pools[i].Sequence, t.Pools[j].Sequence = t.Pools[j].Sequence, t.Pools[i].Sequence
	return t, nil
}

// Overridden reports whether a pool sits anywhere other than where the default mapping
// would put it -- on another mat, or out of number order on its own. The organizer view
// shows this, so nobody has to wonder why mat 2 is running pool 3.
func Overridden(t store.Tournament, number int) bool {
	i := poolIndex(t, number)
	if i < 0 {
		return false
	}
	p := t.Pools[i]
	if p.Mat != DefaultMat(p.Number, t.Mats) {
		return true
	}
	queue := onMat(t, p.Mat)
	byNumber := append([]int(nil), queue...)
	sort.Slice(byNumber, func(a, b int) bool { return t.Pools[byNumber[a]].Number < t.Pools[byNumber[b]].Number })
	for k := range queue {
		if queue[k] == i {
			return byNumber[k] != i
		}
	}
	return false
}

// RunOrder is the pools sorted the way the mats run them: by mat, then by queue position.
// Every consumer that filters pools by mat gets the running order for free from this.
func RunOrder(t store.Tournament) []store.Pool {
	normalise(&t)
	out := append([]store.Pool(nil), t.Pools...)
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Mat != out[b].Mat {
			return out[a].Mat < out[b].Mat
		}
		return out[a].Sequence < out[b].Sequence
	})
	return out
}

func poolIndex(t store.Tournament, number int) int {
	for i, p := range t.Pools {
		if p.Number == number {
			return i
		}
	}
	return -1
}

// onMat is the indices of the pools on a mat, in queue order.
func onMat(t store.Tournament, mat int) []int {
	var idx []int
	for i, p := range t.Pools {
		if p.Mat == mat {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := t.Pools[idx[a]], t.Pools[idx[b]]
		if pa.Sequence != pb.Sequence {
			return pa.Sequence < pb.Sequence
		}
		return pa.Number < pb.Number
	})
	return idx
}

// normalise renumbers each mat's queue 1..k in its current order, so a tournament saved
// before the field existed (all zero, meaning number order) and one after a series of
// moves both have unique, dense positions that a swap can act on.
func normalise(t *store.Tournament) {
	for mat := 1; mat <= t.Mats; mat++ {
		for k, i := range onMat(*t, mat) {
			t.Pools[i].Sequence = k + 1
		}
	}
}
