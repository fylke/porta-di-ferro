package tournament_test

import (
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// How many mats the eliminations are worth (issue #94). A mat is a referee, a score
// keeper and a pair of screens, and the bracket is seven matches: spreading it over
// everything the pools used buys nothing once the passes come out the same.
func TestEliminationMatsTakesTheFewestThatCostsNoPass(t *testing.T) {
	cases := []struct {
		mats, size, want, rounds int
	}{
		// Four quarter-finals, two semi-finals, then bronze beside the final. Three mats
		// take two passes through the quarters and so do two: the third is staffed for
		// nothing, which is the report.
		{mats: 3, size: 8, want: 2, rounds: 4},
		{mats: 4, size: 8, want: 4, rounds: 3},
		{mats: 2, size: 8, want: 2, rounds: 4},
		{mats: 1, size: 8, want: 1, rounds: 8},
		// A cut at four is two semi-finals and then two matches: a third mat can never
		// be busy.
		{mats: 4, size: 4, want: 2, rounds: 2},
		{mats: 3, size: 4, want: 2, rounds: 2},
		{mats: 2, size: 4, want: 2, rounds: 2},
		// A lone final is one match however many mats the hall has.
		{mats: 4, size: 2, want: 1, rounds: 1},
	}
	for _, c := range cases {
		if got := tournament.EliminationMats(c.mats, c.size); got != c.want {
			t.Errorf("EliminationMats(%d, %d) = %d, want %d", c.mats, c.size, got, c.want)
		}
		if got := tournament.EliminationRounds(c.mats, c.size); got != c.rounds {
			t.Errorf("EliminationRounds(%d, %d) = %d, want %d", c.mats, c.size, got, c.rounds)
		}
		// Whatever it suggests has to be worth suggesting: the same number of passes as
		// the full set of mats.
		if a, b := tournament.EliminationRounds(c.want, c.size), tournament.EliminationRounds(c.mats, c.size); a != b {
			t.Errorf("suggesting %d mats for a cut of %d costs %d passes against %d", c.want, c.size, a, b)
		}
	}
}

// The organizer has the last word, within the mats the tournament has.
func TestTheOrganizerOverridesTheSuggestion(t *testing.T) {
	cases := []struct {
		mats, elim, size, want int
	}{
		{mats: 3, elim: 0, size: 8, want: 2}, // unset: the suggestion
		{mats: 3, elim: 3, size: 8, want: 3}, // "I have the people, use them"
		{mats: 3, elim: 1, size: 8, want: 1}, // "one mat, everyone watches"
		{mats: 3, elim: 9, size: 8, want: 3}, // there is no ninth mat
		{mats: 0, elim: 0, size: 8, want: 1}, // an unconfigured tournament still draws
	}
	for _, c := range cases {
		tr := store.Tournament{Mats: c.mats, ElimMats: c.elim}
		if got := tournament.EliminationMatsFor(tr, c.size); got != c.want {
			t.Errorf("mats=%d elimMats=%d size=%d: got %d, want %d", c.mats, c.elim, c.size, got, c.want)
		}
	}
}

// And the draw itself follows it: a three-mat tournament lays its quarter-finals out
// over two mats, not three.
func TestBracketSpreadsOverTheMatsItWasGiven(t *testing.T) {
	ranked := make([]tournament.Standing, 8)
	for i := range ranked {
		ranked[i] = tournament.Standing{Competitor: string(rune('a' + i)), Rank: i + 1}
	}

	matsUsed := func(tr store.Tournament) map[int]int {
		bracket, err := tournament.Bracket(tr, ranked)
		if err != nil {
			t.Fatalf("drawing the bracket: %v", err)
		}
		used := map[int]int{}
		for _, m := range bracket {
			if m.Round == tournament.RoundQuarter {
				used[m.Mat]++
			}
		}
		return used
	}

	used := matsUsed(store.Tournament{Mats: 3})
	if len(used) != 2 || used[1] != 2 || used[2] != 2 {
		t.Errorf("on three mats the quarter-finals should sit two and two on mats 1 and 2, got %v", used)
	}
	if used := matsUsed(store.Tournament{Mats: 3, ElimMats: 3}); len(used) != 3 {
		t.Errorf("the organizer asked for three mats and should get them, got %v", used)
	}
	if used := matsUsed(store.Tournament{Mats: 4}); len(used) != 4 {
		t.Errorf("four mats run the quarter-finals in one pass and should all be used, got %v", used)
	}
	if used := matsUsed(store.Tournament{Mats: 2}); len(used) != 2 {
		t.Errorf("two mats should both be used, got %v", used)
	}
}

func TestBracketSize(t *testing.T) {
	for ranked, want := range map[int]int{0: 0, 1: 0, 2: 2, 3: 2, 4: 4, 7: 4, 8: 8, 20: 8} {
		if got := tournament.BracketSize(ranked); got != want {
			t.Errorf("BracketSize(%d) = %d, want %d", ranked, got, want)
		}
	}
}
