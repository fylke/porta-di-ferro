// Package people is the event's registry of people (docs/proposals/one-event-many-
// disciplines.md §8, phase 3): one record per human, and the rules for which competitor
// entries are the same one.
//
// The rule is the project's usual one: the app reports, the organizer decides. A signup's
// submission id is exact, so a response that enters two disciplines is one person in both
// without asking. A name is never enough -- two Anna Nilssons from the same club are a real
// thing, and "Karl-Johan" and "Karl Johan" are one person -- so people with the same name
// are offered to the organizer as possible duplicates, to merge or keep apart, and a merge
// can be undone.
//
// Everything here is a pure function over the list of people. Which competitor entries
// point at whom lives with the entries, in each discipline's competitors.json; the
// coordinator and the browser demo apply what these functions decide.
package people

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// NewID is a fresh person id: random, so it never collides the way sequential competitor
// ids do across disciplines and devices.
func NewID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic("people: no randomness: " + err.Error())
	}
	return "pr-" + hex.EncodeToString(b)
}

// Normalize is a name folded for comparison: case, the Nordic letters, and the spaces and
// hyphens people disagree about. Only ever used to suggest, never to decide.
func Normalize(name string) string { return signup.Slug(name) }

func index(people []store.Person, id string) int {
	for i, p := range people {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// Resolve is who an id means now: itself, or whoever it was merged into.
func Resolve(people []store.Person, id string) string {
	for seen := 0; seen < len(people)+1; seen++ {
		i := index(people, id)
		if i < 0 || people[i].MergedInto == "" {
			return id
		}
		id = people[i].MergedInto
	}
	return id
}

// Active says a person is in the registry and has not been merged into somebody else.
func Active(people []store.Person, id string) bool {
	i := index(people, id)
	return i >= 0 && people[i].MergedInto == ""
}

// For is the person a new entry belongs to: the one asked for if there is one, else the
// one who came in on the same signup, else somebody new. Returns the people, a new one
// added if need be, and the id.
func For(people []store.Person, name, club, submission, want string) ([]store.Person, string) {
	if want != "" {
		if id := Resolve(people, want); Active(people, id) {
			return people, id
		}
	}
	if submission != "" {
		for _, p := range people {
			if p.Signup == submission && p.MergedInto == "" {
				return people, p.ID
			}
		}
	}
	p := store.Person{ID: NewID(), Name: name, Club: club, Signup: submission}
	return append(people, p), p.ID
}

// Entry is one competitor record of a person, in one discipline.
type Entry struct {
	Discipline     string `json:"discipline"`
	DisciplineName string `json:"disciplineName"`
	Competitor     string `json:"competitor"`
	Name           string `json:"name"`
	Club           string `json:"club,omitempty"`
	Withdrawn      bool   `json:"withdrawn,omitempty"`
}

// Key names an entry across the event: "open-sabre/c7".
func (e Entry) Key() string { return e.Discipline + "/" + e.Competitor }

// Duplicates are the groups of people who might be one: the same name, folded, and not
// every pair of them already said to be somebody else. The organizer is asked; nothing
// here merges anybody.
func Duplicates(people []store.Person, entries map[string][]Entry) [][]string {
	groups := map[string][]string{}
	for _, p := range people {
		if p.MergedInto != "" {
			continue
		}
		names := map[string]bool{}
		for _, e := range entries[p.ID] {
			names[Normalize(e.Name)] = true
		}
		if len(names) == 0 {
			names[Normalize(p.Name)] = true
		}
		for n := range names {
			if n != "" {
				groups[n] = append(groups[n], p.ID)
			}
		}
	}
	apart := map[[2]string]bool{}
	for _, p := range people {
		for _, o := range p.Apart {
			apart[[2]string{p.ID, o}] = true
			apart[[2]string{o, p.ID}] = true
		}
	}
	var out [][]string
	seen := map[string]bool{}
	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		open := false
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				open = open || !apart[[2]string{ids[i], ids[j]}]
			}
		}
		sort.Strings(ids)
		key := fmt.Sprint(ids)
		if open && !seen[key] {
			seen[key] = true
			out = append(out, ids)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// Merge records that from is into: their entries, listed in moved, now point at into.
// The caller moves the entries; this keeps what is needed to move them back.
func Merge(people []store.Person, from, into string, moved []string) ([]store.Person, error) {
	if from == into {
		return people, errors.New("that is the same person")
	}
	i, j := index(people, from), index(people, into)
	if i < 0 || j < 0 {
		return people, errors.New("no such person")
	}
	if people[i].MergedInto != "" || people[j].MergedInto != "" {
		return people, errors.New("that person has already been merged into somebody else")
	}
	out := append([]store.Person{}, people...)
	out[i].MergedInto, out[i].Moved = into, append([]string{}, moved...)
	if out[j].Signup == "" {
		out[j].Signup = out[i].Signup
	}
	return out, nil
}

// Unmerge undoes a merge: the person is themselves again, and the entries that were
// moved, returned, go back to them.
func Unmerge(people []store.Person, id string) ([]store.Person, []string, error) {
	i := index(people, id)
	if i < 0 {
		return people, nil, errors.New("no such person")
	}
	if people[i].MergedInto == "" {
		return people, nil, errors.New("that person was not merged into anybody")
	}
	out := append([]store.Person{}, people...)
	moved := out[i].Moved
	out[i].MergedInto, out[i].Moved = "", nil
	return out, moved, nil
}

// KeepApart records that two people with the same name are two people, so they stop being
// offered as a duplicate.
func KeepApart(people []store.Person, a, b string) ([]store.Person, error) {
	i, j := index(people, a), index(people, b)
	if i < 0 || j < 0 || a == b {
		return people, errors.New("no such pair of people")
	}
	out := append([]store.Person{}, people...)
	add := func(k int, other string) {
		for _, x := range out[k].Apart {
			if x == other {
				return
			}
		}
		out[k].Apart = append(append([]string{}, out[k].Apart...), other)
	}
	add(i, b)
	add(j, a)
	return out, nil
}

// Roster is one discipline's entries, for Ensure.
type Roster struct {
	Discipline  string
	Competitors []store.Competitor
}

// Links are entries to point at people: discipline, then competitor, then person.
type Links map[string]map[string]string

func (l Links) set(discipline, competitor, person string) {
	if l[discipline] == nil {
		l[discipline] = map[string]string{}
	}
	l[discipline][competitor] = person
}

// Ensure gives every entry a person: an entry from before people existed, or one typed
// into a file by hand. One that came in on a signup joins whoever has that submission
// already; anybody else is somebody new -- never matched on the name. An entry pointing
// at somebody merged away is pointed at whom they were merged into. Returns the people,
// with any new ones, and the entries whose person changes.
func Ensure(people []store.Person, rosters []Roster) ([]store.Person, Links) {
	links := Links{}
	for _, r := range rosters {
		for _, c := range r.Competitors {
			id := Resolve(people, c.Person)
			if c.Person == "" || !Active(people, id) {
				people, id = For(people, c.Name, c.Club, c.Signup, "")
			}
			if id != c.Person {
				links.set(r.Discipline, c.ID, id)
			}
		}
	}
	return people, links
}

// LinksTo points every one of entries at a person: the entries a merge moves.
func LinksTo(entries []Entry, person string) Links {
	links := Links{}
	for _, e := range entries {
		links.set(e.Discipline, e.Competitor, person)
	}
	return links
}

// LinksBack points the entries an undone merge recorded, by Key, back at the person.
func LinksBack(moved []string, person string) Links {
	links := Links{}
	for _, key := range moved {
		if slug, comp, ok := strings.Cut(key, "/"); ok {
			links.set(slug, comp, person)
		}
	}
	return links
}

// EntriesOf is every roster's entries, by the person each is now.
func EntriesOf(people []store.Person, rosters []Roster, names map[string]string) map[string][]Entry {
	out := map[string][]Entry{}
	for _, r := range rosters {
		for _, c := range r.Competitors {
			id := Resolve(people, c.Person)
			out[id] = append(out[id], Entry{
				Discipline: r.Discipline, DisciplineName: names[r.Discipline], Competitor: c.ID,
				Name: c.Name, Club: c.Club, Withdrawn: c.Withdrawn,
			})
		}
	}
	return out
}

// Prune drops the people nobody needs any more (#127): a competitor removed -- a typo
// fixed by removing and entering again -- leaves their person behind, and people.json
// would grow with every one. Dropped is a person no entry points at, who came in on no
// signup, who was merged into nobody and nobody into them, and whom keep does not name:
// the staff, and the entries of retired disciplines. Whoever was kept apart from them
// forgets it.
func Prune(people []store.Person, rosters []Roster, keep map[string]bool) []store.Person {
	used := map[string]bool{}
	for id := range keep {
		used[id] = true
	}
	for _, r := range rosters {
		for _, c := range r.Competitors {
			used[c.Person] = true
		}
	}
	for _, p := range people {
		if p.MergedInto != "" {
			used[p.MergedInto] = true
		}
	}
	gone := map[string]bool{}
	var out []store.Person
	for _, p := range people {
		if !used[p.ID] && p.Signup == "" && p.MergedInto == "" {
			gone[p.ID] = true
			continue
		}
		out = append(out, p)
	}
	if len(gone) == 0 {
		return people
	}
	for i, p := range out {
		var apart []string
		for _, id := range p.Apart {
			if !gone[id] {
				apart = append(apart, id)
			}
		}
		out[i].Apart = apart
	}
	return out
}
