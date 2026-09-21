package e2e

import (
	"fmt"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestFourMatsAndEightPools is the Milestone 2 ceiling against the real binary: 56
// competitors on four mats draw eight pools, pool N lands on mat ((N-1) mod 4)+1, and
// every mat has something to run.
func TestFourMatsAndEightPools(t *testing.T) {
	s := start(t)

	const entrants = 56
	for i := 1; i <= entrants; i++ {
		var created store.Competitor
		s.mustDo(t, "POST", "/api/competitors",
			map[string]string{"name": fmt.Sprintf("Competitor %d", i), "club": clubFor(i)}, &created)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 4, "minPoolSize": 4, "maxPoolSize": 7}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Pools) != 8 {
		t.Fatalf("56 competitors in pools of up to 7 should draw 8 pools, got %d", len(snap.Pools))
	}
	onMat := map[int]int{}
	for _, p := range snap.Pools {
		if want := ((p.Number - 1) % 4) + 1; p.Mat != want {
			t.Errorf("pool %d is on mat %d, want %d", p.Number, p.Mat, want)
		}
		onMat[p.Mat]++
	}
	for mat := 1; mat <= 4; mat++ {
		if onMat[mat] != 2 {
			t.Errorf("mat %d has %d pools, want 2", mat, onMat[mat])
		}
		if snap.Mats[fmt.Sprint(mat)] == "" {
			t.Errorf("mat %d has no match up", mat)
		}
	}

	// One more is over the ceiling, and says so rather than drawing something odd.
	var extra store.Competitor
	s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": "One Too Many", "club": "MSL"}, &extra)
	if code := s.do(t, "POST", "/api/tournament/pools", nil, nil); code < 400 || code > 499 {
		t.Errorf("57 competitors should be refused with a 4xx, got %d", code)
	}
}
