package match

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Vector is one shared test case from testdata/vectors/. Each is a sequence of log
// events and the expected derived state. Both this package's tests and the TypeScript
// mirror's run the same corpus, and a vector that passes in one and fails in the other
// fails the build (docs/tech-stack.md §4).
//
// The corpus is an executable statement of the ruleset, which is why it is worth more
// than the duplication costs: when rules become data-driven in Milestone 3, it is
// already the specification a ruleset definition has to satisfy.
type Vector struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Events      []Event `json:"events"`
	Expect      State   `json:"expect"`
}

// LoadVectors reads every vector in dir, sorted by file name so both suites report in
// the same order.
func LoadVectors(dir string) ([]Vector, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]Vector, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var v Vector
		dec := json.NewDecoder(newTrimReader(b))
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
