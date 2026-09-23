package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/web"
)

// What the organizer sends out is one file with the event written into it. If the slot it
// goes into ever moves, every participant gets an app that asks them for a definition
// they were never sent -- and nobody finds out until a file comes back empty.

func TestTheSignupAppCarriesTheEvent(t *testing.T) {
	page, err := web.SignupApp()
	if err != nil {
		t.Fatalf("the app is not embedded: %v", err)
	}
	if !strings.Contains(string(page), definitionMarker) {
		t.Fatal("web/signup/signup.html no longer has the slot the event is written into")
	}

	def := signup.BuildDefinition(store.Tournament{
		Discipline: "Open steel Longsword",
		Event: store.Event{Signup: store.Signup{
			DefinitionID: "msl-open-2026", Name: "MSL Open",
		}, Schedule: []store.ScheduleItem{
			{Label: "Open Steel Longsword", Kind: "discipline", At: "09:30"},
		}},
	})

	baked, err := bakeDefinition(page, def)
	if err != nil {
		t.Fatalf("baking: %v", err)
	}
	body := string(baked)
	if strings.Contains(body, definitionMarker) {
		t.Error("the slot is still empty after baking")
	}
	if !strings.Contains(body, "msl-open-2026") || !strings.Contains(body, "MSL Open") {
		t.Error("the event did not make it into the file")
	}
	// One file, not two: no network of any kind, or it stops working in a downloads
	// folder.
	for _, forbidden := range []string{"<script src=", "<link rel=\"stylesheet\"", "fetch("} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the app pulls in %q; it has to work from file:// with no server", forbidden)
		}
	}
	// And nothing that needs a secure context, which a downloads folder is not. Matched
	// as calls rather than as words: the file explains in a comment which of these it is
	// deliberately not using, and that comment is not a use.
	for _, forbidden := range []string{
		"crypto.randomUUID(", "navigator.serviceWorker", "navigator.clipboard",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the app uses %s, which a file:// page does not have", forbidden)
		}
	}
}

// The baked JSON sits inside a <script> element, so it must not be able to close it.
func TestBakingCannotEscapeTheScriptElement(t *testing.T) {
	page, err := web.SignupApp()
	if err != nil {
		t.Fatal(err)
	}
	def := signup.BuildDefinition(store.Tournament{
		Event: store.Event{
			Signup:   store.Signup{DefinitionID: "x", Name: `</script><script>alert(1)</script>`},
			Schedule: []store.ScheduleItem{{Label: "Longsword", Kind: "discipline"}},
		},
	})
	baked, err := bakeDefinition(page, def)
	if err != nil {
		t.Fatal(err)
	}
	// The app's own closing tag is there; the injected one must not be.
	if strings.Count(string(baked), "</script><script>alert(1)") != 0 {
		t.Error("a name containing a closing script tag escaped the element")
	}

	// And it is still valid JSON on the way out.
	body := string(baked)
	start := strings.Index(body, `<script id="definition" type="application/json">`)
	if start < 0 {
		t.Fatal("no definition element")
	}
	start += len(`<script id="definition" type="application/json">`)
	end := strings.Index(body[start:], "</script>")
	var round signup.Definition
	if err := json.Unmarshal([]byte(body[start:start+end]), &round); err != nil {
		t.Fatalf("the baked definition is not valid JSON: %v", err)
	}
	if round.Event.Name != `</script><script>alert(1)</script>` {
		t.Errorf("the name did not survive the round trip: %q", round.Event.Name)
	}
}

// The file names an organizer ends up with should say which event they are for.
func TestDownloadNames(t *testing.T) {
	def := signup.BuildDefinition(store.Tournament{
		Event: store.Event{Signup: store.Signup{DefinitionID: "MSL Open 2026"}},
	})
	if got := signupAppFilename(def); got != "signup-msl-open-2026.html" {
		t.Errorf("app file is %q", got)
	}
	if got := definitionFilename(def); got != "signup-msl-open-2026.json" {
		t.Errorf("definition file is %q", got)
	}
	// An event with nothing filled in still downloads to something openable.
	bare := signup.BuildDefinition(store.Tournament{})
	if got := signupAppFilename(bare); got != "signup-event.html" {
		t.Errorf("unconfigured app file is %q", got)
	}
}
