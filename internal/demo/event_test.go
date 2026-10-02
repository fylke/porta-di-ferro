package demo_test

import (
	"encoding/json"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/demo"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The demo is an event of two disciplines (docs/proposals/one-event-many-disciplines.md,
// R11), answering every path the server's coordinator answers, the same way.

func view(t *testing.T, e *demo.Event) httpapi.EventView {
	t.Helper()
	res := e.Request("GET", "/api/event", nil)
	if res.Status != 200 {
		t.Fatalf("GET /api/event returned %d: %s", res.Status, res.Body)
	}
	var v httpapi.EventView
	if err := json.Unmarshal([]byte(res.Body), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func snapIn(t *testing.T, e *demo.Event, slug string) httpapi.Snapshot {
	t.Helper()
	res := e.Request("GET", "/api/d/"+slug+"/state", nil)
	if res.Status != 200 {
		t.Fatalf("GET /api/d/%s/state returned %d: %s", slug, res.Status, res.Body)
	}
	var s httpapi.Snapshot
	if err := json.Unmarshal([]byte(res.Body), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTheDemoIsAnEventOfTwoDisciplines(t *testing.T) {
	e := demo.NewEvent()
	v := view(t, e)
	if len(v.Disciplines) != 2 || v.Disciplines[0].Slug != "open-steel-longsword" || v.Disciplines[1].Slug != "open-sabre" {
		t.Fatalf("the demo should hold the longsword and the sabre, got %+v", v.Disciplines)
	}
	if v.Name != "Stångebroslaget" || v.Info.Welcome == "" {
		t.Errorf("the day should be the event's: %q, %q", v.Name, v.Info.Welcome)
	}
	if v.Disciplines[0].Stage != "pools" || v.Disciplines[1].Stage != "pools" || v.Disciplines[1].MatchesDone != 0 {
		t.Errorf("the longsword is mid-pools and the sabre drawn but unfenced: %+v / %+v", v.Disciplines[0], v.Disciplines[1])
	}

	// Somebody in both is findable in both.
	in := 0
	for _, d := range v.Disciplines {
		for _, en := range d.Entrants {
			if en.Name == "Astrid Lindqvist" {
				in++
			}
		}
	}
	if in != 2 {
		t.Errorf("Astrid fences both disciplines and should be entered in both, found %d", in)
	}

	sabre := snapIn(t, e, "open-sabre")
	if sabre.Instance.Name != "Open Sabre" || sabre.Instance.URL != "/d/open-sabre/" {
		t.Errorf("Sabre's snapshot should say which discipline it is and where: %+v", sabre.Instance)
	}
	if sabre.Tournament.Event.Welcome != v.Info.Welcome || sabre.Tournament.Event.Signup.Tournament != "sabre-pools" {
		t.Errorf("Sabre should show the event's day and keep its own programme row: %+v", sabre.Tournament.Event.Signup)
	}

	if res := e.Request("GET", "/api/state", nil); res.Status != 409 {
		t.Errorf("with two disciplines the unprefixed path should not guess, got %d", res.Status)
	}
	e.Request("DELETE", "/api/disciplines/open-sabre", nil)
	if res := e.Request("GET", "/api/state", nil); res.Status != 200 {
		t.Errorf("with one, the unprefixed path should answer as it, got %d", res.Status)
	}
}

func TestTheDemoEventsDayIsTypedOnce(t *testing.T) {
	e := demo.NewEvent()
	if res := e.Request("PUT", "/api/event/info", []byte(`{"welcome":"Typed once"}`)); res.Status != 200 || !res.Changed {
		t.Fatalf("saving the day returned %d (changed %v)", res.Status, res.Changed)
	}
	if s := snapIn(t, e, "open-steel-longsword"); s.Tournament.Event.Welcome != "Typed once" {
		t.Errorf("the longsword should show it, got %q", s.Tournament.Event.Welcome)
	}
	// Through a discipline's own path, as its admin page saves it.
	e.Request("PUT", "/api/d/open-sabre/event", []byte(`{"welcome":"From Sabre","signup":{"tournament":"sabre-pools"}}`))
	if s := snapIn(t, e, "open-steel-longsword"); s.Tournament.Event.Welcome != "From Sabre" || s.Tournament.Event.Signup.Tournament != "longsword-pools" {
		t.Errorf("the longsword should share the day and keep its row: %+v", s.Tournament.Event)
	}
}

func TestADisciplineCanBeAddedInTheDemo(t *testing.T) {
	e := demo.NewEvent()
	res := e.Request("POST", "/api/disciplines", []byte(`{"name":"Open foam Longsword"}`))
	if res.Status != 201 || !res.Changed {
		t.Fatalf("adding returned %d: %s", res.Status, res.Body)
	}
	if res := e.Request("POST", "/api/disciplines", []byte(`{"name":"open foam longsword"}`)); res.Status != 400 {
		t.Errorf("a second of the same name should be refused, got %d", res.Status)
	}
	e.Request("POST", "/api/d/open-foam-longsword/competitors", []byte(`{"name":"Ny Fäktare","club":"X"}`))
	if s := snapIn(t, e, "open-foam-longsword"); len(s.Competitors) != 1 || s.Instance.Name != "Open foam Longsword" {
		t.Errorf("the new discipline should take entrants of its own: %+v", s.Instance)
	}
	if res := e.Request("PATCH", "/api/disciplines/open-foam-longsword", []byte(`{"name":"Foam"}`)); res.Status != 200 {
		t.Errorf("renaming returned %d", res.Status)
	}
}

// The whole event survives the trip through localStorage (#108): the day, every
// discipline, and one added in the demo.
func TestTheDemoEventSavesAndLoads(t *testing.T) {
	e := demo.NewEvent()
	e.Request("PUT", "/api/event/info", []byte(`{"welcome":"Kept"}`))
	e.Request("POST", "/api/disciplines", []byte(`{"name":"Open foam Longsword"}`))
	e.Request("POST", "/api/d/open-sabre/competitors", []byte(`{"name":"Late Entry","club":"X"}`))
	e.Request("POST", "/api/demo/play", nil)
	before := view(t, e)

	res := e.Request("GET", "/api/demo/save", nil)
	if res.Status != 200 {
		t.Fatalf("save returned %d", res.Status)
	}
	if len(res.Body) > 2<<20 {
		t.Errorf("the event saves to %d bytes, too much for localStorage", len(res.Body))
	}
	next := demo.NewEvent()
	if r := next.Request("POST", "/api/demo/load", []byte(res.Body)); r.Status != 200 {
		t.Fatalf("load returned %d: %s", r.Status, r.Body)
	}
	after := view(t, next)
	if after.Info.Welcome != "Kept" || len(after.Disciplines) != 3 {
		t.Fatalf("the event should come back whole: %q, %d disciplines", after.Info.Welcome, len(after.Disciplines))
	}
	a, _ := json.Marshal(before.Disciplines)
	b, _ := json.Marshal(after.Disciplines)
	if string(a) != string(b) {
		t.Error("the disciplines differ after a round trip")
	}

	// A save from the demo before it was an event is refused, and changes nothing.
	old := demo.New()
	single := old.Request("GET", "/api/demo/save", nil)
	if r := next.Request("POST", "/api/demo/load", []byte(single.Body)); r.Status != 400 {
		t.Errorf("a single-tournament save should be refused, got %d", r.Status)
	}
	if len(view(t, next).Disciplines) != 3 {
		t.Error("a refused load still changed the event")
	}

	if r := next.Request("POST", "/api/demo/reset", nil); r.Status != 200 {
		t.Fatalf("reset returned %d", r.Status)
	}
	if v := view(t, next); len(v.Disciplines) != 2 || v.Info.Welcome == "Kept" {
		t.Errorf("Start over should bring back the two fixtures and the fixture's day, got %d and %q", len(v.Disciplines), v.Info.Welcome)
	}
}
