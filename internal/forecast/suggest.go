package forecast

import (
	"sort"
	"time"
)

// Suggesting a plan (proposal §10, "Suggesting a plan"): greedy list scheduling. Items are
// taken in priority order -- a discipline's pools before its bracket, the discipline with
// the most still to fence first -- and each goes on the mat where it can start soonest,
// after what it waits for, its hold time and the day's start. Fast, deterministic, and
// explainable in a sentence per item: "Sabre pool 2 is on mat 3 because it was free at
// 10:40".
//
// It never overrules the organizer. What a mat is running, or has run, stays where it is;
// an item pinned to a mat stays on that mat; and the suggestion is only that until the
// organizer applies it.

// Move is one item the suggestion puts somewhere else.
type Move struct {
	ID               string
	FromMat, FromSeq int
	ToMat, ToSeq     int
	// Start is when it would start there: the reason, in a sentence.
	Start time.Time
}

// Suggestion is a proposed order for every mat, the moves that make it from the plan, and
// when the day would end with it.
type Suggestion struct {
	// Order is each mat's items in the proposed order, by id.
	Order map[int][]string
	Moves []Move
	End   time.Time
	// Before is when the day ends with the plan as it is.
	Before time.Time
}

// Suggest proposes a plan for the items not yet under way, over mats mats.
func Suggest(in Input, mats int) Suggestion {
	if mats < 1 {
		mats = 1
	}
	before := Run(in)
	if in.Timings.Match <= 0 {
		in.Timings.Match = Defaults.Match
	}
	s := newSuggester(in, mats)
	fixed, free := s.split()
	s.keep(fixed)
	s.byPriority(free)
	for len(free) > 0 {
		pick := s.nextReady(free)
		it := free[pick]
		free = append(free[:pick], free[pick+1:]...)
		s.place(it)
	}
	return s.suggestion(before.End)
}

// busySpan is when a person is fencing, and on which mat.
type busySpan struct {
	start, end time.Time
	mat        int
}

// suggester is a suggested plan as it is built: what each mat has so far, and when it is
// free.
type suggester struct {
	in     Input
	mats   int
	pace   map[int]Pace
	scales map[string]float64
	live   bool
	deps   func(Item) (wait, pause []string)
	order  map[int][]string
	cursor map[int]time.Time
	ends   map[string]time.Time
	// busy is when each person is fencing, so nothing is suggested that puts them in two
	// places at once (#136).
	busy map[string][]busySpan
}

func newSuggester(in Input, mats int) *suggester {
	return &suggester{
		in: in, mats: mats,
		pace:   Paces(in),
		scales: eliminationScales(in),
		live:   anyBegun(in.Items),
		deps:   dependencies(in.Items),
		order:  map[int][]string{},
		cursor: map[int]time.Time{},
		ends:   map[string]time.Time{},
		busy:   map[string][]busySpan{},
	}
}

// split parts what has begun, which stays, from what is free to be placed.
func (s *suggester) split() (fixed, free []Item) {
	for _, it := range s.in.Items {
		if it.Begun() {
			fixed = append(fixed, it)
		} else {
			free = append(free, it)
		}
	}
	return fixed, free
}

// keep leaves what is under way where it is: timed as the forecast times it, with each mat
// free once its last fixed item ends.
func (s *suggester) keep(fixed []Item) {
	fin := s.in
	fin.Items = fixed
	done, _ := simulate(fin, s.pace, s.live)
	sort.SliceStable(fixed, func(i, j int) bool {
		if fixed[i].Mat != fixed[j].Mat {
			return fixed[i].Mat < fixed[j].Mat
		}
		return fixed[i].Seq < fixed[j].Seq
	})
	for _, it := range fixed {
		s.order[it.Mat] = append(s.order[it.Mat], it.ID)
		sp := done.items[it.ID]
		s.ends[it.ID] = sp.end
		if sp.end.After(s.cursor[it.Mat]) {
			s.cursor[it.Mat] = sp.end
		}
		s.markBusy(it, sp.start, sp.end, it.Mat)
	}
}

func (s *suggester) markBusy(it Item, start, end time.Time, mat int) {
	for _, p := range it.People {
		s.busy[p] = append(s.busy[p], busySpan{start, end, mat})
	}
}

// stageRank is an item's place in the order things are placed: pools, then eliminations,
// then bronze matches and finals, then whatever is held to the end.
func stageRank(it Item) int {
	switch {
	case it.Held:
		return 3
	case it.Kind == KindPool:
		return 0
	case it.Kind == KindEliminations:
		return 1
	}
	return 2
}

