package e2e

import (
	"testing"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestARunningMatchReportsHowLongAgoItsLastEventWas covers what a scoreboard needs to
// show the same time as the mat it is standing next to.
//
// The log records the elapsed time at each event and nothing about when that was in
// wall-clock terms, which is what keeps replay deterministic. A display has no writer of
// its own, so without this it can only anchor its clock to the moment its page loaded --
// and a screen switched on two minutes into a match then reads two minutes short of the
// score keeper's for the rest of it (issue #58).
func TestARunningMatchReportsHowLongAgoItsLastEventWas(t *testing.T) {
	s := start(t)

	for _, name := range []string{"Ada", "Bo", "Cai", "Dag"} {
		var created store.Competitor
		s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": name, "club": "MSL"}, &created)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 1, "minPoolSize": 4, "maxPoolSize": 7}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Pools) == 0 || len(snap.Pools[0].Matches) < 2 {
		t.Fatalf("four competitors should draw one pool of several matches, got %+v", snap.Pools)
	}
	running := snap.Pools[0].Matches[0].ID
	untouched := snap.Pools[0].Matches[1].ID

	// Start the clock, then let real time pass without writing anything else -- which is
	// exactly what a mat looks like between two exchanges.
	s.mustDo(t, "POST", "/api/matches/"+running+"/events", []match.Event{{
		Seq: 1, Type: match.TypeTimer, ElapsedMS: 0, Timer: &match.Timer{Action: match.TimerStart},
	}}, nil)

	const away = 1200 * time.Millisecond
	time.Sleep(away)

	s.mustDo(t, "GET", "/api/state", nil, &snap)
	got := find(t, snap, running)
	if !got.State.Running {
		t.Fatalf("the match should be running, got %+v", got.State)
	}
	if got.State.ElapsedMS != 0 {
		t.Errorf("the log's elapsed time should still be 0, got %d", got.State.ElapsedMS)
	}
	// Generous at both ends: this asserts that the age is reported at all and is roughly
	// real, not that the test machine kept to a millisecond.
	if got.SinceMS < away.Milliseconds()/2 {
		t.Errorf("a match idle for %v reported sinceMs %d, want at least %d",
			away, got.SinceMS, away.Milliseconds()/2)
	}
	if got.SinceMS > 60_000 {
		t.Errorf("a match idle for %v reported sinceMs %d, which is nowhere near it",
			away, got.SinceMS)
	}

	// A match nobody has touched has no clock to place, so there is nothing to report.
	if idle := find(t, snap, untouched); idle.SinceMS != 0 {
		t.Errorf("a match that has not started reported sinceMs %d, want 0", idle.SinceMS)
	}
}

func find(t *testing.T, snap snapshot, id string) matchView {
	t.Helper()
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			if m.ID == id {
				return m
			}
		}
	}
	t.Fatalf("match %s is not in the snapshot", id)
	return matchView{}
}
