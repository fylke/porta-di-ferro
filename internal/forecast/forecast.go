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

// The kinds of item, as hall names them.
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

// Defaults are the generic match timings; pools also use the observed pool totals below.
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
	// WithFinal is a bronze match let run at the same time as its final (#149); otherwise
	// the final waits for it.
	WithFinal bool
	// People are everyone fencing in the item, by person, for the overlap check.
	People []string
}

// Begun says a match of the item has started.
func (it Item) Begun() bool {
	for _, m := range it.Matches {
		if m.begun() {
			return true
		}
	}
	return false
}

// Finished says the item has matches and all of them are over.
func (it Item) Finished() bool {
	for _, m := range it.Matches {
		if !m.finished() {
			return false
		}
	}
	return len(it.Matches) > 0
}

// anyBegun says the day is under way: somebody has fenced.
func anyBegun(items []Item) bool {
	for _, it := range items {
		if it.Begun() {
			return true
		}
	}
	return false
}

// Break is a time no mat fences: lunch, the prize giving. Label is what the programme
// calls it.
type Break struct {
	From, To time.Time
	Label    string
}

// Prep is what comes before a discipline's first pool: its gear check and its staff
// briefing (#149). Not timed against the mats; the programme puts them before the pool.
type Prep struct{ GearCheck, Briefing time.Duration }

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
	// Prep is each discipline's gear check and staff briefing, by slug, for the programme.
	Prep map[string]Prep
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
	templateWeight    float64
	changeoverWeight  float64
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

	pace := Paces(in)

	planned, _ := simulate(in, nil, false)
	forecast, warnings := simulate(in, pace, anyBegun(in.Items))

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
	hall := Pace{Match: blend(all.match, in.Timings.Match), Changeover: blend(all.gap, in.Timings.Changeover), Samples: len(all.match),
		templateWeight:   float64(prior) / float64(prior+len(all.match)),
		changeoverWeight: float64(prior) / float64(prior+len(all.gap))}
	out := map[int]Pace{0: hall}
	for _, it := range in.Items {
		s := byMat[it.Mat]
		if s == nil {
			// Nothing measured on this mat: the hall's pace, from none of its own.
			out[it.Mat] = Pace{Match: hall.Match, Changeover: hall.Changeover,
				templateWeight: hall.templateWeight, changeoverWeight: hall.changeoverWeight}
			continue
		}
		out[it.Mat] = Pace{Match: blend(s.match, hall.Match), Changeover: blend(s.gap, hall.Changeover), Samples: len(s.match),
			templateWeight:   hall.templateWeight * float64(prior) / float64(prior+len(s.match)),
			changeoverWeight: hall.changeoverWeight * float64(prior) / float64(prior+len(s.gap))}
	}
	return out
}

func poolPace(it Item, tpl Timings, p Pace) Pace {
	if it.Kind != KindPool || tpl.Match != Defaults.Match || tpl.Changeover != Defaults.Changeover {
		return p
	}
	var total time.Duration
	switch len(it.Matches) {
	case 6:
		total = 40 * time.Minute
	case 10:
		total = 50 * time.Minute
	case 15:
		total = 70 * time.Minute
	case 21:
		total = 95 * time.Minute
	default:
		return p
	}
	count := time.Duration(len(it.Matches))
	match := (total - (count-1)*Defaults.Changeover) / count
	p.Match += time.Duration(float64(match-Defaults.Match) * p.templateWeight)
	return p
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

// overlaps are people in two items at once, by the forecast, among the items not done.
func overlaps(items []Item, tl timeline) []Warning {
	type in struct {
		id   string
		span span
	}
	byPerson := map[string][]in{}
	for _, it := range items {
		if it.Finished() {
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
	// Matches logged at the same instant keep one order, whichever way the mats were walked.
	sort.Slice(out.Matches, func(i, j int) bool {
		if !out.Matches[i].Started.Equal(out.Matches[j].Started) {
			return out.Matches[i].Started.Before(out.Matches[j].Started)
		}
		return out.Matches[i].Key < out.Matches[j].Key
	})
	if out.Samples > 0 {
		out.Match = (sumMatch / time.Duration(out.Samples)).Round(time.Second)
	}
	if gaps > 0 {
		out.Changeover = (sumGap / time.Duration(gaps)).Round(time.Second)
	}
	return out
}
