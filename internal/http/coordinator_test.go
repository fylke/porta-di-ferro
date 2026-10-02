package httpapi_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The event coordinator: one address for every discipline in the hall
// (docs/proposals/one-event-many-disciplines.md). Driven over real HTTP, because the
// routing is most of what it does.

type hall struct {
	t    *testing.T
	url  string
	dir  string
	c    *httpapi.Coordinator
	stop func()
}

func openHall(t *testing.T, dir string) *hall {
	t.Helper()
	f, err := event.Open(dir)
	if err != nil {
		t.Fatalf("opening the event folder: %v", err)
	}
	c, err := httpapi.NewCoordinator(f, nil)
	if err != nil {
		t.Fatalf("starting the coordinator: %v", err)
	}
	srv := httptest.NewServer(c.Handler())
	h := &hall{t: t, url: srv.URL, dir: dir, c: c, stop: func() { srv.Close(); c.Close() }}
	t.Cleanup(h.stop)
	return h
}

// do sends a request and decodes the answer into out, returning the status.
func (h *hall) do(method, path string, body, out any) int {
	h.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, h.url+path, r)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

func (h *hall) must(method, path string, body, out any) {
	h.t.Helper()
	if code := h.do(method, path, body, out); code >= 300 {
		h.t.Fatalf("%s %s returned %d", method, path, code)
	}
}

type snap struct {
	Competitors []struct{ Name string } `json:"competitors"`
	Tournament  struct {
		Discipline string `json:"discipline"`
		Event      struct {
			Welcome string `json:"welcome"`
			Signup  struct {
				Tournament string `json:"tournament"`
			} `json:"signup"`
		} `json:"event"`
	} `json:"tournament"`
	Instance httpapi.Instance `json:"instance"`
}

// A first start is one discipline, answering on the paths it always did. Nothing about an
// event with one discipline may look different from a run of the application before
// events existed (proposal R10).
func TestOneDisciplineAnswersAsItAlwaysDid(t *testing.T) {
	h := openHall(t, t.TempDir())

	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	if len(view.Disciplines) != 1 {
		t.Fatalf("a new event should have one discipline, got %+v", view.Disciplines)
	}
	slug := view.Disciplines[0].Slug

	h.must("POST", "/api/competitors", map[string]string{"name": "Astrid", "club": "Gbg"}, nil)
	var viaAlias, viaPath snap
	h.must("GET", "/api/state", nil, &viaAlias)
	h.must("GET", "/api/d/"+slug+"/state", nil, &viaPath)
	if len(viaAlias.Competitors) != 1 || len(viaPath.Competitors) != 1 {
		t.Errorf("the unprefixed path and the discipline's own should be the same tournament: %d and %d",
			len(viaAlias.Competitors), len(viaPath.Competitors))
	}
	if viaPath.Instance.Slug != slug || viaPath.Instance.URL != "/d/"+slug+"/" {
		t.Errorf("the snapshot should say which discipline it is and where, got %+v", viaPath.Instance)
	}
	if code := h.do("GET", "/api/d/nope/state", nil, nil); code != 404 {
		t.Errorf("a discipline that is not in the event should be a 404, got %d", code)
	}
}

