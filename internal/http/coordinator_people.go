package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

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

// entriesByPerson is every readable discipline's entries, by the person each is now.
func (c *Coordinator) entriesByPerson(reg []store.Person) map[string][]people.Entry {
	out := map[string][]people.Entry{}
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			continue
		}
		comps, err := w.srv.Competitors()
		if err != nil {
			continue
		}
		name := w.srv.Self().Name
		for _, comp := range comps {
			id := people.Resolve(reg, comp.Person)
			out[id] = append(out[id], people.Entry{
				Discipline: w.slug, DisciplineName: name, Competitor: comp.ID,
				Name: comp.Name, Club: comp.Club, Withdrawn: comp.Withdrawn,
			})
		}
	}
	return out
}

// ensurePeople gives every entry a person: an entry from before people existed, or one
// typed into a file by hand. One that came in on a signup joins whoever has that
// submission already; anybody else is somebody new -- never matched on the name. An entry
// pointing at somebody merged away is pointed at whom they were merged into.
func (c *Coordinator) ensurePeople() {
	links := map[string]map[string]string{} // discipline → competitor → person
	c.peopleMu.Lock()
	reg, err := c.folder.People()
	if err != nil {
		c.peopleMu.Unlock()
		return
	}
	before := len(reg)
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			continue
		}
		comps, err := w.srv.Competitors()
		if err != nil {
			continue
		}
		for _, comp := range comps {
			id := people.Resolve(reg, comp.Person)
			if comp.Person == "" || !people.Active(reg, id) {
				reg, id = people.For(reg, comp.Name, comp.Club, comp.Signup, "")
			}
			if id != comp.Person {
				if links[w.slug] == nil {
					links[w.slug] = map[string]string{}
				}
				links[w.slug][comp.ID] = id
			}
		}
	}
	if len(reg) != before {
		_ = c.folder.SavePeople(reg)
	}
	c.peopleMu.Unlock()
	c.applyLinks(links)
}

// applyLinks points entries at people, discipline by discipline.
func (c *Coordinator) applyLinks(links map[string]map[string]string) {
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

// --- the people, over HTTP ---------------------------------------------------------------

// PersonView is one person, with every entry of theirs in the event.
type PersonView struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Club    string         `json:"club,omitempty"`
	Entries []people.Entry `json:"entries"`
	// MergedFrom are the people merged into this one, whose merge can be undone.
	MergedFrom []PersonRef `json:"mergedFrom,omitempty"`
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

// PeopleNow is the registry as the people page shows it.
func (c *Coordinator) PeopleNow() (PeopleView, error) {
	reg, err := c.folder.People()
	if err != nil {
		return PeopleView{}, err
	}
	entries := c.entriesByPerson(reg)
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
	return out, nil
}

func (c *Coordinator) getPeople(w http.ResponseWriter, r *http.Request) {
	v, err := c.PeopleNow()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// getPerson is one person's whole day across the event, by an id they have now or had
// before a merge.
func (c *Coordinator) getPerson(w http.ResponseWriter, r *http.Request) {
	reg, err := c.folder.People()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	id := people.Resolve(reg, r.PathValue("id"))
	for _, p := range reg {
		if p.ID == id {
			writeJSON(w, http.StatusOK, personView(p, reg, c.entriesByPerson(reg)))
			return
		}
	}
	writeErr(w, http.StatusNotFound, fmt.Errorf("nobody with id %q is in this event", r.PathValue("id")))
}

// postPerson is the organizer deciding about people: merge one into another, undo that,
// keep two apart, or say an entry is this person.
func (c *Coordinator) postPerson(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Into       string `json:"into"`
		Other      string `json:"other"`
		Discipline string `json:"discipline"`
		Competitor string `json:"competitor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && r.PathValue("action") != "unmerge" {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var err error
	switch r.PathValue("action") {
	case "merge":
		err = c.mergePeople(id, in.Into)
	case "unmerge":
		err = c.unmergePerson(id)
	case "apart":
		err = c.keepApart(id, in.Other)
	case "link":
		err = c.linkEntry(id, in.Discipline, in.Competitor)
	default:
		writeErr(w, http.StatusNotFound, errors.New("no such thing to do to a person"))
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	c.poke()
	v, _ := c.PeopleNow()
	writeJSON(w, http.StatusOK, v)
}

func (c *Coordinator) mergePeople(from, into string) error {
	c.peopleMu.Lock()
	reg, err := c.folder.People()
	if err != nil {
		c.peopleMu.Unlock()
		return err
	}
	links := map[string]map[string]string{}
	var moved []string
	for _, e := range c.entriesByPerson(reg)[from] {
		moved = append(moved, e.Key())
		if links[e.Discipline] == nil {
			links[e.Discipline] = map[string]string{}
		}
		links[e.Discipline][e.Competitor] = into
	}
	next, err := people.Merge(reg, from, into, moved)
	if err == nil {
		err = c.folder.SavePeople(next)
	}
	c.peopleMu.Unlock()
	if err != nil {
		return err
	}
	c.applyLinks(links)
	return nil
}

func (c *Coordinator) unmergePerson(id string) error {
	c.peopleMu.Lock()
	reg, err := c.folder.People()
	if err != nil {
		c.peopleMu.Unlock()
		return err
	}
	next, moved, err := people.Unmerge(reg, id)
	if err == nil {
		err = c.folder.SavePeople(next)
	}
	c.peopleMu.Unlock()
	if err != nil {
		return err
	}
	links := map[string]map[string]string{}
	for _, key := range moved {
		slug, comp, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		if links[slug] == nil {
			links[slug] = map[string]string{}
		}
		links[slug][comp] = id
	}
	c.applyLinks(links)
	return nil
}

func (c *Coordinator) keepApart(a, b string) error {
	c.peopleMu.Lock()
	defer c.peopleMu.Unlock()
	reg, err := c.folder.People()
	if err != nil {
		return err
	}
	next, err := people.KeepApart(reg, a, b)
	if err != nil {
		return err
	}
	return c.folder.SavePeople(next)
}

// linkEntry says one entry is this person: the fix for an entry that should have been
// linked when it was made.
func (c *Coordinator) linkEntry(id, slug, comp string) error {
	reg, err := c.folder.People()
	if err != nil {
		return err
	}
	if !people.Active(reg, id) {
		return fmt.Errorf("nobody with id %q is in this event", id)
	}
	c.applyLinks(map[string]map[string]string{slug: {comp: id}})
	return nil
}
