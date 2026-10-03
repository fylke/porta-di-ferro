// Package forecast times the event's plan (docs/proposals/one-event-many-disciplines.md
// §10, phase 4).
//
// Three timelines, kept apart. The plan is where each work item runs and in what order,
// and is the organizer's. The actual is what the match logs say happened. The forecast
// is the plan re-timed from the actual: every match already fenced where it was fenced,
// every match to come at the pace its mat is really achieving, so a slow pool on mat 2
// pushes mat 2's later items and nobody else's. Run on the plan alone, with the template
// pace and no clock, the same simulation is the planned day -- the ghost the board draws
// behind each card to show how far the day has drifted.
//
// Everything here is a pure function of its input, so the coordinator and the browser
// demo give the same answer. Warnings are reported, never enforced: the organizer may
// know the Sabre pool will wait for Astrid.
package forecast

import (
	"sort"
	"time"
)

// The kinds of item, as httpapi names them.
const (
	KindPool         = "pool"
	KindEliminations = "eliminations"
	KindBronze       = "bronze"
	KindFinal        = "final"
)

// Timings is the template: how long a match takes and the gap between matches before the
// day has a pace of its own, the pause between a discipline's pools and its eliminations,
// and when the day starts and the venue closes.
type Timings struct {
	Match       time.Duration
	Changeover  time.Duration
	BeforeElims time.Duration
	Start       time.Time
	// Close is zero when the venue gave no time.
	Close time.Time
}

// Defaults are placeholders to calibrate (#64): a match of the SM rules' three minutes of
// fencing takes about four on the mat, and the next pair is on in a minute.
var Defaults = Timings{Match: 4 * time.Minute, Changeover: time.Minute, BeforeElims: 15 * time.Minute}

// Match is one match of an item, with what its log says.
type Match struct {
	// Key names it across the event: "open-sabre/p2m3".
	Key string
	// Started and Ended are zero until the log says so.
	Started, Ended time.Time
	// After are the matches that decide who fences in this one, by key: a semi-final's
	// quarter-finals, wherever they run (#120).
	After []string
	// Done says the match is finished even when its log carries no times -- events posted
	// without them. It is not fenced again, and it teaches the pace nothing.
	Done bool
}

// finished says a match is over, by its times or by its result.
func (m Match) finished() bool { return m.Done || !m.Ended.IsZero() }

// begun says a match has started, by its times or by its result.
func (m Match) begun() bool { return m.Done || !m.Started.IsZero() }

// Item is one work item, placed.
type Item struct {
	ID         string
	Discipline string
	Kind       string
	Mat, Seq   int
	Matches    []Match
	// Projected items are not drawn yet; their matches have no logs.
	Projected bool
	// Pinned items were put on their mat by hand; a suggestion keeps them there.
	Pinned bool
	// Session is the block of the day the item's discipline runs in (#136): it waits for
	// every item of the blocks before. Held is a bronze match or final held to the end of
	// the day, after everything else.
	Session   int
	Held      bool
	NotBefore time.Time
	// People are everyone fencing in the item, by person, for the overlap check.
	People []string
}

// Break is a time no mat fences: lunch, the prize giving.
type Break struct{ From, To time.Time }

// Input is everything the forecast needs.
type Input struct {
	Items   []Item
	Timings Timings
	Breaks  []Break
	// Now is the clock. Work still to come never starts before it once the day is under
	// way; a zero Now, or a day nobody has fenced in yet, is planned from Timings.Start.
	Now time.Time
	// Anomalies are matches, by key, whose times are not to be learned from.
	Anomalies map[string]bool
	// MatBreaks are the times one mat is not available, by mat (#123): breaks for that
	// mat alone.
	MatBreaks map[int][]Break
}

// breaksOn is every break a mat keeps: the hall's and its own.
func (in Input) breaksOn(mat int) []Break {
	if len(in.MatBreaks[mat]) == 0 {
		return in.Breaks
	}
	return append(append([]Break{}, in.Breaks...), in.MatBreaks[mat]...)
}

