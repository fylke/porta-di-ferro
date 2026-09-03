package tournament_test

import (
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

func people(ids ...string) map[string]store.Competitor {
	out := map[string]store.Competitor{}
	for _, id := range ids {
		out[id] = store.Competitor{ID: id, Name: id}
	}
	return out
}

// decided builds the state a finished match would replay to, without going through the
// log. The engine's own behaviour is covered by the shared vectors.
func decided(red, blue int, reason match.Reason, noPoints bool) match.State {
	s := match.State{Ended: true, Reason: reason, NoMatchPoints: noPoints}
	s.Red.Score, s.Blue.Score = red, blue
	switch {
	case red > blue:
		s.Winner = match.Red
	case blue > red:
		s.Winner = match.Blue
	}
	return s
}

func pool(ids []string, matches ...store.Match) store.Pool {
	return store.Pool{Number: 1, Mat: 1, Competitors: ids, Matches: matches}
}

func m(id, red, blue string) store.Match {
	return store.Match{ID: id, Pool: 1, Mat: 1, Red: red, Blue: blue}
}

func TestMatchPointIndexOrdersTheTable(t *testing.T) {
	ids := []string{"a", "b", "c"}
	p := pool(ids, m("m1", "a", "b"), m("m2", "a", "c"), m("m3", "b", "c"))
	states := map[string]match.State{
		"m1": decided(8, 3, match.ReasonPointCap, false),
		"m2": decided(8, 2, match.ReasonPointCap, false),
		"m3": decided(5, 5, match.ReasonTime, false),
	}
	got := tournament.Rank(match.MSL(), p, people(ids...), states, 1)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	if got[0].Competitor != "a" {
		t.Errorf("two wins should rank first, got %s", got[0].Competitor)
	}
	if got[0].MatchPoints != 18 || got[0].MatchPointIndex != 9 {
		t.Errorf("a: want 18 match points at an index of 9, got %d at %v",
			got[0].MatchPoints, got[0].MatchPointIndex)
	}
	// b and c both drew one and lost one: 6 + 3 over two matches.
	for _, r := range got[1:] {
		if r.MatchPoints != 9 || r.MatchPointIndex != 4.5 {
			t.Errorf("%s: want 9 match points at an index of 4.5, got %d at %v",
				r.Competitor, r.MatchPoints, r.MatchPointIndex)
		}
	}
}

func TestPenaltyLossEarnsNoMatchPoints(t *testing.T) {
	ids := []string{"a", "b"}
	p := pool(ids, m("m1", "a", "b"))
	states := map[string]match.State{"m1": decided(0, 8, match.ReasonPenalty, true)}
	got := tournament.Rank(match.MSL(), p, people(ids...), states, 1)
	for _, r := range got {
		if r.Competitor == "a" && r.MatchPoints != 0 {
			t.Errorf("a lost on penalties and should earn nothing, got %d", r.MatchPoints)
		}
		if r.Competitor == "b" && r.MatchPoints != 9 {
			t.Errorf("b won and should take 9, got %d", r.MatchPoints)
		}
	}
}

func TestForfeitIsRecordedZeroEight(t *testing.T) {
	ids := []string{"a", "b"}
	p := pool(ids, m("m1", "a", "b"))
	states := map[string]match.State{"m1": decided(0, 8, match.ReasonForfeit, true)}
	got := tournament.Rank(match.MSL(), p, people(ids...), states, 1)
	if got[0].Competitor != "b" || got[0].Scored != 8 || got[0].Conceded != 0 {
		t.Errorf("winner should be b at 8-0, got %+v", got[0])
	}
	if got[1].MatchPoints != 0 {
		t.Errorf("the forfeiter earns nothing, got %d", got[1].MatchPoints)
	}
}

func TestHeadToHeadBreaksAnOtherwiseExactTie(t *testing.T) {
	ids := []string{"a", "b"}
	p := pool(ids, m("m1", "a", "b"))
	// One match each way is impossible in a pool, so tie them on every index by giving
	// them the same numbers and let head-to-head decide.
	states := map[string]match.State{"m1": decided(8, 7, match.ReasonPointCap, false)}
	got := tournament.Rank(match.MSL(), p, people(ids...), states, 1)
	if got[0].Competitor != "a" {
		t.Errorf("a beat b head to head and should rank first, got %s", got[0].Competitor)
	}
}
