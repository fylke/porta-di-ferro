package httpapi

import (
	"fmt"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Staff on the plan (proposal phase 5; #5), as the staff panel and a person's page see
// it. staffing decides; this builds its input from the forecast -- each item timed, and
// who fences in it -- and turns its answer into what the pages read. Shared with the demo.

// StaffInput is what an assignment needs: every item the forecast times, with the people
// fencing in it, and the staff.
func StaffInput(in forecast.Input, r forecast.Result, st store.Staff) staffing.Input {
	spans := map[string]forecast.Span{}
	for _, s := range r.Items {
		spans[s.ID] = s
	}
	out := staffing.Input{Members: st.Members, Crew: st.Crew, Assignments: st.Assignments}
	for _, it := range in.Items {
		s := spans[it.ID]
		started := false
		for _, m := range it.Matches {
			started = started || m.Done || !m.Started.IsZero()
		}
		out.Items = append(out.Items, staffing.Item{ID: it.ID, Discipline: it.Discipline, Mat: s.Mat,
			Start: s.Start, End: s.End, Started: started, People: it.People})
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
}

// ViewStaffing is the staff as they stand, checked against the plan.
func ViewStaffing(si staffing.Input, st store.Staff, mats MatsView) StaffingView {
	res := staffing.Check(si)
	v := StaffingView{Members: st.Members, Crew: staffing.CrewOf(st.Crew), Assignments: st.Assignments,
		Items: []StaffItem{}, Short: []ShortView{}, Warnings: []StaffWarning{}, Physicians: []string{}}
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