// Span is one item's times.
type Span struct {
	ID         string
	Mat        int
	Start, End time.Time
	// PlannedStart and PlannedEnd are the planned day's: the same plan at the template
	// pace from the day's start, as if nothing had happened yet.
	PlannedStart, PlannedEnd time.Time
}

// Pace is how fast a mat goes: what its matches and changeovers take, from how many
// measured matches.
type Pace struct {
	Match, Changeover time.Duration
	Samples           int
}

// Warning is something the organizer should look at. Kinds: "overlap" (a person in two
// items at once), "dependency" (an item placed before what it waits for can be done) and
// "overrun" (the day ends after the venue closes).
type Warning struct {
	Kind     string
	Items    []string
	Person   string
	From, To time.Time
}

// Result is the forecast.
type Result struct {
	Items []Span
	// Matches is when each match still to be fenced is expected to start, by key.
	Matches map[string]time.Time
	// End and PlannedEnd are when the last item finishes, forecast and planned.
	End, PlannedEnd time.Time
	Pace            map[int]Pace
	Warnings        []Warning
}

// prior is how many template matches a measured average is weighed against, so the
// first slow match of the morning does not reschedule the whole day.
const prior = 3

// longestChangeover is the longest gap between matches still counted as a changeover; a
// longer one was a break, or the end of the pools.
const longestChangeover = 15 * time.Minute

// Run forecasts the plan.
func Run(in Input) Result {
	tpl := in.Timings
	if tpl.Match <= 0 {
		tpl.Match = Defaults.Match
	}
	if tpl.Changeover < 0 {
		tpl.Changeover = 0
	}
	in.Timings = tpl

	live := false
	for _, it := range in.Items {
		for _, m := range it.Matches {
			live = live || m.begun()
		}
	}
	pace := Paces(in)

	planned, _ := simulate(in, nil, false)
	forecast, warnings := simulate(in, pace, live)

	out := Result{Matches: map[string]time.Time{}, Pace: pace}
	for _, it := range in.Items {
		f, p := forecast.items[it.ID], planned.items[it.ID]
		out.Items = append(out.Items, Span{ID: it.ID, Mat: it.Mat, Start: f.start, End: f.end,
			PlannedStart: p.start, PlannedEnd: p.end})
		if f.end.After(out.End) {
			out.End = f.end
		}
		if p.end.After(out.PlannedEnd) {
			out.PlannedEnd = p.end
		}
	}
	for k, t := range forecast.matches {
		out.Matches[k] = t
	}
	out.Warnings = append(warnings, overlaps(in.Items, forecast)...)
	if !tpl.Close.IsZero() && out.End.After(tpl.Close) {
		out.Warnings = append(out.Warnings, Warning{Kind: "overrun", From: tpl.Close, To: out.End})
	}
	return out
}

// Paces is each mat's pace: the template, moved towards what the mat's own matches have
// taken, through what the whole hall's have taken.
func Paces(in Input) map[int]Pace {
	type sample struct{ match, gap []time.Duration }
	byMat := map[int]*sample{}
	all := &sample{}
	for mat, ms := range fenced(in) {
		s := &sample{}
		byMat[mat] = s
		for i, m := range ms {
			s.match = append(s.match, m.Ended.Sub(m.Started))
			if i > 0 {
				if gap := m.Started.Sub(ms[i-1].Ended); gap >= 0 && gap <= longestChangeover {
					s.gap = append(s.gap, gap)
				}
			}
		}
		all.match = append(all.match, s.match...)
		all.gap = append(all.gap, s.gap...)
	}
	hall := Pace{Match: blend(all.match, in.Timings.Match), Changeover: blend(all.gap, in.Timings.Changeover), Samples: len(all.match)}
	out := map[int]Pace{0: hall}
	for _, it := range in.Items {
		s := byMat[it.Mat]
		if s == nil {
			// Nothing measured on this mat: the hall's pace, from none of its own.
			out[it.Mat] = Pace{Match: hall.Match, Changeover: hall.Changeover}
			continue
		}
		out[it.Mat] = Pace{Match: blend(s.match, hall.Match), Changeover: blend(s.gap, hall.Changeover), Samples: len(s.match)}
	}
	return out
}

