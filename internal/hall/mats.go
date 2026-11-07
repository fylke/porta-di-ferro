package hall

import (
	"fmt"
	"sort"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The mats as the hall sees them: every mat's queue, and what it is running.

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
	// Name is what the organizer calls the mat, if anything, and Away the times it is not
	// available (#123).
	Name string       `json:"name,omitempty"`
	Away []store.Away `json:"away,omitempty"`
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
	// WithFinal is a bronze match let run at the same time as the final (#149).
	WithFinal bool `json:"withFinal,omitempty"`
}

// MatsView is the whole hall: every mat, and every work item on them.
type MatsView struct {
	Mats  []MatView  `json:"mats"`
	Items []ItemView `json:"items"`
	// Upcoming is what the mat screens show of what comes next (store.Screens).
	Upcoming string `json:"upcoming"`
	// Event is the event's identity: a tablet that kept logs for another forgets them (#141).
	Event string `json:"event,omitempty"`
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

// NameMats gives the hall's mats their names and the times they are away (#123).
func NameMats(view *MatsView, plan store.Plan) {
	for i := range view.Mats {
		n := view.Mats[i].Mat - 1
		if n >= 0 && n < len(plan.MatSettings) {
			view.Mats[i].Name = plan.MatSettings[n].Name
			view.Mats[i].Away = plan.MatSettings[n].Away
		}
	}
}

// BuildMats lays every discipline's items on the event's mats. held names the match a
// live score keeper is holding on a mat, by discipline and match id, or "" -- a finished
// match stays on the mat, and its result on the screens, until Next match is pressed.
func BuildMats(inputs []MatsInput, placed map[string]store.Placement, mats int,
	held func(mat int) (discipline, matchID string)) MatsView {

	var all []*matEntry
	for _, in := range inputs {
		all = append(all, entriesOf(in, placed)...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].place.Mat != all[j].place.Mat {
			return all[i].place.Mat < all[j].place.Mat
		}
		return all[i].place.Seq < all[j].place.Seq
	})
	holdForBlocks(all)

	out := MatsView{Mats: make([]MatView, 0, mats), Items: []ItemView{}}
	for mat := 1; mat <= mats; mat++ {
		mv := MatView{Mat: mat, Queue: []Slot{}}
		position := 0
		var head *matEntry
		for _, e := range all {
			if e.place.Mat != mat {
				continue
			}
			position++
			out.Items = append(out.Items, e.itemView(mat, position))
			mv.Queue = append(mv.Queue, e.slots...)
			// Work that is not drawn yet is never what a mat waits for.
			if head == nil && e.status != "done" && !e.item.Projected {
				head = e
			}
		}
		mv.Current = heldSlot(mv.Queue, mat, held)
		if head != nil && head.status != "queued" {
			if mv.Current == nil && len(head.playable) > 0 {
				s := head.slots[head.playable[0]]
				mv.Current = &s
			}
			mv.Waiting = head.waiting()
		}
		out.Mats = append(out.Mats, mv)
	}
	return out
}

// matEntry is one work item on the way to a mat's queue.
type matEntry struct {
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

// entriesOf is a discipline's placed items, each with its matches and how far it has got.
func entriesOf(in MatsInput, placed map[string]store.Placement) []*matEntry {
	names := map[string]string{}
	for _, c := range in.Snapshot.Competitors {
		names[c.ID] = c.Name
	}
	views := in.Snapshot.matchesByID()
	var out []*matEntry
	for _, it := range PlanItems(in.Slug, in.Snapshot.Tournament, Entrants(in.Snapshot.Competitors, in.Expected)) {
		p, ok := placed[it.ID()]
		if !ok {
			continue
		}
		e := &matEntry{item: it, place: p, name: in.Name, session: in.Session,
			held: in.FinalsLast && (it.Kind == ItemFinal || it.Kind == ItemBronze)}
		if it.Projected {
			e.status = "planned"
		} else {
			e.fill(in, views, names)
		}
		out = append(out, e)
	}
	return out
}

// fill gives a drawn item its slots, and says where it stands from their matches.
func (e *matEntry) fill(in MatsInput, views map[string]MatchView, names map[string]string) {
	running := false
	for _, id := range e.item.Matches {
		m, ok := views[id]
		if !ok {
			continue
		}
		e.slots = append(e.slots, Slot{
			Discipline: in.Slug, DisciplineName: in.Name, Item: e.item.ID(), Match: m,
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
}

// holdForBlocks queues an item whose block is not open yet -- something of an earlier block
// is not done, or, for a held final, anything that is not held. Its mat waits with it (#136).
func holdForBlocks(all []*matEntry) {
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
}

func (e *matEntry) itemView(mat, position int) ItemView {
	total := len(e.slots)
	if e.item.Projected {
		total = len(e.item.Matches)
	}
	return ItemView{
		ID: e.item.ID(), Discipline: e.item.Discipline, DisciplineName: e.name,
		Kind: e.item.Kind, Number: e.item.Number, Mat: mat, Position: position,
		Status: e.status, Done: e.done, Total: total,
		Movable:   e.status == "ready" || e.status == "waiting" || e.status == "planned" || e.status == "queued",
		Projected: e.item.Projected, Pinned: e.place.Pinned, NotBefore: e.place.NotBefore, WithFinal: e.place.WithFinal,
	}
}

// waiting are the item's matches that cannot be fenced until a feeder is decided.
func (e *matEntry) waiting() []Slot {
	var out []Slot
	for _, s := range e.slots {
		if s.Match.Status != "complete" && (s.Match.Red == "" || s.Match.Blue == "") {
			out = append(out, s)
		}
	}
	return out
}

// heldSlot is the slot a live score keeper on the mat is holding. A score keeper that does
// not say which discipline -- a client from before there were several -- is matched on the
// id alone.
func heldSlot(queue []Slot, mat int, held func(mat int) (discipline, matchID string)) *Slot {
	d, id := held(mat)
	if id == "" {
		return nil
	}
	for i := range queue {
		if (d == "" || queue[i].Discipline == d) && queue[i].Match.ID == id {
			s := queue[i]
			return &s
		}
	}
	return nil
}
