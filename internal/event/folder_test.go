package event_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/event"
	"github.com/fylke/porta-di-ferro/internal/store"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// A tournament folder from before events existed is what every organizer who has used
// the application has on disk. Opening it must give them their tournament as the event's
// one discipline, every file intact, and the day around it in event.json (proposal §7).
func TestALegacyFolderBecomesTheFirstDiscipline(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "tournament.json"), `{
  "discipline": "Open Sabre",
  "mats": 3, "minPoolSize": 4, "maxPoolSize": 7, "seed": 42, "pools": [],
  "event": {
    "welcome": "Welcome to the hall",
    "wifi": {"ssid": "Hall"},
    "signup": {"name": "MSL Open", "tournament": "sabre-pools"}
  }
}`)
	write(t, filepath.Join(dir, "competitors.json"), `[{"id":"c1","name":"Astrid","club":"Gbg","withdrawn":false}]`)
	log := `{"seq":1,"type":"timer","timer":{"action":"start"}}` + "\n"
	write(t, filepath.Join(dir, "matches", "p1m1.ndjson"), log)

	f, err := event.Open(dir)
	if err != nil {
		t.Fatalf("opening the legacy folder: %v", err)
	}

	slugs, err := f.Slugs()
	if err != nil || !reflect.DeepEqual(slugs, []string{"open-sabre"}) {
		t.Fatalf("the event should have one discipline named for the tournament, got %v (%v)", slugs, err)
	}
	disc := f.DisciplineDir("open-sabre")
	for _, name := range []string{"tournament.json", "competitors.json", "writers.json", "displays.json", "matches"} {
		if exists(filepath.Join(dir, name)) {
			t.Errorf("%s is still at the top of the event folder", name)
		}
	}
	got, err := os.ReadFile(filepath.Join(disc, "matches", "p1m1.ndjson"))
	if err != nil || string(got) != log {
		t.Errorf("the match log should come through byte for byte, got %q (%v)", got, err)
	}

	file, err := f.Read()
	if err != nil {
		t.Fatal(err)
	}
	if file.Welcome != "Welcome to the hall" || file.Wifi.SSID != "Hall" || file.Signup.Name != "MSL Open" {
		t.Errorf("the day around the tournament should be lifted into event.json, got %+v", file.Event)
	}
	if file.Signup.Tournament != "" {
		t.Error("which programme row a discipline is belongs to the discipline, not the event")
	}

	st, err := store.Open(disc)
	if err != nil {
		t.Fatal(err)
	}
	tour, err := st.Tournament()
	if err != nil {
		t.Fatal(err)
	}
	if tour.Discipline != "Open Sabre" || tour.Mats != 3 || tour.Seed != 42 {
		t.Errorf("the tournament itself should be unchanged, got %+v", tour)
	}
	if tour.Event.Welcome != "" || tour.Event.Signup.Tournament != "sabre-pools" {
		t.Errorf("the tournament keeps only its own programme row, got %+v", tour.Event)
	}
	comps, _ := st.Competitors()
	if len(comps) != 1 || comps[0].Name != "Astrid" {
		t.Errorf("competitors should come through, got %+v", comps)
	}

	// Opening it again is a no-op, not a second move.
	if _, err := event.Open(dir); err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if slugs, _ := f.Slugs(); len(slugs) != 1 {
		t.Errorf("a reopened event should still have one discipline, got %v", slugs)
	}
}

// A start that died halfway through the move carries on where it stopped.
func TestAnInterruptedMoveCarriesOn(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "event.json"), `{"welcome":"Hi","disciplines":["longsword"]}`)
	write(t, filepath.Join(dir, "disciplines", "longsword", "tournament.json"), `{"discipline":"Longsword","mats":2,"pools":[]}`)
	write(t, filepath.Join(dir, "competitors.json"), `[]`)

	f, err := event.Open(dir)
	if err != nil {
		t.Fatalf("resuming the move: %v", err)
	}
	if !exists(filepath.Join(f.DisciplineDir("longsword"), "competitors.json")) {
		t.Error("what was left at the top should land in the folder the move started")
	}
	if file, _ := f.Read(); file.Welcome != "Hi" {
		t.Error("event.json written by the first attempt should not be replaced")
	}
}

