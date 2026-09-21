package tournament

import (
	"fmt"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Club balancing in pool generation (design §7 item 13, issue #3).
//
// The objectives, in the order they give way:
//
//  1. Pool sizes within one of each other. Never traded away: the deal below keeps it,
//     and the swap pass only ever exchanges one competitor for one.
//  2. Club spread -- no pool holding more of a club than an even spread would give it.
//     Best-effort: a greedy deal gets most of the way, a swap pass fixes what greed
//     left, and what remains is reported rather than prevented.
//  3. The running order and the colours are decided per pool afterwards (schedule.go),
//     and neither can conflict with the two above.
//
// It has no effect at a club-internal event, where everyone is the same club, which is
// why it is tested synthetically rather than at MSL's own night.

// deal spreads an already shuffled field over count pools.
//
// Club by club, largest first, each competitor to the pool with the fewest competitors so
// far; among pools tied on size, the one with the fewest of that club, then the lowest
// index. Size stays within one of even, exactly as a plain round-robin deal did, and the
// tie-break is what spreads a club across the pools instead of leaving it to the shuffle.
// Competitors with no club go last, so the clubs get first pick of the spread and the
// unaffiliated fill in.
func deal(order []store.Competitor, count int) [][]store.Competitor {
	byClub := map[string][]store.Competitor{}
	var clubs []string
	for _, c := range order {
		if _, seen := byClub[c.Club]; !seen {
			clubs = append(clubs, c.Club)
		}
		byClub[c.Club] = append(byClub[c.Club], c)
	}
	// Largest club first, the unaffiliated last; the shuffle's order breaks ties, which is
	// what keeps the draw reproducible from its seed.
	sort.SliceStable(clubs, func(i, j int) bool {
		if (clubs[i] == "") != (clubs[j] == "") {
			return clubs[j] == ""
		}
		return len(byClub[clubs[i]]) > len(byClub[clubs[j]])
	})

	buckets := make([][]store.Competitor, count)
	for _, club := range clubs {
		for _, c := range byClub[club] {
			best := 0
			for b := 1; b < count; b++ {
				if len(buckets[b]) < len(buckets[best]) ||
					(len(buckets[b]) == len(buckets[best]) && ofClub(buckets[b], club) < ofClub(buckets[best], club)) {
					best = b
				}
			}
			buckets[best] = append(buckets[best], c)
		}
	}
	return buckets
}

// balance improves the club spread by swapping competitors between pools, and reports
// whatever it could not fix.
//
// Greed alone leaves the last clubs dealt with whatever gaps remain, so a small club can
// end up together in one pool while a swap with a bigger club's member would have spread
// both. Each swap exchanges exactly one competitor for one, so sizes never move, and it
// is taken only when the total overspill strictly falls, so it terminates.
func balance(buckets [][]store.Competitor) []string {
	for improved := true; improved; {
		improved = false
		before := overspill(buckets)
		if before == 0 {
			break
		}
	search:
		for i := range buckets {
			for j := i + 1; j < len(buckets); j++ {
				for a := range buckets[i] {
					for b := range buckets[j] {
						if buckets[i][a].Club == buckets[j][b].Club {
							continue
						}
						buckets[i][a], buckets[j][b] = buckets[j][b], buckets[i][a]
						if overspill(buckets) < before {
							improved = true
							break search
						}
						buckets[i][a], buckets[j][b] = buckets[j][b], buckets[i][a]
					}
				}
			}
		}
	}

	var violations []string
	for club, total := range clubSizes(buckets) {
		if club == "" {
			continue
		}
		ideal := (total + len(buckets) - 1) / len(buckets)
		for i, bucket := range buckets {
			if n := ofClub(bucket, club); n > ideal {
				violations = append(violations, fmt.Sprintf(
					"club %s has %d of its %d in pool %d; spread evenly it would have at most %d",
					club, n, total, i+1, ideal))
			}
		}
	}
	sort.Strings(violations)
	return violations
}

// overspill is how far the spread is from even: for every club and pool, the members
// beyond what an even spread would put there, summed.
func overspill(buckets [][]store.Competitor) int {
	total := 0
	for club, size := range clubSizes(buckets) {
		if club == "" {
			continue
		}
		ideal := (size + len(buckets) - 1) / len(buckets)
		for _, bucket := range buckets {
			if n := ofClub(bucket, club); n > ideal {
				total += n - ideal
			}
		}
	}
	return total
}

func clubSizes(buckets [][]store.Competitor) map[string]int {
	sizes := map[string]int{}
	for _, bucket := range buckets {
		for _, c := range bucket {
			sizes[c.Club]++
		}
	}
	return sizes
}

func ofClub(bucket []store.Competitor, club string) int {
	n := 0
	for _, c := range bucket {
		if c.Club == club {
			n++
		}
	}
	return n
}
