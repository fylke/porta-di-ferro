package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Disciplines in one event (docs/proposals/one-event-many-disciplines.md, phase 1),
// against the real binary.

type summary struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Error string `json:"error"`
	Stage string `json:"stage"`
}

type eventView struct {
	Name        string    `json:"name"`
	Disciplines []summary `json:"disciplines"`
}

type instance struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Dir  string `json:"dir"`
	URL  string `json:"url"`
}

// Naming the discipline you are actually on (issue #80). A first start is one discipline
// with no name; the organizer picks one from the usual list, and it is kept with the
// tournament so it survives the restart a long day in a sports hall will involve.
func TestNamingThisDiscipline(t *testing.T) {
	s := start(t)

	var ev eventView
	s.mustDo(t, "GET", "/api/event", nil, &ev)
	if len(ev.Disciplines) != 1 || ev.Disciplines[0].Name != "" {
		t.Fatalf("a first start should be one discipline with no name, got %+v", ev.Disciplines)
	}
	slug := ev.Disciplines[0].Slug

	var presets []string
	s.mustDo(t, "GET", "/api/disciplines/presets", nil, &presets)
	for _, want := range []string{
		"Open steel Longsword",
		"Women's and underrepresented genders Longsword",
		"Open Sabre",
		"Open foam Longsword",
	} {
		found := false
		for _, p := range presets {
			found = found || p == want
		}
		if !found {
			t.Errorf("%q should be on the list, got %v", want, presets)
		}
	}
	if len(presets) == 0 || presets[0] != "Open steel Longsword" {
		t.Errorf("the default should come first, got %v", presets)
	}

	s.mustDo(t, "PATCH", "/api/disciplines/"+slug, map[string]string{"name": "Open Sabre"}, nil)
	var snap struct {
		Instance instance `json:"instance"`
	}
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	// Made before it had a name, it moves to an address from the name, once.
	if snap.Instance.Name != "Open Sabre" || snap.Instance.Slug != "open-sabre" {
		t.Errorf("the snapshot should name the discipline at its new address, got %+v", snap.Instance)
	}
	if code := s.do(t, "GET", "/api/d/"+slug+"/state", nil, nil); code != 200 {
		t.Errorf("the address it had before it was named should still answer, got %d", code)
	}
	slug = "open-sabre"

	// Written down with the tournament, not just held in the process.
	b, err := os.ReadFile(filepath.Join(s.disciplineDir(t), "tournament.json"))
	if err != nil {
		t.Fatalf("reading tournament.json: %v", err)
	}
	var saved struct {
		Discipline string `json:"discipline"`
	}
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatalf("parsing tournament.json: %v", err)
	}
	if saved.Discipline != "Open Sabre" {
		t.Errorf("the name should be saved with the tournament, got %q", saved.Discipline)
	}

	// Nonsense is refused rather than written to every screen in the hall.
	for _, bad := range []string{"", "   ", "!!!"} {
		if code := s.do(t, "PATCH", "/api/disciplines/"+slug, map[string]string{"name": bad}, nil); code != 400 {
			t.Errorf("renaming to %q should be refused, got %d", bad, code)
		}
	}
	if code := s.do(t, "PATCH", "/api/disciplines/ghost", map[string]string{"name": "Ghost"}, nil); code != 404 {
		t.Errorf("renaming a discipline that is not in the event should be a 404, got %d", code)
	}
	if code := s.do(t, "POST", "/api/disciplines", map[string]string{"name": "open sabre"}, nil); code != 400 {
		t.Errorf("adding a discipline named after one already there should be refused, got %d", code)
	}
}

// The name is read back on the way up, so a restart mid-event does not go back to
// "Unnamed" on every screen in the hall.
func TestTheDisciplineNameSurvivesARestart(t *testing.T) {
	s := start(t)
	var ev eventView
	s.mustDo(t, "GET", "/api/event", nil, &ev)
	s.mustDo(t, "PATCH", "/api/disciplines/"+ev.Disciplines[0].Slug,
		map[string]string{"name": "Open foam Longsword"}, nil)

	again := restart(t, s)
	var snap struct {
		Instance instance `json:"instance"`
	}
	again.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Instance.Name != "Open foam Longsword" {
		t.Errorf("after a restart the discipline should still be named, got %q", snap.Instance.Name)
	}
}

