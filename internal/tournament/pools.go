// Package tournament is the server-only logic: pool generation, match ordering, colour
// assignment, mat assignment and the ranking indices. None of it ever runs on a client,
// which is why it is not part of the duplicated match engine (docs/tech-stack.md §4).
package tournament

import (
	"fmt"
	"math/rand"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Limits are the ceiling a run of the application accepts. The code below is written in
// the general form; the ceiling is only these numbers.
type Limits struct {
	MaxMats  int
	MaxPools int
	MaxPool  int
}

// DefaultLimits is the Milestone 2 ceiling (design §7 item 7): up to 4 mats and 8 pools
// of up to 7, so 56 competitors per run. The MVP ran under 2, 4 and 7, and the mat
// assignment below is the same rule at either size.
func DefaultLimits() Limits { return Limits{MaxMats: 4, MaxPools: 8, MaxPool: 7} }

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
		// more than throughput: pool N on mat ((N-1) mod mats)+1. With two mats that is
		// exactly design §6, odd pools to mat 1 and even to mat 2; with four, pools 1
		// and 5 share mat 1.
		mat := DefaultMat(number, t.Mats)
		ids := make([]string, len(bucket))
		for j, c := range bucket {
			ids[j] = c.ID
		}
		matches, poolViolations := schedule(number, mat, ids)
		violations = append(violations, poolViolations...)
		pools = append(pools, store.Pool{
			Number:      number,
			Mat:         mat,
			Sequence:    number,
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
