package forecast

import (
	"sort"
	"time"
)

type span struct{ start, end time.Time }

type timeline struct {
	items   map[string]span
	matches map[string]time.Time
}

// simulate runs the plan through the day. With pace nil it is the planned day: the
// template, from the day's start, as if nothing had been fenced. Otherwise matches with a
// log are where the log puts them, the rest go at their mat's pace, and -- once the day is
// live -- nothing still to come starts before now.
func simulate(in Input, pace map[int]Pace, live bool) (timeline, []Warning) {
	return simulateCalibrated(in, pace, live, eliminationScales(in))
}

func simulateCalibrated(in Input, pace map[int]Pace, live bool, scales map[string]float64) (timeline, []Warning) {
	return newSimulation(in, pace, live, scales).run()
}

// step is what a mat does next: one match of an item, or an item with no matches.
type step struct {
	item  Item
	index int // the match's place in its item; -1 for an item with no matches
}

// timing is when a step would start and end.
type timing struct {
	start, end time.Time
	base       time.Time // when the item may begin, for its later matches
	skip       bool      // finished at a time nobody wrote down
	logged     bool
}

// simulation is one run of the plan: every mat's queue, and how far each has got.
type simulation struct {
	in     Input
	pace   map[int]Pace
	live   bool
	scales map[string]float64
	deps   func(Item) (wait, pause []string)
	mats   []int
	queues map[int][]step
	head   map[int]int

	cursor    map[int]time.Time
	fencedOn  map[int]bool
	ends      map[string]time.Time // when each match ends, by key, for the matches it feeds
	left      map[string]int       // matches still to time, by item
	itemReady map[string]time.Time
	tl        timeline
}

func newSimulation(in Input, pace map[int]Pace, live bool, scales map[string]float64) *simulation {
	s := &simulation{
		in: in, pace: pace, live: live, scales: scales,
		deps:      dependencies(in.Items),
		queues:    map[int][]step{},
		head:      map[int]int{},
		cursor:    map[int]time.Time{},
		fencedOn:  map[int]bool{},
		ends:      map[string]time.Time{},
		left:      map[string]int{},
		itemReady: map[string]time.Time{},
		tl:        timeline{items: map[string]span{}, matches: map[string]time.Time{}},
	}
	byMat := map[int][]Item{}
	for _, it := range in.Items {
		if _, ok := byMat[it.Mat]; !ok {
			s.mats = append(s.mats, it.Mat)
		}
		byMat[it.Mat] = append(byMat[it.Mat], it)
		s.left[it.ID] = max(len(it.Matches), 1)
	}
	sort.Ints(s.mats)
	for _, mat := range s.mats {
		items := byMat[mat]
		sort.SliceStable(items, func(i, j int) bool { return items[i].Seq < items[j].Seq })
		for _, it := range items {
			if len(it.Matches) == 0 {
				s.queues[mat] = append(s.queues[mat], step{it, -1})
			}
			for i := range it.Matches {
				s.queues[mat] = append(s.queues[mat], step{it, i})
			}
		}
	}
	return s
}

// actual says this run follows the logs, rather than being the planned day.
func (s *simulation) actual() bool { return s.pace != nil }

func (s *simulation) run() (timeline, []Warning) {
	warned := map[string]bool{}
	var warnings []Warning
	for s.pending() {
		if mat, tm := s.soonest(); mat >= 0 {
			s.commit(mat, s.next(mat), tm)
			continue
		}
		// Every mat's next match waits for something queued behind another wait: the plan
		// has an item before what it depends on. Time it anyway, on the mat that is free
		// soonest, and say so.
		mat := s.freest()
		st := s.next(mat)
		tm, _ := s.when(mat, st, true)
		s.commit(mat, st, tm)
		if !warned[st.item.ID] {
			warned[st.item.ID] = true
			warnings = append(warnings, Warning{Kind: "dependency", Items: []string{st.item.ID}})
		}
	}
	return s.tl, warnings
}

func (s *simulation) next(mat int) step { return s.queues[mat][s.head[mat]] }

func (s *simulation) exhausted(mat int) bool { return s.head[mat] >= len(s.queues[mat]) }

func (s *simulation) pending() bool {
	for _, mat := range s.mats {
		if !s.exhausted(mat) {
			return true
		}
	}
	return false
}

// soonest is the step that can start soonest across the mats, or a mat of -1 when none can
// be timed yet. A match finished at a time nobody wrote down takes no time, so it goes first.
func (s *simulation) soonest() (int, timing) {
	best, bestAt := -1, timing{}
	for _, mat := range s.mats {
		if s.exhausted(mat) {
			continue
		}
		tm, ok := s.when(mat, s.next(mat), false)
		if !ok {
			continue
		}
		if tm.skip {
			return mat, tm
		}
		if best < 0 || tm.start.Before(bestAt.start) {
			best, bestAt = mat, tm
		}
	}
	return best, bestAt
}

// freest is the mat with something left to do that is free soonest.
func (s *simulation) freest() int {
	best := -1
	for _, mat := range s.mats {
		if s.exhausted(mat) {
			continue
		}
		if best < 0 || s.cursor[mat].Before(s.cursor[best]) {
			best = mat
		}
	}
	return best
}

func (s *simulation) paceOn(mat int, it Item) Pace {
	p := Pace{Match: s.in.Timings.Match, Changeover: s.in.Timings.Changeover, templateWeight: 1, changeoverWeight: 1}
	if s.actual() {
		p = s.pace[mat]
	}
	return calibratedPace(it, s.in.Timings, p, s.scales)
}

func (s *simulation) allTimed(ids []string) bool {
	for _, id := range ids {
		if s.left[id] > 0 {
			return false
		}
	}
	return true
}