// byPriority sorts the free items into the order they are placed in. The order never depends
// on where the plan has things now, so suggesting again right after applying a suggestion
// proposes nothing: ties go to the mat an item is on.
func (s *suggester) byPriority(free []Item) {
	remaining := map[string]int{}
	for _, it := range free {
		remaining[it.Discipline] += len(it.Matches)
	}
	index := map[string]int{}
	for i, it := range s.in.Items {
		index[it.ID] = i
	}
	sort.SliceStable(free, func(i, j int) bool {
		a, b := free[i], free[j]
		if stageRank(a) != stageRank(b) {
			return stageRank(a) < stageRank(b)
		}
		// The finals held to the end run the later blocks' first, so the first
		// discipline's final closes the day (#136).
		if a.Held && b.Held {
			if a.Session != b.Session {
				return a.Session > b.Session
			}
			return index[a.ID] < index[b.ID]
		}
		if remaining[a.Discipline] != remaining[b.Discipline] {
			return remaining[a.Discipline] > remaining[b.Discipline]
		}
		return index[a.ID] < index[b.ID]
	})
}

// nextReady is the first free item whose dependencies are placed. When there is none, what
// it waits for is nowhere on the plan: the first is timed anyway.
func (s *suggester) nextReady(free []Item) int {
	for i, it := range free {
		wait, pause := s.deps(it)
		if s.allPlaced(wait) && s.allPlaced(pause) {
			return i
		}
	}
	return 0
}

func (s *suggester) allPlaced(ids []string) bool {
	for _, id := range ids {
		if _, ok := s.ends[id]; !ok {
			return false
		}
	}
	return true
}

// paceFor is how fast an item would go on a mat.
func (s *suggester) paceFor(mat int, it Item) Pace {
	p := s.pace[mat]
	if p.Match <= 0 {
		p = s.pace[0]
	}
	return calibratedPace(it, s.in.Timings, p, s.scales)
}

// timeOn is when an item would run on a mat, and -- when somebody in it is fencing
// elsewhere at that time -- the mat of the clash: waiting for a fencer is not something a
// queue can promise, so such a place is a last resort.
func (s *suggester) timeOn(mat int, it Item) (time.Time, time.Time, int) {
	p := s.paceFor(mat, it)
	ready := s.readyOn(mat, it, p)
	// Later, until nobody in it is fencing somewhere else at the time.
	clashMat := 0
	for tries := 0; ; tries++ {
		start, end := s.lay(mat, it, p, ready)
		clash, on := s.clashWith(it, start, end)
		if !clash.IsZero() {
			clashMat = on
		}
		if clash.IsZero() || tries > 50 {
			return start, end, clashMat
		}
		ready = clash.Add(p.Changeover)
	}
}

// readyOn is when an item could begin on a mat, before asking whether its fencers are free.
func (s *suggester) readyOn(mat int, it Item, p Pace) time.Time {
	ready := s.in.Timings.Start
	if c := s.cursor[mat]; c.After(ready) {
		ready = c.Add(p.Changeover)
	}
	wait, pause := s.deps(it)
	if latest := waitsUntil(wait, pause, s.in.Timings.BeforeElims, func(id string) time.Time { return s.ends[id] }); latest.After(ready) {
		ready = latest
	}
	if it.NotBefore.After(ready) {
		ready = it.NotBefore
	}
	if s.live && s.in.Now.After(ready) {
		ready = s.in.Now
	}
	return ready
}

// lay runs an item's matches one after another from a time, around the mat's breaks.
func (s *suggester) lay(mat int, it Item, p Pace, ready time.Time) (time.Time, time.Time) {
	start, at := time.Time{}, ready
	for i := range it.Matches {
		t := at
		if i > 0 {
			t = t.Add(p.Changeover)
		}
		t = afterBreaks(t, s.in.breaksOn(mat))
		if start.IsZero() {
			start = t
		}
		at = t.Add(p.Match)
	}
	if start.IsZero() {
		start = ready
	}
	return start, at
}

// clashWith is when the last of an item's fencers who is elsewhere between two times is free,
// and on which mat they are; zero and 0 when all of them are free.
func (s *suggester) clashWith(it Item, start, end time.Time) (time.Time, int) {
	var clash time.Time
	mat := 0
	for _, person := range it.People {
		for _, b := range s.busy[person] {
			if b.start.Before(end) && start.Before(b.end) && b.end.After(clash) {
				clash, mat = b.end, b.mat
			}
		}
	}
	return clash, mat
}

// candidates are the mats an item may go on.
func (s *suggester) candidates(it Item) []int {
	switch {
	case it.Pinned && it.Mat >= 1 && it.Mat <= s.mats:
		return []int{it.Mat}
	case it.Held:
		// The finals are held for mat 1, one after another (#136).
		return []int{1}
	}
	out := make([]int, 0, s.mats)
	for m := 1; m <= s.mats; m++ {
		out = append(out, m)
	}
	return out
}

