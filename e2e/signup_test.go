package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The whole offline signup round trip against the real binary (issue #91): the organizer
// configures the event, downloads the one file that goes out, and imports what comes
// back -- twice, because importing the same folder twice is the case the design exists
// for.

func configureEvent(t *testing.T, s *server) {
	t.Helper()
	s.mustDo(t, "PUT", "/api/event", map[string]any{
		"signup": map[string]any{
			"definitionId": "msl-open-2026",
			"name":         "MSL Open",
			"venue":        "Linköping",
			"date":         "2026-11-15",
			"tournament":   "longsword",
		},
		"schedule": []map[string]any{
			{"at": "08:30", "label": "Gear check"},
			{"at": "09:30", "label": "Open Steel Longsword", "kind": "discipline",
				"tournament": "longsword", "capacity": 28},
			{"at": "12:00", "ends": "13:00", "label": "Lunch", "kind": "break"},
			{"at": "13:00", "label": "Open Sabre", "kind": "discipline", "tournament": "sabre"},
		},
	}, nil)
}

func response(t *testing.T, name, submission string, entries ...string) map[string]string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format":       "porta.signup.response",
		"version":      1,
		"definitionId": "msl-open-2026",
		"submissionId": submission,
		"participant":  map[string]string{"name": name, "club": "Example HEMA"},
		"entries":      entries,
	})
	if err != nil {
		t.Fatalf("building a response: %v", err)
	}
	return map[string]string{"source": name + ".json", "body": string(body)}
}

func TestOfflineSignupRoundTrip(t *testing.T) {
	s := start(t)
	configureEvent(t, s)

	// What goes out: one HTML file with the event written into it, so the organizer
	// sends one attachment rather than two with an instruction between them.
	res, err := http.Get(s.base + "/api/signup/app.html?download=1")
	if err != nil {
		t.Fatalf("downloading the app: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("the app download returned %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("served as %q", ct)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "signup-msl-open-2026.html") {
		t.Errorf("it should save under the event's name, got %q", cd)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	for _, want := range []string{"msl-open-2026", "MSL Open", "Open Steel Longsword", "Open Sabre"} {
		if !strings.Contains(page, want) {
			t.Errorf("the app does not carry %q", want)
		}
	}
	// It has to work with no server behind it.
	if strings.Contains(page, "<script src=") {
		t.Error("the app loads a script from somewhere; it opens from a downloads folder")
	}

	// The definition on its own, for anyone who would rather send two files.
	var def signup.Definition
	s.mustDo(t, "GET", "/api/signup/definition.json", nil, &def)
	if def.Format != signup.DefinitionFormat || len(def.Tournaments) != 2 {
		t.Fatalf("the definition came out as %+v", def)
	}

	// What comes back. One for this discipline, one for both, one for the sabre only,
	// one that is not a signup at all.
	files := []map[string]string{
		response(t, "Ada Example", "sub-1", "longsword"),
		response(t, "Bo Example", "sub-2", "longsword", "sabre"),
		response(t, "Cilla Example", "sub-3", "sabre"),
		{"source": "notes.json", "body": "{}"},
	}

	var preview struct {
		Rows       []signup.Row `json:"rows"`
		Adding     int          `json:"adding"`
		Tournament string       `json:"tournament"`
		PoolsDrawn bool         `json:"poolsDrawn"`
	}
	s.mustDo(t, "POST", "/api/signup/preview", map[string]any{"files": files}, &preview)
	if preview.Adding != 2 {
		t.Errorf("two of those are for this discipline; the preview says %d", preview.Adding)
	}
	if preview.Tournament != "longsword" {
		t.Errorf("the preview should name this discipline, got %q", preview.Tournament)
	}

	// A preview writes nothing. That is the whole reason it is a separate call.
	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Competitors) != 0 {
		t.Fatalf("the preview added %d competitors; it must add none", len(snap.Competitors))
	}

	var first struct {
		Added int `json:"added"`
	}
	s.mustDo(t, "POST", "/api/signup/import", map[string]any{"files": files}, &first)
	if first.Added != 2 {
		t.Fatalf("the import added %d, want 2", first.Added)
	}

	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Competitors) != 2 {
		t.Fatalf("the register has %d competitors", len(snap.Competitors))
	}
	byName := map[string]store.Competitor{}
	for _, c := range snap.Competitors {
		byName[c.Name] = c
	}
	if ada, ok := byName["Ada Example"]; !ok || ada.Club != "Example HEMA" || ada.Signup != "sub-1" {
		t.Errorf("Ada came in as %+v", ada)
	}
	if _, ok := byName["Cilla Example"]; ok {
		t.Error("Cilla only entered the sabre; another run of the application imports her")
	}

	// Again, with the same folder. This is what an organizer does when they are not sure
	// whether they already did it.
	var second struct {
		Added int `json:"added"`
	}
	s.mustDo(t, "POST", "/api/signup/import", map[string]any{"files": files}, &second)
	if second.Added != 0 {
		t.Errorf("importing the same folder again added %d", second.Added)
	}
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Competitors) != 2 {
		t.Fatalf("after importing twice the register has %d competitors", len(snap.Competitors))
	}

	// A response corrected and sent again replaces rather than duplicates: the
	// submission id is what carries that, not the name.
	corrected := response(t, "Ada Example", "sub-1", "longsword")
	s.mustDo(t, "POST", "/api/signup/import", map[string]any{"files": []map[string]string{corrected}}, &second)
	if second.Added != 0 {
		t.Errorf("the same submission id came back and added %d", second.Added)
	}
}

