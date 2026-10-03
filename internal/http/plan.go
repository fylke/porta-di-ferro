package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// Work items and the event's mats (docs/proposals/one-event-many-disciplines.md §9,
// phase 2).
//
// Mats belong to the event. Every mat has one queue, and its entries are work items from
// any discipline: Longsword pool 3, then Sabre pool 1, then the Longsword final. The item
// at the head is what the mat is running, so a mat "belongs" to a discipline exactly as
// long as one of its items is at the head -- there is no lease to hand back.
//
// The boundary is this. A discipline says what its items are and how the matches inside
// each are ordered: the pool's running order, back-to-back avoidance and the bracket are
// untouched. The plan says which mat runs each item and in what order. Everything here is
// a pure function over a discipline's stored tournament or its snapshot, so the
// coordinator and the browser demo run the same code.

// The kinds of work item.
const (
	ItemPool         = "pool"
	ItemEliminations = "eliminations"
	ItemBronze       = "bronze"
	ItemFinal        = "final"
)

// WorkItem is one unit of mat time.
type WorkItem struct {
	Discipline string `json:"discipline"`
	// Key names the item within its discipline: "pool-3", "elim-1", "bronze", "final".
	Key  string `json:"key"`
	Kind string `json:"kind"`
	// Number is the pool's number, or the lane of an eliminations item.
	Number int `json:"number,omitempty"`
	// Lane is the discipline's own mat for the item -- where it would run on its own,
	// and where it lands on the event's mats until somebody moves it.
	Lane  int    `json:"lane"`
	Stamp string `json:"stamp,omitempty"`
	// Matches are the item's match ids, in the order its mat runs them.
	Matches []string `json:"matches"`
	// Projected says the item is not drawn yet: what the draw would make (projected.go).
	Projected bool `json:"projected,omitempty"`
	// Feeders are, for each bracket match, the matches that decide who fences in it, by
	// id: what the forecast waits for (#120).
	Feeders map[string][]string `json:"-"`
}

// ID is the item's name across the event: "open-sabre/pool-3".
func (w WorkItem) ID() string { return w.Discipline + "/" + w.Key }

// ItemKeyFor is the item a match belongs to. Pools are one item each. The bracket is the
// eliminations on each of the discipline's lanes, and then the bronze match and the final
// as items of their own: finals are normally held back and moved on their own (§15).
func ItemKeyFor(m store.Match) string {
	switch {
	case m.Pool > 0:
		return fmt.Sprintf("pool-%d", m.Pool)
	case m.Round == tournament.RoundBronze:
		return ItemBronze
	case m.Round == tournament.RoundFinal:
		return ItemFinal
	default:
		lane := m.Mat
		if lane < 1 {
			lane = 1
		}
		return fmt.Sprintf("elim-%d", lane)
	}
}

// ItemsOf is a discipline's work items, read off its stored tournament: the pools in
// their running order, then the bracket's.
func ItemsOf(slug string, t store.Tournament) []WorkItem {
	var out []WorkItem
	for _, p := range tournament.RunOrder(t) {
		ids := make([]string, 0, len(p.Matches))
		for _, m := range p.Matches {
			ids = append(ids, m.ID)
		}
		out = append(out, WorkItem{
			Discipline: slug, Key: fmt.Sprintf("pool-%d", p.Number), Kind: ItemPool,
			Number: p.Number, Lane: p.Mat, Stamp: t.GeneratedAt, Matches: ids,
		})
	}
	var bracket []*WorkItem
	byKey := map[string]*WorkItem{}
	for _, m := range t.Bracket {
		key := ItemKeyFor(m)
		it, ok := byKey[key]
		if !ok {
			it = &WorkItem{Discipline: slug, Key: key, Lane: max(m.Mat, 1), Stamp: t.BracketAt}
			switch key {
			case ItemBronze:
				it.Kind = ItemBronze
			case ItemFinal:
				it.Kind = ItemFinal
			default:
				it.Kind, it.Number = ItemEliminations, max(m.Mat, 1)
			}
			byKey[key] = it
			bracket = append(bracket, it)
		}
		it.Matches = append(it.Matches, m.ID)
		for _, feed := range []string{m.FeedRed, m.FeedBlue} {
			if _, from, ok := strings.Cut(feed, ":"); ok && from != "" {
				if it.Feeders == nil {
					it.Feeders = map[string][]string{}
				}
				it.Feeders[m.ID] = append(it.Feeders[m.ID], from)
			}
		}
	}
	// The eliminations first, in lane order, then the bronze match before the final.
	rank := func(it *WorkItem) int {
		switch it.Kind {
		case ItemBronze:
			return 1000
		case ItemFinal:
			return 1001
		}
		return it.Number
	}
	sort.SliceStable(bracket, func(i, j int) bool { return rank(bracket[i]) < rank(bracket[j]) })
	for _, it := range bracket {
		out = append(out, *it)
	}
	return out
}