// place puts an item on the mat where it can start soonest.
func (s *suggester) place(it Item) {
	candidates := s.candidates(it)
	best, bestStart, bestEnd := 0, time.Time{}, time.Time{}
	clashes := map[int]int{}
	for _, m := range candidates {
		start, end, clash := s.timeOn(m, it)
		if clash != 0 {
			clashes[m] = clash
			continue
		}
		// Soonest wins; on a tie the mat it is on already, so nothing moves for nothing.
		if best == 0 || start.Before(bestStart) || (start.Equal(bestStart) && m == it.Mat && best != it.Mat) {
			best, bestStart, bestEnd = m, start, end
		}
	}
	if best == 0 {
		best = lastResort(candidates, clashes)
		bestStart, bestEnd, _ = s.timeOn(best, it)
	}
	s.order[best] = append(s.order[best], it.ID)
	s.ends[it.ID] = bestEnd
	s.cursor[best] = bestEnd
	s.markBusy(it, bestStart, bestEnd, best)
}

// lastResort is where an item goes when every mat would have to wait for one of its fencers:
// behind the clash, on that mat, where it cannot start before they are free.
func lastResort(candidates []int, clashes map[int]int) int {
	for _, m := range candidates {
		if c := clashes[m]; c != 0 {
			return c
		}
	}
	return candidates[0]
}

// suggestion is the plan built, timed again by the forecast itself, and what moves to make it.
func (s *suggester) suggestion(before time.Time) Suggestion {
	type place struct{ mat, seq int }
	placed := map[string]place{}
	for mat, ids := range s.order {
		for i, id := range ids {
			placed[id] = place{mat, i + 1}
		}
	}
	proposed := s.in
	proposed.Items = append([]Item{}, s.in.Items...)
	for i := range proposed.Items {
		if p, ok := placed[proposed.Items[i].ID]; ok {
			proposed.Items[i].Mat, proposed.Items[i].Seq = p.mat, p.seq
		}
	}
	result := Run(proposed)
	startOf := map[string]time.Time{}
	for _, sp := range result.Items {
		startOf[sp.ID] = sp.Start
	}
	out := Suggestion{Order: s.order, Before: before, End: result.End}
	for _, it := range s.in.Items {
		p, ok := placed[it.ID]
		if !ok {
			continue
		}
		// Position in the queue, from 1, against where the plan has it now.
		if now := position(s.in.Items, it); p.mat != it.Mat || p.seq != now {
			out.Moves = append(out.Moves, Move{ID: it.ID, FromMat: it.Mat, FromSeq: now,
				ToMat: p.mat, ToSeq: p.seq, Start: startOf[it.ID]})
		}
	}
	sort.Slice(out.Moves, func(i, j int) bool { return out.Moves[i].Start.Before(out.Moves[j].Start) })
	return out
}

// position is an item's place in its mat's queue now, from 1.
func position(items []Item, it Item) int {
	n := 1
	for _, o := range items {
		if o.Mat == it.Mat && o.ID != it.ID && o.Seq < it.Seq {
			n++
		}
	}
	return n
}

// dependencies says what each item waits for. A final also waits for its bronze match,
// unless the organizer lets the two run at once (#149). Within a discipline: its eliminations wait
// for all its pools, and its bronze match and final for its eliminations -- or for its
// pools, when its bracket is a final alone; pause lists the pools, after which the break
// before the bracket applies. Across the event (#136): an item waits for every item of the
// blocks before its own, and a held final for everything that is not held.
func dependencies(items []Item) func(Item) (wait, pause []string) {
	byDisc := map[string]map[string][]string{}
	for _, it := range items {
		if byDisc[it.Discipline] == nil {
			byDisc[it.Discipline] = map[string][]string{}
		}
		byDisc[it.Discipline][it.Kind] = append(byDisc[it.Discipline][it.Kind], it.ID)
		if it.Kind == KindBronze && !it.WithFinal {
			byDisc[it.Discipline]["bronze-first"] = append(byDisc[it.Discipline]["bronze-first"], it.ID)
		}
	}
	return func(it Item) ([]string, []string) {
		k := byDisc[it.Discipline]
		var wait, pause []string
		switch it.Kind {
		case KindEliminations:
			pause = k[KindPool]
		case KindBronze, KindFinal:
			if len(k[KindEliminations]) > 0 {
				wait = append(wait, k[KindEliminations]...)
			} else {
				pause = k[KindPool]
			}
			if it.Kind == KindFinal {
				wait = append(wait, k["bronze-first"]...)
			}
		}
		for _, o := range items {
			if o.Held || o.ID == it.ID {
				continue
			}
			if it.Held || o.Session < it.Session {
				wait = append(wait, o.ID)
			}
		}
		return wait, pause
	}
}
