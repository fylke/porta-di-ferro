package staffing

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Assigning staff to the plan (#5, proposal phase 5). Every work item needs its crew for
// as long as the forecast says it runs. The rules, in order:
//
//   - Never someone who is fencing at the time, in any discipline: their person's items
//     come from the plan, and an item they fence in overlapping this one rules them out.
//   - Never someone working another mat at the time.
//   - Only roles they offered, in disciplines they will work.
//
// Among those who may, #5 asks for as little hopping about as possible: the same role, on
// the same mat, for contiguous blocks of time. So each slot goes to whoever held this role
// on this mat just before, then whoever held this role anywhere, then whoever was on this
// mat; and otherwise to whoever has worked least, so the day is shared. Nobody is kept on
// past LongestStint without a pause while somebody else could take over. Greedy over the
// items in the order they run, which keeps it fast, deterministic and explainable.
//
// The organizer's choices stand: an assignment made by hand is kept, and so is anything on
// an item already under way. If one of those breaks a rule it is reported, not undone.

// Item is one work item to staff, timed by the forecast.
type Item struct {
	ID         string
	Discipline string
	Mat        int
	Start, End time.Time
	// Started items keep the crew they have.
	Started bool
	// People are the persons fencing in it.
	People []string
}

// Input is everything an assignment needs.
type Input struct {
	Items       []Item
	Members     []store.StaffMember
	Crew        map[string]int
	Assignments []store.Assignment
}

// Short is a slot nobody can fill.
type Short struct {
	Item string
	Role string
	Slot int
}

// Warning is a kept assignment that breaks a rule. Kinds: "fencing" (the member fences at
// the time), "double" (on another mat at the time), "role" (a role they did not offer),
// "discipline" (a discipline they will not work).
type Warning struct {
	Kind  string
	Item  string
	Role  string
	Staff string
	// Other is the item that clashes, for "fencing" and "double".
	Other string
}

// Result is the assignments, every slot that could not be filled, and the warnings.
type Result struct {
	Assignments []store.Assignment
	Short       []Short
	Warnings    []Warning
}

type span struct {
	item       string
	mat        int
	role       string
	start, end time.Time
}

func overlaps(a0, a1, b0, b1 time.Time) bool { return a0.Before(b1) && b0.Before(a1) }

// Assign staffs every item.
func Assign(in Input) Result { return run(in, true) }

// Check is the assignments as they are, judged: every slot left empty and every rule
// broken, with nothing filled in or taken away.
func Check(in Input) Result { return run(in, false) }

func run(in Input, fill bool) Result {
	crew := CrewOf(in.Crew)
	items := append([]Item{}, in.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Start.Equal(items[j].Start) {
			return items[i].Start.Before(items[j].Start)
		}
		if items[i].Mat != items[j].Mat {
			return items[i].Mat < items[j].Mat
		}
		return items[i].ID < items[j].ID
	})
	byID := map[string]Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	members := map[string]store.StaffMember{}
	for _, m := range in.Members {
		members[m.ID] = m
	}

	// When each person is fencing.
	fencing := map[string][]span{}
	for _, it := range items {
		for _, p := range it.People {
			fencing[p] = append(fencing[p], span{item: it.ID, mat: it.Mat, start: it.Start, end: it.End})
		}
	}

	// What stays: by hand, or on an item already under way.
	key := func(item, role string, slot int) string { return fmt.Sprintf("%s|%s|%d", item, role, slot) }
	kept := map[string]store.Assignment{}
	for _, a := range in.Assignments {
		it, ok := byID[a.Item]
		if _, known := members[a.Staff]; !ok || !known {
			continue
		}
		if a.Pinned || it.Started || !fill {
			kept[key(a.Item, a.Role, a.Slot)] = a
		}
	}

	working := map[string][]span{}
	minutes := map[string]time.Duration{}
	var out Result

	busyWith := func(member string, it Item) (string, string) {
		m := members[member]
		for _, f := range fencing[m.Person] {
			if m.Person != "" && overlaps(f.start, f.end, it.Start, it.End) {
				return "fencing", f.item
			}
		}
		for _, w := range working[member] {
			if w.item != it.ID && overlaps(w.start, w.end, it.Start, it.End) {
				return "double", w.item
			}
		}
		return "", ""
	}
	inItem := func(member string, it Item) bool {
		for _, w := range working[member] {
			if w.item == it.ID {
				return true
			}
		}
		return false
	}
	take := func(a store.Assignment, it Item) {
		out.Assignments = append(out.Assignments, a)
		working[a.Staff] = append(working[a.Staff], span{item: it.ID, mat: it.Mat, role: a.Role, start: it.Start, end: it.End})
		minutes[a.Staff] += it.End.Sub(it.Start)
	}
	last := func(member string, before time.Time) (span, bool) {
		var best span
		found := false
		for _, w := range working[member] {
			if !w.end.After(before) && (!found || w.end.After(best.end)) {
				best, found = w, true
			}
		}
		return best, found
	}

	// stint is how long a member has worked without a real pause, up to a start.
	stint := func(member string, before time.Time) time.Duration {
		var spans []span
		for _, w := range working[member] {
			if !w.end.After(before) {
				spans = append(spans, w)
			}
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].end.After(spans[j].end) })
		total, edge := time.Duration(0), before
		for _, w := range spans {
			if edge.Sub(w.end) > Pause {
				break
			}
			total += w.end.Sub(w.start)
			edge = w.start
		}
		return total
	}

	ids := make([]string, 0, len(members))
	for id := range members {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, it := range items {
		if it.Start.IsZero() || !it.End.After(it.Start) {
			continue
		}
		// The organizer's choices for this item first, so nobody else takes their time.
		for _, role := range MatRoles {
			for slot := 1; slot <= crew[role]; slot++ {
				a, ok := kept[key(it.ID, role, slot)]
				if !ok {
					continue
				}
				m := members[a.Staff]
				if kind, other := busyWith(a.Staff, it); kind != "" {
					out.Warnings = append(out.Warnings, Warning{Kind: kind, Item: it.ID, Role: role, Staff: a.Staff, Other: other})
				}
				if !Has(m, role) {
					out.Warnings = append(out.Warnings, Warning{Kind: "role", Item: it.ID, Role: role, Staff: a.Staff})
				}
				if !Works(m, it.Discipline) {
					out.Warnings = append(out.Warnings, Warning{Kind: "discipline", Item: it.ID, Role: role, Staff: a.Staff})
				}
				take(a, it)
			}
		}
		for _, role := range MatRoles {
			for slot := 1; slot <= crew[role]; slot++ {
				if _, ok := kept[key(it.ID, role, slot)]; ok {
					continue
				}
				if !fill {
					out.Short = append(out.Short, Short{Item: it.ID, Role: role, Slot: slot})
					continue
				}
				best, bestScore := "", 0.0
				for _, id := range ids {
					m := members[id]
					if !Has(m, role) || !Works(m, it.Discipline) || inItem(id, it) {
						continue
					}
					if kind, _ := busyWith(id, it); kind != "" {
						continue
					}
					score := -minutes[id].Minutes() / 10
					// A long stint wants a break: past the limit, anybody else first.
					if stint(id, it.Start)+it.End.Sub(it.Start) > LongestStint {
						score -= 150
					}
					if prev, ok := last(id, it.Start); ok {
						switch {
						case prev.mat == it.Mat && prev.role == role:
							score += 100
						case prev.role == role:
							score += 40
						case prev.mat == it.Mat:
							score += 20
						}
					}
					if best == "" || score > bestScore {
						best, bestScore = id, score
					}
				}
				if best == "" {
					out.Short = append(out.Short, Short{Item: it.ID, Role: role, Slot: slot})
					continue
				}
				take(store.Assignment{Item: it.ID, Role: role, Slot: slot, Staff: best}, it)
			}
		}
	}
	return out
}

// LongestStint is how long somebody should work without a break before somebody else is
// preferred, and Pause the gap that counts as one. Soft: when nobody else is free, the
// slot is still better filled than empty.
const (
	LongestStint = 150 * time.Minute
	Pause        = 20 * time.Minute
)

// Signature names a set of assignments, so applying a suggestion can check it is still
// the one the organizer looked at.
func Signature(as []store.Assignment) string {
	keys := make([]string, 0, len(as))
	for _, a := range as {
		keys = append(keys, fmt.Sprintf("%s|%s|%d|%s", a.Item, a.Role, a.Slot, a.Staff))
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

// Physicians are the members on call for the hall: a physician is not on a mat.
func Physicians(members []store.StaffMember) []store.StaffMember {
	out := []store.StaffMember{}
	for _, m := range members {
		if Has(m, Physician) {
			out = append(out, m)
		}
	}
	return out
}
