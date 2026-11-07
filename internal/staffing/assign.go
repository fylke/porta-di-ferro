package staffing

import (
	"fmt"
	"slices"
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
//   - Never someone competing in another discipline at the time, even in a hole in their
//     own schedule (#153): between their pool and the eliminations they may be on the
//     strip at any moment, and they came to fence. Competing runs from the discipline's
//     first item until they are out of it. In their own discipline, a fencer between
//     matches may still referee a pool they are not in.
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
	// Held says it is a final held to the end of the day, which only those named in it
	// are still competing for.
	Held bool
}

// Input is everything an assignment needs.
type Input struct {
	Items       []Item
	Members     []store.StaffMember
	Crew        map[string]int
	Assignments []store.Assignment
	// Competing is who is still in each discipline, by slug, whether or not the draw has
	// put them in an item yet.
	Competing map[string][]string
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

// contest is a stretch of the day a person is competing in a discipline.
type contest struct {
	discipline string
	// item is the discipline's first, for a warning to name.
	item       string
	start, end time.Time
}

// contests is when each person is competing, from the items in the order they run: from a
// discipline's first item to its last, leaving out finals held to the end of the day unless
// the person is named in one.
func contests(items []Item, competing map[string][]string) map[string][]contest {
	out := map[string][]contest{}
	// In the order of the slugs: which contest a warning names should not change between looks.
	disciplines := make([]string, 0, len(competing))
	for disc := range competing {
		disciplines = append(disciplines, disc)
	}
	sort.Strings(disciplines)
	for _, disc := range disciplines {
		people := competing[disc]
		var whole contest
		for _, it := range items {
			if it.Discipline != disc || it.Start.IsZero() {
				continue
			}
			if whole.item == "" {
				whole = contest{discipline: disc, item: it.ID, start: it.Start, end: it.End}
			}
			if !it.Held && it.End.After(whole.end) {
				whole.end = it.End
			}
		}
		if whole.item == "" {
			continue
		}
		for _, p := range people {
			c := whole
			for _, it := range items {
				if it.Discipline == disc && it.End.After(c.end) && slices.Contains(it.People, p) {
					c.end = it.End
				}
			}
			out[p] = append(out[p], c)
		}
	}
	return out
}

// Assign staffs every item.
func Assign(in Input) Result { return run(in, true) }

// Check is the assignments as they are, judged: every slot left empty and every rule
// broken, with nothing filled in or taken away.
func Check(in Input) Result { return run(in, false) }

func run(in Input, fill bool) Result {
	a := newAssigner(in, fill)
	for _, it := range a.items {
		if it.Start.IsZero() || !it.End.After(it.Start) {
			continue
		}
		a.staff(it)
	}
	return a.out
}

// assigner is an assignment as it is made, item by item in the order they run.
type assigner struct {
	fill bool
	crew map[string]int
	// items are in the order they run, and ids the members in the order they are considered.
	items     []Item
	members   map[string]store.StaffMember
	ids       []string
	fencing   map[string][]span
	competing map[string][]contest
	// kept is what stays: by hand, or on an item already under way.
	kept map[string]store.Assignment
	// working is what each member has been given so far, and minutes how long it comes to.
	working map[string][]span
	minutes map[string]time.Duration
	out     Result
}

func newAssigner(in Input, fill bool) *assigner {
	a := &assigner{
		fill:    fill,
		crew:    CrewOf(in.Crew),
		items:   inRunningOrder(in.Items),
		members: map[string]store.StaffMember{},
		fencing: map[string][]span{},
		kept:    map[string]store.Assignment{},
		working: map[string][]span{},
		minutes: map[string]time.Duration{},
	}
	byID := map[string]Item{}
	for _, it := range a.items {
		byID[it.ID] = it
		for _, p := range it.People {
			a.fencing[p] = append(a.fencing[p], span{item: it.ID, mat: it.Mat, start: it.Start, end: it.End})
		}
	}
	for _, m := range in.Members {
		a.members[m.ID] = m
	}
	a.competing = contests(a.items, in.Competing)
	for _, as := range in.Assignments {
		it, ok := byID[as.Item]
		if _, known := a.members[as.Staff]; !ok || !known {
			continue
		}
		if as.Pinned || it.Started || !fill {
			a.kept[slotKey(as.Item, as.Role, as.Slot)] = as
		}
	}
	a.ids = make([]string, 0, len(a.members))
	for id := range a.members {
		a.ids = append(a.ids, id)
	}
	sort.Strings(a.ids)
	return a
}

func inRunningOrder(in []Item) []Item {
	items := append([]Item{}, in...)
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Start.Equal(items[j].Start) {
			return items[i].Start.Before(items[j].Start)
		}
		if items[i].Mat != items[j].Mat {
			return items[i].Mat < items[j].Mat
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func slotKey(item, role string, slot int) string { return fmt.Sprintf("%s|%s|%d", item, role, slot) }

// staff gives one item its crew: the organizer's choices first, so nobody else takes their
// time, and then whoever is best for each slot left.
func (a *assigner) staff(it Item) {
	for _, role := range MatRoles {
		for slot := 1; slot <= a.crew[role]; slot++ {
			if kept, ok := a.kept[slotKey(it.ID, role, slot)]; ok {
				a.keep(kept, it, role)
			}
		}
	}
	for _, role := range MatRoles {
		for slot := 1; slot <= a.crew[role]; slot++ {
			if _, ok := a.kept[slotKey(it.ID, role, slot)]; !ok {
				a.fillSlot(it, role, slot)
			}
		}
	}
}

// keep takes an assignment that stays, and reports every rule it breaks.
func (a *assigner) keep(kept store.Assignment, it Item, role string) {
	m := a.members[kept.Staff]
	if kind, other := a.busyWith(kept.Staff, it); kind != "" {
		a.out.Warnings = append(a.out.Warnings, Warning{Kind: kind, Item: it.ID, Role: role, Staff: kept.Staff, Other: other})
	}
	if !Has(m, role) {
		a.out.Warnings = append(a.out.Warnings, Warning{Kind: "role", Item: it.ID, Role: role, Staff: kept.Staff})
	}
	if !Works(m, it.Discipline) {
		a.out.Warnings = append(a.out.Warnings, Warning{Kind: "discipline", Item: it.ID, Role: role, Staff: kept.Staff})
	}
	a.take(kept, it)
}

// fillSlot gives an empty slot to whoever is best for it, or reports that nobody can.
func (a *assigner) fillSlot(it Item, role string, slot int) {
	best := ""
	if a.fill {
		best = a.bestFor(it, role)
	}
	if best == "" {
		a.out.Short = append(a.out.Short, Short{Item: it.ID, Role: role, Slot: slot})
		return
	}
	a.take(store.Assignment{Item: it.ID, Role: role, Slot: slot, Staff: best}, it)
}

// bestFor is the member who may take this role on this item and scores highest, or "".
func (a *assigner) bestFor(it Item, role string) string {
	best, bestScore := "", 0.0
	for _, id := range a.ids {
		m := a.members[id]
		if !Has(m, role) || !Works(m, it.Discipline) || a.inItem(id, it) {
			continue
		}
		if kind, _ := a.busyWith(id, it); kind != "" {
			continue
		}
		if score := a.score(id, it, role); best == "" || score > bestScore {
			best, bestScore = id, score
		}
	}
	return best
}

// score is how well a member suits a slot: little hopping about, the day shared, and a
// break after a long stint.
func (a *assigner) score(id string, it Item, role string) float64 {
	score := -a.minutes[id].Minutes() / 10
	// A long stint wants a break: past the limit, anybody else first.
	if a.stint(id, it.Start)+it.End.Sub(it.Start) > LongestStint {
		score -= 150
	}
	if prev, ok := a.last(id, it.Start); ok {
		switch {
		case prev.mat == it.Mat && prev.role == role:
			score += 100
		case prev.role == role:
			score += 40
		case prev.mat == it.Mat:
			score += 20
		}
	}
	return score
}

func (a *assigner) busyWith(member string, it Item) (string, string) {
	m := a.members[member]
	for _, f := range a.fencing[m.Person] {
		if m.Person != "" && overlaps(f.start, f.end, it.Start, it.End) {
			return "fencing", f.item
		}
	}
	for _, c := range a.competing[m.Person] {
		if m.Person != "" && c.discipline != it.Discipline && overlaps(c.start, c.end, it.Start, it.End) {
			return "fencing", c.item
		}
	}
	for _, w := range a.working[member] {
		if w.item != it.ID && overlaps(w.start, w.end, it.Start, it.End) {
			return "double", w.item
		}
	}
	return "", ""
}

func (a *assigner) inItem(member string, it Item) bool {
	for _, w := range a.working[member] {
		if w.item == it.ID {
			return true
		}
	}
	return false
}

func (a *assigner) take(as store.Assignment, it Item) {
	a.out.Assignments = append(a.out.Assignments, as)
	a.working[as.Staff] = append(a.working[as.Staff], span{item: it.ID, mat: it.Mat, role: as.Role, start: it.Start, end: it.End})
	a.minutes[as.Staff] += it.End.Sub(it.Start)
}

// last is the member's last work ending by a time.
func (a *assigner) last(member string, before time.Time) (span, bool) {
	var best span
	found := false
	for _, w := range a.working[member] {
		if !w.end.After(before) && (!found || w.end.After(best.end)) {
			best, found = w, true
		}
	}
	return best, found
}

// stint is how long a member has worked without a real pause, up to a start.
func (a *assigner) stint(member string, before time.Time) time.Duration {
	var spans []span
	for _, w := range a.working[member] {
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

// Needed is the fewest people who could staff the plan: a full crew on every mat running
// at the busiest moment of the day. Nobody works two mats at once, so no fewer will do.
func Needed(items []Item, crew map[string]int) int {
	per := 0
	for _, r := range MatRoles {
		per += CrewOf(crew)[r]
	}
	type edge struct {
		at    time.Time
		delta int
	}
	var edges []edge
	for _, it := range items {
		if it.Start.IsZero() || !it.End.After(it.Start) {
			continue
		}
		edges = append(edges, edge{it.Start, 1}, edge{it.End, -1})
	}
	// An item ending as another starts frees its mat first.
	sort.Slice(edges, func(i, j int) bool {
		if !edges[i].at.Equal(edges[j].at) {
			return edges[i].at.Before(edges[j].at)
		}
		return edges[i].delta < edges[j].delta
	})
	peak, now := 0, 0
	for _, e := range edges {
		now += e.delta
		peak = max(peak, now)
	}
	return peak * per
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
