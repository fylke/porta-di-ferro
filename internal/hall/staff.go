package hall

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Staff on the plan (proposal phase 5; #5), as the staff panel and a person's page see
// it. staffing decides; this builds its input from the forecast -- each item timed, and
// who fences in it -- and turns its answer into what the pages read. Shared with the demo.

// StillIn is who is still competing in each discipline, by slug: every entry until the
// bracket is drawn, since any of them may go through, and after that whoever has a match
// to come.
func StillIn(inputs []MatsInput) map[string][]string {
	out := map[string][]string{}
	for _, d := range inputs {
		person := func(c store.Competitor) string {
			if c.Person != "" {
				return c.Person
			}
			return d.Slug + "/" + c.ID
		}
		drawn := d.Snapshot.Bracket != nil && len(d.Snapshot.Bracket.Matches) > 0
		left := map[string]bool{}
		if drawn {
			for _, m := range d.Snapshot.Bracket.Matches {
				if m.Status != "complete" {
					left[m.Red], left[m.Blue] = true, true
				}
			}
		}
		for _, c := range d.Snapshot.Competitors {
			if !c.Withdrawn && (!drawn || left[c.ID]) {
				out[d.Slug] = append(out[d.Slug], person(c))
			}
		}
	}
	return out
}

// StaffInput is what an assignment needs: every item the forecast times, with the people
// fencing in it, who is still competing where, and the staff.
func StaffInput(in forecast.Input, r forecast.Result, st store.Staff, competing map[string][]string) staffing.Input {
	spans := map[string]forecast.Span{}
	for _, s := range r.Items {
		spans[s.ID] = s
	}
	out := staffing.Input{Members: st.Members, Crew: st.Crew, Assignments: st.Assignments, Competing: competing}
	for _, it := range in.Items {
		s := spans[it.ID]
		out.Items = append(out.Items, staffing.Item{ID: it.ID, Discipline: it.Discipline, Mat: s.Mat,
			Start: s.Start, End: s.End, Started: it.Begun(), People: it.People, Held: it.Held})
	}
	return out
}

// StaffItem is one work item as the staff panel lists it: what, where and when.
type StaffItem struct {
	ID             string `json:"id"`
	Discipline     string `json:"discipline"`
	DisciplineName string `json:"disciplineName"`
	Kind           string `json:"kind"`
	Number         int    `json:"number,omitempty"`
	Mat            int    `json:"mat"`
	Start          string `json:"start"`
	End            string `json:"end"`
	Started        bool   `json:"started,omitempty"`
	Projected      bool   `json:"projected,omitempty"`
}

// ShortView is a slot nobody fills.
type ShortView struct {
	Item string `json:"item"`
	Role string `json:"role"`
	Slot int    `json:"slot"`
}

// StaffWarning is an assignment that breaks a rule.
type StaffWarning struct {
	Kind  string `json:"kind"`
	Item  string `json:"item"`
	Role  string `json:"role"`
	Staff string `json:"staff"`
	Other string `json:"other,omitempty"`
}

// StaffingView is the event's staff, the crew each mat needs, who works what, and what is
// missing or wrong: the staff panel.
type StaffingView struct {
	Members     []store.StaffMember `json:"members"`
	Crew        map[string]int      `json:"crew"`
	Assignments []store.Assignment  `json:"assignments"`
	Items       []StaffItem         `json:"items"`
	Short       []ShortView         `json:"short"`
	Warnings    []StaffWarning      `json:"warnings"`
	Physicians  []string            `json:"physicians"`
	// Needed is the fewest people who could staff the plan: a crew on every mat running
	// at the busiest moment (#149).
	Needed int `json:"needed"`
	// Unsure are the members, by id, whose person may be somebody else in the event too
	// (#133): two records of one human, one fencing and one on the staff, and a
	// suggestion could put them on a mat while they fence. The People panel settles it.
	Unsure []string `json:"unsure"`
}

// UnsureOf is the members whose person is in one of the groups of possible duplicates.
func UnsureOf(members []store.StaffMember, duplicates [][]string) []string {
	maybe := map[string]bool{}
	for _, g := range duplicates {
		for _, id := range g {
			maybe[id] = true
		}
	}
	out := []string{}
	for _, m := range members {
		if m.Person != "" && maybe[m.Person] {
			out = append(out, m.ID)
		}
	}
	return out
}

// ViewStaffing is the staff as they stand, checked against the plan.
func ViewStaffing(si staffing.Input, st store.Staff, mats MatsView) StaffingView {
	res := staffing.Check(si)
	v := StaffingView{Members: st.Members, Crew: staffing.CrewOf(st.Crew), Assignments: st.Assignments,
		Items: []StaffItem{}, Short: []ShortView{}, Warnings: []StaffWarning{}, Physicians: []string{}, Unsure: []string{}}
	if v.Members == nil {
		v.Members = []store.StaffMember{}
	}
	if v.Assignments == nil {
		v.Assignments = []store.Assignment{}
	}
	about := map[string]ItemView{}
	for _, it := range mats.Items {
		about[it.ID] = it
	}
	for _, it := range si.Items {
		if it.Start.IsZero() {
			continue
		}
		a := about[it.ID]
		v.Items = append(v.Items, StaffItem{ID: it.ID, Discipline: it.Discipline, DisciplineName: a.DisciplineName,
			Kind: a.Kind, Number: a.Number, Mat: it.Mat, Start: stamp(it.Start), End: stamp(it.End),
			Started: it.Started, Projected: a.Projected})
	}
	sort.SliceStable(v.Items, func(i, j int) bool {
		if v.Items[i].Start != v.Items[j].Start {
			return v.Items[i].Start < v.Items[j].Start
		}
		return v.Items[i].Mat < v.Items[j].Mat
	})
	for _, s := range res.Short {
		v.Short = append(v.Short, ShortView{Item: s.Item, Role: s.Role, Slot: s.Slot})
	}
	for _, w := range res.Warnings {
		v.Warnings = append(v.Warnings, StaffWarning{Kind: w.Kind, Item: w.Item, Role: w.Role, Staff: w.Staff, Other: w.Other})
	}
	for _, m := range staffing.Physicians(st.Members) {
		v.Physicians = append(v.Physicians, m.ID)
	}
	v.Needed = staffing.Needed(si.Items, si.Crew)
	return v
}

// StaffSuggestion is a proposed set of assignments: how many slots it fills or changes,
// what would still be missing, and the signature applying it must match.
type StaffSuggestion struct {
	Assignments []store.Assignment `json:"assignments"`
	Changes     int                `json:"changes"`
	Short       []ShortView        `json:"short"`
	Signature   string             `json:"signature"`
}

// SuggestStaff proposes assignments for every slot, keeping the organizer's choices.
func SuggestStaff(si staffing.Input) StaffSuggestion {
	res := staffing.Assign(si)
	out := StaffSuggestion{Assignments: res.Assignments, Short: []ShortView{}, Signature: staffing.Signature(res.Assignments)}
	if out.Assignments == nil {
		out.Assignments = []store.Assignment{}
	}
	now := map[string]string{}
	for _, a := range si.Assignments {
		now[fmt.Sprintf("%s|%s|%d", a.Item, a.Role, a.Slot)] = a.Staff
	}
	for _, a := range res.Assignments {
		if now[fmt.Sprintf("%s|%s|%d", a.Item, a.Role, a.Slot)] != a.Staff {
			out.Changes++
		}
	}
	for _, s := range res.Short {
		out.Short = append(out.Short, ShortView{Item: s.Item, Role: s.Role, Slot: s.Slot})
	}
	return out
}

// SetAssignment puts a member in a slot by hand, pinned, or empties it with staff "".
func SetAssignment(st store.Staff, item, role string, slot int, staff string) (store.Staff, error) {
	if slot < 1 {
		return st, fmt.Errorf("slots count from 1")
	}
	known := staff == ""
	for _, m := range st.Members {
		known = known || m.ID == staff
	}
	if !known {
		return st, fmt.Errorf("no staff member %s", staff)
	}
	var out []store.Assignment
	for _, a := range st.Assignments {
		if a.Item == item && a.Role == role && a.Slot == slot {
			continue
		}
		out = append(out, a)
	}
	if staff != "" {
		out = append(out, store.Assignment{Item: item, Role: role, Slot: slot, Staff: staff, Pinned: true})
	}
	st.Assignments = out
	return st, nil
}

// Duty is one stretch of work for a member: what, where, in which role, and when.
type Duty struct {
	Item           string `json:"item"`
	Discipline     string `json:"discipline"`
	DisciplineName string `json:"disciplineName"`
	Kind           string `json:"kind"`
	Number         int    `json:"number,omitempty"`
	Mat            int    `json:"mat"`
	Role           string `json:"role"`
	Start          string `json:"start"`
	End            string `json:"end"`
}

// DutiesOf is a person's work as staff, in the order it comes: what their page shows next
// to their fencing (proposal §8).
func DutiesOf(person string, st store.Staff, view StaffingView) ([]Duty, bool) {
	mine := map[string]bool{}
	physician := false
	for _, m := range st.Members {
		if person != "" && m.Person == person {
			mine[m.ID] = true
			physician = physician || staffing.Has(m, staffing.Physician)
		}
	}
	items := map[string]StaffItem{}
	for _, it := range view.Items {
		items[it.ID] = it
	}
	out := []Duty{}
	for _, a := range st.Assignments {
		it, ok := items[a.Item]
		if !mine[a.Staff] || !ok {
			continue
		}
		out = append(out, Duty{Item: it.ID, Discipline: it.Discipline, DisciplineName: it.DisciplineName, Kind: it.Kind,
			Number: it.Number, Mat: it.Mat, Role: a.Role, Start: it.Start, End: it.End})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, physician
}

// NewMember is somebody added to the staff by hand, checked, with an id of their own.
func NewMember(in store.StaffMember) (store.StaffMember, error) {
	m, err := staffing.Clean(in)
	if err != nil {
		return m, err
	}
	m.ID, m.Signup = staffing.NewID(), ""
	return m, nil
}

// SetCrew writes how many of each role a mat needs.
func SetCrew(st *store.Staff, in map[string]int) error {
	crew := staffing.CrewOf(st.Crew)
	for role, n := range in {
		if _, ok := crew[role]; !ok || n < 0 || n > 6 {
			return fmt.Errorf("a mat needs between 0 and 6 of %q", role)
		}
		crew[role] = n
	}
	st.Crew = crew
	return nil
}

// MemberPatch is what the staff panel changes about a member: what is left out stays.
type MemberPatch struct {
	Name        *string   `json:"name"`
	Club        *string   `json:"club"`
	Roles       *[]string `json:"roles"`
	Disciplines *[]string `json:"disciplines"`
}

// ErrNoMember is a member the staff does not have.
type ErrNoMember string

func (e ErrNoMember) Error() string { return fmt.Sprintf("no staff member %s", string(e)) }

// PatchMember changes a member's name, club, roles or disciplines.
func PatchMember(st *store.Staff, id string, patch MemberPatch) error {
	for i, m := range st.Members {
		if m.ID != id {
			continue
		}
		if patch.Name != nil {
			m.Name = *patch.Name
		}
		if patch.Club != nil {
			m.Club = *patch.Club
		}
		if patch.Roles != nil {
			m.Roles = *patch.Roles
		}
		if patch.Disciplines != nil {
			m.Disciplines = *patch.Disciplines
		}
		clean, err := staffing.Clean(m)
		if err != nil {
			return err
		}
		st.Members[i] = clean
		return nil
	}
	return ErrNoMember(id)
}

// RemoveFromDiscipline is a discipline's Remove: the member off that discipline, and off its
// work. False when they did not work it.
func RemoveFromDiscipline(st store.Staff, slug, id string, slugs []string) (store.Staff, bool) {
	members, changed := staffing.RemoveFrom(st.Members, slug, id, slugs)
	if !changed {
		return st, false
	}
	st.Members = members
	var kept []store.Assignment
	for _, a := range st.Assignments {
		if a.Staff == id && strings.HasPrefix(a.Item, slug+"/") {
			continue
		}
		kept = append(kept, a)
	}
	st.Assignments = kept
	return st, true
}