// A hand-edited tournament.json that no longer parses is still the organizer's file. It
// moves with its logs, and the discipline reports the error where it can be fixed.
func TestAnUnreadableTournamentStillMoves(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "tournament.json"), `{"mats": 3,`)
	f, err := event.Open(dir)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	slugs, _ := f.Slugs()
	if len(slugs) != 1 {
		t.Fatalf("the broken tournament should still be a discipline, got %v", slugs)
	}
	b, _ := os.ReadFile(filepath.Join(f.DisciplineDir(slugs[0]), "tournament.json"))
	if string(b) != `{"mats": 3,` {
		t.Errorf("a file that does not parse must not be rewritten, got %q", b)
	}
}

func TestAFreshFolderIsAnEmptyEvent(t *testing.T) {
	f, err := event.Open(filepath.Join(t.TempDir(), "new"))
	if err != nil {
		t.Fatal(err)
	}
	slugs, _ := f.Slugs()
	if len(slugs) != 0 {
		t.Errorf("a new event has no disciplines until one is made, got %v", slugs)
	}
	if exists(filepath.Join(f.Dir(), "matches")) {
		t.Error("a fresh event folder should not look like a tournament folder")
	}
}

func TestDisciplinesAreCreatedOrderedAndRetired(t *testing.T) {
	f, err := event.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := f.Create("Open steel Longsword")
	b, _ := f.Create("Open Sabre")
	c, _ := f.Create("Open Sabre")
	if a != "open-steel-longsword" || b != "open-sabre" || c != "open-sabre-2" {
		t.Errorf("slugs should come from the names and never collide, got %q %q %q", a, b, c)
	}
	if slugs, _ := f.Slugs(); !reflect.DeepEqual(slugs, []string{a, b, c}) {
		t.Errorf("disciplines should list in the order they were made, got %v", slugs)
	}

	// A folder dropped in by hand is part of the event, after the ones on the list.
	if err := os.MkdirAll(f.DisciplineDir("aaa-by-hand"), 0o755); err != nil {
		t.Fatal(err)
	}
	if slugs, _ := f.Slugs(); slugs[len(slugs)-1] != "aaa-by-hand" {
		t.Errorf("an unlisted folder should come last, got %v", slugs)
	}

	where, err := f.Retire(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(where), "/retired/open-sabre-") || !exists(where) {
		t.Errorf("a retired discipline should be kept under retired/, got %s", where)
	}
	if slugs, _ := f.Slugs(); reflect.DeepEqual(slugs, []string{a, c, "aaa-by-hand"}) == false {
		t.Errorf("the retired discipline should be gone from the list, got %v", slugs)
	}
	if _, err := f.Retire("nope"); err == nil {
		t.Error("retiring a discipline that is not there should say so")
	}
}

func TestSlugs(t *testing.T) {
	for in, want := range map[string]string{
		"Open steel Longsword": "open-steel-longsword",
		"Långsvärd, damer":     "langsvard-damer",
		"Women's and underrepresented genders Longsword": "women-s-and-underrepresented-genders-longsword",
		"  !!  ": "",
	} {
		got := event.Slugify(in)
		if strings.HasPrefix(want, "women") {
			if !strings.HasPrefix(got, "women-s-and") || len(got) > 48 {
				t.Errorf("Slugify(%q) = %q, want a prefix of %q no longer than 48", in, got, want)
			}
			continue
		}
		if got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}

	f, _ := event.Open(t.TempDir())
	if slug, _ := f.Create("Admin"); slug == "admin" {
		t.Error("a slug that reads as another page's address should be avoided")
	}
	if slug, _ := f.Create(""); slug != "discipline" {
		t.Errorf("an unnamed discipline should still get a slug, got %q", slug)
	}
}

func TestABrokenEventFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "event.json"), `{"welcome": `)
	f, err := event.Open(dir)
	if err != nil {
		t.Fatalf("a broken event.json must not stop the event opening: %v", err)
	}
	if _, err := f.Read(); err == nil {
		t.Error("reading it should report the parse error")
	}
	var file event.File
	if err := json.Unmarshal([]byte(`{"welcome":"x","disciplines":["a"]}`), &file); err != nil || file.Welcome != "x" {
		t.Errorf("event.json should be store.Event's fields flat beside the order, got %+v (%v)", file, err)
	}
}
