package httpapi

import (
	"reflect"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The organizer reads names, not identifiers. The warning is stored with identifiers and
// named on the way out, so a rename after the draw is reflected and older files read the
// same way.
func TestDrawWarningsNameTheCompetitors(t *testing.T) {
	byID := map[string]store.Competitor{
		"c3": {ID: "c3", Name: "Torun Sandström Wadell"},
		"c7": {ID: "c7", Name: "Bo Ek"},
	}
	stored := []string{
		"pool 2: c7 or c3 fences two matches in a row",
		// Somebody removed from the register since: left as it was rather than blank.
		"pool 1: c7 or c99 fences two matches in a row",
		// A club warning is about clubs, and is left alone even when a club looks like an id.
		"club c3 has 3 of its 4 in pool 1; spread evenly it would have at most 2",
	}
	got := namedViolations(stored, byID)
	want := []string{
		"pool 2: Bo Ek or Torun Sandström Wadell fences two matches in a row",
		"pool 1: Bo Ek or c99 fences two matches in a row",
		"club c3 has 3 of its 4 in pool 1; spread evenly it would have at most 2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if stored[0] != "pool 2: c7 or c3 fences two matches in a row" {
		t.Error("the stored warnings were changed in place")
	}
}
