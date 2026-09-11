package e2e

import (
	"fmt"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestOrganizerOverridesTheMatAssignment drives the override through the API and checks
// that the one thing every screen reads -- which match is up on each mat -- follows it.
func TestOrganizerOverridesTheMatAssignment(t *testing.T) {
	s := start(t)
	for i := 1; i <= 16; i++ {
		var c store.Competitor
		s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": fmt.Sprintf("C%d", i), "club": "MSL"}, &c)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 2, "minPoolSize": 4, "maxPoolSize": 4}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Pools) != 4 {
		t.Fatalf("16 in pools of 4 should be 4 pools, got %d", len(snap.Pools))
	}
	if !hasPrefix(snap.Mats["2"], "p2m") {
		t.Fatalf("mat 2 should start on pool 2, is on %q", snap.Mats["2"])
	}

	// Move pool 2 to mat 1. It queues behind pools 1 and 3; mat 2 falls back to pool 4.
	s.mustDo(t, "PATCH", "/api/tournament/pools/2", map[string]int{"mat": 1}, nil)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if got := order(snap, 1); got != "1,3,2" {
		t.Errorf("mat 1 should run pools 1,3,2, runs %s", got)
	}
	if !hasPrefix(snap.Mats["2"], "p4m") {
		t.Errorf("mat 2 should now start on pool 4, is on %q", snap.Mats["2"])
	}
	for _, p := range snap.Pools {
		if p.Number == 2 && !p.Overridden {
			t.Error("the moved pool should be marked as overridden")
		}
		if p.Number == 1 && p.Overridden {
			t.Error("pool 1 has not moved and should not be marked")
		}
	}

	// Move it up twice: it now goes first on mat 1, and mat 1 is up on a pool-2 match.
	s.mustDo(t, "PATCH", "/api/tournament/pools/2", map[string]string{"move": "up"}, nil)
	s.mustDo(t, "PATCH", "/api/tournament/pools/2", map[string]string{"move": "up"}, nil)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if got := order(snap, 1); got != "2,1,3" {
		t.Errorf("mat 1 should run pools 2,1,3, runs %s", got)
	}
	if !hasPrefix(snap.Mats["1"], "p2m") {
		t.Errorf("mat 1 should be up on pool 2, is on %q", snap.Mats["1"])
	}

	// Nonsense is refused rather than ignored.
	if code := s.do(t, "PATCH", "/api/tournament/pools/2", map[string]int{"mat": 9}, nil); code != 400 {
		t.Errorf("a mat that does not exist should be a 400, got %d", code)
	}
	if code := s.do(t, "PATCH", "/api/tournament/pools/2", map[string]string{}, nil); code != 400 {
		t.Errorf("an empty override should be a 400, got %d", code)
	}
}

func order(snap snapshot, mat int) string {
	out := ""
	for _, p := range snap.Pools {
		if p.Mat != mat {
			continue
		}
		if out != "" {
			out += ","
		}
		out += fmt.Sprint(p.Number)
	}
	return out
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