// A run that has not been told which discipline it is takes everything valid, which is
// the right answer for an event with one.
func TestSignupImportWithoutADisciplineTakesEverything(t *testing.T) {
	s := start(t)
	s.mustDo(t, "PUT", "/api/event", map[string]any{
		"signup": map[string]any{"definitionId": "small-open", "name": "Small Open"},
		"schedule": []map[string]any{
			{"label": "Longsword", "kind": "discipline", "tournament": "longsword"},
			{"label": "Sabre", "kind": "discipline", "tournament": "sabre"},
		},
	}, nil)

	files := []map[string]string{}
	for i, entry := range []string{"longsword", "sabre"} {
		body, _ := json.Marshal(map[string]any{
			"format": "porta.signup.response", "version": 1,
			"definitionId": "small-open", "submissionId": fmt.Sprintf("s-%d", i),
			"participant": map[string]string{"name": fmt.Sprintf("Person %d", i)},
			"entries":     []string{entry},
		})
		files = append(files, map[string]string{"source": entry + ".json", "body": string(body)})
	}

	var out struct {
		Added int `json:"added"`
	}
	s.mustDo(t, "POST", "/api/signup/import", map[string]any{"files": files}, &out)
	if out.Added != 2 {
		t.Errorf("with no discipline set both belong here; added %d", out.Added)
	}
}

// What is missing is reported before the files go out, not by a participant who cannot
// use one.
func TestSignupReadySaysWhatIsMissing(t *testing.T) {
	s := start(t)

	var ready struct {
		Missing  []string `json:"missing"`
		Filename string   `json:"filename"`
	}
	s.mustDo(t, "GET", "/api/signup/ready", nil, &ready)
	if len(ready.Missing) == 0 {
		t.Error("a fresh tournament is not ready to publish and should say so")
	}

	configureEvent(t, s)
	s.mustDo(t, "GET", "/api/signup/ready", nil, &ready)
	if len(ready.Missing) != 0 {
		t.Errorf("that event is ready; it reports %v", ready.Missing)
	}
	if ready.Filename != "signup-msl-open-2026.html" {
		t.Errorf("the download name is %q", ready.Filename)
	}
}
