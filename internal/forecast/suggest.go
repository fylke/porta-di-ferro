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
	pace := Paces(in)
	live := false
	for _, it := range in.Items {
		for _, m := range it.Matches {
			live = live || m.begun()
		}
	}

	var fixed, free []Item
	for _, it := range in.Items {
		started := false
		for _, m := range it.Matches {
			started = started || m.begun()
		}
		if started {
			fixed = append(fixed, it)
		} else {
			free = append(free, it)
		}
	}

	// What is under way stays: it is timed as the forecast times it, and each mat is free
	// once its last fixed item ends.
	fin := in
	fin.Items = fixed
	done, _ := simulate(fin, pace, live)
	order := map[int][]string{}
	cursor := map[int]time.Time{}
	sort.SliceStable(fixed, func(i, j int) bool {
		if fixed[i].Mat != fixed[j].Mat {
			return fixed[i].Mat < fixed[j].Mat
		}
		return fixed[i].Seq < fixed[j].Seq
	})
	ends := map[string]time.Time{}
	// When each person is busy fencing, so nothing is suggested that puts them in two
	// places at once (#136).
	type busySpan struct {
		start, end time.Time
		mat        int
	}
	busy := map[string][]busySpan{}
	for _, it := range fixed {
		order[it.Mat] = append(order[it.Mat], it.ID)
		s := done.items[it.ID]
		ends[it.ID] = s.end
		if s.end.After(cursor[it.Mat]) {
			cursor[it.Mat] = s.end
		}
		for _, p := range it.People {
			busy[p] = append(busy[p], busySpan{s.start, s.end, it.Mat})
		}
	}

	deps := dependencies(in.Items)
	remaining := map[string]int{}
	for _, it := range free {
		remaining[it.Discipline] += len(it.Matches)
	}
	rank := func(it Item) int {
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
	// The order never depends on where the plan has things now, so suggesting again right
	// after applying a suggestion proposes nothing: ties go to the mat an item is on.
	index := map[string]int{}
	for i, it := range in.Items {
		index[it.ID] = i
	}
	sort.SliceStable(free, func(i, j int) bool {
		a, b := free[i], free[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
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

	// timeOn is when an item would run on a mat, and -- when somebody in it is fencing
	// elsewhere at that time -- the mat of the clash: waiting for a fencer is not
	// something a queue can promise, so such a place is a last resort.
	timeOn := func(mat int, it Item) (time.Time, time.Time, int) {
		p := pace[mat]
		if p.Match <= 0 {
			p = Pace{Match: in.Timings.Match, Changeover: in.Timings.Changeover}
		}
		ready := in.Timings.Start
		if c := cursor[mat]; c.After(ready) {
			ready = c.Add(p.Changeover)
		}
		wait, pause := deps(it)
		var latest, pools time.Time
		for _, id := range wait {
			if e := ends[id]; e.After(latest) {
				latest = e
			}
		}
		for _, id := range pause {
			if e := ends[id]; e.After(pools) {
				pools = e
			}
		}
		if !pools.IsZero() {
			pools = pools.Add(in.Timings.BeforeElims)
		}
		if pools.After(latest) {
			latest = pools
		}
		if latest.After(ready) {
			ready = latest
		}
		if it.NotBefore.After(ready) {
			ready = it.NotBefore
		}
		if live && in.Now.After(ready) {
			ready = in.Now
		}
		lay := func(ready time.Time) (time.Time, time.Time) {
			start, at := time.Time{}, ready
			for i := range it.Matches {
				s := at
				if i > 0 {
					s = s.Add(p.Changeover)
				}
				s = afterBreaks(s, in.Breaks)
				if start.IsZero() {
					start = s
				}
				at = s.Add(p.Match)
			}
			if start.IsZero() {
				start = ready
			}
			return start, at
		}
		// Later, until nobody in it is fencing somewhere else at the time.
		clashMat := 0
		for tries := 0; ; tries++ {
			start, end := lay(ready)
			var clash time.Time
			for _, person := range it.People {
				for _, b := range busy[person] {
					if b.start.Before(end) && start.Before(b.end) && b.end.After(clash) {
						clash, clashMat = b.end, b.mat
					}
				}
			}
			if clash.IsZero() || tries > 50 {
				return start, end, clashMat
			}
			ready = clash.Add(p.Changeover)
		}
	}

	placedAt := map[string]time.Time{}
	for len(free) > 0 {
		pick := -1
		for i, it := range free {
			wait, pause := deps(it)
			ids := append(append([]string{}, wait...), pause...)
			ready := true
			for _, id := range ids {
				if _, ok := ends[id]; !ok {
					ready = false
				}
			}
			if ready {
				pick = i
				break
			}
		}
		if pick < 0 {
			pick = 0 // what it waits for is nowhere on the plan; time it anyway
		}
		it := free[pick]
		free = append(free[:pick], free[pick+1:]...)

		candidates := []int{}
		switch {
		case it.Pinned && it.Mat >= 1 && it.Mat <= mats:
			candidates = append(candidates, it.Mat)
		case it.Held:
			// The finals are held for mat 1, one after another (#136).
			candidates = append(candidates, 1)
		default:
			for m := 1; m <= mats; m++ {
				candidates = append(candidates, m)
			}
		}
		best, bestStart, bestEnd := 0, time.Time{}, time.Time{}
		clashes := map[int]int{}
		for _, m := range candidates {
			s, e, clash := timeOn(m, it)
			if clash != 0 {
				clashes[m] = clash
				continue
			}
			// Soonest wins; on a tie the mat it is on already, so nothing moves for nothing.
			if best == 0 || s.Before(bestStart) || (s.Equal(bestStart) && m == it.Mat && best != it.Mat) {
				best, bestStart, bestEnd = m, s, e
			}
		}
		if best == 0 {
			// Every mat would have to wait for one of its fencers: queue it behind the
			// clash, on that mat, where it cannot start before they are free.
			for _, m := range candidates {
				if c := clashes[m]; c != 0 {
					best = c
					break
				}
			}
			if best == 0 {
				best = candidates[0]
			}
			bestStart, bestEnd, _ = timeOn(best, it)
		}
		order[best] = append(order[best], it.ID)
		ends[it.ID] = bestEnd
		cursor[best] = bestEnd
		placedAt[it.ID] = bestStart
		for _, p := range it.People {
			busy[p] = append(busy[p], busySpan{bestStart, bestEnd, best})
		}
	}

	out := Suggestion{Order: order, Before: before.End}
	for _, e := range ends {
		if e.After(out.End) {
			out.End = e
		}
	}
	for _, it := range in.Items {
		for mat, ids := range order {
			for i, id := range ids {
				if id != it.ID {
					continue
				}
				// Position in the queue, from 1, against where the plan has it now.
				if mat != it.Mat || i+1 != position(in.Items, it) {
					out.Moves = append(out.Moves, Move{ID: it.ID, FromMat: it.Mat, FromSeq: position(in.Items, it),
						ToMat: mat, ToSeq: i + 1, Start: placedAt[it.ID]})
				}
			}
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

// dependencies says what each item waits for. Within a discipline: its eliminations wait
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
