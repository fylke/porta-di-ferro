package tournament_test

import (
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// drawn is a tournament of n competitors in pools of exactly four, so the tests below can
// say which pool numbers they expect.
func drawn(t *testing.T, mats, n int) store.Tournament {
	t.Helper()
	setup := store.Tournament{Mats: mats, MinPoolSize: 4, MaxPoolSize: 4}
	out, err := tournament.Generate(setup, field(n), tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func numbersOnMat(t store.Tournament, mat int) []int {
	var out []int
	for _, p := range tournament.RunOrder(t) {
		if p.Mat == mat {
			out = append(out, p.Number)
		}
	}
	return out
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTheDefaultIsNotAnOverride(t *testing.T) {
	tr := drawn(t, 2, 16) // four pools: 1 and 3 on mat 1, 2 and 4 on mat 2
	for _, p := range tr.Pools {
		if tournament.Overridden(tr, p.Number) {
			t.Errorf("pool %d is where the default puts it and should not read as overridden", p.Number)
		}
	}
	if got := numbersOnMat(tr, 1); !equal(got, []int{1, 3}) {
		t.Errorf("mat 1 runs %v, want [1 3]", got)
	}
}

func TestMovePoolQueuesBehindTheMatsOwnPools(t *testing.T) {
	tr := drawn(t, 2, 16)
	moved, err := tournament.MovePool(tr, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := numbersOnMat(moved, 1); !equal(got, []int{1, 3, 2}) {
		t.Errorf("after moving pool 2 to mat 1 it should queue last: got %v", got)
	}
	if got := numbersOnMat(moved, 2); !equal(got, []int{4}) {
		t.Errorf("mat 2 should be left with pool 4, got %v", got)
	}
	if !tournament.Overridden(moved, 2) {
		t.Error("pool 2 has been moved and should read as overridden")
	}
	for _, p := range moved.Pools {
		if p.Number != 2 {
			continue
		}
		for _, m := range p.Matches {
			if m.Mat != 1 {
				t.Errorf("match %s still says mat %d", m.ID, m.Mat)
			}
		}
	}
	if _, err := tournament.MovePool(tr, 2, 3); err == nil {
		t.Error("mat 3 does not exist on a two-mat tournament and should be refused")
	}
	if _, err := tournament.MovePool(tr, 9, 1); err == nil {
		t.Error("pool 9 does not exist and should be refused")
	}
}

func TestReorderPoolStepsThroughTheQueueAndStopsAtTheEnds(t *testing.T) {
	tr := drawn(t, 1, 16) // pools 1-4, all on mat 1
	up, err := tournament.ReorderPool(tr, 3, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := numbersOnMat(up, 1); !equal(got, []int{1, 3, 2, 4}) {
		t.Errorf("moving pool 3 up should give [1 3 2 4], got %v", got)
	}
	if !tournament.Overridden(up, 3) || !tournament.Overridden(up, 2) {
		t.Error("both pools that swapped places are out of number order and should read as overridden")
	}
	if tournament.Overridden(up, 1) {
		t.Error("pool 1 has not moved")
	}
	down, _ := tournament.ReorderPool(up, 3, false)
	if got := numbersOnMat(down, 1); !equal(got, []int{1, 2, 3, 4}) {
		t.Errorf("moving it back down should restore [1 2 3 4], got %v", got)
	}
	top, _ := tournament.ReorderPool(tr, 1, true)
	if got := numbersOnMat(top, 1); !equal(got, []int{1, 2, 3, 4}) {
		t.Errorf("moving the first pool up should change nothing, got %v", got)
	}
}

// TestASavedTournamentWithoutSequencesReadsInNumberOrder: tournament.json files written
// before the field existed have every sequence at zero, and must run by pool number.
func TestASavedTournamentWithoutSequencesReadsInNumberOrder(t *testing.T) {
	tr := drawn(t, 2, 16)
	for i := range tr.Pools {
		tr.Pools[i].Sequence = 0
	}
	if got := numbersOnMat(tr, 2); !equal(got, []int{2, 4}) {
		t.Errorf("mat 2 should run [2 4] with no sequences recorded, got %v", got)
	}
	up, _ := tournament.ReorderPool(tr, 4, true)
	if got := numbersOnMat(up, 2); !equal(got, []int{4, 2}) {
		t.Errorf("a reorder on an old file should still work, got %v", got)
	}
}
