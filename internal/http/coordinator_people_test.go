package httpapi_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The event's people (proposal §8, phase 3): every entry is somebody, the same somebody
// in every discipline they entered, and the organizer -- never a name match -- decides who
// is one person.

type entries struct {
	Competitors []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Person string `json:"person"`
	} `json:"competitors"`
}

func (h *hall) people() httpapi.PeopleView {
	h.t.Helper()
	var v httpapi.PeopleView
	h.must("GET", "/api/people", nil, &v)
	return v
}

func (h *hall) personOf(slug, name string) string {
	h.t.Helper()
	var s entries
	h.must("GET", "/api/d/"+slug+"/state", nil, &s)
	for _, c := range s.Competitors {
		if c.Name == name {
			return c.Person
		}
	}
	h.t.Fatalf("%s is not entered in %s", name, slug)
	return ""
}

func twoDisciplines(t *testing.T) (*hall, string) {
	t.Helper()
	h := openHall(t, t.TempDir())
	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	ls := view.Disciplines[0].Slug
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)
	return h, ls
}

func TestEveryEntryIsSomebody(t *testing.T) {
	h, ls := twoDisciplines(t)
	h.must("POST", "/api/d/"+ls+"/competitors", map[string]string{"name": "Astrid", "club": "Gbg"}, nil)
	astrid := h.personOf(ls, "Astrid")
	if !strings.HasPrefix(astrid, "pr-") {
		t.Fatalf("a desk entry should be a person of the event's, got %q", astrid)
	}

	// The desk says this is Astrid again: the same person in Sabre.
	h.must("POST", "/api/d/open-sabre/competitors", map[string]string{"name": "Astrid", "club": "Gbg", "person": astrid}, nil)
	if got := h.personOf("open-sabre", "Astrid"); got != astrid {
		t.Errorf("an entry the desk linked should be that person: %q, want %q", got, astrid)
	}
	// The same name with nobody saying so is somebody else, offered as a duplicate.
	h.must("POST", "/api/d/open-sabre/competitors", map[string]string{"name": "astrid", "club": "Gbg"}, nil)
	twin := h.personOf("open-sabre", "astrid")
	if twin == astrid {
		t.Fatal("a name match must never make two entries one person on its own")
	}
	v := h.people()
	if len(v.Duplicates) != 1 || len(v.Duplicates[0]) != 2 {
		t.Errorf("the two Astrids should be offered as one: %+v", v.Duplicates)
	}

	var one httpapi.PersonView
	h.must("GET", "/api/people/"+astrid, nil, &one)
	if len(one.Entries) != 2 || one.Entries[0].Discipline != ls || one.Entries[1].DisciplineName != "Open Sabre" {
		t.Errorf("Astrid's page should have both her entries: %+v", one.Entries)
	}

	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	if e := view.Disciplines[0].Entrants; len(e) != 1 || e[0].Person != astrid {
		t.Errorf("the landing page's entrants should say who each is: %+v", e)
	}
	if code := h.do("GET", "/api/people/pr-nobody", nil, nil); code != 404 {
		t.Errorf("somebody not in the event should be a 404, got %d", code)
	}
}