// MatCount is how many mats the event has: what the organizer said, or as many as the
// disciplines ask for, so an event of one discipline has exactly that discipline's mats.
func MatCount(plan store.Plan, tournaments []store.Tournament) int {
	if plan.Mats > 0 {
		return plan.Mats
	}
	n := 1
	for _, t := range tournaments {
		n = max(n, t.Mats)
	}
	return n
}

// Place gives every item a mat and a place in its queue. An item the plan has already
// placed keeps its place; a new one -- just drawn, or redrawn, or on a mat that no longer
// exists -- goes to the end of the queue of the mat its lane maps to, so a discipline on
// its own runs exactly as it always did. Reports whether anything differs from the plan,
// so the caller writes only then.
//
// Projected items (phase 4) keep a place only once the organizer has given them one --
// moved or held to a time. Until then they go after every real item, worked out afresh
// each time, so the projected bracket of a discipline never sits in front of the pools
// it has just drawn. A placement the organizer made for a projected item is the real
// item's once it is drawn. So is a place from a suggestion the organizer applied.
func Place(items []WorkItem, plan store.Plan, mats int) (map[string]store.Placement, bool) {
	return PlaceIn(items, plan, mats, PlaceOrder{})
}

// PlaceOrder is what places new work in the order of the day (#136): the block each
// discipline runs in, its place in the event, whether finals are held to the end, and
// which items have started -- nothing is ever put in front of one of those.
type PlaceOrder struct {
	Session    func(slug string) int
	Position   func(slug string) int
	FinalsLast bool
	Started    func(it WorkItem) bool
}

// Held says an item is held to the end of the day: a bronze match or final, with finals
// last.
func (o PlaceOrder) Held(it WorkItem) bool {
	return o.FinalsLast && (it.Kind == ItemFinal || it.Kind == ItemBronze)
}

// key is where an item belongs in the day: by block, and within a block the pools before
// the eliminations before the bronze matches and finals, so disciplines side by side run
// stage by stage; the held finals after everything -- the later blocks' first, so the
// first discipline's final closes the day.
func (o PlaceOrder) key(it WorkItem) int {
	session, position := 0, 0
	if o.Session != nil {
		session = o.Session(it.Discipline)
	}
	if o.Position != nil {
		position = o.Position(it.Discipline)
	}
	if !o.Held(it) {
		stage := map[string]int{ItemPool: 0, ItemEliminations: 1, ItemBronze: 2, ItemFinal: 3}[it.Kind]
		return session*10 + stage
	}
	final := 0
	if it.Kind == ItemFinal {
		final = 1
	}
	return 1_000_000 + (999-session)*10_000 + position*10 + final
}

