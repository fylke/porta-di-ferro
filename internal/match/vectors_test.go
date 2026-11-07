package match_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// TestVectors runs the shared corpus. The TypeScript mirror runs the same files, so a
// vector that passes here and fails there fails the build (docs/tech-stack.md §4).
func TestVectors(t *testing.T) {
	vectors, err := loadVectors("../../testdata/vectors")
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
	vectors, err := loadVectors("../../testdata/vectors")
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

// vector is one shared test case from testdata/vectors/. Each is a sequence of log
// events and the expected derived state. Both this package's tests and the TypeScript
// mirror's run the same corpus, and a vector that passes in one and fails in the other
// fails the build (docs/tech-stack.md §4).
//
// The corpus is an executable statement of the ruleset, which is why it is worth more
// than the duplication costs: when rules become data-driven in Milestone 3, it is
// already the specification a ruleset definition has to satisfy.
type vector struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Events      []match.Event `json:"events"`
	Expect      match.State   `json:"expect"`
}

// loadVectors reads every vector in dir, sorted by file name so both suites report in
// the same order.
func loadVectors(dir string) ([]vector, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]vector, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var v vector
		// A UTF-8 byte order mark, which Windows editors add, is not the decoder's to reject.
		dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
		if v.Name == "" {
			v.Name = filepath.Base(p)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no vectors found in %s", dir)
	}
	return out, nil
}
