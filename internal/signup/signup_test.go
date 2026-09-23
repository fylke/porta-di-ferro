package signup_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The import rules are the part of this that has to be right. A response that arrives
// twice must not become two competitors, a response for last year's open must not be
// importable at all, and the organizer has to be told which is which before anything is
// written. All of it is data in and data out, so all of it is tested here.

func event() store.Tournament {
	return store.Tournament{
		Discipline: "Open steel Longsword",
		Event: store.Event{
			Signup: store.Signup{
				DefinitionID: "msl-open-2026",
				Name:         "MSL Open",
				Venue:        "Linköping",
				Date:         "2026-11-15",
				Tournament:   "longsword",
			},
			Schedule: []store.ScheduleItem{
				{At: "08:30", Label: "Gear check"},
				{At: "09:30", Label: "Open Steel Longsword", Kind: "discipline", Tournament: "longsword", Capacity: 3},
				{At: "12:00", Ends: "13:00", Label: "Lunch", Kind: "break"},
				{At: "13:00", Label: "Open Sabre", Kind: "discipline", Tournament: "sabre"},
			},
		},
	}
}

func responseFile(t *testing.T, name, submission string, entries ...string) signup.File {
	t.Helper()
	body, err := json.Marshal(signup.Response{
		Format:       signup.ResponseFormat,
		Version:      signup.Version,
		DefinitionID: "msl-open-2026",
		SubmissionID: submission,
		Participant:  signup.Participant{Name: name, Club: "Example HEMA"},
		Entries:      entries,
	})
	if err != nil {
		t.Fatalf("building a response: %v", err)
	}
	return signup.File{Source: name + ".json", Body: body}
}

// The definition is built from the organizer's own programme rather than from a second
// list they have to keep in step with it.
func TestTheProgrammeIsTheDefinition(t *testing.T) {
	def := signup.BuildDefinition(event())

	if def.Format != signup.DefinitionFormat || def.Version != signup.Version {
		t.Errorf("format markers are %q v%d", def.Format, def.Version)
	}
	if def.Event.Name != "MSL Open" || def.Event.Venue != "Linköping" {
		t.Errorf("event came out as %+v", def.Event)
	}
	if len(def.Tournaments) != 2 {
		t.Fatalf("two rows are disciplines; got %d tournaments: %+v", len(def.Tournaments), def.Tournaments)
	}
	if def.Tournaments[0].ID != "longsword" || def.Tournaments[0].Capacity != 3 {
		t.Errorf("first tournament is %+v", def.Tournaments[0])
	}
	if got := len(def.Schedule); got != 4 {
		t.Errorf("the whole programme is published, including what cannot be entered; got %d rows", got)
	}
	// Lunch keeps its end time; the gear check is neither a discipline nor a break.
	var lunch, gear signup.ScheduleEntry
	for _, e := range def.Schedule {
		if e.Label == "Lunch" {
			lunch = e
		}
		if e.Label == "Gear check" {
			gear = e
		}
	}
	if lunch.Type != "break" || lunch.EndsAt != "13:00" {
		t.Errorf("lunch came out as %+v", lunch)
	}
	if gear.Type != "item" {
		t.Errorf("the gear check came out as %q", gear.Type)
	}
	// Nothing in the file is a port or a path.
	raw, _ := json.Marshal(def)
	for _, leak := range []string{"8080", "localhost", "C:\\", "/api/"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("the definition leaks %q: %s", leak, raw)
		}
	}
}

// A discipline row with no identifier of its own still publishes a stable one.
func TestIdentifiersAreDerivedWhenNotGiven(t *testing.T) {
	tr := event()
	tr.Event.Schedule = []store.ScheduleItem{
		{Label: "Långsvärd, öppen klass", Kind: "discipline"},
	}
	def := signup.BuildDefinition(tr)
	if len(def.Tournaments) != 1 {
		t.Fatalf("got %+v", def.Tournaments)
	}
	// Folded, not dropped: the identifier has to survive a round trip through a file
	// name and a hand edit.
	if id := def.Tournaments[0].ID; id != "langsvard-oppen-klass" {
		t.Errorf("identifier came out as %q", id)
	}
	if signup.Slug("Open Sabre") != "open-sabre" {
		t.Errorf("Slug(Open Sabre) = %q", signup.Slug("Open Sabre"))
	}
}