// A merge is the organizer's, it moves the entries, an old link still finds the person,
// and it can be undone exactly.
func TestTheOrganizerMergesAndUndoes(t *testing.T) {
	h, ls := twoDisciplines(t)
	h.must("POST", "/api/d/"+ls+"/competitors", map[string]string{"name": "Karl-Johan"}, nil)
	h.must("POST", "/api/d/open-sabre/competitors", map[string]string{"name": "Karl Johan"}, nil)
	a, b := h.personOf(ls, "Karl-Johan"), h.personOf("open-sabre", "Karl Johan")

	var after httpapi.PeopleView
	h.must("POST", "/api/people/"+b+"/merge", map[string]string{"into": a}, &after)
	if got := h.personOf("open-sabre", "Karl Johan"); got != a {
		t.Errorf("the merged person's entries should now be the other's: %q", got)
	}
	if len(after.People) != 1 || len(after.People[0].Entries) != 2 || len(after.Duplicates) != 0 {
		t.Errorf("after the merge there should be one Karl-Johan with both entries: %+v", after)
	}
	if len(after.People[0].MergedFrom) != 1 || after.People[0].MergedFrom[0].ID != b {
		t.Errorf("the merge should be listed for undoing: %+v", after.People[0])
	}
	var viaOld httpapi.PersonView
	h.must("GET", "/api/people/"+b, nil, &viaOld)
	if viaOld.ID != a {
		t.Errorf("a link to the merged person should find whom they became, got %q", viaOld.ID)
	}

	h.must("POST", "/api/people/"+b+"/unmerge", nil, nil)
	if got := h.personOf("open-sabre", "Karl Johan"); got != b {
		t.Errorf("undoing the merge should give the entry back: %q, want %q", got, b)
	}
	if v := h.people(); len(v.People) != 2 || len(v.Duplicates) != 1 {
		t.Errorf("after undoing there should be two again, offered again: %+v", v)
	}

	h.must("POST", "/api/people/"+a+"/apart", map[string]string{"other": b}, nil)
	if v := h.people(); len(v.Duplicates) != 0 {
		t.Errorf("two kept apart should stop being offered: %+v", v.Duplicates)
	}
	if code := h.do("POST", "/api/people/"+a+"/merge", map[string]string{"into": a}, nil); code != 400 {
		t.Errorf("merging somebody into themselves should be refused, got %d", code)
	}
}

// Entries from before people existed, or typed into a file by hand, become somebody when
// the event opens: the same somebody when they came in on the same signup, and never by
// their name.
func TestEntriesFromBeforePeopleAreLinked(t *testing.T) {
	dir := t.TempDir()
	write := func(slug, comps string) {
		d := filepath.Join(dir, "disciplines", slug)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "competitors.json"), []byte(comps), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "tournament.json"),
			[]byte(`{"discipline":"`+slug+`","mats":1,"minPoolSize":4,"maxPoolSize":7,"seed":1,"pools":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("longsword", `[{"id":"c1","name":"Bo","signup":"sub-2"},{"id":"c2","name":"Ada"}]`)
	write("sabre", `[{"id":"c1","name":"Bo","signup":"sub-2"},{"id":"c2","name":"Ada"}]`)
	h := openHall(t, dir)

	if a, b := h.personOf("longsword", "Bo"), h.personOf("sabre", "Bo"); a == "" || a != b {
		t.Errorf("one submission should be one person in both: %q %q", a, b)
	}
	if a, b := h.personOf("longsword", "Ada"), h.personOf("sabre", "Ada"); a == "" || a == b {
		t.Errorf("the same name alone should be two people, offered: %q %q", a, b)
	}
	if v := h.people(); len(v.People) != 3 || len(v.Duplicates) != 1 {
		t.Errorf("three people and the Adas offered: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(dir, "people.json")); err != nil {
		t.Errorf("the people should be kept in the event folder: %v", err)
	}
}

// A typo fixed by removing the entry and entering it again leaves a person nobody needs;
// the next time the event opens, they are gone (#127).
func TestARemovedEntrysPersonIsDroppedOnOpening(t *testing.T) {
	dir := t.TempDir()
	h := openHall(t, dir)
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	var typo struct {
		ID string `json:"id"`
	}
	h.must("POST", "/api/d/"+ls+"/competitors", map[string]string{"name": "Astird", "club": "Gbg"}, &typo)
	gone := h.personOf(ls, "Astird")
	h.must("DELETE", "/api/d/"+ls+"/competitors/"+typo.ID, nil, nil)
	h.must("POST", "/api/d/"+ls+"/competitors", map[string]string{"name": "Astrid", "club": "Gbg"}, nil)
	kept := h.personOf(ls, "Astrid")
	h.stop()

	h = openHall(t, dir)
	b, _ := os.ReadFile(filepath.Join(dir, "people.json"))
	if strings.Contains(string(b), gone) || !strings.Contains(string(b), kept) {
		t.Errorf("people.json should keep Astrid and drop the typo's person %s:\n%s", gone, b)
	}
}
