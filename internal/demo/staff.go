package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The demo's staff (phase 5): staff.json in memory, with the coordinator's own decisions --
// staffing.Assign and Check, httpapi.StaffInput, ViewStaffing, SuggestStaff, DutiesOf.

// demoStaff are the volunteers the visitor finds: enough referees for the morning's three
// mats, short of assistants -- a real club open's usual problem -- and Clara, who fences
// the longsword and referees the sabre, so the panel can show she is never put on a mat
// while she fences.
var demoStaff = []struct {
	name, club  string
	roles       []string
	disciplines []string
}{
	{"Hedda Wallin", "MSL Linköping", []string{"head-ref", "assistant-ref"}, nil},
	{"Ivar Lund", "Uppsala HEMA", []string{"head-ref", "assistant-ref"}, nil},
	{"Jenny Falk", "Gotlands Fäktskola", []string{"head-ref"}, nil},
	{"Clara Wikström", "MSL Linköping", []string{"head-ref", "assistant-ref"}, []string{"open-sabre"}},
	{"Kerstin Ahl", "Malmö Svärdsgille", []string{"assistant-ref", "score-keeper"}, nil},
	{"Leo Brandt", "Oslo Fribryterlag", []string{"assistant-ref", "score-keeper"}, nil},
	{"Mira Saar", "Helsinki Longsword", []string{"score-keeper"}, nil},
	{"Nils Ek", "Uppsala HEMA", []string{"score-keeper"}, nil},
	{"Olga Berg", "MSL Linköping", []string{"score-keeper"}, nil},
	{"Dr. Pia Holm", "", []string{"physician"}, nil},
}

// seedStaff gives the event its volunteers, Clara as the person she already is.
func (e *Event) seedStaff() {
	e.staff = store.Staff{Members: []store.StaffMember{}}
	for _, s := range demoStaff {
		want := ""
		if s.name == "Clara Wikström" {
			if d := e.find("open-steel-longsword"); d != nil {
				for _, c := range d.competitors {
					if c.Name == s.name {
						want = c.Person
					}
				}
			}
		}
		e.staff.Members = append(e.staff.Members, store.StaffMember{
			ID: staffing.NewID(), Name: s.name, Club: s.club, Roles: s.roles, Disciplines: s.disciplines,
			Person: e.personFor(s.name, s.club, "", want),
		})
	}
}

func (e *Event) staffing() (httpapi.StaffingView, httpapi.StaffSuggestion) {
	in, r, _ := e.times()
	si := httpapi.StaffInput(in, r, e.staff)
	return httpapi.ViewStaffing(si, e.staff, e.mats()), httpapi.SuggestStaff(si)
}

func (e *Event) staffView() Response {
	v, _ := e.staffing()
	return ok(v)
}

func (e *Event) slugs() []string {
	var out []string
	for _, d := range e.disciplines {
		out = append(out, d.slug)
	}
	return out
}

// addStaffFromSignup is a discipline's import putting offers to work on the event's staff.
func (e *Event) addStaffFromSignup(slug string, rows []signup.Row) int {
	members, taken := staffing.FromSignup(e.staff.Members, slug, rows, func(name, club, sub string) string {
		return e.personFor(name, club, sub, "")
	})
	e.staff.Members = members
	return taken
}

// removeStaffFrom is a discipline's Remove: off that discipline, and off its work.
func (e *Event) removeStaffFrom(slug, id string) bool {
	members, changed := staffing.RemoveFrom(e.staff.Members, slug, id, e.slugs())
	if !changed {
		return false
	}
	e.staff.Members = members
	var kept []store.Assignment
	for _, a := range e.staff.Assignments {
		if !(a.Staff == id && strings.HasPrefix(a.Item, slug+"/")) {
			kept = append(kept, a)
		}
	}
	e.staff.Assignments = kept
	return true
}