// A second discipline is another folder in the event and another tournament in the same
// process, at the same address (#4, #102). It used to be a second copy of the
// application on the next free port (#49).
func TestAnEventRunsASecondDiscipline(t *testing.T) {
	s := start(t)

	var sabre summary
	if code := s.do(t, "POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, &sabre); code != 201 {
		t.Fatalf("adding a discipline returned %d", code)
	}
	if sabre.Slug != "open-sabre" || sabre.URL != "/d/open-sabre/" {
		t.Errorf("the discipline should have an address in the event, got %+v", sabre)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "disciplines", "open-sabre")); err != nil {
		t.Errorf("the discipline should have a folder of its own under the event: %v", err)
	}

	s.mustDo(t, "POST", "/api/d/open-sabre/competitors", map[string]string{"name": "Astrid", "club": "Gbg"}, nil)
	var snap struct {
		Competitors []struct{ Name string } `json:"competitors"`
		Instance    instance                `json:"instance"`
	}
	s.mustDo(t, "GET", "/api/d/open-sabre/state", nil, &snap)
	if snap.Instance.Name != "Open Sabre" || len(snap.Competitors) != 1 {
		t.Errorf("Sabre should answer for itself at the event's address: %+v", snap)
	}
	if code := s.do(t, "GET", "/api/state", nil, nil); code != 409 {
		t.Errorf("with two disciplines the unprefixed paths should not guess, got %d", code)
	}

	// Both survive a restart: the folders are the truth about what the event holds.
	again := restart(t, s)
	var ev eventView
	again.mustDo(t, "GET", "/api/event", nil, &ev)
	if len(ev.Disciplines) != 2 || ev.Disciplines[1].Name != "Open Sabre" {
		t.Fatalf("after a restart the event should still hold both, got %+v", ev.Disciplines)
	}

	var out struct {
		Retired string `json:"retired"`
	}
	again.mustDo(t, "DELETE", "/api/disciplines/open-sabre", nil, &out)
	if _, err := os.Stat(out.Retired); err != nil {
		t.Errorf("a retired discipline's folder should be kept: %v", err)
	}
	if code := again.do(t, "GET", "/api/state", nil, nil); code != 200 {
		t.Errorf("back to one discipline, the unprefixed paths should answer again, got %d", code)
	}
}

// Every organizer who has used the application has a tournament folder from before
// events existed. Pointing this version at it gives them their tournament, unchanged, as
// the event's one discipline (proposal §7).
func TestAnOldTournamentFolderOpensAsTheEventsDiscipline(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tournament.json", `{"discipline":"Open steel Longsword","mats":2,"minPoolSize":4,"maxPoolSize":7,"seed":7,"pools":[],"event":{"welcome":"Välkomna"}}`)
	write("competitors.json", `[{"id":"c1","name":"Astrid","club":"Gbg","withdrawn":false}]`)

	s := startIn(t, dir)
	var snap struct {
		Competitors []struct{ Name string } `json:"competitors"`
		Tournament  struct {
			Seed  int64 `json:"seed"`
			Event struct {
				Welcome string `json:"welcome"`
			} `json:"event"`
		} `json:"tournament"`
		Instance instance `json:"instance"`
	}
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Competitors) != 1 || snap.Tournament.Seed != 7 || snap.Instance.Name != "Open steel Longsword" {
		t.Errorf("the old tournament should answer as it did: %+v", snap)
	}
	if snap.Tournament.Event.Welcome != "Välkomna" {
		t.Errorf("its welcome should now be the event's, got %q", snap.Tournament.Event.Welcome)
	}
	if _, err := os.Stat(filepath.Join(dir, "disciplines", "open-steel-longsword", "competitors.json")); err != nil {
		t.Errorf("its files should be in the event's first discipline: %v", err)
	}
}
