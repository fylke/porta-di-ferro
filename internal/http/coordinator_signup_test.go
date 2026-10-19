package httpapi_test

import (
	"encoding/json"
	"strings"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/signup"
)

// Signup for the whole event (phase 3): one folder back, and every discipline takes its
// share by the same check it runs on its own. What can be entered is the disciplines
// themselves, by name, and an email is a person (#142).

func reply(t *testing.T, name, submission string, entries ...string) map[string]string {
	return replyStaffing(t, name, submission, nil, entries...)
}

// replyStaffing is a response that also offers to work the disciplines named in staff.
func replyStaffing(t *testing.T, name, submission string, staff []string, entries ...string) map[string]string {
	t.Helper()
	res := map[string]any{
		"format": "porta.signup.response", "version": signup.Version, "definitionId": "msl-open-2026",
		"submissionId": submission,
		"participant":  map[string]string{"name": name, "club": "Example HEMA", "email": strings.ToLower(name) + "@example.org"},
		"entries":      entries,
	}
	if len(staff) > 0 {
		res["staff"] = map[string]any{"tournaments": staff, "roles": []string{"head-ref", "assistant-ref"}}
	}
	body, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"source": name + "-" + submission + ".json", "body": string(body)}
}

// threeDisciplines is an event of a named longsword, the sabre and the rapier, set up for
// signup. It answers the longsword's slug.
func threeDisciplines(t *testing.T) (*hall, string) {
	t.Helper()
	h, first := twoDisciplines(t)
	var ls httpapi.DisciplineSummary
	h.must("PATCH", "/api/disciplines/"+first, map[string]string{"name": "Open Steel Longsword"}, &ls)
	h.must("POST", "/api/disciplines", map[string]string{"name": "Rapier"}, nil)
	h.must("PUT", "/api/event/info", map[string]any{
		"signup": map[string]any{"definitionId": "msl-open-2026", "name": "MSL Open"},
		"schedule": []map[string]any{
			{"at": "09:30", "label": "Longsword pools", "kind": "discipline"},
			{"at": "12:00", "ends": "13:00", "label": "Lunch", "kind": "break"},
		},
	}, nil)
	return h, ls.Slug
}