// The whole point of the preview: the organizer sees what would happen, and nothing has.
func TestPreviewSortsTheFolderOut(t *testing.T) {
	def := signup.BuildDefinition(event())
	existing := []store.Competitor{
		{ID: "c1", Name: "Already Imported", Signup: "sub-old"},
		{ID: "c2", Name: "Typed In By Hand"},
	}

	wrongEvent, _ := json.Marshal(signup.Response{
		Format: signup.ResponseFormat, Version: signup.Version,
		DefinitionID: "some-other-open-2025", SubmissionID: "sub-x",
		Participant: signup.Participant{Name: "Wrong Event"}, Entries: []string{"longsword"},
	})

	files := []signup.File{
		responseFile(t, "Ada Example", "sub-1", "longsword"),
		responseFile(t, "Bo Example", "sub-2", "longsword", "sabre"),
		// Only the sabre: somebody else's run imports this one.
		responseFile(t, "Cilla Example", "sub-3", "sabre"),
		// The same submission twice in one folder, under two file names.
		responseFile(t, "Ada Example", "sub-1", "longsword"),
		// Already in the competitor list from a previous import.
		responseFile(t, "Already Imported", "sub-old", "longsword"),
		{Source: "other-event.json", Body: wrongEvent},
		{Source: "nonsense.json", Body: []byte("this is not json")},
		{Source: "empty.json", Body: nil},
		responseFile(t, "Dag Example", "sub-4", "rapier-and-dagger"),
	}

	p := signup.Check(def, "longsword", files, existing)
	if len(p.Rows) != len(files) {
		t.Fatalf("every file gets a row; got %d for %d files", len(p.Rows), len(files))
	}
	want := []signup.Verdict{
		signup.New, signup.New, signup.NotHere, signup.Repeat, signup.Already,
		signup.OtherEvent, signup.Invalid, signup.Invalid, signup.Unknown,
	}
	for i, w := range want {
		if p.Rows[i].Verdict != w {
			t.Errorf("row %d (%s) is %q, want %q -- %s",
				i, p.Rows[i].Source, p.Rows[i].Verdict, w, p.Rows[i].Problem)
		}
	}
	if p.Adding != 2 {
		t.Errorf("two of those are new; the preview says %d", p.Adding)
	}
	if p.Tournament != "longsword" {
		t.Errorf("the preview should say which discipline it is for, got %q", p.Tournament)
	}
	// Two entered already, two more coming, capacity three.
	if len(p.Capacity) == 0 {
		t.Error("that is over capacity and the organizer should be warned")
	}
	// A refusal says why, in words an organizer can act on.
	for _, row := range p.Rows {
		switch row.Verdict {
		case signup.Invalid, signup.Unknown, signup.OtherEvent:
			if row.Problem == "" {
				t.Errorf("%s was refused with no reason given", row.Source)
			}
		}
	}
}

// Importing the same folder twice is the case this is built around: an organizer who is
// not sure whether they already did it should be able to just do it again.
func TestImportingTwiceAddsNobodyTwice(t *testing.T) {
	def := signup.BuildDefinition(event())
	files := []signup.File{
		responseFile(t, "Ada Example", "sub-1", "longsword"),
		responseFile(t, "Bo Example", "sub-2", "longsword"),
	}
	nextID := func(existing []store.Competitor) string {
		return fmt.Sprintf("c%d", len(existing)+1)
	}

	var roster []store.Competitor
	first := signup.Check(def, "longsword", files, roster)
	roster = signup.Import(first, nextID, roster)
	if len(roster) != 2 {
		t.Fatalf("the first import should add two, got %d", len(roster))
	}
	if roster[0].Signup != "sub-1" || roster[0].Name != "Ada Example" {
		t.Errorf("the competitor should remember where they came from: %+v", roster[0])
	}

	second := signup.Check(def, "longsword", files, roster)
	if second.Adding != 0 {
		t.Errorf("the second import should add nobody, it says %d", second.Adding)
	}
	roster = signup.Import(second, nextID, roster)
	if len(roster) != 2 {
		t.Fatalf("importing twice made %d competitors", len(roster))
	}
	for _, row := range second.Rows {
		if row.Verdict != signup.Already {
			t.Errorf("%s came back as %q on the second pass", row.Source, row.Verdict)
		}
	}
}