// fenced is every finished, unremarkable match on each mat, in the order it was fenced.
func fenced(in Input) map[int][]Match {
	out := map[int][]Match{}
	for _, it := range in.Items {
		for _, m := range it.Matches {
			if m.Started.IsZero() || m.Ended.IsZero() || !m.Ended.After(m.Started) || in.Anomalies[m.Key] {
				continue
			}
			out[it.Mat] = append(out[it.Mat], m)
		}
	}
	for _, ms := range out {
		sort.Slice(ms, func(i, j int) bool { return ms[i].Started.Before(ms[j].Started) })
	}
	return out
}

func blend(samples []time.Duration, toward time.Duration) time.Duration {
	sum := time.Duration(prior) * toward
	for _, d := range samples {
		sum += d
	}
	return sum / time.Duration(prior+len(samples))
}

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
	actual := pace != nil
	type step struct {
		item  Item
		index int // the match's place in its item; -1 for an item with no matches
	}
	queues := map[int][]step{}
	var mats []int
	byMat := map[int][]Item{}
	for _, it := range in.Items {
		if _, ok := byMat[it.Mat]; !ok {
			mats = append(mats, it.Mat)
		}
		byMat[it.Mat] = append(byMat[it.Mat], it)
	}
	sort.Ints(mats)
	for _, mat := range mats {
		items := byMat[mat]
		sort.SliceStable(items, func(i, j int) bool { return items[i].Seq < items[j].Seq })
		for _, it := range items {
			if len(it.Matches) == 0 {
				queues[mat] = append(queues[mat], step{it, -1})
			}
			for i := range it.Matches {
				queues[mat] = append(queues[mat], step{it, i})
			}
		}
	}

	deps := dependencies(in.Items)
	tl := timeline{items: map[string]span{}, matches: map[string]time.Time{}}
	cursor := map[int]time.Time{}
	fencedOn := map[int]bool{}
	head := map[int]int{}
	ends := map[string]time.Time{} // when each match ends, by key, for the matches it feeds
	left := map[string]int{}       // matches still to time, by item
	for _, it := range in.Items {
		left[it.ID] = max(len(it.Matches), 1)
	}
	itemReady := map[string]time.Time{}
	warned := map[string]bool{}
	var warnings []Warning

	paceOn := func(mat int) Timings {
		p := in.Timings
		if actual {
			pc := pace[mat]
			p.Match, p.Changeover = pc.Match, pc.Changeover
		}
		return p
	}

	// ready says when an item may begin on a mat: after what it waits for, its hold, the
	// day's start, and -- once the day is live -- now. False when what it waits for is
	// not timed yet.
	ready := func(mat int, it Item, ignore bool) (time.Time, bool) {
		at := in.Timings.Start
		if c, ok := cursor[mat]; ok && c.After(at) {
			at = c
		}
		if !ignore {
			wait, pause := deps(it)
			var latest, pools time.Time
			for _, id := range wait {
				if left[id] > 0 {
					return at, false
				}
				if s := tl.items[id]; s.end.After(latest) {
					latest = s.end
				}
			}
			for _, id := range pause {
				if left[id] > 0 {
					return at, false
				}
				if s := tl.items[id]; s.end.After(pools) {
					pools = s.end
				}
			}
			if !pools.IsZero() {
				pools = pools.Add(in.Timings.BeforeElims)
			}
			if pools.After(latest) {
				latest = pools
			}
			if latest.After(at) {
				at = latest
			}
		}
		if it.NotBefore.After(at) {
			at = it.NotBefore
		}
		if actual && live && !in.Now.IsZero() && in.Now.After(at) {
			at = in.Now
		}
		return at, true
	}

	// when is when the step at the head of a mat would start and end, or false when it
	// cannot be timed yet: its item waits, or a match that feeds it is not timed (#120).
	type timing struct {
		start, end time.Time
		base       time.Time // when the item may begin, for its later matches
		skip       bool      // finished at a time nobody wrote down
		logged     bool
	}
	when := func(mat int, st step, ignore bool) (timing, bool) {
		p := paceOn(mat)
		base, begun := itemReady[st.item.ID]
		if !begun {
			var ok bool
			if base, ok = ready(mat, st.item, ignore); !ok {
				return timing{}, false
			}
		}
		if st.index < 0 {
			return timing{start: base, end: base, base: base}, true
		}
		m := st.item.Matches[st.index]
		switch {
		case actual && m.Done && m.Ended.IsZero():
			return timing{skip: true, base: base}, true
		case actual && !m.Ended.IsZero():
			start := m.Started
			if start.IsZero() {
				start = m.Ended.Add(-p.Match)
			}
			return timing{start: start, end: m.Ended, base: base, logged: true}, true
		case actual && !m.Started.IsZero():
			end := m.Started.Add(p.Match)
			if live && in.Now.After(end) {
				end = in.Now
			}
			return timing{start: m.Started, end: end, base: base, logged: true}, true
		}
		at := base
		if c, ok := cursor[mat]; ok && c.After(at) {
			at = c
		}
		if !ignore {
			for _, f := range m.After {
				e, ok := ends[f]
				if !ok {
					return timing{}, false
				}
				if e.After(at) {
					at = e
				}
			}
		}
		if fencedOn[mat] {
			at = at.Add(p.Changeover)
		}
		at = afterBreaks(at, in.breaksOn(mat))
		return timing{start: at, end: at.Add(p.Match), base: base}, true
	}

	commit := func(mat int, st step, tm timing) {
		it := st.item
		if _, ok := itemReady[it.ID]; !ok {
			itemReady[it.ID] = tm.base
		}
		end := tm.end
		if tm.skip {
			end = cursor[mat]
			if end.IsZero() {
				end = itemReady[it.ID]
			}
		} else {
			if st.index >= 0 && !tm.logged {
				tl.matches[it.Matches[st.index].Key] = tm.start
			}
			s, seen := tl.items[it.ID]
			if !seen || tm.start.Before(s.start) {
				s.start = tm.start
			}
			if tm.end.After(s.end) {
				s.end = tm.end
			}
			tl.items[it.ID] = s
			if tm.end.After(cursor[mat]) {
				cursor[mat] = tm.end
			}
			if st.index >= 0 {
				fencedOn[mat] = true
			}
		}
		if st.index >= 0 {
			ends[it.Matches[st.index].Key] = end
		}
		left[it.ID]--
		if left[it.ID] == 0 {
			if _, seen := tl.items[it.ID]; !seen {
				r := itemReady[it.ID]
				tl.items[it.ID] = span{start: r, end: r}
			}
		}
		head[mat]++
	}

	for {
		// The step that can start soonest, across the mats.
		best, bestAt := -1, timing{}
		pending := false
		for _, mat := range mats {
			if head[mat] >= len(queues[mat]) {
				continue
			}
			pending = true
			tm, ok := when(mat, queues[mat][head[mat]], false)
			if !ok {
				continue
			}
			if best < 0 || (!tm.skip && tm.start.Before(bestAt.start)) || tm.skip {
				best, bestAt = mat, tm
				if tm.skip {
					break
				}
			}
		}
		if !pending {
			break
		}
		if best >= 0 {
			commit(best, queues[best][head[best]], bestAt)
			continue
		}
		// Every mat's next match waits for something queued behind another wait: the plan
		// has an item before what it depends on. Time it anyway, on the mat that is free
		// soonest, and say so.
		for _, mat := range mats {
			if head[mat] >= len(queues[mat]) {
				continue
			}
			if best < 0 || cursor[mat].Before(cursor[best]) {
				best = mat
			}
		}
		st := queues[best][head[best]]
		tm, _ := when(best, st, true)
		commit(best, st, tm)
		if !warned[st.item.ID] {
			warned[st.item.ID] = true
			warnings = append(warnings, Warning{Kind: "dependency", Items: []string{st.item.ID}})
		}
	}
	return tl, warnings
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

