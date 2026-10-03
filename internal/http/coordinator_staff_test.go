package httpapi_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

func (h *hall) staff() httpapi.StaffingView {
	h.t.Helper()
	var v httpapi.StaffingView
	h.must("GET", "/api/staff", nil, &v)
	return v
}

type snapWithStaff struct {
	Tournament struct {
		Staff []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"staff"`
	} `json:"tournament"`
}

// Staff lists kept by each discipline before staff were the event's come up to the event
// once, the same response in two disciplines as one member.
func TestEachDisciplinesStaffIsLifted(t *testing.T) {
	dir := t.TempDir()
	write := func(slug, staff string) {
		d := filepath.Join(dir, "disciplines", slug)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(d, "competitors.json"), []byte(`[]`), 0o644)
		_ = os.WriteFile(filepath.Join(d, "tournament.json"), []byte(`{"discipline":"`+slug+
			`","mats":1,"minPoolSize":4,"maxPoolSize":7,"seed":1,"pools":[],"staff":`+staff+`}`), 0o644)
	}
	write("longsword", `[{"id":"s1","name":"Dag","roles":["head-ref"],"signup":"sub-9"}]`)
	write("sabre", `[{"id":"s1","name":"Dag","roles":["score-keeper"],"signup":"sub-9"},{"id":"s2","name":"Eva","roles":["physician"]}]`)
	h := openHall(t, dir)
	v := h.staff()
	if len(v.Members) != 2 || len(v.Members[0].Disciplines) != 2 || v.Members[0].Person == "" {
		t.Fatalf("Dag is one member in both, with a person: %+v", v.Members)
	}
	if v.Crew["assistant-ref"] != 2 {
		t.Errorf("the crew should default to the SM rules': %+v", v.Crew)
	}
	var s snapWithStaff
	h.must("GET", "/api/d/sabre/state", nil, &s)
	if len(s.Tournament.Staff) != 2 {
		t.Errorf("Sabre's own page lists the event's members who work it: %+v", s.Tournament.Staff)
	}
}

func TestStaffByHand(t *testing.T) {
	h, ls := twoDisciplines(t)
	h.must("POST", "/api/d/"+ls+"/competitors", map[string]string{"name": "Astrid", "club": "Gbg"}, nil)
	astrid := h.personOf(ls, "Astrid")

	// Astrid fences longsword and referees sabre: the desk says she is the same person.
	var v httpapi.StaffingView
	if code := h.do("POST", "/api/staff", map[string]any{"name": "Astrid", "club": "Gbg", "roles": []string{"head-ref", "juggler"},
		"disciplines": []string{"open-sabre"}, "want": astrid}, &v); code != 201 {
		t.Fatalf("adding staff returned %d", code)
	}
	m := v.Members[0]
	if m.Person != astrid || len(m.Roles) != 1 || m.Disciplines[0] != "open-sabre" {
		t.Errorf("Astrid should be the same person, a head referee for Sabre: %+v", m)
	}
	if code := h.do("POST", "/api/staff", map[string]any{"name": " "}, nil); code != 400 {
		t.Errorf("a member without a name should be refused, got %d", code)
	}
	h.must("PATCH", "/api/staff/"+m.ID, map[string]any{"roles": []string{"head-ref", "score-keeper"}, "disciplines": []string{"open-sabre", ls}}, &v)
	if len(v.Members[0].Roles) != 2 || len(v.Members[0].Disciplines) != 2 {
		t.Errorf("roles and disciplines should change: %+v", v.Members[0])
	}
	// Removing her from the longsword's page takes her off the longsword only.
	h.must("DELETE", "/api/d/"+ls+"/staff/"+m.ID, nil, nil)
	if v := h.staff(); len(v.Members) != 1 || len(v.Members[0].Disciplines) != 1 {
		t.Errorf("she still works Sabre: %+v", v.Members)
	}
	h.must("DELETE", "/api/staff/"+m.ID, nil, &v)
	if len(v.Members) != 0 {
		t.Errorf("removed from the event, she is gone: %+v", v.Members)
	}
	if code := h.do("DELETE", "/api/staff/"+m.ID, nil, nil); code != 404 {
		t.Errorf("removing somebody not there is a 404, got %d", code)
	}
}

type drawnState struct {
	Competitors []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Person string `json:"person"`
	} `json:"competitors"`
	Tournament struct {
		Pools []struct {
			Number      int      `json:"number"`
			Competitors []string `json:"competitors"`
		} `json:"pools"`
	} `json:"tournament"`
}

