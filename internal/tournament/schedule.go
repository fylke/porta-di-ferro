package tournament

import (
	"fmt"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// schedule builds a pool's running order: every competitor fences all the others once,
// ordered to minimise consecutive matches, with red and blue assigned per match.
//
// The rounds come from the circle method, which already guarantees that nobody appears
// twice within a round. What is left is the seam between rounds, which a greedy pass
// smooths over: at each step it prefers a match sharing nobody with the one just placed.
func schedule(pool, mat int, ids []string) ([]store.Match, []string) {
	rounds := circle(ids)

	ordered := make([]pair, 0)
	var lastA, lastB string
	var violations []string

	for _, round := range rounds {
		remaining := append([]pair(nil), round...)
		for len(remaining) > 0 {
			pick := -1
			for i, p := range remaining {
				if p.a != lastA && p.a != lastB && p.b != lastA && p.b != lastB {
					pick = i
					break
				}
			}
			if pick < 0 {
				// Unavoidable at this seam. Reported rather than guaranteed away.
				pick = 0
				violations = append(violations, fmt.Sprintf(
					"pool %d: %s or %s fences two matches in a row", pool, remaining[0].a, remaining[0].b))
			}
			p := remaining[pick]
			ordered = append(ordered, p)
			lastA, lastB = p.a, p.b
			remaining = append(remaining[:pick], remaining[pick+1:]...)
		}
	}

	// Colours are fixed at pool creation, for every match in the pool. The aim is a
	// roughly even split across each competitor's own matches -- best-effort, like the
	// ordering, so the competitor with fewer reds so far takes red.
	reds := map[string]int{}
	blues := map[string]int{}
	matches := make([]store.Match, 0, len(ordered))
	for i, p := range ordered {
		red, blue := p.a, p.b
		switch {
		case reds[p.b] < reds[p.a]:
			red, blue = p.b, p.a
		case reds[p.a] == reds[p.b] && blues[p.a] < blues[p.b]:
			red, blue = p.b, p.a
		}
		reds[red]++
		blues[blue]++
		matches = append(matches, store.Match{
			ID:    fmt.Sprintf("p%dm%d", pool, i+1),
			Pool:  pool,
			Order: i + 1,
			Mat:   mat,
			Red:   red,
			Blue:  blue,
		})
	}
	return matches, violations
}

// pair is one meeting, before colours are assigned.
type pair struct{ a, b string }

// circle is the round-robin circle method. With an odd number of competitors one sits out
// each round, which is how uneven pool sizes stay legal rather than special.
func circle(ids []string) [][]pair {
	players := append([]string(nil), ids...)
	const bye = ""
	if len(players)%2 == 1 {
		players = append(players, bye)
	}
	n := len(players)
	rounds := make([][]pair, 0, n-1)
	for r := 0; r < n-1; r++ {
		round := make([]pair, 0, n/2)
		for i := 0; i < n/2; i++ {
			x, y := players[i], players[n-1-i]
			if x != bye && y != bye {
				// Alternating which side of the circle leads spreads the colours before
				// the balancing pass above ever runs.
				if r%2 == 1 {
					x, y = y, x
				}
				round = append(round, pair{x, y})
			}
		}
		rounds = append(rounds, round)
		// Rotate everything but the first player.
		last := players[n-1]
		copy(players[2:], players[1:n-1])
		players[1] = last
	}
	return rounds
}
