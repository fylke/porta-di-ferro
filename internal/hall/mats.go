package hall

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
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
	// Held is the item the mat has stopped in front of, waiting for the organizer to
	// release it, and HeldFor why: HeldByOrganizer or HeldAtDiscipline.
	Held    string `json:"held,omitempty"`
	HeldFor string `json:"heldFor,omitempty"`
}

// Why a mat stops in front of an item.
const (
	// HeldByOrganizer: the organizer held the item from its card.
	HeldByOrganizer = "organizer"
	// HeldAtDiscipline: the item is another discipline's than the one the mat last ran.
	HeldAtDiscipline = "discipline"
)

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
	// is not open yet, #136), "held" (its mat waits for the organizer to release it),
	// "ready", "running", "done", or "planned" (not drawn yet).
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
	// Hold is the organizer's hold on the item: its mat stops in front of it.
	Hold bool `json:"hold,omitempty"`
	// HeldFor is why the mat has stopped in front of the item, when Status is "held".
	HeldFor string `json:"heldFor,omitempty"`
}

// MatsView is the whole hall: every mat, and every work item on them.
type MatsView struct {
	Mats  []MatView  `json:"mats"`
	Items []ItemView `json:"items"`
	// Upcoming is what the mat screens show of what comes next (store.Screens).
	Upcoming string `json:"upcoming"`
	// Event is the event's identity: a tablet that kept logs for another forgets them (#141).
	Event string `json:"event,omitempty"`
	// Halt is a global timeout in force (#157): every score keeper stops its clock and
	// every screen says so, until the organizer ends it.
	Halt *store.Halt `json:"halt,omitempty"`
}

// SetHalt calls a global timeout, or ends it (#157). Calling one already in force keeps its
// start; the reason is the organizer's, trimmed and kept short.
func SetHalt(cur *store.Halt, on bool, reason string, now time.Time) *store.Halt {
	if !on {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 80 {
		reason = reason[:80]
	}
	if cur != nil {
		next := *cur
		next.Reason = reason
		return &next
	}
	return &store.Halt{Since: now.Format(time.RFC3339), Reason: reason}
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
	snap.UsableMats = EffectiveMatCount(mats, snap.Tournament.Mats)
	// The eliminations' mats are answered for the hall's mats, not the discipline's: a
	// discipline set up for two in a hall cut to one has one to offer.
	if size := tournament.BracketSize(len(snap.Overall)); size > 0 {
		t := snap.OnHall()
		snap.ElimMats = tournament.EliminationMatsFor(t, size)
		snap.ElimMatsSuggested = tournament.EliminationMats(t.Mats, size)
	}
	snap.Mats = map[int]string{}
}

// CurrentFrom is the Mats half of what Hall answers, from a view of the hall already built.
// REQ: FR_DISPLAY_03
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
// REQ: FR_MATS_02, DC_ARCH_02
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
		// head is the first item the mat has still to run, and prev the drawn item it ran
		// before that: what decides whether it may go on by itself.
		var on []*matEntry
		var head, prev *matEntry
		for _, e := range all {
			if e.place.Mat != mat {
				continue
			}
			on = append(on, e)
			// Work that is not drawn yet is never what a mat waits for.
			if head == nil && e.status != "done" && !e.item.Projected {
				head = e
			}
			if head == nil && !e.item.Projected {
				prev = e
			}
		}
		if head != nil {
			if why := stopsBefore(head, prev); why != "" {
				head.status, head.heldFor = "held", why
				mv.Held, mv.HeldFor = head.item.ID(), why
			}
		}
		for i, e := range on {
			out.Items = append(out.Items, e.itemView(mat, i+1))
			mv.Queue = append(mv.Queue, e.slots...)
		}
		mv.Current = heldSlot(mv.Queue, mat, held)
		if head != nil && head.status != "queued" && head.status != "held" {
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
	heldFor  string
}

// stopsBefore says why a mat stops in front of its head item rather than start it on its
// own, or "" when it goes on: the organizer held the item, or it is another discipline's
// than the one the mat ran last and the organizer has not released it. An item under way
// is never stopped -- a mat does not change what it runs in the middle of one.
func stopsBefore(head, prev *matEntry) string {
	if head.status != "ready" && head.status != "waiting" {
		return ""
	}
	if head.place.Hold {
		return HeldByOrganizer
	}
	if prev != nil && !ShouldAdvanceToDiscipline(prev.item, head.item, head.place) {
		return HeldAtDiscipline
	}
	return ""
}

// ShouldAdvanceToDiscipline says a mat that has just run from may go on to next by itself:
// within one discipline it may, and into another only once the organizer has released next.
// Pools, eliminations and finals of one discipline follow each other; another discipline's
// work is somebody else's day, and the organizer decides when it starts.
// REQ: FR_MATS_02
func ShouldAdvanceToDiscipline(from, next WorkItem, p store.Placement) bool {
	return from.Discipline == next.Discipline || p.Released
}

// entriesOf is a discipline's placed items, each with its matches and how far it has got.
func entriesOf(in MatsInput, placed map[string]store.Placement) []*matEntry {
	names := map[string]string{}
	for _, c := range in.Snapshot.Competitors {
		names[c.ID] = c.Name
	}
	views := in.Snapshot.matchesByID()
	var out []*matEntry
	for _, it := range PlanItems(in.Slug, in.Snapshot.OnHall(), Entrants(in.Snapshot.Competitors, in.Expected)) {
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
		case m.Void:
			// A withdrawn competitor's match: nothing to fence, nothing to wait for (#162).
			e.done++
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
		Movable:   e.status == "ready" || e.status == "waiting" || e.status == "planned" || e.status == "queued" || e.status == "held",
		Projected: e.item.Projected, Pinned: e.place.Pinned, NotBefore: e.place.NotBefore, WithFinal: e.place.WithFinal,
		Hold: e.place.Hold, HeldFor: e.heldFor,
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
