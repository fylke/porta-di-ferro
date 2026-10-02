package people_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/people"
	"github.com/fylke/porta-di-ferro/internal/store"
)

func TestIdsAreRandomAndMarked(t *testing.T) {
	a, b := people.NewID(), people.NewID()
	if a == b || !strings.HasPrefix(a, "pr-") || len(a) != 15 {
		t.Errorf("ids should be random and look like pr-xxxxxxxxxxxx: %q %q", a, b)
	}
}

// A signup is exact: the same submission is the same person in every discipline. A name
// is never enough.
func TestWhoANewEntryIs(t *testing.T) {
	var reg []store.Person
	reg, astrid := people.For(reg, "Astrid", "Gbg", "sub-1", "")
	reg, again := people.For(reg, "Astrid", "Gbg", "sub-1", "")
	if again != astrid || len(reg) != 1 {
		t.Errorf("the same submission should be the same person: %q %q", astrid, again)
	}
	reg, twin := people.For(reg, "Astrid", "Gbg", "", "")
	if twin == astrid {
		t.Error("the same name must never be made the same person on its own")
	}
	reg, chosen := people.For(reg, "Astrid L.", "Gbg", "", astrid)
	if chosen != astrid || len(reg) != 2 {
		t.Errorf("a person the organizer picked should be that person: %q", chosen)
	}
	if _, ghost := people.For(reg, "X", "", "", "pr-nobody"); ghost == "pr-nobody" {
		t.Error("asking for a person who does not exist should make a new one, not a dangling link")
	}
}

func TestDuplicatesAreOfferedNotMerged(t *testing.T) {
	reg := []store.Person{{ID: "a", Name: "Karl-Johan"}, {ID: "b", Name: "Karl Johan"}, {ID: "c", Name: "Bo"}}
	entries := map[string][]people.Entry{
		"a": {{Discipline: "ls", Competitor: "c1", Name: "Karl-Johan"}},
		"b": {{Discipline: "sa", Competitor: "c4", Name: "karl johan"}},
		"c": {{Discipline: "ls", Competitor: "c2", Name: "Bo"}},
	}
	if got := people.Duplicates(reg, entries); !reflect.DeepEqual(got, [][]string{{"a", "b"}}) {
		t.Fatalf("Karl-Johan and Karl Johan should be offered as one: %v", got)
	}
	reg, _ = people.KeepApart(reg, "a", "b")
	if got := people.Duplicates(reg, entries); len(got) != 0 {
		t.Errorf("a pair kept apart should stop being offered: %v", got)
	}
}

// A merge moves the entries and can be undone exactly.
func TestAMergeCanBeUndone(t *testing.T) {
	reg := []store.Person{{ID: "a", Name: "Astrid", Signup: "s"}, {ID: "b", Name: "Astrid"}}
	reg, err := people.Merge(reg, "a", "b", []string{"sa/c3"})
	if err != nil {
		t.Fatal(err)
	}
	if people.Resolve(reg, "a") != "b" || people.Active(reg, "a") || reg[1].Signup != "s" {
		t.Errorf("a should now mean b, and b take a's signup: %+v", reg)
	}
	if _, err := people.Merge(reg, "a", "b", nil); err == nil {
		t.Error("merging somebody already merged should be refused")
	}
	reg, moved, err := people.Unmerge(reg, "a")
	if err != nil || !reflect.DeepEqual(moved, []string{"sa/c3"}) || !people.Active(reg, "a") {
		t.Errorf("undoing the merge should give back a and the entries to move back: %v %v %+v", moved, err, reg)
	}
	if _, err := people.Merge(reg, "a", "a", nil); err == nil {
		t.Error("a person cannot be merged into themselves")
	}
}