// PlaceIn is Place in the order of the day: new work goes after whatever on its mat comes
// before it in the day -- its block, then the held finals -- and never in front of an item
// already under way. A mat's queue is renumbered only where something was put in.
func PlaceIn(items []WorkItem, plan store.Plan, mats int, o PlaceOrder) (map[string]store.Placement, bool) {
	if mats < 1 {
		mats = 1
	}
	placed := make(map[string]store.Placement, len(items))
	byID := map[string]WorkItem{}
	var fresh, later []WorkItem
	for _, it := range items {
		byID[it.ID()] = it
		p, ok := plan.Items[it.ID()]
		chosen := p.Pinned || p.NotBefore != "" || p.Planned
		switch {
		case !ok || p.Mat < 1 || p.Mat > mats:
		case p.Stamp == it.Stamp && (!it.Projected || chosen):
			placed[it.ID()] = p
			continue
		case p.Stamp == "" && it.Stamp != "" && chosen:
			// Planned before the draw, by hand: the drawn item takes it over.
			p.Stamp = it.Stamp
			placed[it.ID()] = p
			continue
		}
		if it.Projected {
			later = append(later, it)
		} else {
			fresh = append(fresh, it)
		}
	}

	queues := map[int][]string{}
	for id, p := range placed {
		queues[p.Mat] = append(queues[p.Mat], id)
	}
	for mat := range queues {
		q := queues[mat]
		sort.SliceStable(q, func(i, j int) bool {
			if placed[q[i]].Seq != placed[q[j]].Seq {
				return placed[q[i]].Seq < placed[q[j]].Seq
			}
			return q[i] < q[j]
		})
	}
	started := map[string]bool{}
	isStarted := func(id string) bool {
		if o.Started == nil {
			return false
		}
		v, ok := started[id]
		if !ok {
			v = o.Started(byID[id])
			started[id] = v
		}
		return v
	}

	touched := map[int]bool{}
	for _, list := range [][]WorkItem{fresh, later} {
		sort.SliceStable(list, func(i, j int) bool { return o.key(list[i]) < o.key(list[j]) })
		for _, it := range list {
			mat := (max(it.Lane, 1)-1)%mats + 1
			if o.Held(it) {
				mat = 1
			}
			q := queues[mat]
			k := o.key(it)
			at := 0
			for i, id := range q {
				if o.key(byID[id]) <= k || isStarted(id) {
					at = i + 1
				}
			}
			q = append(q, "")
			copy(q[at+1:], q[at:])
			q[at] = it.ID()
			queues[mat] = q
			placed[it.ID()] = store.Placement{Mat: mat, Stamp: it.Stamp}
			touched[mat] = true
		}
	}
	for mat := range touched {
		for i, id := range queues[mat] {
			p := placed[id]
			p.Seq = i + 1
			placed[id] = p
		}
	}
	return placed, !samePlacements(placed, plan.Items)
}

func samePlacements(a, b map[string]store.Placement) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// --- the mats as the hall sees them ---------------------------------------------------

// MatsInput is one discipline, for BuildMats.
type MatsInput struct {
	Slug     string
	Name     string
	Snapshot Snapshot
	// Expected is how many the discipline expects to enter, for its projected items.
	Expected int
	// Session is the block of the day the discipline runs in, and FinalsLast says the
	// finals are held to the end (#136). A mat waits rather than start an item before its
	// block is open.
	Session    int
	FinalsLast bool
}

// Slot is one match in a mat's queue, with the discipline it belongs to and the names in
// it, so a score keeper or a display needs nothing else to show it.
type Slot struct {
	Discipline     string    `json:"discipline"`
	DisciplineName string    `json:"disciplineName"`
	Item           string    `json:"item"`
	Match          MatchView `json:"match"`
	Red            string    `json:"red"`
	Blue           string    `json:"blue"`
}

// MatView is one physical mat: everything queued on it, and what it is running now.
type MatView struct {
	Mat int `json:"mat"`
	// Current is the match the mat is on: the one its live score keeper is holding, or
	// the first match of its head item still to be fenced. Nil when the head item is
	// waiting -- for a semi-final's feeders, say -- or the mat has nothing queued.
	Current *Slot `json:"current,omitempty"`
	// Queue is every match of every item on the mat, in the order the mat runs them,
	// finished ones included.
	Queue []Slot `json:"queue"`
	// Waiting are matches of the head item that cannot be fenced yet, because a match
	// that feeds them is still to be decided.
	Waiting []Slot `json:"waiting,omitempty"`
}

