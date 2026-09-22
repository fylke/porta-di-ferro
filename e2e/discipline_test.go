package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Naming the discipline you are actually on (issue #80).
//
// The name only ever arrived as a command-line flag. A sibling gets one because the
// instance that spawned it passes it; the first run is started by the installer's
// shortcut, which passes nothing -- so the one discipline every organizer has was called
// "Unnamed" on every page, with nowhere to say otherwise.
//
// It is now kept with the tournament, which is what makes it survive the restart that a
// long day in a sports hall will involve.
func TestNamingThisDiscipline(t *testing.T) {
	s := start(t)

	var list []instance
	s.mustDo(t, "GET", "/api/instances", nil, &list)
	if len(list) != 1 || list[0].Name != "" {
		t.Fatalf("a run started without a name should have none, got %+v", list)
	}
	port := portOf(s)

	// The preloaded list: the runs that come round at every event, spelled once.
	var presets []string
	s.mustDo(t, "GET", "/api/disciplines", nil, &presets)
	if len(presets) < 4 || presets[0] != "Open steel Longsword" {
		t.Fatalf("the usual disciplines should be offered, with the default first: %v", presets)
	}
	for _, want := range []string{
		"Open steel Longsword",
		"Women's and underrepresented genders Longsword",
		"Open Sabre",
		"Open foam Longsword",
	} {
		found := false
		for _, p := range presets {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q should be on the list, got %v", want, presets)
		}
	}

	s.mustDo(t, "PATCH", fmt.Sprintf("/api/instances/%d", port),
		map[string]string{"name": "Open Sabre"}, &list)
	if len(list) != 1 || list[0].Name != "Open Sabre" {
		t.Fatalf("the rename should come back in the list, got %+v", list)
	}

	// Every page carries the name, so the snapshot has to have it.
	var snap struct {
		Instance instance `json:"instance"`
	}
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Instance.Name != "Open Sabre" {
		t.Errorf("the snapshot should name the discipline, got %+v", snap.Instance)
	}

	// Written down with the tournament, not just held in the process.
	b, err := os.ReadFile(filepath.Join(s.dir, "tournament.json"))
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
		if code := s.do(t, "PATCH", fmt.Sprintf("/api/instances/%d", port),
			map[string]string{"name": bad}, nil); code != 400 {
			t.Errorf("renaming to %q should be refused, got %d", bad, code)
		}
	}
	if code := s.do(t, "PATCH", "/api/instances/1", map[string]string{"name": "Ghost"}, nil); code != 400 {
		t.Errorf("renaming a discipline that is not running should be refused, got %d", code)
	}

	// And a name this run already has is not a second discipline to start.
	if code := s.do(t, "POST", "/api/instances", map[string]string{"name": "open sabre"}, nil); code != 400 {
		t.Errorf("starting a sibling named after this one should be refused, got %d", code)
	}
}

// The name is read back on the way up, so a restart mid-event does not go back to
// "Unnamed" on every screen in the hall.
func TestTheDisciplineNameSurvivesARestart(t *testing.T) {
	s := start(t)
	s.mustDo(t, "PATCH", fmt.Sprintf("/api/instances/%d", portOf(s)),
		map[string]string{"name": "Open foam Longsword"}, nil)

	again := restart(t, s)
	var snap struct {
		Instance instance `json:"instance"`
	}
	again.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Instance.Name != "Open foam Longsword" {
		t.Errorf("after a restart the run should still know its discipline, got %q", snap.Instance.Name)
	}
}
