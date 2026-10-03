package demo_test

import (
	"encoding/json"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/demo"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The demo's volunteers are already at work, and the rules hold: nobody on a mat while
// they fence or while they work another (phase 5).
func TestTheDemoIsStaffed(t *testing.T) {
	e := demo.NewEvent()
	var v httpapi.StaffingView
	res := e.Request("GET", "/api/staff", nil)
	if err := json.Unmarshal([]byte(res.Body), &v); err != nil || res.Status != 200 {
		t.Fatalf("GET /api/staff: %d %v", res.Status, err)
	}
	if len(v.Members) < 8 || len(v.Assignments) == 0 || len(v.Physicians) != 1 {
		t.Fatalf("the demo should have its volunteers at work and a physician on call: %d members, %d assignments, %v", len(v.Members), len(v.Assignments), v.Physicians)
	}
	for _, w := range v.Warnings {
		t.Errorf("the demo's own suggestion should break no rule: %+v", w)
	}

	clara := ""
	for _, m := range v.Members {
		if m.Name == "Clara Wikström" {
			clara = m.Person
		}
	}
	res = e.Request("GET", "/api/people/"+clara, nil)
	var pv httpapi.PersonView
	_ = json.Unmarshal([]byte(res.Body), &pv)
	if len(pv.Entries) != 1 || pv.Entries[0].Discipline != "open-steel-longsword" {
		t.Errorf("Clara fences the longsword and is the same person as the volunteer: %+v", pv)
	}

	// A discipline's own staff list is the event's members who work it.
	s := snapIn(t, e, "open-steel-longsword")
	for _, m := range s.Tournament.Staff {
		if m.Name == "Clara Wikström" {
			t.Error("Clara works only the sabre, so the longsword's page should not list her")
		}
	}

	var sug httpapi.StaffSuggestion
	_ = json.Unmarshal([]byte(e.Request("POST", "/api/staff/suggest", nil).Body), &sug)
	b, _ := json.Marshal(map[string]string{"signature": sug.Signature})
	if res := e.Request("POST", "/api/staff/apply", b); res.Status != 200 {
		t.Errorf("applying the suggestion: %d %s", res.Status, res.Body)
	}
}
