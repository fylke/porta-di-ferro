package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The event's staff (proposal phase 5; #5), in staff.json. Locks: a discipline's import
// holds its own lock and asks for staff, and staff ask the registry of people for who a
// member is, so the order is discipline, then staff, then people -- never the other way.

// StaffFor is the members who work a discipline (EventStaff).
func (c *Coordinator) StaffFor(slug string) ([]store.StaffMember, error) {
	st, err := c.folder.Staff()
	if err != nil {
		return nil, err
	}
	return staffing.For(st.Members, slug), nil
}

// AddStaff takes an import's offers to work (EventStaff).
func (c *Coordinator) AddStaff(slug string, rows []signup.Row) (int, error) {
	c.staffMu.Lock()
	defer c.staffMu.Unlock()
	st, err := c.folder.Staff()
	if err != nil {
		return 0, err
	}
	var perr error
	members, taken := staffing.FromSignup(st.Members, slug, rows, func(name, club, sub string) string {
		id, err := c.PersonFor(name, club, sub, "")
		if err != nil {
			perr = err
		}
		return id
	})
	if perr != nil || taken == 0 {
		return 0, perr
	}
	st.Members = members
	if err := c.folder.SaveStaff(st); err != nil {
		return 0, err
	}
	c.poke()
	return taken, nil
}

// RemoveStaff takes a member off a discipline, and off that discipline's work (EventStaff).
func (c *Coordinator) RemoveStaff(slug, id string) (bool, error) {
	c.staffMu.Lock()
	defer c.staffMu.Unlock()
	st, err := c.folder.Staff()
	if err != nil {
		return false, err
	}
	members, changed := staffing.RemoveFrom(st.Members, slug, id, c.slugs())
	if !changed {
		return false, nil
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
	if err := c.folder.SaveStaff(st); err != nil {
		return false, err
	}
	c.poke()
	return true, nil
}

func (c *Coordinator) slugs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.order...)
}

// liftStaff brings every discipline's own staff list up to the event the first time it
// opens with staff of its own (phase 5), and gives every member a person.
func (c *Coordinator) liftStaff() {
	c.staffMu.Lock()
	defer c.staffMu.Unlock()
	person := func(name, club, sub string) string {
		id, _ := c.PersonFor(name, club, sub, "")
		return id
	}
	st, err := c.folder.Staff()
	if err != nil {
		return
	}
	changed := false
	if !c.folder.HasStaff() {
		byDiscipline := map[string][]store.StaffMember{}
		for _, w := range c.snapshotWorkers() {
			if w.srv == nil {
				continue
			}
			if t, err := w.srv.store.Tournament(); err == nil && len(t.Staff) > 0 {
				byDiscipline[w.slug] = t.Staff
			}
		}
		st.Members = staffing.Lift(byDiscipline, person)
		changed = true
	}
	for i, m := range st.Members {
		if m.Person == "" {
			st.Members[i].Person = person(m.Name, m.Club, m.Signup)
			changed = true
		}
	}
	if changed {
		_ = c.folder.SaveStaff(st)
	}
}

// --- the staff, over HTTP ---------------------------------------------------------------

// StaffView is the event's staff as the staff panel reads it.
type StaffView struct {
	Members     []store.StaffMember `json:"members"`
	Crew        map[string]int      `json:"crew"`
	Assignments []store.Assignment  `json:"assignments"`
}

func viewStaff(st store.Staff) StaffView {
	v := StaffView{Members: st.Members, Crew: staffing.CrewOf(st.Crew), Assignments: st.Assignments}
	if v.Members == nil {
		v.Members = []store.StaffMember{}
	}
	if v.Assignments == nil {
		v.Assignments = []store.Assignment{}
	}
	return v
}

// changeStaff makes a change to the staff under its lock and tells every page.
func (c *Coordinator) changeStaff(change func(*store.Staff) error) (store.Staff, error) {
	c.staffMu.Lock()
	st, err := c.folder.Staff()
	if err == nil {
		err = change(&st)
	}
	if err == nil {
		err = c.folder.SaveStaff(st)
	}
	c.staffMu.Unlock()
	if err == nil {
		// Every discipline's snapshot lists its staff.
		c.republish()
	}
	return st, err
}

func (c *Coordinator) getStaff(w http.ResponseWriter, r *http.Request) {
	st, err := c.folder.Staff()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, viewStaff(st))
}

// memberIn is a member as the panel sends it, with the person the desk picked.
type memberIn struct {
	store.StaffMember
	Want string `json:"want"`
}

// addStaff is somebody added by hand: #5's "add staff on the organizer page".
func (c *Coordinator) addStaff(w http.ResponseWriter, r *http.Request) {
	var in memberIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	m, err := staffing.Clean(in.StaffMember)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	m.ID, m.Signup = staffing.NewID(), ""
	if m.Person, err = c.PersonFor(m.Name, m.Club, "", in.Want); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	st, err := c.changeStaff(func(st *store.Staff) error {
		st.Members = append(st.Members, m)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewStaff(st))
}

// patchStaff changes a member's name, club, roles or disciplines.
func (c *Coordinator) patchStaff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Name        *string   `json:"name"`
		Club        *string   `json:"club"`
		Roles       *[]string `json:"roles"`
		Disciplines *[]string `json:"disciplines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	st, err := c.changeStaff(func(st *store.Staff) error {
		for i, m := range st.Members {
			if m.ID != id {
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
				return err
			}
			st.Members[i] = clean
			return nil
		}
		return errNoMember(id)
	})
	var missing errNoMember
	switch {
	case errors.As(err, &missing):
		writeErr(w, http.StatusNotFound, err)
	case err != nil:
		writeErr(w, http.StatusBadRequest, err)
	default:
		writeJSON(w, http.StatusOK, viewStaff(st))
	}
}

type errNoMember string

func (e errNoMember) Error() string { return fmt.Sprintf("no staff member %s", string(e)) }

func (c *Coordinator) deleteMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := c.changeStaff(func(st *store.Staff) error {
		next, found := staffing.Without(*st, id)
		if !found {
			return errNoMember(id)
		}
		*st = next
		return nil
	})
	var missing errNoMember
	switch {
	case errors.As(err, &missing):
		writeErr(w, http.StatusNotFound, err)
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err)
	default:
		writeJSON(w, http.StatusOK, viewStaff(st))
	}
}
