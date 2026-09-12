package tournament_test

import (
	"fmt"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

func field(n int) []store.Competitor {
	out := make([]store.Competitor, n)
	for i := range out {
		out[i] = store.Competitor{ID: fmt.Sprintf("c%d", i+1), Name: fmt.Sprintf("Competitor %d", i+1)}
	}
	return out
}

func TestPoolsHonourTheSizeRangeAndMatAssignment(t *testing.T) {
	for _, mats := range []int{1, 2, 3, 4} {
		for _, n := range []int{4, 5, 7, 8, 12, 20, 28, 41, 56} {
			setup := store.Tournament{Mats: mats, MinPoolSize: 4, MaxPoolSize: 7}
			drawn, err := tournament.Generate(setup, field(n), tournament.DefaultLimits())
			if err != nil {
				t.Fatalf("%d mats, %d competitors: %v", mats, n, err)
			}
			total := 0
			for _, p := range drawn.Pools {
				total += len(p.Competitors)
				if len(p.Competitors) > 7 {
					t.Errorf("%d mats, %d competitors: pool %d has %d, over the ceiling of 7",
						mats, n, p.Number, len(p.Competitors))
				}
				// Pool N on mat ((N-1) mod mats)+1: with two mats, odd to 1 and even to 2.
				if want := ((p.Number - 1) % mats) + 1; p.Mat != want {
					t.Errorf("%d mats, %d competitors: pool %d is on mat %d, want %d",
						mats, n, p.Number, p.Mat, want)
				}
				for _, m := range p.Matches {
					if m.Mat != p.Mat {
						t.Errorf("pool %d is on mat %d but match %s says mat %d", p.Number, p.Mat, m.ID, m.Mat)
					}
				}
				// Everyone fences everyone else once.
				size := len(p.Competitors)
				if want := size * (size - 1) / 2; len(p.Matches) != want {
					t.Errorf("%d competitors: pool %d has %d matches, want %d",
						n, p.Number, len(p.Matches), want)
				}
			}
			if total != n {
				t.Errorf("%d mats, %d competitors: %d were drawn into pools", mats, n, total)
			}
			if len(drawn.Pools) > 8 {
				t.Errorf("%d competitors drew %d pools, over the ceiling of 8", n, len(drawn.Pools))
			}
		}
	}
}

func TestEveryPairMeetsExactlyOnce(t *testing.T) {
	setup := store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7}
	drawn, err := tournament.Generate(setup, field(13), tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range drawn.Pools {
		seen := map[string]int{}
		for _, m := range p.Matches {
			key := m.Red + "-" + m.Blue
			if m.Blue < m.Red {
				key = m.Blue + "-" + m.Red
			}
			seen[key]++
		}
		for pair, count := range seen {
			if count != 1 {
				t.Errorf("pool %d: %s meet %d times", p.Number, pair, count)
			}
		}
	}
}

// TestColoursAreRoughlyEven checks the best-effort split: nobody should be red or blue
// far more often than the other within their own pool.
func TestColoursAreRoughlyEven(t *testing.T) {
	setup := store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7}
	drawn, err := tournament.Generate(setup, field(24), tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range drawn.Pools {
		reds, blues := map[string]int{}, map[string]int{}
		for _, m := range p.Matches {
			reds[m.Red]++
			blues[m.Blue]++
		}
		for _, id := range p.Competitors {
			if d := reds[id] - blues[id]; d > 1 || d < -1 {
				t.Errorf("pool %d: %s is red %d times and blue %d", p.Number, id, reds[id], blues[id])
			}
		}
	}
}

// TestConsecutiveMatchesAreMinimised checks the running order. The circle method plus the
// greedy pass at each seam should leave nothing to report at these sizes.
func TestConsecutiveMatchesAreMinimised(t *testing.T) {
	setup := store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7}
	drawn, err := tournament.Generate(setup, field(22), tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range drawn.Pools {
		for i := 1; i < len(p.Matches); i++ {
			prev, cur := p.Matches[i-1], p.Matches[i]
			for _, id := range []string{cur.Red, cur.Blue} {
				if id == prev.Red || id == prev.Blue {
					// Allowed, but only if the generator said so.
					if len(drawn.Violations) == 0 {
						t.Errorf("pool %d: %s fences matches %d and %d back to back, unreported",
							p.Number, id, prev.Order, cur.Order)
					}
				}
			}
		}
	}
}

func TestPastTheCeilingIsRejected(t *testing.T) {
	setup := store.Tournament{Mats: 4, MinPoolSize: 4, MaxPoolSize: 7}
	if _, err := tournament.Generate(setup, field(56), tournament.DefaultLimits()); err != nil {
		t.Errorf("56 competitors is exactly the ceiling and should draw: %v", err)
	}
	if _, err := tournament.Generate(setup, field(57), tournament.DefaultLimits()); err == nil {
		t.Error("57 competitors is past the ceiling of 56 and should be refused")
	}
	five := store.Tournament{Mats: 5, MinPoolSize: 4, MaxPoolSize: 7}
	if _, err := tournament.Generate(five, field(20), tournament.DefaultLimits()); err == nil {
		t.Error("5 mats is past the ceiling of 4 and should be refused")
	}
}