// overlaps are people in two items at once, by the forecast, among the items not done.
func overlaps(items []Item, tl timeline) []Warning {
	type in struct {
		id   string
		span span
	}
	byPerson := map[string][]in{}
	for _, it := range items {
		done := len(it.Matches) > 0
		for _, m := range it.Matches {
			done = done && m.finished()
		}
		if done {
			continue
		}
		for _, p := range it.People {
			byPerson[p] = append(byPerson[p], in{it.ID, tl.items[it.ID]})
		}
	}
	var people []string
	for p := range byPerson {
		people = append(people, p)
	}
	sort.Strings(people)
	var out []Warning
	for _, p := range people {
		list := byPerson[p]
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				a, b := list[i].span, list[j].span
				from, to := later(a.start, b.start), earlier(a.end, b.end)
				if from.Before(to) {
					out = append(out, Warning{Kind: "overlap", Person: p, Items: []string{list[i].id, list[j].id}, From: from, To: to})
				}
			}
		}
	}
	return out
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Measured is one fenced match as the logs time it, for the report the organizer reads
// after the event and marks anomalies in (#64).
type Measured struct {
	Key     string
	Mat     int
	Started time.Time
	Ended   time.Time
	// Changeover is the gap since the match before it on the same mat, or 0 for the
	// first, or after a gap too long to be one.
	Changeover time.Duration
	Anomaly    bool
}

