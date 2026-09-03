// Package tournament is the server-only logic: pool generation, match ordering, colour
// assignment, mat assignment and the ranking indices. None of it ever runs on a client,
// which is why it is not part of the duplicated match engine (docs/tech-stack.md §4).
package tournament

import (
	"fmt"
	"math/rand"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Limits are the MVP ceiling from design §6: up to 2 mats, up to 4 pools, up to 7
// competitors each. Milestone 2 raises them to 4 mats and 8 pools; the code below is
// already written in the general form, so that is a change to these numbers.
type Limits struct {
	MaxMats  int
	MaxPools int
	MaxPool  int
}

// MVPLimits is the ceiling the 15 November event runs under.
func MVPLimits() Limits { return Limits{MaxMats: 2, MaxPools: 4, MaxPool: 7} }

// Generate draws the pools for a tournament: sizes honouring the configured minimum and
// maximum with uneven sizes accepted, a running order that minimises consecutive matches
// on a best-effort basis, and red and blue assigned for every match.
//
// Anything it could not satisfy comes back in Violations rather than being guaranteed
// away (design §6 item 9).
func Generate(t store.Tournament, competitors []store.Competitor, lim Limits) (store.Tournament, error) {
	entered := make([]store.Competitor, 0, len(competitors))
	for _, c := range competitors {
		if !c.Withdrawn {
			entered = append(entered, c)
		}
	}
	if len(entered) < 2 {
		return t, fmt.Errorf("need at least 2 competitors to draw pools, have %d", len(entered))
	}
	if t.Mats < 1 || t.Mats > lim.MaxMats {
		return t, fmt.Errorf("mats must be between 1 and %d", lim.MaxMats)
	}
	if t.MinPoolSize < 2 || t.MaxPoolSize < t.MinPoolSize {
		return t, fmt.Errorf("pool size range %d-%d is not usable", t.MinPoolSize, t.MaxPoolSize)
	}
	maxPool := t.MaxPoolSize
	if maxPool > lim.MaxPool {
		maxPool = lim.MaxPool
	}
	if len(entered) > lim.MaxPools*maxPool {
		return t, fmt.Errorf("%d competitors is past the ceiling of %d for this milestone",
			len(entered), lim.MaxPools*maxPool)
	}

	var violations []string
	count := poolCount(len(entered), t.MinPoolSize, maxPool, lim.MaxPools, &violations)

	if t.Seed == 0 {
		t.Seed = rand.Int63()
	}
	// A seeded shuffle keeps the draw reproducible: a standing can be explained after the
	// fact rather than being a fresh coin toss every time the file is reopened.
	order := make([]store.Competitor, len(entered))
	copy(order, entered)
	rng := rand.New(rand.NewSource(t.Seed))
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })

	// Dealt round-robin, so sizes differ by at most one. Club balancing is Milestone 2.
	buckets := make([][]store.Competitor, count)
	for i, c := range order {
		b := i % count
		buckets[b] = append(buckets[b], c)
	}

	pools := make([]store.Pool, 0, count)
	for i, bucket := range buckets {
		number := i + 1
		// Mat assignment is fixed and predictable, because confusion at the mat costs
		// more than throughput. With two mats this is exactly design §6: odd pools to
		// mat 1, even to mat 2.
		mat := ((number - 1) % t.Mats) + 1
		ids := make([]string, len(bucket))
		for j, c := range bucket {
			ids[j] = c.ID
		}
		matches, poolViolations := schedule(number, mat, ids)
		violations = append(violations, poolViolations...)
		pools = append(pools, store.Pool{
			Number:      number,
			Mat:         mat,
			Competitors: ids,
			Matches:     matches,
		})
	}

	t.Pools = pools
	t.Violations = violations
	return t, nil
}

// poolCount picks how many pools to draw. Uneven sizes are accepted; what is reported is
// a range that could not be honoured at all.
func poolCount(n, min, max, ceiling int, violations *[]string) int {
	count := (n + max - 1) / max
	if count < 1 {
		count = 1
	}
	// Prefer the fewest pools that keeps every pool at or above the minimum.
	for count < ceiling && n/(count+1) >= min && (n+count)/(count+1) <= max {
		count++
	}
	if count > ceiling {
		count = ceiling
		*violations = append(*violations, fmt.Sprintf(
			"capped at %d pools for this milestone; some pools are over the requested maximum", ceiling))
	}
	if n/count < min {
		*violations = append(*violations, fmt.Sprintf(
			"%d competitors cannot fill %d pools of at least %d; pools are smaller than requested",
			n, count, min))
	}
	return count
}