// Only the rows the organizer saw as new are written. A preview they approved and the
// import that follows cannot come apart.
func TestImportWritesOnlyWhatThePreviewCalledNew(t *testing.T) {
	def := signup.BuildDefinition(event())
	files := []signup.File{
		responseFile(t, "Ada Example", "sub-1", "longsword"),
		responseFile(t, "Cilla Example", "sub-3", "sabre"),
		{Source: "bad.json", Body: []byte("{")},
	}
	p := signup.Check(def, "longsword", files, nil)
	roster := signup.Import(p, func(e []store.Competitor) string {
		return fmt.Sprintf("c%d", len(e)+1)
	}, nil)

	if len(roster) != 1 || roster[0].Name != "Ada Example" {
		t.Fatalf("only the longsword entry belongs here, got %+v", roster)
	}
}

// A single-discipline event that never set an identifier still works: everything valid
// is importable, because there is nowhere else for it to go.
func TestASingleDisciplineEventNeedsNoIdentifier(t *testing.T) {
	tr := event()
	tr.Event.Signup.Tournament = ""
	def := signup.BuildDefinition(tr)

	p := signup.Check(def, "", []signup.File{
		responseFile(t, "Ada Example", "sub-1", "longsword"),
		responseFile(t, "Cilla Example", "sub-3", "sabre"),
	}, nil)
	if p.Adding != 2 {
		t.Errorf("with no discipline set, both are importable; the preview says %d", p.Adding)
	}
}

func TestResponsesThatCannotBeUsedAreRefusedWithAReason(t *testing.T) {
	ok := signup.Response{
		Format: signup.ResponseFormat, Version: signup.Version,
		DefinitionID: "msl-open-2026", SubmissionID: "sub-1",
		Participant: signup.Participant{Name: "Ada"}, Entries: []string{"longsword"},
	}
	mutate := func(fn func(*signup.Response)) []byte {
		r := ok
		fn(&r)
		b, _ := json.Marshal(r)
		return b
	}

	cases := []struct {
		name string
		body []byte
		want string
	}{
		{"no name", mutate(func(r *signup.Response) { r.Participant.Name = "  " }), "no name"},
		{"no submission id", mutate(func(r *signup.Response) { r.SubmissionID = "" }), "submission id"},
		{"nothing chosen", mutate(func(r *signup.Response) { r.Entries = nil }), "no disciplines"},
		{"no version", mutate(func(r *signup.Response) { r.Version = 0 }), "which version"},
		{"from the future", mutate(func(r *signup.Response) { r.Version = signup.Version + 1 }), "newer signup app"},
		{"wrong format", mutate(func(r *signup.Response) { r.Format = "something.else" }), "not a signup response"},
		{"no format", mutate(func(r *signup.Response) { r.Format = "" }), "no format marker"},
		{"not json", []byte("<html>"), "not readable"},
		{"empty", nil, "empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := signup.ParseResponse(c.body)
			if err == nil {
				t.Fatal("that should not have been accepted")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the reason given was %q, wanted something about %q", err, c.want)
			}
		})
	}

	// The one mistake a participant will actually make: sending the definition back.
	def, _ := json.Marshal(signup.BuildDefinition(event()))
	if _, err := signup.ParseResponse(def); err == nil ||
		!strings.Contains(err.Error(), "definition") {
		t.Errorf("sending the definition back should say so, got %v", err)
	}
}

// The organizer is told what is missing before a definition goes out, rather than by a
// participant who cannot use the file.
func TestReadySaysWhatIsMissing(t *testing.T) {
	if missing := signup.Ready(signup.BuildDefinition(event())); len(missing) != 0 {
		t.Errorf("that event is ready; got %v", missing)
	}
	bare := signup.BuildDefinition(store.Tournament{})
	missing := signup.Ready(bare)
	if len(missing) != 3 {
		t.Errorf("an unconfigured event is missing three things, got %v", missing)
	}
}

// A hand-edited response with the label in it rather than the identifier still matches.
func TestEntriesAreNormalisedBeforeComparing(t *testing.T) {
	def := signup.BuildDefinition(event())
	body, _ := json.Marshal(signup.Response{
		Format: signup.ResponseFormat, Version: signup.Version,
		DefinitionID: "msl-open-2026", SubmissionID: "sub-1",
		Participant: signup.Participant{Name: "Ada"},
		Entries:     []string{"Longsword", "longsword"},
	})
	p := signup.Check(def, "longsword", []signup.File{{Source: "a.json", Body: body}}, nil)
	if p.Rows[0].Verdict != signup.New {
		t.Fatalf("got %q: %s", p.Rows[0].Verdict, p.Rows[0].Problem)
	}
	if len(p.Rows[0].Entries) != 1 {
		t.Errorf("the duplicate entry should have been folded away, got %v", p.Rows[0].Entries)
	}
}