// Report is every fenced match, and what the unremarkable ones took on average: what a
// template learned from this event would say.
type Report struct {
	Matches    []Measured
	Match      time.Duration
	Changeover time.Duration
	Samples    int
}

// Measure is the report.
func Measure(in Input) Report {
	var out Report
	var sumMatch, sumGap time.Duration
	gaps := 0
	byMat := map[int][]Measured{}
	for _, it := range in.Items {
		for _, m := range it.Matches {
			if m.Started.IsZero() || m.Ended.IsZero() || !m.Ended.After(m.Started) {
				continue
			}
			byMat[it.Mat] = append(byMat[it.Mat], Measured{Key: m.Key, Mat: it.Mat, Started: m.Started, Ended: m.Ended, Anomaly: in.Anomalies[m.Key]})
		}
	}
	for _, ms := range byMat {
		sort.Slice(ms, func(i, j int) bool { return ms[i].Started.Before(ms[j].Started) })
		for i := range ms {
			if i > 0 {
				if gap := ms[i].Started.Sub(ms[i-1].Ended); gap >= 0 && gap <= longestChangeover {
					ms[i].Changeover = gap
				}
			}
			if !ms[i].Anomaly {
				sumMatch += ms[i].Ended.Sub(ms[i].Started)
				out.Samples++
				if ms[i].Changeover > 0 {
					sumGap += ms[i].Changeover
					gaps++
				}
			}
		}
		out.Matches = append(out.Matches, ms...)
	}
	sort.Slice(out.Matches, func(i, j int) bool { return out.Matches[i].Started.Before(out.Matches[j].Started) })
	if out.Samples > 0 {
		out.Match = (sumMatch / time.Duration(out.Samples)).Round(time.Second)
	}
	if gaps > 0 {
		out.Changeover = (sumGap / time.Duration(gaps)).Round(time.Second)
	}
	return out
}