// ready says when an item may begin on a mat: after what it waits for, its hold, the day's
// start, and -- once the day is live -- now. False when what it waits for is not timed yet.
func (s *simulation) ready(mat int, it Item, ignore bool) (time.Time, bool) {
	at := s.in.Timings.Start
	if c, ok := s.cursor[mat]; ok && c.After(at) {
		at = c
	}
	if !ignore {
		wait, pause := s.deps(it)
		if !s.allTimed(wait) || !s.allTimed(pause) {
			return at, false
		}
		at = later(waitsUntil(wait, pause, s.in.Timings.BeforeElims, func(id string) time.Time { return s.tl.items[id].end }), at)
	}
	if it.NotBefore.After(at) {
		at = it.NotBefore
	}
	if s.actual() && s.live && !s.in.Now.IsZero() && s.in.Now.After(at) {
		at = s.in.Now
	}
	return at, true
}

// when is when a step at the head of a mat would start and end, or false when it cannot be
// timed yet: its item waits, or a match that feeds it is not timed (#120).
func (s *simulation) when(mat int, st step, ignore bool) (timing, bool) {
	p := s.paceOn(mat, st.item)
	base, begun := s.itemReady[st.item.ID]
	if !begun {
		var ok bool
		if base, ok = s.ready(mat, st.item, ignore); !ok {
			return timing{}, false
		}
	}
	if st.index < 0 {
		return timing{start: base, end: base, base: base}, true
	}
	m := st.item.Matches[st.index]
	if tm, ok := s.fromLog(m, base, p); ok {
		return tm, true
	}
	return s.toCome(mat, m, base, p, ignore)
}

// fromLog is a match's timing as its log has it, when it has one and this run follows them.
func (s *simulation) fromLog(m Match, base time.Time, p Pace) (timing, bool) {
	if !s.actual() {
		return timing{}, false
	}
	switch {
	case m.Done && m.Ended.IsZero():
		return timing{skip: true, base: base}, true
	case !m.Ended.IsZero():
		start := m.Started
		if start.IsZero() {
			start = m.Ended.Add(-p.Match)
		}
		return timing{start: start, end: m.Ended, base: base, logged: true}, true
	case !m.Started.IsZero():
		end := m.Started.Add(p.Match)
		if s.live && s.in.Now.After(end) {
			end = s.in.Now
		}
		return timing{start: m.Started, end: end, base: base, logged: true}, true
	}
	return timing{}, false
}

// toCome is the timing of a match still to be fenced: once its mat is free and the matches
// that feed it are over, after a changeover, and out of any break.
func (s *simulation) toCome(mat int, m Match, base time.Time, p Pace, ignore bool) (timing, bool) {
	at := base
	if c, ok := s.cursor[mat]; ok && c.After(at) {
		at = c
	}
	if !ignore {
		for _, f := range m.After {
			e, ok := s.ends[f]
			if !ok {
				return timing{}, false
			}
			if e.After(at) {
				at = e
			}
		}
	}
	if s.fencedOn[mat] {
		at = at.Add(p.Changeover)
	}
	at = afterBreaks(at, s.in.breaksOn(mat))
	return timing{start: at, end: at.Add(p.Match), base: base}, true
}

func (s *simulation) commit(mat int, st step, tm timing) {
	it := st.item
	if _, ok := s.itemReady[it.ID]; !ok {
		s.itemReady[it.ID] = tm.base
	}
	end := tm.end
	if tm.skip {
		end = s.cursor[mat]
		if end.IsZero() {
			end = s.itemReady[it.ID]
		}
	} else {
		s.record(mat, st, tm)
	}
	if st.index >= 0 {
		s.ends[it.Matches[st.index].Key] = end
	}
	s.left[it.ID]--
	if s.left[it.ID] == 0 {
		if _, seen := s.tl.items[it.ID]; !seen {
			r := s.itemReady[it.ID]
			s.tl.items[it.ID] = span{start: r, end: r}
		}
	}
	s.head[mat]++
}

// record writes a step that took time into the timeline, and moves its mat on.
func (s *simulation) record(mat int, st step, tm timing) {
	it := st.item
	if st.index >= 0 && !tm.logged {
		s.tl.matches[it.Matches[st.index].Key] = tm.start
	}
	sp, seen := s.tl.items[it.ID]
	if !seen || tm.start.Before(sp.start) {
		sp.start = tm.start
	}
	if tm.end.After(sp.end) {
		sp.end = tm.end
	}
	s.tl.items[it.ID] = sp
	if tm.end.After(s.cursor[mat]) {
		s.cursor[mat] = tm.end
	}
	if st.index >= 0 {
		s.fencedOn[mat] = true
	}
}

// waitsUntil is when what an item waits for is over: the latest of what it waits for and,
// when it follows pools, their end plus the pause before the bracket. Zero when nothing.
func waitsUntil(wait, pause []string, beforeElims time.Duration, endOf func(string) time.Time) time.Time {
	var latest, pools time.Time
	for _, id := range wait {
		if e := endOf(id); e.After(latest) {
			latest = e
		}
	}
	for _, id := range pause {
		if e := endOf(id); e.After(pools) {
			pools = e
		}
	}
	if !pools.IsZero() {
		pools = pools.Add(beforeElims)
	}
	return later(pools, latest)
}

// afterBreaks moves a start out of any break it falls in.
func afterBreaks(t time.Time, breaks []Break) time.Time {
	for moved := true; moved; {
		moved = false
		for _, b := range breaks {
			if !t.Before(b.From) && t.Before(b.To) {
				t, moved = b.To, true
			}
		}
	}
	return t
}