func TestOneImportForTheWholeEvent(t *testing.T) {
	h, ls := threeDisciplines(t)

	// The file offers the disciplines themselves, under their own names.
	var def signup.Definition
	h.must("GET", "/api/event/signup/definition.json", nil, &def)
	var offered []string
	for _, d := range def.Tournaments {
		offered = append(offered, d.ID+"="+d.Label)
	}
	if strings.Join(offered, ",") != ls+"=Open Steel Longsword,open-sabre=Open Sabre,rapier=Rapier" {
		t.Errorf("the file should offer the three disciplines by name, got %v", offered)
	}
	if strings.Join(def.Fields, ",") != "name,club,email" || len(def.Schedule) != 2 {
		t.Errorf("everybody gives an email, and the programme goes out to read: %+v", def)
	}

	files := []map[string]string{
		replyStaffing(t, "Ada", "sub-1", []string{"open-sabre"}, ls),
		reply(t, "Bo", "sub-2", ls, "open-sabre"),
		reply(t, "Cilla", "sub-3", "rapier-and-dagger"),
		reply(t, "Dan", "sub-4", "rapier"),
	}
	var preview httpapi.EventSignupPreview
	h.must("POST", "/api/event/signup/preview", map[string]any{"files": files}, &preview)
	if preview.Adding != 4 {
		t.Errorf("Ada once, Bo twice and Dan once is four entries, got %d", preview.Adding)
	}
	if bo := preview.Rows[1]; bo.Verdict != signup.New || len(bo.Into) != 2 {
		t.Errorf("Bo should go into both: %+v", bo)
	}
	if cilla := preview.Rows[2]; cilla.Verdict != signup.Unknown || len(cilla.Into) != 0 {
		t.Errorf("Cilla named no discipline of the event's, and should be told as much: %+v", cilla)
	}
	if v := h.people(); len(v.People) != 0 {
		t.Fatalf("a preview writes nothing: %+v", v)
	}

	var done struct {
		Added   int                        `json:"added"`
		Preview httpapi.EventSignupPreview `json:"preview"`
	}
	h.must("POST", "/api/event/signup/import", map[string]any{"files": files}, &done)
	if done.Added != 4 || done.Preview.Rows[1].Verdict != signup.Already {
		t.Errorf("the import should add four and then call them done: %d %+v", done.Added, done.Preview.Rows[1])
	}
	if a, b := h.personOf(ls, "Bo"), h.personOf("open-sabre", "Bo"); a == "" || a != b {
		t.Errorf("Bo should be one person in both: %q %q", a, b)
	}
	// Ada fences longsword and offered to referee Sabre: one person, on the event's staff.
	if st := h.staff(); len(st.Members) != 1 || st.Members[0].Person != h.personOf(ls, "Ada") {
		t.Errorf("Ada's offer to work Sabre should be a member of the event's staff, the same person: %+v", st.Members)
	}
	h.must("POST", "/api/event/signup/import", map[string]any{"files": files}, &done)
	if done.Added != 0 {
		t.Errorf("importing the same folder twice should add nobody, added %d", done.Added)
	}

	// Bo signs up again from his phone, adding the rapier: a new submission, the same
	// email. Already in the two he had, new in the rapier -- and the same person there.
	again := []map[string]string{reply(t, "Bo", "sub-from-his-phone", ls, "open-sabre", "rapier")}
	h.must("POST", "/api/event/signup/import", map[string]any{"files": again}, &done)
	if done.Added != 1 {
		t.Errorf("only the rapier is new for Bo, added %d", done.Added)
	}
	if got := h.personOf("rapier", "Bo"); got != h.personOf(ls, "Bo") {
		t.Errorf("Bo in the rapier should be the same Bo: %q", got)
	}
	// And the people the signup made are nobody's to ask about.
	if v := h.people(); len(v.Duplicates) != 0 {
		t.Errorf("signups with an email leave nobody to decide about: %v", v.Duplicates)
	}
}

// A discipline's own import, in an event of several, takes only what names it.
func TestADisciplinesOwnImportTakesOnlyItsOwn(t *testing.T) {
	h, ls := threeDisciplines(t)
	files := []map[string]string{
		reply(t, "Ada", "sub-1", ls),
		reply(t, "Bo", "sub-2", "open-sabre"),
	}
	var done struct {
		Added int `json:"added"`
	}
	h.must("POST", "/api/d/rapier/signup/import", map[string]any{"files": files}, &done)
	if done.Added != 0 {
		t.Errorf("nobody named the rapier, added %d", done.Added)
	}
	h.must("POST", "/api/d/open-sabre/signup/import", map[string]any{"files": files}, &done)
	if done.Added != 1 {
		t.Errorf("Sabre takes only Bo: added %d", done.Added)
	}
}

// An event of one discipline signs up for that discipline, by name, on the old paths.
func TestOneDisciplineSignsUpForItself(t *testing.T) {
	h := openHall(t, t.TempDir())
	var view httpapi.EventView
	h.must("GET", "/api/event", nil, &view)
	var ready struct{ Missing []string }
	h.must("GET", "/api/signup/ready", nil, &ready)
	if !strings.Contains(strings.Join(ready.Missing, ","), "discipline with a name") {
		t.Errorf("an unnamed discipline is nothing to sign up for, and the panel should say so: %v", ready.Missing)
	}
	var ls httpapi.DisciplineSummary
	h.must("PATCH", "/api/disciplines/"+view.Disciplines[0].Slug, map[string]string{"name": "Open Sabre"}, &ls)
	h.must("PUT", "/api/event/info", map[string]any{"signup": map[string]any{"definitionId": "msl-open-2026", "name": "MSL Open"}}, nil)
	var done struct {
		Added int `json:"added"`
	}
	h.must("POST", "/api/signup/import", map[string]any{"files": []map[string]string{reply(t, "Ada", "sub-1", "open-sabre")}}, &done)
	if done.Added != 1 {
		t.Errorf("Ada should be entered, added %d", done.Added)
	}
}
