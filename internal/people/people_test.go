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
	reg, astrid, _ := people.For(reg, "Astrid", "Gbg", "sub-1", "astrid@example.org", "")
	reg, again, _ := people.For(reg, "Astrid", "Gbg", "sub-1", "", "")
	if again != astrid || len(reg) != 1 {
		t.Errorf("the same submission should be the same person: %q %q", astrid, again)
	}
	reg, twin, _ := people.For(reg, "Astrid", "Gbg", "", "", "")
	if twin == astrid {
		t.Error("the same name must never be made the same person on its own")
	}
	reg, chosen, _ := people.For(reg, "Astrid L.", "Gbg", "", "", astrid)
	if chosen != astrid || len(reg) != 2 {
		t.Errorf("a person the organizer picked should be that person: %q", chosen)
	}
	if _, ghost, _ := people.For(reg, "X", "", "", "", "pr-nobody"); ghost == "pr-nobody" {
		t.Error("asking for a person who does not exist should make a new one, not a dangling link")
	}
	// The same email from another submission -- a second signup from her phone -- is her
	// again (#142), however it is capitalised; a new address is somebody new.
	reg, phone, changed := people.For(reg, "Astrid L", "Gbg", "sub-9", " Astrid@Example.org", "")
	if phone != astrid || changed {
		t.Errorf("the same email should be the same person, unchanged: %q %v", phone, changed)
	}
	if people.ByEmail(reg, "ASTRID@example.org") != astrid {
		t.Error("ByEmail should find her whatever the case")
	}
	reg, other, _ := people.For(reg, "Astrid", "Gbg", "sub-10", "another@example.org", "")
	if other == astrid {
		t.Error("another address is another person")
	}
	// Somebody first entered without an email learns it from their signup.
	reg, desk, _ := people.For(reg, "Bo", "Sthlm", "sub-11", "", "")
	reg, _, learnt := people.For(reg, "Bo", "Sthlm", "sub-11", "bo@example.org", "")
	if !learnt || people.ByEmail(reg, "bo@example.org") != desk {
		t.Error("a person found by submission should keep the email they signed up with")
	}
}

// Two people with the same name and two different addresses are two people: nobody
// asks the organizer about them (#142).
func TestTwoAddressesAreNeverTheSamePerson(t *testing.T) {
	reg := []store.Person{
		{ID: "a", Name: "Anna Nilsson", Email: "anna@one.se"},
		{ID: "b", Name: "Anna Nilsson", Email: "anna@two.se"},
		{ID: "c", Name: "Anna Nilsson"},
	}
	got := people.Duplicates(reg, nil)
	if len(got) != 1 || len(got[0]) != 3 {
		t.Fatalf("the one without an address may be either, so the group is still asked about: %v", got)
	}
	reg = reg[:2]
	if got := people.Duplicates(reg, nil); len(got) != 0 {
		t.Errorf("two addresses are two people, got %v", got)
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

// A person nobody needs is dropped when the event opens (#127); anyone with an entry, a
// signup, a merge or a staff place is kept.
func TestPeopleNobodyNeedsAreDropped(t *testing.T) {
	reg := []store.Person{
		{ID: "pr-typo", Name: "Astird", Apart: []string{"pr-entry"}},
		{ID: "pr-entry", Name: "Astrid", Apart: []string{"pr-typo"}},
		{ID: "pr-signup", Name: "Bo", Signup: "sub-1"},
		{ID: "pr-merged", Name: "Bo B", MergedInto: "pr-into"},
		{ID: "pr-into", Name: "Bo Berg"},
		{ID: "pr-staff", Name: "Cleo"},
	}
	rosters := []people.Roster{{Discipline: "ls", Competitors: []store.Competitor{{ID: "c1", Name: "Astrid", Person: "pr-entry"}}}}
	got := people.Prune(reg, rosters, map[string]bool{"pr-staff": true})
	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	if want := []string{"pr-entry", "pr-signup", "pr-merged", "pr-into", "pr-staff"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("kept %v, want %v", ids, want)
	}
	if len(got[0].Apart) != 0 {
		t.Errorf("Astrid should forget being kept apart from a person who is gone: %v", got[0].Apart)
	}
}
