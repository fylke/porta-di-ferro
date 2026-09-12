package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

type bracketView struct {
	Matches  []matchView       `json:"matches"`
	Podium   tournament.Podium `json:"podium"`
	Complete bool              `json:"complete"`
}

type snapshotWithBracket struct {
	snapshot
	Overall       []tournament.Standing `json:"overall"`
	PoolsComplete bool                  `json:"poolsComplete"`
	Bracket       *bracketView          `json:"bracket"`
}

// winFor ends a bracket match with the named side winning, through the write path.
func winFor(t *testing.T, s *server, id string, side match.Side) {
	t.Helper()
	red, blue := 2, 0
	if side == match.Blue {
		red, blue = 0, 2
	}
	events := []match.Event{{Seq: 1, Type: match.TypeTimer, ElapsedMS: 0, Timer: &match.Timer{Action: match.TimerStart}}}
	for i := 0; i < 4; i++ {
		events = append(events, match.Event{Seq: i + 2, Type: match.TypeExchange, ElapsedMS: int64((i + 1) * 10000),
			Exchange: &match.Exchange{Red: match.Assessment{Value: red}, Blue: match.Assessment{Value: blue}}})
	}
	events = append(events, match.Event{Seq: 6, Type: match.TypeEnd, ElapsedMS: 40000, End: &match.End{Reason: match.ReasonPointCap}})
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", events, nil)
}

func inBracket(views []matchView, id string) matchView {
	for _, m := range views {
		if m.ID == id {
			return m
		}
	}
	return matchView{}
}

// TestEliminationsFromPoolsToPodium is design §7 item 3 against the real binary: the
// cut waits for finished pools, seeds from the overall ranking, fills each round from the
// results of the last, and ends with a podium.
func TestEliminationsFromPoolsToPodium(t *testing.T) {
	s := start(t)
	for i := 1; i <= 10; i++ {
		var c store.Competitor
		s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": fmt.Sprintf("C%d", i), "club": clubFor(i)}, &c)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 2, "minPoolSize": 5, "maxPoolSize": 5}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	// Too early: the pools are not done.
	if code := s.do(t, "POST", "/api/tournament/bracket", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("drawing the bracket before the pools are done should be refused, got %d", code)
	}

	var snap snapshotWithBracket
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	for _, p := range snap.Pools {
		for i, m := range p.Matches {
			scoreMatch(t, s, m.ID, i)
		}
	}
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if !snap.PoolsComplete {
		t.Fatal("every pool match is scored; pools should be complete")
	}
	if len(snap.Overall) != 10 || snap.Overall[0].Rank != 1 {
		t.Fatalf("the overall ranking should list all ten, ranked: %+v", snap.Overall)
	}

	s.mustDo(t, "POST", "/api/tournament/bracket", nil, nil)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Bracket == nil || len(snap.Bracket.Matches) != 8 {
		t.Fatalf("ten competitors should draw a bracket of eight matches, got %+v", snap.Bracket)
	}
	seed := func(n int) string { return snap.Overall[n-1].Competitor }
	qf1 := inBracket(snap.Bracket.Matches, "e-qf1")
	if qf1.Red != seed(1) || qf1.Blue != seed(8) {
		t.Errorf("the first quarter-final should be seed 1 v seed 8, is %s v %s", qf1.Red, qf1.Blue)
	}
	if sf := inBracket(snap.Bracket.Matches, "e-sf1"); sf.Red != "" || sf.Blue != "" {
		t.Errorf("a semi-final has no competitors until its quarter-finals are played, has %s v %s", sf.Red, sf.Blue)
	}
	// The mats pick the quarter-finals up as soon as the pools are done.
	if snap.Mats["1"] != "e-qf1" || snap.Mats["2"] != "e-qf2" {
		t.Errorf("the mats should be on the first quarter-finals, are on %q and %q", snap.Mats["1"], snap.Mats["2"])
	}

	// Play the quarter-finals: the higher seed wins each.
	for _, id := range []string{"e-qf1", "e-qf2", "e-qf3", "e-qf4"} {
		winFor(t, s, id, match.Red)
	}
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	sf1 := inBracket(snap.Bracket.Matches, "e-sf1")
	if sf1.Red != seed(1) || sf1.Blue != seed(4) {
		t.Errorf("the first semi should be seed 1 v seed 4, is %s v %s", sf1.Red, sf1.Blue)
	}
	if snap.Mats["1"] != "e-sf1" {
		t.Errorf("mat 1 should move on to the semi-final, is on %q", snap.Mats["1"])
	}

	winFor(t, s, "e-sf1", match.Red)  // seed 1 to the final, seed 4 to the bronze match
	winFor(t, s, "e-sf2", match.Blue) // seed 3 to the final, seed 2 to the bronze match
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	final := inBracket(snap.Bracket.Matches, "e-f")
	bronze := inBracket(snap.Bracket.Matches, "e-b")
	if final.Red != seed(1) || final.Blue != seed(3) {
		t.Errorf("the final should be seed 1 v seed 3, is %s v %s", final.Red, final.Blue)
	}
	if bronze.Red != seed(4) || bronze.Blue != seed(2) {
		t.Errorf("the bronze match should be seed 4 v seed 2, is %s v %s", bronze.Red, bronze.Blue)
	}

	winFor(t, s, "e-b", match.Blue)
	winFor(t, s, "e-f", match.Blue)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	want := tournament.Podium{First: seed(3), Second: seed(1), Third: seed(2)}
	if snap.Bracket.Podium != want {
		t.Errorf("podium should be %+v, got %+v", want, snap.Bracket.Podium)
	}
	if !snap.Bracket.Complete {
		t.Error("every bracket match is played; the bracket should be complete")
	}
	if snap.Mats["1"] != "" || snap.Mats["2"] != "" {
		t.Errorf("nothing is left to run, but the mats are on %q and %q", snap.Mats["1"], snap.Mats["2"])
	}
}
