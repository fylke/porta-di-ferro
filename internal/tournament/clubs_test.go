package tournament_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// clubbed builds a field from club sizes: {"A": 5, "B": 3} is five from A and three from B.
// Order matters for reproducibility, so the map is walked in the order given.
func clubbed(sizes ...any) []store.Competitor {
	var out []store.Competitor
	for i := 0; i+1 < len(sizes); i += 2 {
		club, n := sizes[i].(string), sizes[i+1].(int)
		for j := 0; j < n; j++ {
			id := fmt.Sprintf("%s%d", strings.ToLower(strings.ReplaceAll(club, " ", "")), j+1)
			if club == "" {
				id = fmt.Sprintf("solo%d", len(out)+1)
			}
			out = append(out, store.Competitor{ID: id, Name: id, Club: club})
		}
	}
	return out
}

// spread is, per club, the most members any one pool holds.
func spread(t store.Tournament, competitors []store.Competitor) map[string]int {
	club := map[string]string{}
	for _, c := range competitors {
		club[c.ID] = c.Club
	}
	worst := map[string]int{}
	for _, p := range t.Pools {
		count := map[string]int{}
		for _, id := range p.Competitors {
			count[club[id]]++
		}
		for c, n := range count {
			if n > worst[c] {
				worst[c] = n
			}
		}
	}
	return worst
}

func ideal(n, pools int) int { return (n + pools - 1) / pools }

// clubViolations filters out the ordering reports, which a pool of four can never avoid
// and which are not what these tests are about.
func clubViolations(tr store.Tournament) []string {
	var out []string
	for _, v := range tr.Violations {
		if strings.HasPrefix(v, "club ") {
			out = append(out, v)
		}
	}
	return out
}

func sizesWithinOne(t *testing.T, tr store.Tournament) {
	t.Helper()
	lo, hi := 1<<30, 0
	for _, p := range tr.Pools {
		if len(p.Competitors) < lo {
			lo = len(p.Competitors)
		}
		if len(p.Competitors) > hi {
			hi = len(p.Competitors)
		}
	}
	if hi-lo > 1 {
		t.Errorf("pool sizes range from %d to %d; club balancing must not cost size evenness", lo, hi)
	}
}

func TestManyClubsSpreadOnePerPool(t *testing.T) {
	field := clubbed("MSL", 4, "GHFS", 4, "Uppsala", 4, "Malmö", 4, "Örebro", 4, "Umeå", 4, "Lund", 4)
	tr, err := tournament.Generate(store.Tournament{Mats: 2, MinPoolSize: 7, MaxPoolSize: 7, Seed: 7}, field, tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Pools) != 4 {
		t.Fatalf("28 in pools of 7 should be 4 pools, got %d", len(tr.Pools))
	}
	for club, worst := range spread(tr, field) {
		if worst != 1 {
			t.Errorf("%s has %d in one pool; with four members and four pools it should be one each", club, worst)
		}
	}
	if v := clubViolations(tr); len(v) != 0 {
		t.Errorf("a perfectly balanceable field reported club violations: %v", v)
	}
	sizesWithinOne(t, tr)
}

// TestOneClubSuppliesMostOfTheField is the hard case: the big club has to share pools with
// itself, but no more than an even spread requires.
func TestOneClubSuppliesMostOfTheField(t *testing.T) {
	field := clubbed("MSL", 14, "GHFS", 2, "Uppsala", 2, "Lund", 1, "Örebro", 1)
	tr, err := tournament.Generate(store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7, Seed: 3}, field, tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	pools := len(tr.Pools)
	worst := spread(tr, field)
	if want := ideal(14, pools); worst["MSL"] > want {
		t.Errorf("MSL has %d in one pool; 14 over %d pools spreads to at most %d", worst["MSL"], pools, want)
	}
	for _, club := range []string{"GHFS", "Uppsala"} {
		if worst[club] > ideal(2, pools) {
			t.Errorf("%s's two members are in the same pool", club)
		}
	}
	if v := clubViolations(tr); len(v) != 0 {
		t.Errorf("unexpected club violations: %v", v)
	}
	sizesWithinOne(t, tr)
}

// TestGreedAloneIsNotEnough: dealt largest club first, the last club is left with the
// gaps and lands together. The swap pass has to find the exchange that spreads it.
func TestGreedAloneIsNotEnough(t *testing.T) {
	field := clubbed("A", 3, "B", 3, "C", 2)
	tr, err := tournament.Generate(store.Tournament{Mats: 1, MinPoolSize: 4, MaxPoolSize: 4, Seed: 1}, field, tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Pools) != 2 {
		t.Fatalf("want 2 pools, got %d", len(tr.Pools))
	}
	worst := spread(tr, field)
	if worst["C"] != 1 {
		t.Errorf("C's two members should be in different pools, worst pool has %d", worst["C"])
	}
	if worst["A"] > 2 || worst["B"] > 2 {
		t.Errorf("A or B over-concentrated: %v", worst)
	}
	if v := clubViolations(tr); len(v) != 0 {
		t.Errorf("unexpected club violations: %v", v)
	}
}

func TestUnevenPoolSizesStayWithinOne(t *testing.T) {
	field := clubbed("MSL", 9, "GHFS", 6, "Uppsala", 5, "", 3)
	for seed := int64(1); seed <= 20; seed++ {
		tr, err := tournament.Generate(store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7, Seed: seed}, field, tournament.DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		sizesWithinOne(t, tr)
		pools := len(tr.Pools)
		for club, worst := range spread(tr, field) {
			if club == "" {
				continue
			}
			n := 0
			for _, c := range field {
				if c.Club == club {
					n++
				}
			}
			if worst > ideal(n, pools) {
				// Allowed only if reported.
				reported := false
				for _, v := range tr.Violations {
					if strings.Contains(v, "club "+club) {
						reported = true
					}
				}
				if !reported {
					t.Errorf("seed %d: %s has %d in one pool, over %d, and nothing was reported", seed, club, worst, ideal(n, pools))
				}
			}
		}
	}
}

func TestTheDrawIsReproducibleFromItsSeed(t *testing.T) {
	field := clubbed("MSL", 6, "GHFS", 5, "Uppsala", 4, "Lund", 3)
	a, _ := tournament.Generate(store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7, Seed: 42}, field, tournament.DefaultLimits())
	b, _ := tournament.Generate(store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7, Seed: 42}, field, tournament.DefaultLimits())
	for i := range a.Pools {
		if strings.Join(a.Pools[i].Competitors, ",") != strings.Join(b.Pools[i].Competitors, ",") {
			t.Fatalf("the same seed drew different pools:\n%v\n%v", a.Pools[i].Competitors, b.Pools[i].Competitors)
		}
	}
}

// TestAClubInternalEventIsUnaffected: everyone is the same club, the deal is a plain
// round-robin, and nothing is reported -- balancing has nothing to do and says nothing.
func TestAClubInternalEventIsUnaffected(t *testing.T) {
	field := clubbed("MSL", 22)
	tr, err := tournament.Generate(store.Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7, Seed: 5}, field, tournament.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	sizesWithinOne(t, tr)
	for _, v := range tr.Violations {
		if strings.HasPrefix(v, "club ") {
			t.Errorf("a one-club field reported a club violation: %s", v)
		}
	}
}
