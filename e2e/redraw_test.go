package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Redrawing the pools is a restart (issue #93).
//
// Match ids are positional -- pool 1's first match is p1m1 in every draw -- so the defect
// was not in the drawing but in what was left behind: the log of the old p1m1 stayed on
// disk, and the new p1m1 picked it up. The pool came back with new competitors and the
// previous pool's results already in it, which is worse than either keeping the old draw
// or losing the scores, because nothing on screen says the numbers are not theirs.
//
// So this drives the real binary: score a match, draw again, and read the same slot back.
func TestRedrawingThePoolsRestartsThem(t *testing.T) {
	s := start(t)
	for i := 1; i <= 10; i++ {
		var c store.Competitor
		s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": fmt.Sprintf("C%d", i), "club": clubFor(i)}, &c)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 2, "minPoolSize": 5, "maxPoolSize": 5}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var before snapshot
	s.mustDo(t, "GET", "/api/state", nil, &before)
	first := before.Pools[0].Matches[0]
	if first.ID != "p1m1" {
		t.Fatalf("expected the first match of pool 1 to be p1m1, got %q", first.ID)
	}
	winFor(t, s, first.ID, match.Red)

	var scored snapshot
	s.mustDo(t, "GET", "/api/state", nil, &scored)
	if got := scored.Pools[0].Matches[0].State.Red.Score; got != 8 {
		t.Fatalf("the match should have been scored 8, got %d", got)
	}

	// The pool is unfinished -- one match in, four to go -- which is exactly the case the
	// issue was reported for.
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var after snapshot
	s.mustDo(t, "GET", "/api/state", nil, &after)
	redrawn := after.Pools[0].Matches[0]
	if redrawn.State.Red.Score != 0 || redrawn.State.Blue.Score != 0 {
		t.Fatalf("the redrawn %s starts with %d-%d on it, from the draw before",
			redrawn.ID, redrawn.State.Red.Score, redrawn.State.Blue.Score)
	}
	if redrawn.Status != "pending" {
		t.Fatalf("the redrawn %s is %q, not pending", redrawn.ID, redrawn.Status)
	}
	for _, p := range after.Pools {
		for _, m := range p.Matches {
			if m.Status != "pending" {
				t.Fatalf("%s is %q after a redraw; every match should be pending", m.ID, m.Status)
			}
		}
	}

	// Nothing is lost: the log that was retired is on disk as a dated backup, which is
	// the same promise the history editor makes.
	entries, err := os.ReadDir(filepath.Join(s.dir, "matches"))
	if err != nil {
		t.Fatalf("reading the matches directory: %v", err)
	}
	backups := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "p1m1.ndjson.") && strings.HasSuffix(e.Name(), ".bak") {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("expected the retired p1m1 log kept as one backup, found %d", backups)
	}

	// And the new match writes from sequence 1 without tripping over the index the old
	// log left behind.
	winFor(t, s, redrawn.ID, match.Blue)
	var rescored snapshot
	s.mustDo(t, "GET", "/api/state", nil, &rescored)
	if got := rescored.Pools[0].Matches[0].State.Blue.Score; got != 8 {
		t.Fatalf("the redrawn match scored %d for blue, want 8", got)
	}
}
