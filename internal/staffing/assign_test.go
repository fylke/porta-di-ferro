package staffing_test

import (
	"testing"
	"time"

	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

var nine = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

func at(min int) time.Time { return nine.Add(time.Duration(min) * time.Minute) }

func item(id, disc string, mat, from, to int, people ...string) staffing.Item {
	return staffing.Item{ID: id, Discipline: disc, Mat: mat, Start: at(from), End: at(to), People: people}
}

func member(id, person string, roles ...string) store.StaffMember {
	return store.StaffMember{ID: id, Name: id, Person: person, Roles: roles}
}

// One head referee per mat, nothing else, to keep the tests readable.
var heads = map[string]int{"head-ref": 1, "assistant-ref": 0, "score-keeper": 0}

func who(r staffing.Result, item, role string) []string {
	var out []string
	for _, a := range r.Assignments {
		if a.Item == item && a.Role == role {
			out = append(out, a.Staff)
		}
	}
	return out
}

// #5: the same role on the same mat for contiguous blocks, not hopping about.
func TestTheSameRefereeStaysOnTheMat(t *testing.T) {
	r := staffing.Assign(staffing.Input{Crew: heads,
		Members: []store.StaffMember{member("ann", "p1", "head-ref"), member("bo", "p2", "head-ref")},
		Items: []staffing.Item{
			item("ls/pool-1", "ls", 1, 0, 60), item("ls/pool-2", "ls", 2, 0, 60),
			item("ls/pool-3", "ls", 1, 61, 120), item("ls/pool-4", "ls", 2, 61, 120),
		}})
	if a, b := who(r, "ls/pool-1", "head-ref"), who(r, "ls/pool-3", "head-ref"); len(a) != 1 || len(b) != 1 || a[0] != b[0] {
		t.Errorf("mat 1 should keep its head referee: %v then %v", a, b)
	}
	if a, b := who(r, "ls/pool-2", "head-ref"), who(r, "ls/pool-4", "head-ref"); len(a) != 1 || len(b) != 1 || a[0] != b[0] {
		t.Errorf("mat 2 should keep its head referee: %v then %v", a, b)
	}
	if len(r.Short) != 0 || len(r.Warnings) != 0 {
		t.Errorf("two referees for two mats is enough: %+v %+v", r.Short, r.Warnings)
	}
}

// Never on a mat while fencing, in any discipline; never on two mats at once.
func TestNeverWhileFencingOrOnAnotherMat(t *testing.T) {
	r := staffing.Assign(staffing.Input{Crew: heads,
		Members: []store.StaffMember{member("ann", "p-ann", "head-ref"), member("bo", "p-bo", "head-ref")},
		Items: []staffing.Item{
			item("ls/pool-1", "ls", 1, 0, 60, "p-ann"),
			item("sa/pool-1", "sa", 2, 30, 90),
			item("sa/pool-2", "sa", 3, 30, 90),
		}})
	for _, id := range []string{"sa/pool-1", "sa/pool-2"} {
		for _, s := range who(r, id, "head-ref") {
			if s == "ann" {
				t.Errorf("Ann fences longsword until 10:00 and must not referee %s from 09:30", id)
			}
		}
	}
	if len(r.Short) != 2 {
		t.Errorf("only Bo is free from 09:30 and he cannot be on two mats; longsword's own pool has nobody but Bo either: %+v", r.Short)
	}
}

func TestOnlyOfferedRolesAndDisciplines(t *testing.T) {
	sabreOnly := member("cia", "p3", "head-ref")
	sabreOnly.Disciplines = []string{"sa"}
	r := staffing.Assign(staffing.Input{Crew: map[string]int{"head-ref": 1, "assistant-ref": 1, "score-keeper": 0},
		Members: []store.StaffMember{member("dag", "p4", "score-keeper"), sabreOnly},
		Items:   []staffing.Item{item("ls/pool-1", "ls", 1, 0, 60)}})
	if len(r.Assignments) != 0 || len(r.Short) != 2 {
		t.Errorf("a score keeper cannot referee, and Cia works sabre only: %+v %+v", r.Assignments, r.Short)
	}
}

// The organizer's choices stand, and are reported when they break a rule; an item under
// way keeps its crew.
func TestTheOrganizersChoicesStand(t *testing.T) {
	r := staffing.Assign(staffing.Input{Crew: heads,
		Members: []store.StaffMember{member("ann", "p-ann", "head-ref"), member("bo", "p-bo", "head-ref")},
		Items: []staffing.Item{
			item("ls/pool-1", "ls", 1, 0, 60, "p-ann"),
			item("sa/pool-1", "sa", 2, 0, 60),
			{ID: "sa/pool-2", Discipline: "sa", Mat: 3, Start: at(0), End: at(60), Started: true},
		},
		Assignments: []store.Assignment{
			{Item: "sa/pool-1", Role: "head-ref", Slot: 1, Staff: "ann", Pinned: true},
			{Item: "sa/pool-2", Role: "head-ref", Slot: 1, Staff: "bo"},
		}})
	if got := who(r, "sa/pool-1", "head-ref"); len(got) != 1 || got[0] != "ann" {
		t.Errorf("the pinned choice stays: %v", got)
	}
	if got := who(r, "sa/pool-2", "head-ref"); len(got) != 1 || got[0] != "bo" {
		t.Errorf("an item under way keeps its crew: %v", got)
	}
	found := false
	for _, w := range r.Warnings {
		found = found || (w.Kind == "fencing" && w.Staff == "ann" && w.Other == "ls/pool-1")
	}
	if !found {
		t.Errorf("Ann is pinned to referee while she fences; that should be reported: %+v", r.Warnings)
	}
}

func TestPhysiciansAreOnCall(t *testing.T) {
	got := staffing.Physicians([]store.StaffMember{member("eva", "p", "physician"), member("dag", "q", "head-ref")})
	if len(got) != 1 || got[0].ID != "eva" {
		t.Errorf("only Eva is a physician: %+v", got)
	}
}