// staffRequest answers the staff paths, or reports it has none.
func (e *Event) staffRequest(method, bare string, parts []string, body []byte) (Response, bool) {
	switch {
	case method == "GET" && bare == "/api/staff":
		return e.staffView(), true
	case method == "POST" && bare == "/api/staff":
		var in struct {
			store.StaffMember
			Want string `json:"want"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		m, err := staffing.Clean(in.StaffMember)
		if err != nil {
			return fail(400, err), true
		}
		m.ID, m.Signup = staffing.NewID(), ""
		m.Person = e.personFor(m.Name, m.Club, "", in.Want)
		e.staff.Members = append(e.staff.Members, m)
		res := e.staffView()
		res.Status, res.Changed = 201, true
		return res, true
	case method == "PUT" && bare == "/api/staff/crew":
		var in map[string]int
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		crew := staffing.CrewOf(e.staff.Crew)
		for role, n := range in {
			if _, ok := crew[role]; !ok || n < 0 || n > 6 {
				return fail(400, fmt.Errorf("a mat needs between 0 and 6 of %q", role)), true
			}
			crew[role] = n
		}
		e.staff.Crew = crew
		return changedView(e.staffView()), true
	case method == "PUT" && bare == "/api/staff/assignments":
		var in struct {
			Item  string `json:"item"`
			Role  string `json:"role"`
			Slot  int    `json:"slot"`
			Staff string `json:"staff"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		next, err := httpapi.SetAssignment(e.staff, in.Item, in.Role, in.Slot, in.Staff)
		if err != nil {
			return fail(400, err), true
		}
		e.staff = next
		return changedView(e.staffView()), true
	case method == "POST" && bare == "/api/staff/suggest":
		_, s := e.staffing()
		return ok(s), true
	case method == "POST" && bare == "/api/staff/apply":
		var in struct {
			Signature string `json:"signature"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		_, s := e.staffing()
		if s.Signature != in.Signature {
			b, _ := json.Marshal(map[string]any{
				"error":      "the staff or the plan changed since that suggestion; here is a new one",
				"suggestion": s,
			})
			return Response{Status: 409, ContentType: "application/json; charset=utf-8", Body: string(b)}, true
		}
		e.staff.Assignments = s.Assignments
		return changedView(e.staffView()), true
	case len(parts) == 3 && parts[1] == "staff" && method == "PATCH":
		var in struct {
			Name        *string   `json:"name"`
			Club        *string   `json:"club"`
			Roles       *[]string `json:"roles"`
			Disciplines *[]string `json:"disciplines"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		for i, m := range e.staff.Members {
			if m.ID != parts[2] {
				continue
			}
			if in.Name != nil {
				m.Name = *in.Name
			}
			if in.Club != nil {
				m.Club = *in.Club
			}
			if in.Roles != nil {
				m.Roles = *in.Roles
			}
			if in.Disciplines != nil {
				m.Disciplines = *in.Disciplines
			}
			clean, err := staffing.Clean(m)
			if err != nil {
				return fail(400, err), true
			}
			e.staff.Members[i] = clean
			return changedView(e.staffView()), true
		}
		return fail(404, fmt.Errorf("no staff member %s", parts[2])), true
	case len(parts) == 3 && parts[1] == "staff" && method == "DELETE":
		next, found := staffing.Without(e.staff, parts[2])
		if !found {
			return fail(404, errors.New("no staff member "+parts[2])), true
		}
		e.staff = next
		return changedView(e.staffView()), true
	}
	return Response{}, false
}

func changedView(r Response) Response {
	r.Changed = true
	return r
}

// withDuties adds a person's work as staff to their page.
func (e *Event) withDuties(v httpapi.PersonView) httpapi.PersonView {
	sv, _ := e.staffing()
	v.Duties, v.Physician = httpapi.DutiesOf(v.ID, e.staff, sv)
	if len(v.Entries) == 0 {
		for _, m := range e.staff.Members {
			if m.Person == v.ID {
				v.Name, v.Club = m.Name, m.Club
			}
		}
	}
	return v
}