// A second discipline is another folder and server in the same process, at the same
// address (#4). The unprefixed paths then stop meaning anything and say so.
func TestASecondDisciplineSharesTheAddress(t *testing.T) {
	h := openHall(t, t.TempDir())

	var sabre httpapi.DisciplineSummary
	if code := h.do("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, &sabre); code != 201 {
		t.Fatalf("adding a discipline returned %d", code)
	}
	if sabre.Slug != "open-sabre" || sabre.Name != "Open Sabre" || sabre.Stage != "setup" {
		t.Errorf("the new discipline should be named, slugged and not yet drawn: %+v", sabre)
	}
	if code := h.do("POST", "/api/disciplines", map[string]string{"name": "open sabre"}, nil); code != 400 {
		t.Errorf("a second discipline of the same name should be refused, got %d", code)
	}

	var conflict struct {
		Error       string   `json:"error"`
		Disciplines []string `json:"disciplines"`
	}
	if code := h.do("GET", "/api/state", nil, &conflict); code != 409 || len(conflict.Disciplines) != 2 {
		t.Errorf("with two disciplines the unprefixed path should name them rather than guess: %d %+v", code, conflict)
	}

	h.must("POST", "/api/d/open-sabre/competitors", map[string]string{"name": "Bo", "club": "Lund"}, nil)
	var s snap
	h.must("GET", "/api/d/open-sabre/state", nil, &s)
	if s.Instance.Name != "Open Sabre" || len(s.Competitors) != 1 {
		t.Errorf("Sabre should have its own name and its own competitor: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "disciplines", "open-sabre", "competitors.json")); err != nil {
		t.Errorf("Sabre's competitors should be in its own folder: %v", err)
	}

	// The first discipline was made before it had a name. Naming it gives it an address
	// from the name, and the old one still answers.
	var first httpapi.EventView
	h.must("GET", "/api/event", nil, &first)
	old := first.Disciplines[0].Slug
	var named httpapi.DisciplineSummary
	h.must("PATCH", "/api/disciplines/"+old, map[string]string{"name": "Open steel Longsword"}, &named)
	if named.Slug != "open-steel-longsword" || named.URL != "/d/open-steel-longsword/" {
		t.Errorf("naming an unnamed discipline should give it an address from the name: %+v", named)
	}
	if code := h.do("GET", "/api/d/"+old+"/state", nil, nil); code != 200 {
		t.Errorf("the address it had before it was named should still answer, got %d", code)
	}

	var renamed httpapi.DisciplineSummary
	h.must("PATCH", "/api/disciplines/open-sabre", map[string]string{"name": "Sabre, open"}, &renamed)
	if renamed.Name != "Sabre, open" || renamed.Slug != "open-sabre" {
		t.Errorf("renaming changes the name and never the address: %+v", renamed)
	}

	var presets []string
	h.must("GET", "/api/disciplines/presets", nil, &presets)
	if len(presets) < 4 || presets[0] != "Open steel Longsword" {
		t.Errorf("the usual disciplines should be offered: %v", presets)
	}
}

// The welcome, the programme and the wifi are typed once for the whole hall, and every
// discipline's pages read the same copy (proposal §7).
func TestTheDayIsTypedOnce(t *testing.T) {
	h := openHall(t, t.TempDir())
	var first httpapi.EventView
	h.must("GET", "/api/event", nil, &first)
	one := first.Disciplines[0].Slug
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)

	h.must("PUT", "/api/event/info", map[string]any{"welcome": "  Welcome to the hall  "}, nil)
	for _, slug := range []string{one, "open-sabre"} {
		var s snap
		h.must("GET", "/api/d/"+slug+"/state", nil, &s)
		if s.Tournament.Event.Welcome != "Welcome to the hall" {
			t.Errorf("%s should show the event's welcome, got %q", slug, s.Tournament.Event.Welcome)
		}
	}

	// The admin screens of a discipline still save the whole day through its own path, as
	// they always have. The event takes the day; the discipline keeps which programme row
	// it is.
	h.must("PUT", "/api/d/open-sabre/event", map[string]any{
		"welcome": "Changed from Sabre",
		"signup":  map[string]string{"name": "MSL Open", "tournament": "sabre-pools"},
	}, nil)
	var sabre, other snap
	h.must("GET", "/api/d/open-sabre/state", nil, &sabre)
	h.must("GET", "/api/d/"+one+"/state", nil, &other)
	if sabre.Tournament.Event.Signup.Tournament != "sabre-pools" {
		t.Errorf("Sabre should keep its programme row, got %q", sabre.Tournament.Event.Signup.Tournament)
	}
	if other.Tournament.Event.Welcome != "Changed from Sabre" || other.Tournament.Event.Signup.Tournament != "" {
		t.Errorf("the other discipline should share the day and not Sabre's row: %+v", other.Tournament.Event)
	}
	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	if view.Name != "MSL Open" || view.Info.Signup.Tournament != "" {
		t.Errorf("the event should be named from its signup settings and hold no discipline's row: %q %+v", view.Name, view.Info.Signup)
	}

	b, _ := os.ReadFile(filepath.Join(h.dir, "disciplines", "open-sabre", "tournament.json"))
	if strings.Contains(string(b), "Changed from Sabre") {
		t.Error("the day must not be copied into a discipline's tournament.json")
	}
	if code := h.do("PUT", "/api/event/info", map[string]any{"welcome": strings.Repeat("x", 5000)}, nil); code != 400 {
		t.Errorf("the event should hold the welcome to the same limit a discipline did, got %d", code)
	}
}

