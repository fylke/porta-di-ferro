package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/people"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The event's people (proposal §8, phase 3). The registry is people.json; which entry is
// whom is on each entry, in each discipline's competitors.json. The registry's lock is
// never held while a discipline's is wanted: a signup import holds its discipline's lock
// and asks for a person, so the other order would deadlock. Every change here is decided
// under the registry's lock and written to the entries after.

// PersonFor is who a new entry is (EventPeople, for the disciplines).
func (c *Coordinator) PersonFor(name, club, submission, email, want string) (string, error) {
	c.peopleMu.Lock()
	defer c.peopleMu.Unlock()
	reg, err := c.folder.People()
	if err != nil {
		return "", err
	}
	next, id, changed := people.For(reg, name, club, submission, email, want)
	if changed {
		if err := c.folder.SavePeople(next); err != nil {
			return "", err
		}
	}
	return id, nil
}

// PersonByEmail is who signed up with an email, or "" (EventPeople, for the disciplines).
func (c *Coordinator) PersonByEmail(email string) string {
	c.peopleMu.Lock()
	defer c.peopleMu.Unlock()
	reg, err := c.folder.People()
	if err != nil {
		return ""
	}
	return people.ByEmail(reg, email)
}

// rosters is every readable discipline's entries, and the disciplines' names.
func (c *Coordinator) rosters() ([]people.Roster, map[string]string) {
	var out []people.Roster
	names := map[string]string{}
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			continue
		}
		comps, err := w.srv.Competitors()
		if err != nil {
			continue
		}
		out = append(out, people.Roster{Discipline: w.slug, Competitors: comps})
		names[w.slug] = w.srv.Self().Name
	}
	return out, names
}

func (c *Coordinator) entriesByPerson(reg []store.Person) map[string][]people.Entry {
	rosters, names := c.rosters()
	return people.EntriesOf(reg, rosters, names)
}

// ensurePeople gives every entry a person (people.Ensure).
func (c *Coordinator) ensurePeople() {
	c.peopleMu.Lock()
	reg, err := c.folder.People()
	if err != nil {
		c.peopleMu.Unlock()
		return
	}
	rosters, _ := c.rosters()
	next, links := people.Ensure(reg, rosters)
	if len(next) != len(reg) {
		_ = c.folder.SavePeople(next)
	}
	c.peopleMu.Unlock()
	c.applyLinks(links)
}

// prunePeople drops the people nobody needs any more, when the event opens (#127). Not
// while any discipline cannot be read: its entries may be the ones that need them.
func (c *Coordinator) prunePeople() {
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			return
		}
	}
	keep := map[string]bool{}
	if st, err := c.folder.Staff(); err == nil {
		for _, m := range st.Members {
			keep[m.Person] = true
		}
	} else {
		return
	}
	// A retired discipline is a folder to move back by hand: its entries keep their people.
	retired, _ := filepath.Glob(filepath.Join(c.folder.Dir(), "retired", "*", "competitors.json"))
	for _, name := range retired {
		var comps []store.Competitor
		b, err := os.ReadFile(name)
		if err != nil || json.Unmarshal(b, &comps) != nil {
			return
		}
		for _, comp := range comps {
			keep[comp.Person] = true
		}
	}
	c.peopleMu.Lock()
	defer c.peopleMu.Unlock()
	reg, err := c.folder.People()
	if err != nil {
		return
	}
	rosters, _ := c.rosters()
	if next := people.Prune(reg, rosters, keep); len(next) != len(reg) {
		_ = c.folder.SavePeople(next)
	}
}

// applyLinks points entries at people, discipline by discipline.
func (c *Coordinator) applyLinks(links people.Links) {
	for slug, byComp := range links {
		c.mu.Lock()
		w := c.workers[slug]
		c.mu.Unlock()
		if w == nil || w.srv == nil {
			continue
		}
		_ = w.srv.LinkPeople(func(comp store.Competitor) (string, bool) {
			id, ok := byComp[comp.ID]
			return id, ok
		})
	}
}

// --- the people, as the pages see them ----------------------------------------------------

// PeopleNow is the registry as the people panel shows it.
func (c *Coordinator) PeopleNow() (hall.PeopleView, error) {
	reg, err := c.folder.People()
	if err != nil {
		return hall.PeopleView{}, err
	}
	return hall.ViewPeople(reg, c.entriesByPerson(reg)), nil
}

func (c *Coordinator) getPeople(w http.ResponseWriter, r *http.Request) {
	v, err := c.PeopleNow()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// getPerson is one person's whole day across the event.
func (c *Coordinator) getPerson(w http.ResponseWriter, r *http.Request) {
	reg, err := c.folder.People()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	v, ok := hall.ViewPerson(reg, r.PathValue("id"), c.entriesByPerson(reg))
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("nobody with id %q is in this event", r.PathValue("id")))
		return
	}
	if sv, _, st, err := c.StaffingNow(); err == nil {
		v = hall.WithDuties(v, st, sv)
	}
	writeJSON(w, http.StatusOK, v)
}

// postPerson is DecidePerson over HTTP.
func (c *Coordinator) postPerson(w http.ResponseWriter, r *http.Request) {
	var in hall.PersonAction
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}
	c.peopleMu.Lock()
	reg, err := c.folder.People()
	if err != nil {
		c.peopleMu.Unlock()
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	next, links, err := hall.DecidePerson(reg, c.entriesByPerson(reg), r.PathValue("id"), r.PathValue("action"), in)
	if err == nil {
		err = c.folder.SavePeople(next)
	}
	c.peopleMu.Unlock()
	if errors.Is(err, hall.ErrNoSuchAction) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	c.applyLinks(links)
	c.poke()
	v, _ := c.PeopleNow()
	writeJSON(w, http.StatusOK, v)
}