// The staff on the plan: suggested, applied, checked, chosen by hand, and on a person's page.
func TestStaffingThePlan(t *testing.T) {
	h, ls := twoDisciplines(t)
	h.c.Clock = func() time.Time { return time.Date(2026, 11, 14, 8, 0, 0, 0, time.Local) }
	h.draw("/api/d/"+ls, 8, 2)
	h.must("PUT", "/api/staff/crew", map[string]int{"assistant-ref": 0, "score-keeper": 0}, nil)
	if code := h.do("PUT", "/api/staff/crew", map[string]int{"juggler": 1}, nil); code != 400 {
		t.Errorf("a role that is not one should be refused, got %d", code)
	}
	var s drawnState
	h.must("GET", "/api/d/"+ls+"/state", nil, &s)
	astrid := s.Competitors[0]
	ownPool := ""
	for _, p := range s.Tournament.Pools {
		for _, c := range p.Competitors {
			if c == astrid.ID {
				ownPool = fmt.Sprintf("%s/pool-%d", ls, p.Number)
			}
		}
	}
	for _, name := range []string{"Bo", "Cia"} {
		h.must("POST", "/api/staff", map[string]any{"name": name, "roles": []string{"head-ref"}}, nil)
	}
	// One of the longsword's own fencers referees too.
	var v httpapi.StaffingView
	h.must("POST", "/api/staff", map[string]any{"name": astrid.Name, "roles": []string{"head-ref"}, "want": astrid.Person}, &v)
	astridStaff, bo := "", ""
	for _, m := range v.Members {
		switch {
		case m.Person == astrid.Person:
			astridStaff = m.ID
		case m.Name == "Bo":
			bo = m.Person
		}
	}
	if len(v.Items) == 0 || len(v.Short) == 0 || v.Crew["head-ref"] != 1 || v.Crew["assistant-ref"] != 0 {
		t.Fatalf("with nobody assigned every item is short of its head referee: %+v", v)
	}

	var sug httpapi.StaffSuggestion
	h.must("POST", "/api/staff/suggest", nil, &sug)
	if sug.Changes == 0 || sug.Signature == "" {
		t.Fatalf("a suggestion should fill the slots: %+v", sug)
	}
	if len(h.staff().Assignments) != 0 {
		t.Error("a suggestion writes nothing")
	}
	if code := h.do("POST", "/api/staff/apply", map[string]string{"signature": "stale"}, nil); code != 409 {
		t.Errorf("a stale suggestion should be refused, got %d", code)
	}
	h.must("POST", "/api/staff/apply", map[string]string{"signature": sug.Signature}, &v)
	if len(v.Assignments) == 0 {
		t.Fatal("applied, the slots should be filled")
	}
	for _, w := range v.Warnings {
		t.Errorf("a suggestion never puts anyone on a mat while they fence or work elsewhere: %+v", w)
	}
	for _, a := range v.Assignments {
		if a.Staff == astridStaff && a.Item == ownPool {
			t.Errorf("Astrid must not referee the pool she fences in: %+v", a)
		}
	}

	// By hand: Astrid on her own pool is kept, and reported.
	h.must("PUT", "/api/staff/assignments", map[string]any{"item": ownPool, "role": "head-ref", "slot": 1, "staff": astridStaff}, &v)
	reported := false
	for _, w := range v.Warnings {
		reported = reported || (w.Kind == "fencing" && w.Staff == astridStaff && w.Item == ownPool)
	}
	if !reported {
		t.Errorf("Astrid refereeing the pool she fences in should be reported: %+v", v.Warnings)
	}
	if code := h.do("PUT", "/api/staff/assignments", map[string]any{"item": ownPool, "role": "head-ref", "slot": 1, "staff": "st-nobody"}, nil); code != 400 {
		t.Errorf("somebody not on the staff cannot be assigned, got %d", code)
	}

	// Her page shows her duty beside her fencing; a member who only works has a page too.
	var pv httpapi.PersonView
	h.must("GET", "/api/people/"+astrid.Person, nil, &pv)
	if len(pv.Duties) == 0 || pv.Duties[0].Role != "head-ref" || pv.Duties[0].Start == "" || len(pv.Entries) != 1 {
		t.Errorf("Astrid's page should list her refereeing beside her entry: %+v", pv)
	}
	h.must("GET", "/api/people/"+bo, nil, &pv)
	if pv.Name != "Bo" || len(pv.Entries) != 0 || len(pv.Duties) == 0 {
		t.Errorf("Bo only works, and his page says who he is and what he does: %+v", pv)
	}
}

// Bo typed in at the desk and again on the staff is two records of one human: the staff
// panel says so, until the People panel settles it (#133).
func TestAStaffMemberWhoMayBeSomebodyElseIsFlagged(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.must("POST", "/api/d/"+ls+"/competitors", map[string]string{"name": "Bo Berg", "club": "Gbg"}, nil)
	h.must("POST", "/api/staff", map[string]any{"name": "Bo Berg", "roles": []string{"head-ref"}}, nil)
	h.must("POST", "/api/staff", map[string]any{"name": "Cleo", "roles": []string{"head-ref"}}, nil)
	v := h.staff()
	if len(v.Unsure) != 1 || v.Members[0].Name != "Bo Berg" || v.Unsure[0] != v.Members[0].ID {
		t.Fatalf("Bo on the staff may be Bo fencing, and only Bo: %v of %+v", v.Unsure, v.Members)
	}
	// Kept apart, the question is answered.
	fencer := h.personOf(ls, "Bo Berg")
	h.must("POST", "/api/people/"+fencer+"/apart", map[string]string{"other": v.Members[0].Person}, nil)
	if v := h.staff(); len(v.Unsure) != 0 {
		t.Errorf("two Bos kept apart are no longer a question: %v", v.Unsure)
	}
}

// The landing page lists everyone, the staff too (#135): each with their page.
func TestTheEventViewListsTheStaff(t *testing.T) {
	h := openHall(t, t.TempDir())
	h.must("POST", "/api/staff", map[string]any{"name": "Cleo", "club": "Gbg", "roles": []string{"head-ref"}}, nil)
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	if len(ev.Staff) != 1 || ev.Staff[0].Name != "Cleo" || ev.Staff[0].Person == "" {
		t.Errorf("the event view should list Cleo, with a page: %+v", ev.Staff)
	}
}
