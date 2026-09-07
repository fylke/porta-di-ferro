package e2e

import (
	"fmt"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestSimulatedEvent is the whole thing end to end against the real binary: enter
// competitors, draw the pools, score every match, and assert the standings that come out
// (docs/design.md §12).
func TestSimulatedEvent(t *testing.T) {
	s := start(t)

	const entrants = 12
	for i := 1; i <= entrants; i++ {
		var created store.Competitor
		s.mustDo(t, "POST", "/api/competitors",
			map[string]string{"name": fmt.Sprintf("Competitor %d", i), "club": clubFor(i)}, &created)
		if created.ID == "" {
			t.Fatalf("competitor %d came back without an id", i)
		}
	}

	s.mustDo(t, "PUT", "/api/tournament",
		map[string]int{"mats": 2, "minPoolSize": 4, "maxPoolSize": 7}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Pools) < 2 {
		t.Fatalf("12 competitors in pools of 4 to 7 should draw more than one pool, got %d",
			len(snap.Pools))
	}
	for _, p := range snap.Pools {
		if want := ((p.Number - 1) % 2) + 1; p.Mat != want {
			t.Errorf("pool %d is on mat %d, want %d", p.Number, p.Mat, want)
		}
	}

	// Score every match. The pattern alternates so the pools produce a real spread of
	// results rather than one competitor winning everything.
	for _, p := range snap.Pools {
		for i, m := range p.Matches {
			scoreMatch(t, s, m.ID, i)
		}
	}

	s.mustDo(t, "GET", "/api/state", nil, &snap)
	for _, p := range snap.Pools {
		if !p.Complete {
			t.Errorf("pool %d is not complete after every match was scored", p.Number)
		}
		if len(p.Standings) != len(p.Competitors) {
			t.Errorf("pool %d: %d standings for %d competitors",
				p.Number, len(p.Standings), len(p.Competitors))
		}
		played := len(p.Competitors) - 1
		for _, row := range p.Standings {
			if row.Completed != played {
				t.Errorf("pool %d: %s completed %d matches, want %d",
					p.Number, row.Name, row.Completed, played)
			}
			if row.Wins+row.Draws+row.Losses != played {
				t.Errorf("pool %d: %s has %d results for %d matches",
					p.Number, row.Name, row.Wins+row.Draws+row.Losses, played)
			}
		}
		// The table is ordered by the match point index, highest first.
		for i := 1; i < len(p.Standings); i++ {
			if p.Standings[i-1].MatchPointIndex < p.Standings[i].MatchPointIndex {
				t.Errorf("pool %d: standings are out of order at rank %d", p.Number, i+1)
			}
			if p.Standings[i].Rank != i+1 {
				t.Errorf("pool %d: rank %d is labelled %d", p.Number, i+1, p.Standings[i].Rank)
			}
		}
	}

	// Withdrawal voids a competitor's results as though they never entered, and everyone
	// else's indices recompute because they divide by matches completed.
	victim := snap.Pools[0].Standings[0]
	before := len(snap.Pools[0].Standings)
	s.mustDo(t, "PATCH", "/api/competitors/"+victim.Competitor,
		map[string]bool{"withdrawn": true}, nil)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	after := snap.Pools[0].Standings
	if len(after) != before-1 {
		t.Fatalf("withdrawing one competitor left %d rows, want %d", len(after), before-1)
	}
	for _, row := range after {
		if row.Completed != before-2 {
			t.Errorf("%s should have %d matches left after the withdrawal, has %d",
				row.Name, before-2, row.Completed)
		}
	}

	var export map[string]any
	s.mustDo(t, "GET", "/api/export.json", nil, &export)
	if _, ok := export["tournament"]; !ok {
		t.Error("the export is missing the tournament")
	}
}

func clubFor(i int) string {
	clubs := []string{"MSL", "GHFS", "Uppsala HEMA", "Malmö Fri Fäktning"}
	return clubs[i%len(clubs)]
}

// scoreMatch plays a match out through the write path, one confirmed exchange at a time,
// exactly as a score keeper client would.
func scoreMatch(t *testing.T, s *server, id string, variation int) {
	t.Helper()
	seq := 1
	push := func(e match.Event) match.State {
		e.Seq = seq
		seq++
		var res struct {
			Written int         `json:"written"`
			State   match.State `json:"state"`
		}
		s.mustDo(t, "POST", "/api/matches/"+id+"/events", []match.Event{e}, &res)
		return res.State
	}

	push(match.Event{Type: match.TypeTimer, ElapsedMS: 0, Timer: &match.Timer{Action: match.TimerStart}})

	// A draw every fourth match, so the pool tables exercise draws as well as wins.
	draw := variation%4 == 3
	elapsed := int64(0)
	var st match.State
	for i := 0; i < 12; i++ {
		elapsed += 12000
		red, blue := 2, 0
		if draw && i%2 == 1 {
			red, blue = 0, 2
		}
		st = push(match.Event{
			Type:      match.TypeExchange,
			ElapsedMS: elapsed,
			Exchange: &match.Exchange{
				Red:  match.Assessment{Value: red},
				Blue: match.Assessment{Value: blue},
			},
		})
		if st.Pending != match.PendingNone {
			break
		}
	}

	reason := match.ReasonTime
	if st.Pending == match.PendingPointCap {
		reason = match.ReasonPointCap
	}
	st = push(match.Event{Type: match.TypeEnd, ElapsedMS: elapsed, End: &match.End{Reason: reason}})
	if !st.Ended {
		t.Fatalf("match %s did not end", id)
	}
}