// ItemView is one work item, placed, for the mat board.
type ItemView struct {
	ID             string `json:"id"`
	Discipline     string `json:"discipline"`
	DisciplineName string `json:"disciplineName"`
	Kind           string `json:"kind"`
	Number         int    `json:"number,omitempty"`
	Mat            int    `json:"mat"`
	// Position is the item's place in its mat's queue, from 1.
	Position int `json:"position"`
	// Status is "waiting" (its matches cannot start yet), "queued" (its block of the day
	// is not open yet, #136), "ready", "running", "done", or "planned" (not drawn yet).
	Status string `json:"status"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	// Movable is false for an item under way or finished: a mat never changes what it
	// is running in the middle of an item (§9, rules that keep it safe).
	Movable bool `json:"movable"`
	// Projected says the item is not drawn yet; Status is then "planned".
	Projected bool `json:"projected,omitempty"`
	// Pinned and NotBefore are the organizer's, from the plan (phase 4).
	Pinned    bool   `json:"pinned,omitempty"`
	NotBefore string `json:"notBefore,omitempty"`
}

// MatsView is the whole hall: every mat, and every work item on them.
type MatsView struct {
	Mats  []MatView  `json:"mats"`
	Items []ItemView `json:"items"`
	// Upcoming is what the mat screens show of what comes next (store.Screens).
	Upcoming string `json:"upcoming"`
}

// BuildMats lays every discipline's items on the event's mats. held names the match a
// live score keeper is holding on a mat, by discipline and match id, or "" -- a finished
// match stays on the mat, and its result on the screens, until Next match is pressed.
func BuildMats(inputs []MatsInput, placed map[string]store.Placement, mats int,
	held func(mat int) (discipline, matchID string)) MatsView {

	type entry struct {
		item     WorkItem
		place    store.Placement
		name     string
		slots    []Slot
		status   string
		done     int
		playable []int
		session  int
		held     bool
	}
	var all []*entry
	for _, in := range inputs {
		names := map[string]string{}
		for _, c := range in.Snapshot.Competitors {
			names[c.ID] = c.Name
		}
		views := map[string]MatchView{}
		for _, p := range in.Snapshot.Pools {
			for _, m := range p.Matches {
				views[m.ID] = m
			}
		}
		if in.Snapshot.Bracket != nil {
			for _, m := range in.Snapshot.Bracket.Matches {
				views[m.ID] = m
			}
		}
		entrants := Entrants(in.Snapshot.Competitors, in.Expected)
		for _, it := range PlanItems(in.Slug, in.Snapshot.Tournament, entrants) {
			p, ok := placed[it.ID()]
			if !ok {
				continue
			}
			e := &entry{item: it, place: p, name: in.Name, session: in.Session,
				held: in.FinalsLast && (it.Kind == ItemFinal || it.Kind == ItemBronze)}
			if it.Projected {
				e.status, e.done = "planned", 0
				e.slots = nil
				all = append(all, e)
				continue
			}
			running := false
			for _, id := range it.Matches {
				m, ok := views[id]
				if !ok {
					continue
				}
				e.slots = append(e.slots, Slot{
					Discipline: in.Slug, DisciplineName: in.Name, Item: it.ID(), Match: m,
					Red: names[m.Red], Blue: names[m.Blue],
				})
				switch {
				case m.Status == "complete":
					e.done++
					running = true
				case m.Status == "running":
					running = true
					e.playable = append(e.playable, len(e.slots)-1)
				case m.Red != "" && m.Blue != "":
					e.playable = append(e.playable, len(e.slots)-1)
				}
			}
			switch {
			case len(e.slots) > 0 && e.done == len(e.slots):
				e.status = "done"
			case running:
				e.status = "running"
			case len(e.playable) == 0:
				e.status = "waiting"
			default:
				e.status = "ready"
			}
			all = append(all, e)
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].place.Mat != all[j].place.Mat {
			return all[i].place.Mat < all[j].place.Mat
		}
		return all[i].place.Seq < all[j].place.Seq
	})

	// An item whose block is not open yet -- something of an earlier block is not done,
	// or, for a held final, anything that is not held -- waits, and so does its mat (#136).
	for _, e := range all {
		if e.status == "done" || e.status == "running" {
			continue
		}
		for _, o := range all {
			if o == e || o.held || o.status == "done" {
				continue
			}
			if e.held || o.session < e.session {
				if !e.item.Projected {
					e.status = "queued"
				}
				break
			}
		}
	}

	out := MatsView{Mats: make([]MatView, 0, mats), Items: []ItemView{}}
	for mat := 1; mat <= mats; mat++ {
		mv := MatView{Mat: mat, Queue: []Slot{}}
		position := 0
		var head *entry
		for _, e := range all {
			if e.place.Mat != mat {
				continue
			}
			position++
			total := len(e.slots)
			if e.item.Projected {
				total = len(e.item.Matches)
			}
			out.Items = append(out.Items, ItemView{
				ID: e.item.ID(), Discipline: e.item.Discipline, DisciplineName: e.name,
				Kind: e.item.Kind, Number: e.item.Number, Mat: mat, Position: position,
				Status: e.status, Done: e.done, Total: total,
				Movable:   e.status == "ready" || e.status == "waiting" || e.status == "planned" || e.status == "queued",
				Projected: e.item.Projected, Pinned: e.place.Pinned, NotBefore: e.place.NotBefore,
			})
			mv.Queue = append(mv.Queue, e.slots...)
			// Work that is not drawn yet is never what a mat waits for.
			if head == nil && e.status != "done" && !e.item.Projected {
				head = e
			}
		}
		if d, id := held(mat); id != "" {
			// A score keeper that does not say which discipline -- a client from before
			// there were several -- is matched on the id alone.
			for i := range mv.Queue {
				if (d == "" || mv.Queue[i].Discipline == d) && mv.Queue[i].Match.ID == id {
					s := mv.Queue[i]
					mv.Current = &s
					break
				}
			}
		}
		if head != nil && head.status != "queued" {
			if mv.Current == nil && len(head.playable) > 0 {
				s := head.slots[head.playable[0]]
				mv.Current = &s
			}
			for _, s := range head.slots {
				if s.Match.Status != "complete" && (s.Match.Red == "" || s.Match.Blue == "") {
					mv.Waiting = append(mv.Waiting, s)
				}
			}
		}
		out.Mats = append(out.Mats, mv)
	}
	return out
}

// --- moving items ---------------------------------------------------------------------

// ErrNotMovable is a move of an item under way or finished.
type ErrNotMovable struct{ Status string }

func (e ErrNotMovable) Error() string {
	if e.Status == "done" {
		return "that item is already fenced"
	}
	return "that item is under way; it can be moved once it is done, or what comes after it can"
}

// Move puts an item at index (from 0) in a mat's queue, and returns the new placements and
// whether anything changed. The index is clamped to the queue, and to after any item there
// that is under way or finished: nothing jumps in front of what a mat is already running.
// Dropping an item where it already is changes nothing.
func Move(view MatsView, placed map[string]store.Placement, id string, mat, index, mats int) (map[string]store.Placement, bool, error) {
	if mat < 1 || mat > mats {
		return placed, false, fmt.Errorf("there is no mat %d", mat)
	}
	var moving *ItemView
	for i := range view.Items {
		if view.Items[i].ID == id {
			moving = &view.Items[i]
		}
	}
	if moving == nil {
		return placed, false, fmt.Errorf("no work item %q", id)
	}
	if !moving.Movable {
		return placed, false, ErrNotMovable{Status: moving.Status}
	}

	var queue []ItemView
	for _, it := range view.Items {
		if it.Mat == mat && it.ID != id {
			queue = append(queue, it)
		}
	}
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].Position < queue[j].Position })
	floor := 0
	for i, it := range queue {
		if !it.Movable {
			floor = i + 1
		}
	}
	index = min(max(index, floor), len(queue))

	order := make([]string, 0, len(queue)+1)
	for _, it := range queue[:index] {
		order = append(order, it.ID)
	}
	order = append(order, id)
	for _, it := range queue[index:] {
		order = append(order, it.ID)
	}

	next := make(map[string]store.Placement, len(placed))
	for k, v := range placed {
		next[k] = v
	}
	changed := false
	for i, itemID := range order {
		p := next[itemID]
		if p.Mat != mat || p.Seq != i+1 {
			changed = true
		}
		p.Mat, p.Seq = mat, i+1
		next[itemID] = p
	}
	if changed {
		// Put there by hand, so a suggested plan keeps it on this mat (phase 4).
		p := next[id]
		p.Pinned = true
		next[id] = p
	}
	if !changed {
		return placed, false, nil
	}
	return next, true, nil
}

// Step moves an item one place earlier or later on its own mat, as the arrows on a card
// and on a pool's header do.
func Step(view MatsView, placed map[string]store.Placement, id string, earlier bool, mats int) (map[string]store.Placement, bool, error) {
	for _, it := range view.Items {
		if it.ID != id {
			continue
		}
		index := it.Position // the position from 1 is the index after it, from 0
		if earlier {
			index = it.Position - 2
		}
		if index < 0 {
			return placed, false, nil
		}
		return Move(view, placed, id, it.Mat, index, mats)
	}
	return placed, false, fmt.Errorf("no work item %q", id)
}

// OnEventMats makes a discipline's snapshot speak the event's mats: every pool and bracket
// match says the mat the plan runs it on, the pools come in that order, and EventMats says
// how many mats there are. The standings, the roster, the pool sheets and a person's page
// then name the right mat without knowing there is a plan. Mats -- which match each mat is
// on -- is emptied here, because only the whole hall can say: the discipline fills it from
// CurrentFrom over the hall's mats.
func OnEventMats(snap *Snapshot, slug string, placed map[string]store.Placement, mats int) {
	matOf := func(m store.Match) (int, int, bool) {
		p, ok := placed[slug+"/"+ItemKeyFor(m)]
		return p.Mat, p.Seq, ok
	}
	for i := range snap.Pools {
		p := &snap.Pools[i]
		key := slug + "/" + fmt.Sprintf("pool-%d", p.Number)
		if pl, ok := placed[key]; ok {
			p.Overridden = p.Overridden || pl.Mat != p.Mat
			p.Mat, p.Sequence = pl.Mat, pl.Seq
		}
		for j := range p.Matches {
			if mat, _, ok := matOf(p.Matches[j].Match); ok {
				p.Matches[j].Mat = mat
			}
		}
	}
	sort.SliceStable(snap.Pools, func(i, j int) bool {
		if snap.Pools[i].Mat != snap.Pools[j].Mat {
			return snap.Pools[i].Mat < snap.Pools[j].Mat
		}
		return snap.Pools[i].Sequence < snap.Pools[j].Sequence
	})
	if snap.Bracket != nil {
		for i := range snap.Bracket.Matches {
			if mat, _, ok := matOf(snap.Bracket.Matches[i].Match); ok {
				snap.Bracket.Matches[i].Mat = mat
			}
		}
	}
	snap.EventMats = mats
	snap.Mats = map[int]string{}
}

// CurrentFrom is the Mats half of what Hall answers, from a view of the hall already built.
func CurrentFrom(view MatsView, slug string) map[int]string {
	out := map[int]string{}
	for _, m := range view.Mats {
		out[m.Mat] = ""
		if m.Current != nil && m.Current.Discipline == slug {
			out[m.Mat] = m.Current.Match.ID
		}
	}
	return out
}
