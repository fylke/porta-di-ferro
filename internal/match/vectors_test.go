package match_test

import (
	"encoding/json"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// TestVectors runs the shared corpus. The TypeScript mirror runs the same files, so a
// vector that passes here and fails there fails the build (docs/tech-stack.md §4).
func TestVectors(t *testing.T) {
	vectors, err := match.LoadVectors("../../testdata/vectors")
	if err != nil {
		t.Fatalf("loading vectors: %v", err)
	}
	if len(vectors) < 20 {
		t.Fatalf("expected the corpus to cover the ruleset, found only %d vectors", len(vectors))
	}
	rules := match.MSL()
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			got := match.Replay(rules, v.Events)
			gotJSON, _ := json.MarshalIndent(got, "", "  ")
			wantJSON, _ := json.MarshalIndent(v.Expect, "", "  ")
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("%s\n%s\n\ngot:\n%s\n\nwant:\n%s",
					v.Name, v.Description, gotJSON, wantJSON)
			}
		})
	}
}

// TestVectorsAreDeterministic replays every vector twice. A derived score depends only on
// the log and not on when it is replayed.
func TestVectorsAreDeterministic(t *testing.T) {
	vectors, err := match.LoadVectors("../../testdata/vectors")
	if err != nil {
		t.Fatalf("loading vectors: %v", err)
	}
	rules := match.MSL()
	for _, v := range vectors {
		first := match.Replay(rules, v.Events)
		second := match.Replay(rules, v.Events)
		if first != second {
			t.Errorf("%s: replay is not deterministic\nfirst:  %+v\nsecond: %+v", v.Name, first, second)
		}
	}
}
