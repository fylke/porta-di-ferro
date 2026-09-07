package tournament

import (
	"hash/fnv"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Standing is one competitor's line in a pool table. Every index divides by matches
// *completed*, which is what makes retroactive withdrawal work correctly.
type Standing struct {
	Competitor  string `json:"competitor"`
	Name        string `json:"name"`
	Club        string `json:"club"`
	Completed   int    `json:"completed"`
	Wins        int    `json:"wins"`
	Draws       int    `json:"draws"`
	Losses      int    `json:"losses"`
	MatchPoints int    `json:"matchPoints"`
	Scored      int    `json:"scored"`
	Conceded    int    `json:"conceded"`

	// The four indices, in the order they are applied.
	MatchPointIndex float64 `json:"matchPointIndex"`
	VictoryIndex    float64 `json:"victoryIndex"`
	ScoreIndex      float64 `json:"scoreIndex"`
	ReceptionIndex  float64 `json:"receptionIndex"`

	Rank int `json:"rank"`
}

// Rank produces a pool's standings, in order.
//
// The chain is: match point index, victory index, score index, reception index (lowest
// wins), head-to-head, random draw. A withdrawn competitor is not in the table at all,
// and their matches count for nobody -- their results are voided as though they never
// entered.
func Rank(r match.Ruleset, pool store.Pool, competitors map[string]store.Competitor,
	states map[string]match.State, seed int64) []Standing {

	rows := map[string]*Standing{}
	for _, id := range pool.Competitors {
		c := competitors[id]
		if c.Withdrawn {
			continue
		}
		rows[id] = &Standing{Competitor: id, Name: c.Name, Club: c.Club}
	}

	for _, m := range pool.Matches {
		red, redOK := rows[m.Red]
		blue, blueOK := rows[m.Blue]
		// A match involving a withdrawn competitor is voided for the other one too.
		if !redOK || !blueOK {
			continue
		}
		st, ok := states[m.ID]
		if !ok || !st.Ended {
			continue
		}
		red.Completed++
		blue.Completed++
		red.Scored += st.Red.Score
		red.Conceded += st.Blue.Score
		blue.Scored += st.Blue.Score
		blue.Conceded += st.Red.Score

		switch st.Winner {
		case match.Red:
			red.Wins++
			red.MatchPoints += r.WinPoints
			blue.Losses++
			if !st.NoMatchPoints {
				blue.MatchPoints += r.LossPoints
			}
		case match.Blue:
			blue.Wins++
			blue.MatchPoints += r.WinPoints
			red.Losses++
			if !st.NoMatchPoints {
				red.MatchPoints += r.LossPoints
			}
		default:
			red.Draws++
			blue.Draws++
			red.MatchPoints += r.DrawPoints
			blue.MatchPoints += r.DrawPoints
		}
	}

	out := make([]Standing, 0, len(rows))
	for _, s := range rows {
		if s.Completed > 0 {
			d := float64(s.Completed)
			s.MatchPointIndex = float64(s.MatchPoints) / d
			s.VictoryIndex = float64(s.Wins) / d
			s.ScoreIndex = float64(s.Scored-s.Conceded) / d
			s.ReceptionIndex = float64(s.Conceded) / d
		}
		out = append(out, *s)
	}

	head := headToHead(pool, states)
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
			// Lowest wins: fewest points conceded per match.
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
		// Random draw, seeded so the same tournament file always resolves the same way.
		return drawValue(seed, a.Competitor) < drawValue(seed, b.Competitor)
	})

	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func pairKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

// headToHead records who won each decided meeting, for the fifth tie-break.
func headToHead(pool store.Pool, states map[string]match.State) map[string]string {
	out := map[string]string{}
	for _, m := range pool.Matches {
		st, ok := states[m.ID]
		if !ok || !st.Ended || st.Winner == "" {
			continue
		}
		winner := m.Red
		if st.Winner == match.Blue {
			winner = m.Blue
		}
		out[pairKey(m.Red, m.Blue)] = winner
	}
	return out
}

func drawValue(seed int64, id string) uint64 {
	h := fnv.New64a()
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(seed >> (8 * i))
	}
	h.Write(b[:])
	h.Write([]byte(id))
	return h.Sum64()
}
