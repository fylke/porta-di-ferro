package httpapi_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// Deleting the event (#141) puts it aside and starts a new one in its place: no
// disciplines (#149), nothing else, and a new identity -- with every file of the old one kept.
func TestDeletingTheEventStartsANewOne(t *testing.T) {
	h := openHall(t, t.TempDir())
	h.must("PUT", "/api/event/info", map[string]any{"welcome": "Hi", "signup": map[string]string{"name": "Autumn Open"}}, nil)
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)
	h.must("POST", "/api/d/open-sabre/competitors", map[string]string{"name": "Astrid", "club": "Gbg"}, nil)
	var before httpapi.EventView
	h.must("GET", "/api/event", nil, &before)
	var mats httpapi.MatsView
	h.must("GET", "/api/mats", nil, &mats)
	if before.ID == "" || mats.Event != before.ID {
		t.Fatalf("the event and its mats should carry the event's identity: %q and %q", before.ID, mats.Event)
	}

	var out struct{ Deleted string }
	h.must("DELETE", "/api/event", nil, &out)

	var after httpapi.EventView
	h.must("GET", "/api/event", nil, &after)
	if len(after.Disciplines) != 0 {
		t.Errorf("the new event should have no disciplines: %+v", after.Disciplines)
	}
	if after.Info.Welcome != "" || after.Name != "" {
		t.Errorf("the new event should have nothing of the old one's day: %+v", after.Info)
	}
	if after.ID == "" || after.ID == before.ID {
		t.Errorf("the new event should have an identity of its own, got %q after %q", after.ID, before.ID)
	}
	h.must("GET", "/api/mats", nil, &mats)
	if mats.Event != after.ID {
		t.Errorf("the mats should say they are the new event's, got %q", mats.Event)
	}
	if code := h.do("GET", "/api/d/open-sabre/state", nil, nil); code != 404 {
		t.Errorf("the old event's disciplines should be gone from the hall, got %d", code)
	}
	// The unprefixed paths have no discipline to answer as, as on a first start.
	if code := h.do("GET", "/api/state", nil, nil); code != 409 {
		t.Errorf("the new event has no discipline for the unprefixed paths; want 409, got %d", code)
	}

	// Kept, not destroyed: under deleted/, named for when and for what.
	if filepath.Dir(out.Deleted) != filepath.Join(h.dir, "deleted") || !strings.HasSuffix(out.Deleted, "-autumn-open") {
		t.Errorf("the event should be kept under deleted/ with its name, got %q", out.Deleted)
	}
	b, err := os.ReadFile(filepath.Join(out.Deleted, "disciplines", "open-sabre", "competitors.json"))
	if err != nil || !strings.Contains(string(b), "Astrid") {
		t.Errorf("the old event's entries should be kept: %v %s", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(out.Deleted, "event.json")); err != nil || !strings.Contains(string(b), before.ID) {
		t.Errorf("the old event's own file should be kept, with its identity: %v %s", err, b)
	}

	// And the new event is an event like any other: it can be set up, and deleted in turn.
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)
	h.must("POST", "/api/competitors", map[string]string{"name": "Bo", "club": "Sthlm"}, nil)
	h.must("DELETE", "/api/event", nil, nil)
	entries, _ := os.ReadDir(filepath.Join(h.dir, "deleted"))
	if len(entries) != 2 {
		t.Errorf("both deleted events should be kept, got %d", len(entries))
	}
}

// An event's identity is made once and survives a restart: only a delete changes it.
func TestTheEventKeepsItsIdentity(t *testing.T) {
	dir := t.TempDir()
	h := openHall(t, dir)
	var first httpapi.EventView
	h.must("GET", "/api/event", nil, &first)
	h.stop()

	again := openHall(t, dir)
	var second httpapi.EventView
	again.must("GET", "/api/event", nil, &second)
	if first.ID == "" || second.ID != first.ID {
		t.Errorf("a restart should keep the event's identity, got %q then %q", first.ID, second.ID)
	}
}
