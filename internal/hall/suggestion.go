package hall

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// A suggested plan, as the board reads it and as it is applied.

// MoveView is one item a suggestion moves, with where from, where to and when it would
// start there.
type MoveView struct {
	ID           string `json:"id"`
	FromMat      int    `json:"fromMat"`
	FromPosition int    `json:"fromPosition"`
	ToMat        int    `json:"toMat"`
	ToPosition   int    `json:"toPosition"`
	Start        string `json:"start"`
}

// SuggestionView is a suggested plan: what moves, when the day would end with it and
// without it, and a signature that applying it must match -- so a plan that changed since
// the organizer looked is looked at again rather than overwritten.
type SuggestionView struct {
	Moves     []MoveView `json:"moves"`
	End       string     `json:"end"`
	Before    string     `json:"before"`
	Signature string     `json:"signature"`
}

// ViewSuggestion is a suggestion as the board reads it.
func ViewSuggestion(s forecast.Suggestion) SuggestionView {
	out := SuggestionView{Moves: []MoveView{}, End: stamp(s.End), Before: stamp(s.Before), Signature: SignatureOf(s)}
	for _, m := range s.Moves {
		out.Moves = append(out.Moves, MoveView{ID: m.ID, FromMat: m.FromMat, FromPosition: m.FromSeq,
			ToMat: m.ToMat, ToPosition: m.ToSeq, Start: stamp(m.Start)})
	}
	return out
}

// SignatureOf names a suggestion by the order it proposes.
func SignatureOf(s forecast.Suggestion) string {
	var mats []int
	for m := range s.Order {
		mats = append(mats, m)
	}
	sort.Ints(mats)
	var b strings.Builder
	for _, m := range mats {
		fmt.Fprintf(&b, "%d:%s;", m, strings.Join(s.Order[m], ","))
	}
	return b.String()
}

// ApplySuggestion is the placements a suggestion makes. Every item keeps its draw stamp,
// its pin and its hold; its mat and place are the suggestion's, settled as planned.
func ApplySuggestion(placed map[string]store.Placement, s forecast.Suggestion) map[string]store.Placement {
	next := make(map[string]store.Placement, len(placed))
	for k, v := range placed {
		next[k] = v
	}
	for mat, ids := range s.Order {
		for i, id := range ids {
			p := next[id]
			p.Mat, p.Seq, p.Planned = mat, i+1, true
			next[id] = p
		}
	}
	return next
}
