package hall

// The event's people as the pages see them, and the organizer's decisions about them. The
// rules are internal/people's; keeping them in a file is the coordinator's, and keeping
// them in memory the demo's.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/people"
	"github.com/fylke/porta-di-ferro/internal/store"
)

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
