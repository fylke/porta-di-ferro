package tournament_test

import (
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// TestWithdrawalVoidsResultsRetroactively is the case that makes dividing by matches
// *completed* worth the trouble: a competitor who leaves during the pools is treated as
// though they never entered, and everyone else's indices recompute correctly.
func TestWithdrawalVoidsResultsRetroactively(t *testing.T) {
	ids := []string{"a", "b", "c"}
	p := pool(ids, m("m1", "a", "b"), m("m2", "a", "c"), m("m3", "b", "c"))
	states := map[string]match.State{
		"m1": decided(8, 0, match.ReasonPointCap, false), // a beat b
		"m2": decided(2, 8, match.ReasonPointCap, false), // c beat a
		"m3": decided(8, 4, match.ReasonPointCap, false), // b beat c
	}
	rules := match.MSL()

	before := tournament.Rank(rules, p, people(ids...), states, 1)
	if len(before) != 3 {
		t.Fatalf("expected 3 rows before the withdrawal, got %d", len(before))
	}
	for _, r := range before {
		if r.Completed != 2 {
			t.Fatalf("%s should have completed 2 matches, got %d", r.Competitor, r.Completed)
		}
	}

	withdrawn := people(ids...)
	c := withdrawn["c"]
	c.Withdrawn = true
	withdrawn["c"] = c

	after := tournament.Rank(rules, p, withdrawn, states, 1)
	if len(after) != 2 {
		t.Fatalf("a withdrawn competitor should not be in the table, got %d rows", len(after))
	}
	for _, r := range after {
		if r.Completed != 1 {
			t.Errorf("%s: only the a-b match survives, want 1 completed, got %d",
				r.Competitor, r.Completed)
		}
	}
	if after[0].Competitor != "a" {
		t.Errorf("a beat b, so a ranks first once c is voided; got %s", after[0].Competitor)
	}
	// a beat b 8-0 and that is now their whole record: 9 match points over one match.
	if after[0].MatchPointIndex != 9 || after[0].ScoreIndex != 8 {
		t.Errorf("a: want a match point index of 9 and a score index of 8, got %v and %v",
			after[0].MatchPointIndex, after[0].ScoreIndex)
	}
	// b's loss to a is all that is left, and their loss to c is gone with them.
	if after[1].MatchPoints != 3 || after[1].ReceptionIndex != 8 {
		t.Errorf("b: want 3 match points and a reception index of 8, got %d and %v",
			after[1].MatchPoints, after[1].ReceptionIndex)
	}
}
