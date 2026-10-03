package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/people"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The event's people (proposal §8, phase 3). The registry is people.json; which entry is
// whom is on each entry, in each discipline's competitors.json. The registry's lock is
// never held while a discipline's is wanted: a signup import holds its discipline's lock
// and asks for a person, so the other order would deadlock. Every change here is decided
// under the registry's lock and written to the entries after.

// PersonFor is who a new entry is (EventPeople, for the disciplines).
func (c *Coordinator) PersonFor(name, club, submission, want string) (string, error) {
	c.peopleMu.Lock()
	defer c.peopleMu.Unlock()
	reg, err := c.folder.People()
	if err != nil {
		return "", err
	}
	next, id := people.For(reg, name, club, submission, want)
	if len(next) != len(reg) {
		if err := c.folder.SavePeople(next); err != nil {
			return "", err
		}
	}
	return id, nil
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

// PersonView is one person, with every entry of theirs in the event.
type PersonView struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Club    string         `json:"club,omitempty"`
	Entries []people.Entry `json:"entries"`
	// MergedFrom are the people merged into this one, whose merge can be undone.
	MergedFrom []PersonRef `json:"mergedFrom,omitempty"`
	// Duties are their work as staff (phase 5), and Physician says they are on call.
	Duties    []Duty `json:"duties,omitempty"`
	Physician bool   `json:"physician,omitempty"`
}

// PersonRef names a person.
type PersonRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PeopleView is everybody, and the groups who might be one.
type PeopleView struct {
	People     []PersonView `json:"people"`
	Duplicates [][]string   `json:"duplicates"`
}

func personView(p store.Person, reg []store.Person, entries map[string][]people.Entry) PersonView {
	v := PersonView{ID: p.ID, Name: p.Name, Club: p.Club, Entries: entries[p.ID]}
	if v.Entries == nil {
		v.Entries = []people.Entry{}
	}
	// The entries' own names: the organizer corrects a name in the discipline, and the
	// person should read as corrected.
	if len(v.Entries) > 0 {
		v.Name, v.Club = v.Entries[0].Name, v.Entries[0].Club
	}
	for _, o := range reg {
		if o.MergedInto == p.ID {
			v.MergedFrom = append(v.MergedFrom, PersonRef{ID: o.ID, Name: o.Name})
		}
	}
	return v
}

// ViewPeople is the registry as the people panel shows it: everybody entered somewhere,
// and who might be one person. Shared with the browser demo.
func ViewPeople(reg []store.Person, entries map[string][]people.Entry) PeopleView {
	out := PeopleView{People: []PersonView{}, Duplicates: people.Duplicates(reg, entries)}
	if out.Duplicates == nil {
		out.Duplicates = [][]string{}
	}
	for _, p := range reg {
		if p.MergedInto != "" || len(entries[p.ID]) == 0 {
			continue
		}
		out.People = append(out.People, personView(p, reg, entries))
	}
	sort.SliceStable(out.People, func(i, j int) bool { return out.People[i].Name < out.People[j].Name })
	return out
}

// ViewPerson is one person by an id they have now or had before a merge.
func ViewPerson(reg []store.Person, id string, entries map[string][]people.Entry) (PersonView, bool) {
	id = people.Resolve(reg, id)
	for _, p := range reg {
		if p.ID == id {
			return personView(p, reg, entries), true
		}
	}
	return PersonView{}, false
}

// PeopleNow is the registry as the people panel shows it.
func (c *Coordinator) PeopleNow() (PeopleView, error) {
	reg, err := c.folder.People()
	if err != nil {
		return PeopleView{}, err
	}
	return ViewPeople(reg, c.entriesByPerson(reg)), nil
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
	v, ok := ViewPerson(reg, r.PathValue("id"), c.entriesByPerson(reg))
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("nobody with id %q is in this event", r.PathValue("id")))
		return
	}
	if sv, _, st, err := c.StaffingNow(); err == nil {
		v.Duties, v.Physician = DutiesOf(v.ID, st, sv)
		if len(v.Entries) == 0 {
			// Somebody who only works: their name is on the staff list.
			for _, m := range st.Members {
				if m.Person == v.ID {
					v.Name, v.Club = m.Name, m.Club
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, v)
}

// PersonAction is what the organizer decided about a person.
type PersonAction struct {
	Into       string `json:"into"`
	Other      string `json:"other"`
	Discipline string `json:"discipline"`
	Competitor string `json:"competitor"`
}

// ErrNoSuchAction is a person action that is not one.
var ErrNoSuchAction = errors.New("no such thing to do to a person")

// DecidePerson is the organizer deciding about people, over the registry: merge one into
// another, undo that, keep two apart, or say an entry is this person. Returns the people
// and the entries to point at someone else. Shared with the browser demo.
func DecidePerson(reg []store.Person, entries map[string][]people.Entry, id, action string, in PersonAction) ([]store.Person, people.Links, error) {
	switch action {
	case "merge":
		var moved []string
		for _, e := range entries[id] {
			moved = append(moved, e.Key())
		}
		next, err := people.Merge(reg, id, in.Into, moved)
		if err != nil {
			return reg, nil, err
		}
		return next, people.LinksTo(entries[id], in.Into), nil
	case "unmerge":
		next, moved, err := people.Unmerge(reg, id)
		if err != nil {
			return reg, nil, err
		}
		return next, people.LinksBack(moved, id), nil
	case "apart":
		next, err := people.KeepApart(reg, id, in.Other)
		return next, nil, err
	case "link":
		if !people.Active(reg, id) {
			return reg, nil, fmt.Errorf("nobody with id %q is in this event", id)
		}
		return reg, people.Links{in.Discipline: {in.Competitor: id}}, nil
	}
	return reg, nil, ErrNoSuchAction
}

// postPerson is DecidePerson over HTTP.
func (c *Coordinator) postPerson(w http.ResponseWriter, r *http.Request) {
	var in PersonAction
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
	next, links, err := DecidePerson(reg, c.entriesByPerson(reg), r.PathValue("id"), r.PathValue("action"), in)
	if err == nil {
		err = c.folder.SavePeople(next)
	}
	c.peopleMu.Unlock()
	if errors.Is(err, ErrNoSuchAction) {
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
