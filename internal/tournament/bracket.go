package tournament

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Eliminations (design §7 item 3): the knockout after the pools, top 8, single
// elimination, with a final and a match for third place -- which is what MSL's SM rules
// run, best-of-three finals aside (that is Milestone 3).
//
// Seeding is the overall ranking across every pool by the same chain the pool tables
// use, and the bracket is the standard one: 1 v 8 and 4 v 5 feed one semi-final, 2 v 7
// and 3 v 6 the other, so the top two seeds cannot meet before the final. A field of
// fewer than eight is cut at four, and fewer than four at two.
//
// Later rounds are not stored with competitors in them. Each bracket match names the
// matches whose results fill its slots, and Fill resolves them from the live states, so
// the bracket is derived from the log like everything else and a corrected quarter-final
// corrects the semi-final for free.

// The rounds, in playing order.
const (
	RoundQuarter = "quarter"
	RoundSemi    = "semi"
	RoundBronze  = "bronze"
	RoundFinal   = "final"
)

// Overall ranks everyone who fenced in the pools, by the same chain as the pool tables.
// Every index is per match completed, which is what makes rows from different pools
// comparable. Head-to-head applies only to two who met, which means the same pool; the
// seeded draw settles what is left, as it does within a pool.
func Overall(r match.Ruleset, t store.Tournament, competitors map[string]store.Competitor,
	states map[string]match.State) []Standing {

	var out []Standing
	head := map[string]string{}
	for _, p := range t.Pools {
		out = append(out, Rank(r, p, competitors, states, t.Seed)...)
		for k, v := range headToHead(p, states) {
			head[k] = v
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.MatchPointIndex != b.MatchPointIndex {
			return a.MatchPointIndex > b.MatchPointIndex
		}
		if a.VictoryIndex != b.VictoryIndex {
			return a.VictoryIndex > b.VictoryIndex
		}
		if a.ScoreIndex != b.ScoreIndex {
			return a.ScoreIndex > b.ScoreIndex
		}
		if a.ReceptionIndex != b.ReceptionIndex {
			return a.ReceptionIndex < b.ReceptionIndex
		}
		if w, ok := head[pairKey(a.Competitor, b.Competitor)]; ok {
			if w == a.Competitor {
				return true
			}
			if w == b.Competitor {
				return false
			}
		}
		return drawValue(t.Seed, a.Competitor) < drawValue(t.Seed, b.Competitor)
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// PoolsComplete reports whether every pool match involving two entered competitors has
// ended. The cut is made from finished pools; a bracket drawn early would seed from a
// table that is still moving.
func PoolsComplete(t store.Tournament, competitors map[string]store.Competitor,
	states map[string]match.State) bool {
	for _, p := range t.Pools {
		for _, m := range p.Matches {
			if competitors[m.Red].Withdrawn || competitors[m.Blue].Withdrawn {
				continue
			}
			if st, ok := states[m.ID]; !ok || !st.Ended {
				return false
			}
		}
	}
	return len(t.Pools) > 0
}

// BracketSize is how many of the ranked make the cut: top 8, cut at 4 for a small field
// and at 2 for a very small one. 0 means there are not enough to run eliminations.
func BracketSize(ranked int) int {
	switch {
	case ranked >= 8:
		return 8
	case ranked >= 4:
		return 4
	case ranked >= 2:
		return 2
	default:
		return 0
	}
}

// waves is the bracket as the groups of matches that can run at once: four
// quarter-finals, then two semi-finals, then the bronze match beside the final. How long
// the eliminations take is the sum over these of how many passes each needs.
func waves(size int) []int {
	switch size {
	case 8:
		return []int{4, 2, 2}
	case 4:
		return []int{2, 2}
	default:
		return []int{1}
	}
}

// EliminationRounds is how many passes a bracket of this size takes on this many mats.
func EliminationRounds(mats, size int) int {
	if mats < 1 {
		mats = 1
	}
	rounds := 0
	for _, w := range waves(size) {
		rounds += (w + mats - 1) / mats
	}
	return rounds
}

// EliminationMats suggests how many mats to dedicate to the eliminations: the fewest
// that finish them in as many passes as the tournament's full set of mats would.
//
// A mat is not free. It is a referee, a score keeper and a pair of screens, and by the
// eliminations those people have been at it all day. Three mats through four
// quarter-finals takes two passes, and so does two -- the third mat is staffed for
// nothing (issue #94). The pools have a reason to spread across everything available,
// because they run for hours; a bracket of seven matches does not.
//
// This is a suggestion. The organizer can say otherwise, and Tournament.ElimMats is
// where that answer lives.
func EliminationMats(mats, size int) int {
	if mats < 1 {
		mats = 1
	}
	target := EliminationRounds(mats, size)
	for m := 1; m < mats; m++ {
		if EliminationRounds(m, size) == target {
			return m
		}
	}
	return mats
}

// EliminationMatsFor resolves what a tournament actually runs the eliminations on: the
// organizer's answer when they gave one, and the suggestion otherwise. An answer past
// the tournament's mats is clamped -- there is no fifth mat to send anyone to.
func EliminationMatsFor(t store.Tournament, size int) int {
	mats := t.Mats
	if mats < 1 {
		mats = 1
	}
	if t.ElimMats <= 0 {
		return EliminationMats(mats, size)
	}
	if t.ElimMats > mats {
		return mats
	}
	return t.ElimMats
}

// Bracket draws the knockout from the overall ranking, assigning mats round-robin so
// the quarter-finals run in parallel, the bronze match and the final each on their own
// mat where there are two. The higher seed takes red.
//
// How many mats it spreads over is EliminationMatsFor: fewest that costs no extra pass,
// unless the organizer said otherwise.
func Bracket(t store.Tournament, ranked []Standing) ([]store.Match, error) {
	size := BracketSize(len(ranked))
	if size == 0 {
		return nil, fmt.Errorf("need at least 2 ranked competitors for eliminations, have %d", len(ranked))
	}
	seed := func(n int) string { return ranked[n-1].Competitor }
	mats := EliminationMatsFor(t, size)
	mat := func(i int) int { return (i % mats) + 1 }

	var out []store.Match
	order := 0
	add := func(id, round string, slot int, red, blue, feedRed, feedBlue string, m int) {
		order++
		out = append(out, store.Match{
			ID: id, Order: order, Mat: m, Round: round, Slot: slot,
			Red: red, Blue: blue, FeedRed: feedRed, FeedBlue: feedBlue,
		})
	}

	switch size {
	case 8:
		// 1 v 8 and 4 v 5 feed the first semi; 2 v 7 and 3 v 6 the second.
		add("e-qf1", RoundQuarter, 1, seed(1), seed(8), "", "", mat(0))
		add("e-qf2", RoundQuarter, 2, seed(4), seed(5), "", "", mat(1))
		add("e-qf3", RoundQuarter, 3, seed(2), seed(7), "", "", mat(2))
		add("e-qf4", RoundQuarter, 4, seed(3), seed(6), "", "", mat(3))
		add("e-sf1", RoundSemi, 1, "", "", "winner:e-qf1", "winner:e-qf2", mat(0))
		add("e-sf2", RoundSemi, 2, "", "", "winner:e-qf3", "winner:e-qf4", mat(1))
	case 4:
		add("e-sf1", RoundSemi, 1, seed(1), seed(4), "", "", mat(0))
		add("e-sf2", RoundSemi, 2, seed(2), seed(3), "", "", mat(1))
	}
	if size >= 4 {
		// Bronze before the final on a one-mat hall; beside it otherwise.
		add("e-b", RoundBronze, 1, "", "", "loser:e-sf1", "loser:e-sf2", mat(1))
		add("e-f", RoundFinal, 1, "", "", "winner:e-sf1", "winner:e-sf2", mat(0))
	} else {
		add("e-f", RoundFinal, 1, seed(1), seed(2), "", "", mat(0))
	}
	return out, nil
}

// Fill resolves the competitors of later rounds from the results of the matches that
// feed them. A slot stays empty until its feeder has ended with a winner.
func Fill(bracket []store.Match, states map[string]match.State) []store.Match {
	byID := map[string]store.Match{}
	out := make([]store.Match, len(bracket))
	for i, m := range bracket {
		if m.FeedRed != "" {
			m.Red = resolve(m.FeedRed, byID, states)
		}
		if m.FeedBlue != "" {
			m.Blue = resolve(m.FeedBlue, byID, states)
		}
		out[i] = m
		byID[m.ID] = m
	}
	return out
}

// resolve turns "winner:e-qf1" into a competitor id, or "" if not decided yet.
func resolve(feed string, byID map[string]store.Match, states map[string]match.State) string {
	want, id, ok := strings.Cut(feed, ":")
	if !ok {
		return ""
	}
	m, known := byID[id]
	st, played := states[id]
	if !known || !played || !st.Ended || st.Winner == "" || m.Red == "" || m.Blue == "" {
		return ""
	}
	winner, loser := m.Red, m.Blue
	if st.Winner == match.Blue {
		winner, loser = m.Blue, m.Red
	}
	if want == "loser" {
		return loser
	}
	return winner
}

// Podium is the tournament's result: empty strings where not yet decided.
type Podium struct {
	First  string `json:"first"`
	Second string `json:"second"`
	Third  string `json:"third"`
}

// Result reads the podium off a filled bracket.
func Result(filled []store.Match, states map[string]match.State) Podium {
	var out Podium
	byID := map[string]store.Match{}
	for _, m := range filled {
		byID[m.ID] = m
	}
	out.First = resolve("winner:e-f", byID, states)
	out.Second = resolve("loser:e-f", byID, states)
	out.Third = resolve("winner:e-b", byID, states)
	return out
}
