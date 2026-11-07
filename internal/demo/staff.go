package demo

import (
	"encoding/json"
	"errors"

	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The demo's staff (phase 5): staff.json in memory, with the coordinator's own decisions --
// staffing.Assign and Check, hall.StaffInput, ViewStaffing, SuggestStaff, DutiesOf.

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
			Person: e.personFor(s.name, s.club, "", "", want),
		})
	}
}

func (e *Event) staffing() (hall.StaffingView, hall.StaffSuggestion) {
	in, r, inputs := e.times()
	si := hall.StaffInput(in, r, e.staff, hall.StillIn(inputs))
	return hall.ViewStaffing(si, e.staff, e.mats()), hall.SuggestStaff(si)
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
	members, taken := staffing.FromSignup(e.staff.Members, slug, rows, func(name, club, sub, email string) string {
		return e.personFor(name, club, sub, email, "")
	})
	e.staff.Members = members
	return taken
}

// removeStaffFrom is a discipline's Remove: off that discipline, and off its work.
func (e *Event) removeStaffFrom(slug, id string) bool {
	next, changed := hall.RemoveFromDiscipline(e.staff, slug, id, e.slugs())
	if changed {
		e.staff = next
	}
	return changed
}

// withDuties adds a person's work as staff to their page.
func (e *Event) withDuties(v hall.PersonView) hall.PersonView {
	sv, _ := e.staffing()
	return hall.WithDuties(v, e.staff, sv)
}

// staffRoutes are the staff's paths.
func (e *Event) staffRoutes() []route {
	return []route{
		on("GET /api/staff", func(call) Response { return e.staffView() }),
		on("POST /api/staff", func(c call) Response {
			var in struct {
				store.StaffMember
				Want string `json:"want"`
			}
			if err := json.Unmarshal(c.body, &in); err != nil {
				return fail(400, err)
			}
			m, err := hall.NewMember(in.StaffMember)
			if err != nil {
				return fail(400, err)
			}
			m.Person = e.personFor(m.Name, m.Club, "", "", in.Want)
			e.staff.Members = append(e.staff.Members, m)
			res := e.staffView()
			res.Status, res.Changed = 201, true
			return res
		}),
		on("PUT /api/staff/crew", e.changeStaff(func(c call) error {
			var in map[string]int
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			return hall.SetCrew(&e.staff, in)
		})),
		on("PUT /api/staff/assignments", e.changeStaff(func(c call) error {
			var in struct {
				Item  string `json:"item"`
				Role  string `json:"role"`
				Slot  int    `json:"slot"`
				Staff string `json:"staff"`
			}
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			next, err := hall.SetAssignment(e.staff, in.Item, in.Role, in.Slot, in.Staff)
			if err != nil {
				return err
			}
			e.staff = next
			return nil
		})),
		on("POST /api/staff/suggest", func(call) Response {
			_, s := e.staffing()
			return ok(s)
		}),
		on("POST /api/staff/apply", e.applyStaff),
		on("PATCH /api/staff/{id}", e.changeStaff(func(c call) error {
			var in hall.MemberPatch
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			return hall.PatchMember(&e.staff, c.param("id"), in)
		})),
		on("DELETE /api/staff/{id}", e.changeStaff(func(c call) error {
			next, found := staffing.Without(e.staff, c.param("id"))
			if !found {
				return hall.ErrNoMember(c.param("id"))
			}
			e.staff = next
			return nil
		})),
	}
}

// changeStaff is a change to the staff that answers with the staff panel: a 404 for a member
// the staff does not have, a 400 for anything else wrong.
func (e *Event) changeStaff(change func(call) error) func(call) Response {
	return func(c call) Response {
		err := change(c)
		var missing hall.ErrNoMember
		switch {
		case errors.As(err, &missing):
			return fail(404, err)
		case err != nil:
			return fail(400, err)
		}
		res := e.staffView()
		res.Changed = true
		return res
	}
}

func (e *Event) applyStaff(c call) Response {
	var in struct {
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(c.body, &in); err != nil {
		return fail(400, err)
	}
	_, s := e.staffing()
	if s.Signature != in.Signature {
		return conflict(map[string]any{
			"error":      "the staff or the plan changed since that suggestion; here is a new one",
			"suggestion": s,
		})
	}
	e.staff.Assignments = s.Assignments
	res := e.staffView()
	res.Changed = true
	return res
}
