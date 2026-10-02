package httpapi

import (
	"fmt"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// Projected work (phase 4, #64's planning mode). The day can be planned before anyone is
// drawn: a discipline with entrants -- or with the number it expects -- and no pools yet
// has the pools and the bracket the draw would give it, made by the draw itself over
// stand-in entrants. They are items like any other on the mat board, with the names the
// real ones will have, so a placement made for "open-sabre/pool-2" at nine in the morning
// is the drawn pool 2's at noon (Place takes it over). They have no matches anybody can
// fence, so a mat never waits on one.

// Entrants is how many a discipline plans for: those entered and not withdrawn, or the
// number it expects when that is more.
func Entrants(competitors []store.Competitor, expected int) int {
	n := 0
	for _, c := range competitors {
		if !c.Withdrawn {
			n++
		}
	}
	return max(n, expected)
}

// PlanItems is a discipline's work items for the plan: what is drawn, and what is not
// drawn yet as the draw would make it for this many entrants.
func PlanItems(slug string, t store.Tournament, entrants int) []WorkItem {
	items := ItemsOf(slug, t)
	return append(items, ProjectedItems(slug, t, entrants)...)
}

// ProjectedItems is what is not drawn yet: the pools while there are none, and the
// bracket while there is none, for this many entrants. Nothing for fewer than two.
func ProjectedItems(slug string, t store.Tournament, entrants int) []WorkItem {
	if entrants < 2 || len(t.Bracket) > 0 {
		return nil
	}
	stand := t
	stand.Bracket, stand.BracketAt = nil, ""
	if len(t.Pools) == 0 {
		if stand.Seed == 0 {
			stand.Seed = 1
		}
		fakes := make([]store.Competitor, entrants)
		for i := range fakes {
			// Every stand-in in a club of their own, so the draw has nothing to spread.
			fakes[i] = store.Competitor{ID: fmt.Sprintf("x%d", i+1), Club: fmt.Sprintf("c%d", i+1)}
		}
		drawn, err := tournament.Generate(stand, fakes, tournament.DefaultLimits())
		if err != nil {
			return nil
		}
		stand.Pools, stand.GeneratedAt = drawn.Pools, ""
	} else {
		stand.Pools = nil
	}
	ranked := make([]tournament.Standing, entrants)
	for i := range ranked {
		ranked[i] = tournament.Standing{Competitor: fmt.Sprintf("x%d", i+1)}
	}
	if bracket, err := tournament.Bracket(stand, ranked); err == nil {
		stand.Bracket = bracket
	}
	items := ItemsOf(slug, stand)
	for i := range items {
		items[i].Stamp, items[i].Projected = "", true
	}
	return items
}
