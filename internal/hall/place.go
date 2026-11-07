package hall

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Where each work item goes: the plan, kept up to date as work is drawn.

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

// Persisted is what of the placements is written down (#129): every drawn item, and an
// undrawn discipline's projected item only where the organizer chose its place. The rest
// is placed again on every read, so typing names at the desk -- which changes how many
// pools there would be -- writes nothing. Each mat's places are counted from 1 again
// without the projected items, so their coming and going does not renumber the others.
func Persisted(placed map[string]store.Placement, projected map[string]bool) map[string]store.Placement {
	out := make(map[string]store.Placement, len(placed))
	queues := map[int][]string{}
	for id, p := range placed {
		if projected[id] && !p.Pinned && p.NotBefore == "" && !p.Planned {
			continue
		}
		out[id] = p
		queues[p.Mat] = append(queues[p.Mat], id)
	}
	for _, q := range queues {
		sort.Slice(q, func(i, j int) bool {
			if out[q[i]].Seq != out[q[j]].Seq {
				return out[q[i]].Seq < out[q[j]].Seq
			}
			return q[i] < q[j]
		})
		for i, id := range q {
			p := out[id]
			p.Seq = i + 1
			out[id] = p
		}
	}
	return out
}

// ItemFlags are what the card menu sets on a work item: a pin, a time to hold it to, and
// for a bronze match whether it may run at the same time as the final (#149).
type ItemFlags struct {
	Pinned    *bool   `json:"pinned"`
	NotBefore *string `json:"notBefore"`
	WithFinal *bool   `json:"withFinal"`
}

// Any says the request sets a flag.
func (f ItemFlags) Any() bool { return f.Pinned != nil || f.NotBefore != nil || f.WithFinal != nil }

// SetItemFlags pins or unpins a work item, holds it to a time, or lets a bronze match run
// with its final, in the placements.
func SetItemFlags(placed map[string]store.Placement, id string, f ItemFlags) (map[string]store.Placement, error) {
	p, ok := placed[id]
	if !ok {
		return placed, errors.New("no work item " + id)
	}
	if f.NotBefore != nil {
		if _, ok := ParseClock(time.Now(), *f.NotBefore); *f.NotBefore != "" && !ok {
			return placed, errors.New("write the time as 16:30")
		}
		p.NotBefore = *f.NotBefore
	}
	if f.Pinned != nil {
		p.Pinned = *f.Pinned
	}
	if f.WithFinal != nil {
		if !strings.HasSuffix(id, "/"+ItemBronze) {
			return placed, errors.New("only a bronze match can run with the final")
		}
		p.WithFinal = *f.WithFinal
	}
	next := make(map[string]store.Placement, len(placed))
	for k, v := range placed {
		next[k] = v
	}
	next[id] = p
	return next, nil
}

// Place gives every item a mat and a place in its queue, in the order of the day. An item
// the plan has already placed keeps its place; a new one -- just drawn, or redrawn, or on a
// mat that no longer exists -- goes to the mat its lane maps to, after whatever on that mat
// comes before it in the day -- its block, then the held finals -- and never in front of an
// item already under way, so a discipline on its own runs exactly as it always did. A mat's
// queue is renumbered only where something was put in. Reports whether anything differs
// from the plan, so the caller writes only then.
//
// Projected items (phase 4) keep a place only once the organizer has given them one --
// moved or held to a time. Until then they go after every real item, worked out afresh
// each time, so the projected bracket of a discipline never sits in front of the pools
// it has just drawn. A placement the organizer made for a projected item is the real
// item's once it is drawn. So is a place from a suggestion the organizer applied.
func Place(items []WorkItem, plan store.Plan, mats int, o PlaceOrder) (map[string]store.Placement, bool) {
	if mats < 1 {
		mats = 1
	}
	placed, fresh, later := sortOutPlaced(items, plan, mats)
	byID := map[string]WorkItem{}
	for _, it := range items {
		byID[it.ID()] = it
	}
	q := placing{order: o, byID: byID, placed: placed, queues: queuesOf(placed), touched: map[int]bool{}, started: map[string]bool{}}
	for _, list := range [][]WorkItem{fresh, later} {
		sort.SliceStable(list, func(i, j int) bool { return o.key(list[i]) < o.key(list[j]) })
		for _, it := range list {
			q.put(it, mats)
		}
	}
	q.renumber()
	return placed, !samePlacements(placed, plan.Items)
}

// sortOutPlaced parts what keeps its place from what needs one: real work, and then work
// that is only projected.
func sortOutPlaced(items []WorkItem, plan store.Plan, mats int) (placed map[string]store.Placement, fresh, later []WorkItem) {
	placed = make(map[string]store.Placement, len(items))
	for _, it := range items {
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
	return placed, fresh, later
}

// queuesOf is each mat's items in their order.
func queuesOf(placed map[string]store.Placement) map[int][]string {
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
	return queues
}

// placing is new work going onto the mats' queues.
type placing struct {
	order   PlaceOrder
	byID    map[string]WorkItem
	placed  map[string]store.Placement
	queues  map[int][]string
	touched map[int]bool
	started map[string]bool
}

func (q placing) isStarted(id string) bool {
	if q.order.Started == nil {
		return false
	}
	v, ok := q.started[id]
	if !ok {
		v = q.order.Started(q.byID[id])
		q.started[id] = v
	}
	return v
}

// put inserts an item behind whatever on its mat comes before it, or is under way.
func (q placing) put(it WorkItem, mats int) {
	mat := (max(it.Lane, 1)-1)%mats + 1
	if q.order.Held(it) {
		mat = 1
	}
	queue := q.queues[mat]
	k := q.order.key(it)
	at := 0
	for i, id := range queue {
		if q.order.key(q.byID[id]) <= k || q.isStarted(id) {
			at = i + 1
		}
	}
	queue = append(queue, "")
	copy(queue[at+1:], queue[at:])
	queue[at] = it.ID()
	q.queues[mat] = queue
	q.placed[it.ID()] = store.Placement{Mat: mat, Stamp: it.Stamp}
	q.touched[mat] = true
}

func (q placing) renumber() {
	for mat := range q.touched {
		for i, id := range q.queues[mat] {
			p := q.placed[id]
			p.Seq = i + 1
			q.placed[id] = p
		}
	}
}
