package httpapi_test

import (
	"encoding/json"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/signup"
)

// Signup for the whole event (phase 3): one folder back, and every discipline takes its
// share by the same check it runs on its own.

func reply(t *testing.T, name, submission string, entries ...string) map[string]string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format": "porta.signup.response", "version": 1, "definitionId": "msl-open-2026",
		"submissionId": submission,
		"participant":  map[string]string{"name": name, "club": "Example HEMA"},
		"entries":      entries,
	})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"source": name + ".json", "body": string(body)}
}

func TestOneImportForTheWholeEvent(t *testing.T) {
	h, ls := twoDisciplines(t)
	h.must("POST", "/api/disciplines", map[string]string{"name": "Rapier"}, nil)
	h.must("PUT", "/api/event/info", map[string]any{
		"signup": map[string]any{"definitionId": "msl-open-2026", "name": "MSL Open"},
		"schedule": []map[string]any{
			{"at": "09:30", "label": "Open Steel Longsword", "kind": "discipline", "tournament": "longsword"},
			{"at": "13:00", "label": "Open Sabre", "kind": "discipline"},
			{"at": "15:00", "label": "Rapier and dagger", "kind": "discipline"},
		},
	}, nil)

	// Sabre is matched by its name; Longsword is told; Rapier matches nothing.
	h.must("PUT", "/api/event/signup/rows", map[string]string{ls: "longsword"}, nil)
	var ready struct {
		Disciplines []httpapi.SignupShare `json:"disciplines"`
		Unclaimed   []struct{ ID string } `json:"unclaimed"`
	}
	h.must("GET", "/api/event/signup/ready", nil, &ready)
	rows := map[string]httpapi.SignupShare{}
	for _, d := range ready.Disciplines {
		rows[d.Discipline] = d
	}
	if !rows[ls].Chosen || rows[ls].Tournament != "longsword" {
		t.Errorf("Longsword should take the row it was told: %+v", rows[ls])
	}
	if rows["open-sabre"].Chosen || rows["open-sabre"].Tournament != "open-sabre" {
		t.Errorf("Sabre should take the row named like it: %+v", rows["open-sabre"])
	}
	if rows["rapier"].Tournament != "" || len(ready.Unclaimed) != 1 || ready.Unclaimed[0].ID != "rapier-and-dagger" {
		t.Errorf("Rapier takes nothing, and its row should be reported as nobody's: %+v %+v", rows["rapier"], ready.Unclaimed)
	}

	files := []map[string]string{
		reply(t, "Ada", "sub-1", "longsword"),
		reply(t, "Bo", "sub-2", "longsword", "open-sabre"),
		reply(t, "Cilla", "sub-3", "rapier-and-dagger"),
	}
	var preview httpapi.EventSignupPreview
	h.must("POST", "/api/event/signup/preview", map[string]any{"files": files}, &preview)
	if preview.Adding != 3 {
		t.Errorf("Ada once and Bo twice is three entries, got %d", preview.Adding)
	}
	if bo := preview.Rows[1]; bo.Verdict != signup.New || len(bo.Into) != 2 {
		t.Errorf("Bo should go into both: %+v", bo)
	}
	if cilla := preview.Rows[2]; cilla.Verdict != signup.NotHere || len(cilla.Into) != 0 {
		t.Errorf("Cilla's discipline is nobody's, and she should be told as much: %+v", cilla)
	}
	if v := h.people(); len(v.People) != 0 {
		t.Fatalf("a preview writes nothing: %+v", v)
	}

	var done struct {
		Added   int                        `json:"added"`
		Preview httpapi.EventSignupPreview `json:"preview"`
	}
	h.must("POST", "/api/event/signup/import", map[string]any{"files": files}, &done)
	if done.Added != 3 || done.Preview.Rows[1].Verdict != signup.Already {
		t.Errorf("the import should add three and then call them done: %d %+v", done.Added, done.Preview.Rows[1])
	}
	if a, b := h.personOf(ls, "Bo"), h.personOf("open-sabre", "Bo"); a == "" || a != b {
		t.Errorf("Bo should be one person in both: %q %q", a, b)
	}
	h.must("POST", "/api/event/signup/import", map[string]any{"files": files}, &done)
	if done.Added != 0 {
		t.Errorf("importing the same folder twice should add nobody, added %d", done.Added)
	}
}
