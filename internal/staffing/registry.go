// Package staffing is the event's staff (docs/proposals/one-event-many-disciplines.md,
// phase 5; #5): who has offered to work, in which roles and disciplines, and who works
// which mat when.
//
// Staff belong to the event, because one person cannot referee two mats at once and must
// never be put on a mat while they are fencing. A member is linked to a person of the
// event (internal/people), which is how their fencing is known: a signup response that
// enters Longsword and offers to referee Sabre is one person in both.
//
// Everything here is a pure function over the staff, so the coordinator and the browser
// demo decide the same way.
package staffing

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The roles, as internal/signup names them.
const (
	HeadRef      = "head-ref"
	AssistantRef = "assistant-ref"
	ScoreKeeper  = "score-keeper"
	Physician    = "physician"
)

// MatRoles are the roles a mat needs while it runs, in the order they are filled: the
// scarcest first. A physician is on call for the hall, not on a mat.
var MatRoles = []string{HeadRef, AssistantRef, ScoreKeeper}

// DefaultCrew is a mat's crew under the SM rules: a head referee, two assistants and a
// score keeper.
var DefaultCrew = map[string]int{HeadRef: 1, AssistantRef: 2, ScoreKeeper: 1}

// CrewOf is the crew in force: the organizer's numbers over the defaults.
func CrewOf(c map[string]int) map[string]int {
	out := map[string]int{}
	for _, r := range MatRoles {
		out[r] = DefaultCrew[r]
		if n, ok := c[r]; ok && n >= 0 {
			out[r] = n
		}
	}
	return out
}

// NewID is a fresh member id, random so members lifted from several disciplines -- whose
// own ids were all s1, s2, ... -- never collide.
func NewID() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		panic("staffing: no randomness: " + err.Error())
	}
	return "st-" + hex.EncodeToString(b)
}

// Works says a member will work a discipline.
func Works(m store.StaffMember, slug string) bool {
	if len(m.Disciplines) == 0 {
		return true
	}
	for _, d := range m.Disciplines {
		if d == slug {
			return true
		}
	}
	return false
}

// Has says a member offered a role.
func Has(m store.StaffMember, role string) bool {
	for _, r := range m.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// For is the members who will work a discipline, as its own pages list its staff.
func For(members []store.StaffMember, slug string) []store.StaffMember {
	out := []store.StaffMember{}
	for _, m := range members {
		if Works(m, slug) {
			out = append(out, m)
		}
	}
	return out
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// FromSignup takes the rows a discipline's import calls staff. A response already among
// the members -- it offered another discipline too -- adds this discipline and its roles
// to that member; anybody else is a new member. Never matched by name. person is who a
// new member is (people.For). Returns the members and how many offers were taken.
func FromSignup(members []store.StaffMember, slug string, rows []signup.Row, person func(name, club, submission string) string) ([]store.StaffMember, int) {
	out := append([]store.StaffMember{}, members...)
	taken := 0
	for _, row := range rows {
		if row.Verdict != signup.Staff {
			continue
		}
		found := -1
		for i, m := range out {
			if row.SubmissionID != "" && m.Signup == row.SubmissionID {
				found = i
			}
		}
		if found >= 0 {
			m := out[found]
			if Works(m, slug) {
				continue // already works it, or works any discipline
			}
			m.Disciplines = union(m.Disciplines, []string{slug})
			m.Roles = union(m.Roles, row.Roles)
			out[found] = m
			taken++
			continue
		}
		out = append(out, store.StaffMember{
			ID: NewID(), Name: row.Name, Club: row.Club, Roles: append([]string{}, row.Roles...),
			Signup: row.SubmissionID, Disciplines: []string{slug},
			Person: person(row.Name, row.Club, row.SubmissionID),
		})
		taken++
	}
	return out, taken
}

// Lift brings each discipline's own staff list up to the event, the first time the event
// opens with staff of its own: what every run kept in its tournament before staff were the
// event's. The same submission in two disciplines is one member working both.
func Lift(byDiscipline map[string][]store.StaffMember, person func(name, club, submission string) string) []store.StaffMember {
	var slugs []string
	for slug := range byDiscipline {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	out := []store.StaffMember{}
	for _, slug := range slugs {
		for _, s := range byDiscipline[slug] {
			merged := false
			for i, m := range out {
				if s.Signup != "" && m.Signup == s.Signup {
					out[i].Disciplines = union(m.Disciplines, []string{slug})
					out[i].Roles = union(m.Roles, s.Roles)
					merged = true
				}
			}
			if merged {
				continue
			}
			out = append(out, store.StaffMember{
				ID: NewID(), Name: s.Name, Club: s.Club, Roles: append([]string{}, s.Roles...),
				Signup: s.Signup, Disciplines: []string{slug}, Person: person(s.Name, s.Club, s.Signup),
			})
		}
	}
	return out
}

// RemoveFrom takes a member off one discipline: what a discipline's own Remove means. A
// member who worked only that discipline is off the staff; one who worked any discipline
// keeps the others. all is every discipline of the event. Reports whether anything changed.
func RemoveFrom(members []store.StaffMember, slug, id string, all []string) ([]store.StaffMember, bool) {
	out := []store.StaffMember{}
	changed := false
	for _, m := range members {
		if m.ID != id || !Works(m, slug) {
			out = append(out, m)
			continue
		}
		changed = true
		rest := m.Disciplines
		if len(rest) == 0 {
			rest = all
		}
		var keep []string
		for _, d := range rest {
			if d != slug {
				keep = append(keep, d)
			}
		}
		if len(keep) == 0 {
			continue
		}
		m.Disciplines = keep
		out = append(out, m)
	}
	return out, changed
}

// Clean checks a member the organizer typed in.
func Clean(m store.StaffMember) (store.StaffMember, error) {
	m.Name, m.Club = strings.TrimSpace(m.Name), strings.TrimSpace(m.Club)
	if m.Name == "" {
		return m, errMissingName
	}
	var roles []string
	for _, r := range m.Roles {
		for _, known := range signup.Roles {
			if r == known.ID {
				roles = union(roles, []string{r})
			}
		}
	}
	m.Roles = roles
	if m.Roles == nil {
		m.Roles = []string{}
	}
	return m, nil
}

type staffError string

func (e staffError) Error() string { return string(e) }

const errMissingName = staffError("a staff member needs a name")

// Without is the members less one, and the assignments less theirs.
func Without(s store.Staff, id string) (store.Staff, bool) {
	out := s
	out.Members = []store.StaffMember{}
	found := false
	for _, m := range s.Members {
		if m.ID == id {
			found = true
			continue
		}
		out.Members = append(out.Members, m)
	}
	out.Assignments = nil
	for _, a := range s.Assignments {
		if a.Staff != id {
			out.Assignments = append(out.Assignments, a)
		}
	}
	return out, found
}
