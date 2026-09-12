package tournament_test

import (
	"fmt"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// ranked builds n standings named s1..sn, in rank order.
func ranked(n int) []tournament.Standing {
	out := make([]tournament.Standing, n)
	for i := range out {
		out[i] = tournament.Standing{Competitor: fmt.Sprintf("s%d", i+1), Rank: i + 1}
	}
	return out
}

func byID(ms []store.Match) map[string]store.Match {
	out := map[string]store.Match{}
	for _, m := range ms {
		out[m.ID] = m
	}
	return out
}

// won is an ended state with the named side winning 8-3.
func won(side match.Side) match.State {
	st := match.State{Ended: true, Winner: side, Reason: match.ReasonPointCap}
	if side == match.Red {
		st.Red.Score, st.Blue.Score = 8, 3
	} else {
		st.Red.Score, st.Blue.Score = 3, 8
	}
	return st
}

func TestBracketOfEightSeedsTheStandardWay(t *testing.T) {
	b, err := tournament.Bracket(store.Tournament{Mats: 2}, ranked(9))
	if err != nil {
		t.Fatal(err)
	}
	m := byID(b)
	pairs := map[string][2]string{
		"e-qf1": {"s1", "s8"}, "e-qf2": {"s4", "s5"}, "e-qf3": {"s2", "s7"}, "e-qf4": {"s3", "s6"},
	}
	for id, want := range pairs {
		if m[id].Red != want[0] || m[id].Blue != want[1] {
			t.Errorf("%s is %s v %s, want %s v %s", id, m[id].Red, m[id].Blue, want[0], want[1])
		}
		if m[id].Round != tournament.RoundQuarter {
			t.Errorf("%s should be a quarter-final, is %q", id, m[id].Round)
		}
	}
	if m["e-sf1"].FeedRed != "winner:e-qf1" || m["e-sf1"].FeedBlue != "winner:e-qf2" {
		t.Errorf("the first semi should be fed by qf1 and qf2, is fed by %s and %s", m["e-sf1"].FeedRed, m["e-sf1"].FeedBlue)
	}
	if m["e-sf2"].FeedRed != "winner:e-qf3" || m["e-sf2"].FeedBlue != "winner:e-qf4" {
		t.Errorf("the second semi should be fed by qf3 and qf4")
	}
	if m["e-b"].FeedRed != "loser:e-sf1" || m["e-f"].FeedRed != "winner:e-sf1" {
		t.Errorf("bronze takes the semi losers and the final the winners")
	}
	// Quarter-finals spread over the two mats; the final on mat 1, bronze on mat 2.
	if m["e-qf1"].Mat != 1 || m["e-qf2"].Mat != 2 || m["e-qf3"].Mat != 1 || m["e-qf4"].Mat != 2 {
		t.Errorf("quarter-finals should alternate mats: %d %d %d %d", m["e-qf1"].Mat, m["e-qf2"].Mat, m["e-qf3"].Mat, m["e-qf4"].Mat)
	}
	if m["e-f"].Mat != 1 || m["e-b"].Mat != 2 {
		t.Errorf("final on mat 1 and bronze on mat 2, got %d and %d", m["e-f"].Mat, m["e-b"].Mat)
	}
	if len(b) != 8 {
		t.Errorf("a bracket of eight is 8 matches, got %d", len(b))
	}
	for i, x := range b {
		if x.Order != i+1 || x.Pool != 0 {
			t.Errorf("match %s should be order %d in no pool, is order %d pool %d", x.ID, i+1, x.Order, x.Pool)
		}
	}
}

func TestSmallerFieldsCutAtFourAndTwo(t *testing.T) {
	four, _ := tournament.Bracket(store.Tournament{Mats: 1}, ranked(6))
	if len(four) != 4 || four[0].ID != "e-sf1" || four[0].Red != "s1" || four[0].Blue != "s4" {
		t.Errorf("six ranked should cut at four: %+v", four)
	}
	m := byID(four)
	if m["e-b"].Mat != 1 || m["e-f"].Mat != 1 || m["e-b"].Order >= m["e-f"].Order {
		t.Errorf("on one mat the bronze match runs before the final: bronze %d final %d", m["e-b"].Order, m["e-f"].Order)
	}
	two, _ := tournament.Bracket(store.Tournament{Mats: 1}, ranked(3))
	if len(two) != 1 || two[0].ID != "e-f" || two[0].Red != "s1" || two[0].Blue != "s2" {
		t.Errorf("three ranked should be a straight final: %+v", two)
	}
	if _, err := tournament.Bracket(store.Tournament{Mats: 1}, ranked(1)); err == nil {
		t.Error("one competitor cannot have eliminations")
	}
}

func TestFillFollowsResultsRoundByRound(t *testing.T) {
	b, _ := tournament.Bracket(store.Tournament{Mats: 2}, ranked(8))
	states := map[string]match.State{}

	f := byID(tournament.Fill(b, states))
	if f["e-sf1"].Red != "" || f["e-f"].Red != "" {
		t.Errorf("nothing played: later rounds should be empty")
	}

	states["e-qf1"], states["e-qf2"] = won(match.Red), won(match.Blue) // s1 and s5 through
	f = byID(tournament.Fill(b, states))
	if f["e-sf1"].Red != "s1" || f["e-sf1"].Blue != "s5" {
		t.Errorf("the first semi should be s1 v s5, is %s v %s", f["e-sf1"].Red, f["e-sf1"].Blue)
	}
	if f["e-sf2"].Red != "" {
		t.Errorf("the second semi has no results yet and should be empty")
	}

	states["e-qf3"], states["e-qf4"] = won(match.Red), won(match.Red)  // s2 and s3
	states["e-sf1"], states["e-sf2"] = won(match.Blue), won(match.Red) // s5 and s2 to the final
	f = byID(tournament.Fill(b, states))
	if f["e-f"].Red != "s5" || f["e-f"].Blue != "s2" {
		t.Errorf("the final should be s5 v s2, is %s v %s", f["e-f"].Red, f["e-f"].Blue)
	}
	if f["e-b"].Red != "s1" || f["e-b"].Blue != "s3" {
		t.Errorf("the bronze match should be s1 v s3, is %s v %s", f["e-b"].Red, f["e-b"].Blue)
	}

	states["e-b"], states["e-f"] = won(match.Blue), won(match.Red)
	podium := tournament.Result(tournament.Fill(b, states), states)
	if podium != (tournament.Podium{First: "s5", Second: "s2", Third: "s3"}) {
		t.Errorf("podium should be s5, s2, s3, got %+v", podium)
	}
}

// TestACorrectedQuarterFinalCorrectsTheSemi: the bracket is derived, not stored, so a
// changed result upstream changes who is in the next round -- no migration, no button.
func TestACorrectedQuarterFinalCorrectsTheSemi(t *testing.T) {
	b, _ := tournament.Bracket(store.Tournament{Mats: 2}, ranked(8))
	states := map[string]match.State{"e-qf1": won(match.Red)}
	if got := byID(tournament.Fill(b, states))["e-sf1"].Red; got != "s1" {
		t.Fatalf("s1 should be through, got %q", got)
	}
	states["e-qf1"] = won(match.Blue)
	if got := byID(tournament.Fill(b, states))["e-sf1"].Red; got != "s8" {
		t.Errorf("after the correction s8 should be through, got %q", got)
	}
}

// TestOverallRanksAcrossPools: the same chain as a pool table, over everyone, so two
// competitors from different pools compare on their per-match indices.
func TestOverallRanksAcrossPools(t *testing.T) {
	comps := map[string]store.Competitor{}
	for _, id := range []string{"a", "b", "c", "d"} {
		comps[id] = store.Competitor{ID: id, Name: id}
	}
	tr := store.Tournament{Mats: 2, Seed: 1, Pools: []store.Pool{
		{Number: 1, Competitors: []string{"a", "b"}, Matches: []store.Match{{ID: "p1m1", Pool: 1, Red: "a", Blue: "b"}}},
		{Number: 2, Competitors: []string{"c", "d"}, Matches: []store.Match{{ID: "p2m1", Pool: 2, Red: "c", Blue: "d"}}},
	}}
	states := map[string]match.State{
		"p1m1": {Ended: true, Winner: match.Red, Red: match.Competitor{Score: 8}, Blue: match.Competitor{Score: 1}},
		"p2m1": {Ended: true, Winner: match.Red, Red: match.Competitor{Score: 8}, Blue: match.Competitor{Score: 5}},
	}
	overall := tournament.Overall(match.MSL(), tr, comps, states)
	got := make([]string, len(overall))
	for i, s := range overall {
		got[i] = s.Competitor
	}
	// a and c both won 9 match points; a's score index (+7) beats c's (+3); the losers
	// order the same way on reception: d conceded 8 with a score index of -3, b of -7.
	if fmt.Sprint(got) != "[a c d b]" {
		t.Errorf("overall ranking should be [a c d b], got %v", got)
	}
	if !tournament.PoolsComplete(tr, comps, states) {
		t.Error("both pool matches ended; pools should be complete")
	}
	delete(states, "p2m1")
	if tournament.PoolsComplete(tr, comps, states) {
		t.Error("one pool match unplayed; pools should not be complete")
	}
}
