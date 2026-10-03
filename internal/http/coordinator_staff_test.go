package httpapi_test

import (
	"os"
	"path/filepath"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

func (h *hall) staff() httpapi.StaffView {
	h.t.Helper()
	var v httpapi.StaffView
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
	var v httpapi.StaffView
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