// A discipline whose files will not parse is contained to that discipline: it is listed
// with the reason, every other discipline runs, and once the file is fixed it comes back
// without a restart (proposal §11).
func TestABrokenDisciplineIsContained(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "disciplines", "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "competitors.json"), []byte(`[{"id":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "disciplines", "fine"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := openHall(t, dir)

	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	if len(view.Disciplines) != 2 {
		t.Fatalf("both disciplines should be listed, got %+v", view.Disciplines)
	}
	var bad httpapi.DisciplineSummary
	for _, d := range view.Disciplines {
		if d.Slug == "broken" {
			bad = d
		}
	}
	if !strings.Contains(bad.Error, "competitors.json") && !strings.Contains(bad.Error, "could not be read") {
		t.Errorf("the broken discipline should say why, got %q", bad.Error)
	}
	if code := h.do("GET", "/api/d/broken/state", nil, nil); code != 503 {
		t.Errorf("asking the broken discipline should be a 503, got %d", code)
	}
	if code := h.do("GET", "/api/d/fine/state", nil, nil); code != 200 {
		t.Errorf("the other discipline should run regardless, got %d", code)
	}

	if err := os.WriteFile(filepath.Join(broken, "competitors.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	var back httpapi.DisciplineSummary
	h.must("POST", "/api/disciplines/broken/reload", nil, &back)
	if back.Error != "" {
		t.Errorf("after the fix the discipline should load, got %q", back.Error)
	}
	if code := h.do("GET", "/api/d/broken/state", nil, nil); code != 200 {
		t.Errorf("the reloaded discipline should answer, got %d", code)
	}
}

func TestRetiringADisciplineKeepsItsFolder(t *testing.T) {
	h := openHall(t, t.TempDir())
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)

	var out struct{ Retired string }
	h.must("DELETE", "/api/disciplines/open-sabre", nil, &out)
	if _, err := os.Stat(out.Retired); err != nil {
		t.Errorf("the retired discipline's folder should be kept: %v", err)
	}
	if code := h.do("GET", "/api/d/open-sabre/state", nil, nil); code != 404 {
		t.Errorf("a retired discipline should be gone from the event, got %d", code)
	}
	if code := h.do("GET", "/api/state", nil, nil); code != 200 {
		t.Errorf("back to one discipline, the unprefixed paths should answer again, got %d", code)
	}

	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	if code := h.do("DELETE", "/api/disciplines/"+view.Disciplines[0].Slug, nil, nil); code != 400 {
		t.Errorf("the last discipline should not be retired, got %d", code)
	}
}

// The landing page holds one stream for the whole event, and hears about a change in any
// discipline on it.
func TestTheEventStreamHearsEveryDiscipline(t *testing.T) {
	h := openHall(t, t.TempDir())
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)

	res, err := http.Get(h.url + "/api/event/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	frames := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				frames <- strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)

	h.must("POST", "/api/d/open-sabre/competitors", map[string]string{"name": "Bo", "club": "Lund"}, nil)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case f := <-frames:
			var u struct {
				Kind string            `json:"kind"`
				Data httpapi.EventView `json:"data"`
			}
			if json.Unmarshal([]byte(f), &u) != nil || u.Kind != "event" {
				continue
			}
			for _, d := range u.Data.Disciplines {
				if d.Slug == "open-sabre" && d.Competitors == 1 && len(d.Entrants) == 1 && d.Entrants[0].Name == "Bo" {
					return
				}
			}
		case <-deadline:
			t.Fatal("the event stream never carried Sabre's new entrant")
		}
	}
}

// Opening a tournament folder from before events existed gives the same tournament, at
// the same paths.
func TestALegacyFolderServesAsBefore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "competitors.json"),
		[]byte(`[{"id":"c1","name":"Astrid","club":"Gbg","withdrawn":false}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tournament.json"),
		[]byte(`{"discipline":"Open steel Longsword","mats":2,"minPoolSize":4,"maxPoolSize":7,"seed":1,"pools":[],"event":{"welcome":"Hej"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h := openHall(t, dir)
	var s snap
	h.must("GET", "/api/state", nil, &s)
	if len(s.Competitors) != 1 || s.Instance.Name != "Open steel Longsword" || s.Tournament.Event.Welcome != "Hej" {
		t.Errorf("the migrated tournament should answer as before: %+v", s)
	}
	if s.Instance.Slug != "open-steel-longsword" {
		t.Errorf("it should be the event's discipline by its name, got %q", s.Instance.Slug)
	}
}
