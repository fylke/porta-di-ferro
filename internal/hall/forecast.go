package hall

import (
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// The forecast, as the event's pages see it (proposal §10, phase 4). internal/forecast
// does the timing; this builds its input from the disciplines' snapshots and the plan,
// and turns its answer into what the pages read. Shared with the browser demo.
//
// Times go out as RFC 3339 in the server's own zone, and the pages print the clock as it
// is written there: the hall's time of day is the organizer's PC's, whatever zone a phone
// in the hall thinks it is in.

// parseAt is a log's timestamp in the hall's zone, or zero.
func parseAt(s string, loc *time.Location) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.In(loc)
}

// ItemTimes is one work item's forecast and planned times.
type ItemTimes struct {
	ID           string `json:"id"`
	Mat          int    `json:"mat"`
	Start        string `json:"start"`
	End          string `json:"end"`
	PlannedStart string `json:"plannedStart"`
	PlannedEnd   string `json:"plannedEnd"`
}

// PaceView is how fast a mat is going, in seconds.
type PaceView struct {
	Mat        int `json:"mat"`
	Match      int `json:"match"`
	Changeover int `json:"changeover"`
	Samples    int `json:"samples"`
}

// WarningView is a warning for the board. Items are work item ids; Person and PersonName
// say who is in two places at once.
type WarningView struct {
	Kind       string   `json:"kind"`
	Items      []string `json:"items,omitempty"`
	Person     string   `json:"person,omitempty"`
	PersonName string   `json:"personName,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
}

// ForecastView is the forecast as the board and the planning panel read it.
type ForecastView struct {
	Items      []ItemTimes    `json:"items"`
	End        string         `json:"end,omitempty"`
	PlannedEnd string         `json:"plannedEnd,omitempty"`
	Pace       []PaceView     `json:"pace"`
	Warnings   []WarningView  `json:"warnings"`
	Programme  []ProgrammeRow `json:"programme"`
	// Timings are the ones in force: the event's, filled in from the defaults.
	Timings store.Timings `json:"timings"`
	// Live says the day is under way: somebody has fenced.
	Live bool `json:"live"`
	// Expected is how many each discipline expects, by slug, for the planning panel.
	Expected map[string]int `json:"expected,omitempty"`
	// Sessions are the blocks of the day in force, by slug, and FinalsLast says the
	// finals are held to the end (#136).
	Sessions   map[string]int `json:"sessions,omitempty"`
	FinalsLast bool           `json:"finalsLast,omitempty"`
	// Breaks, Prep and Starts are the plan's, for the planning panel (#149).
	Breaks []store.Break         `json:"breaks"`
	Prep   map[string]store.Prep `json:"prep,omitempty"`
	Starts map[string]string     `json:"starts,omitempty"`
	// Gaps are what the plan looks to be missing: a lunch break, a discipline's gear
	// check or staff briefing. Pointed out, never required.
	Gaps []PlanGap `json:"gaps"`
	// Mats is each discipline's mats and what its numbers recommend, by slug, and
	// MatsRecommended what that comes to for the hall (#153).
	Mats            map[string]MatsAdvice `json:"mats,omitempty"`
	MatsRecommended int                   `json:"matsRecommended,omitempty"`
}

// MatsAdvice is how many mats a discipline's pools use and how many its numbers recommend.
// Recommended is 0 until there is a number to go on; For is that number.
type MatsAdvice struct {
	Mats        int `json:"mats"`
	Recommended int `json:"recommended,omitempty"`
	For         int `json:"for,omitempty"`
	MinPoolSize int `json:"minPoolSize"`
	MaxPoolSize int `json:"maxPoolSize"`
	Pools       int `json:"pools,omitempty"`
}

// adviseMats is what a discipline's numbers recommend: what it expects or, without that,
// the ceiling on its entries, and never fewer than have entered. The hall needs what the
// busiest block of the day needs, its disciplines running side by side.
func adviseMats(v *ForecastView, plan store.Plan, inputs []MatsInput) {
	v.Mats = map[string]MatsAdvice{}
	slugs := make([]string, 0, len(inputs))
	for _, d := range inputs {
		slugs = append(slugs, d.Slug)
	}
	sessions := Sessions(plan, slugs)
	need := map[int]int{}
	for _, d := range inputs {
		t := d.Snapshot.Tournament
		n := d.Expected
		if n == 0 {
			n = t.Capacity
		}
		n = max(n, Entrants(d.Snapshot.Competitors, 0))
		rec := tournament.RecommendedMats(n, t.MaxPoolSize, tournament.DefaultLimits())
		v.Mats[d.Slug] = MatsAdvice{Mats: t.Mats, Recommended: rec, For: n,
			MinPoolSize: t.MinPoolSize, MaxPoolSize: t.MaxPoolSize, Pools: len(d.Snapshot.Pools)}
		need[sessions[d.Slug]] += rec
	}
	for _, n := range need {
		v.MatsRecommended = max(v.MatsRecommended, n)
	}
}

// PlanGap is something a day usually has and this plan does not. Kind is "lunch",
// "gear-check" or "briefing"; Discipline names the discipline for the last two.
type PlanGap struct {
	Kind       string `json:"kind"`
	Discipline string `json:"discipline,omitempty"`
	Name       string `json:"name,omitempty"`
}

// ViewPlan fills in the plan's own part of the forecast view: what the planning panel
// edits, and what it is missing. Shared with the browser demo.
func ViewPlan(v *ForecastView, plan store.Plan, ev store.Event, inputs []MatsInput) {
	v.Breaks = BreaksOf(plan, ev)
	if v.Breaks == nil {
		v.Breaks = []store.Break{}
	}
	v.Starts = plan.Starts
	v.Prep = map[string]store.Prep{}
	for _, d := range inputs {
		v.Prep[d.Slug] = PrepOf(plan, d.Slug)
	}
	adviseMats(v, plan, inputs)
	v.Gaps = []PlanGap{}
	planned := map[string]bool{}
	for _, r := range v.Programme {
		planned[r.Discipline] = true
	}
	if len(planned) > 0 && len(v.Breaks) == 0 {
		v.Gaps = append(v.Gaps, PlanGap{Kind: "lunch"})
	}
	for _, d := range inputs {
		if !planned[d.Slug] {
			continue
		}
		if v.Prep[d.Slug].GearCheck == 0 {
			v.Gaps = append(v.Gaps, PlanGap{Kind: "gear-check", Discipline: d.Slug, Name: d.Name})
		}
		if v.Prep[d.Slug].Briefing == 0 {
			v.Gaps = append(v.Gaps, PlanGap{Kind: "briefing", Discipline: d.Slug, Name: d.Name})
		}
	}
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ViewForecast turns the forecast into what the pages read.
func ViewForecast(in forecast.Input, r forecast.Result, inputs []MatsInput, timings store.Timings) ForecastView {
	out := ForecastView{Items: []ItemTimes{}, Pace: []PaceView{}, Warnings: []WarningView{}, Programme: []ProgrammeRow{},
		End: stamp(r.End), PlannedEnd: stamp(r.PlannedEnd), Timings: timings}
	for _, s := range r.Items {
		out.Items = append(out.Items, ItemTimes{ID: s.ID, Mat: s.Mat, Start: stamp(s.Start), End: stamp(s.End),
			PlannedStart: stamp(s.PlannedStart), PlannedEnd: stamp(s.PlannedEnd)})
	}
	var mats []int
	for mat := range r.Pace {
		if mat > 0 {
			mats = append(mats, mat)
		}
	}
	sort.Ints(mats)
	for _, mat := range mats {
		p := r.Pace[mat]
		out.Pace = append(out.Pace, PaceView{Mat: mat, Match: int(p.Match / time.Second), Changeover: int(p.Changeover / time.Second), Samples: p.Samples})
	}
	names := map[string]string{}
	for _, d := range inputs {
		for _, c := range d.Snapshot.Competitors {
			if c.Person != "" {
				names[c.Person] = c.Name
			}
			names[d.Slug+"/"+c.ID] = c.Name
		}
	}
	for _, w := range r.Warnings {
		out.Warnings = append(out.Warnings, WarningView{Kind: w.Kind, Items: w.Items, Person: w.Person,
			PersonName: names[w.Person], From: stamp(w.From), To: stamp(w.To)})
	}
	for _, it := range in.Items {
		out.Live = out.Live || it.Begun()
	}
	out.Programme = programme(in, r, inputs)
	return out
}

// EtasFor is when each of a discipline's matches still to come is expected, by match id.
func EtasFor(r forecast.Result, slug string) map[string]string {
	out := map[string]string{}
	prefix := slug + "/"
	for k, t := range r.Matches {
		if id, ok := strings.CutPrefix(k, prefix); ok {
			out[id] = stamp(t)
		}
	}
	return out
}

// ApplyEtas writes the expected times into a discipline's snapshot.
func ApplyEtas(snap *Snapshot, etas map[string]string) {
	snap.eachMatch(func(m *MatchView) {
		if m.Status == "pending" {
			m.Eta = etas[m.ID]
		}
	})
}

// ForecastInput is what the forecast needs, from the disciplines and the plan. The day is
// the one the first match was fenced on, or today's.
func ForecastInput(inputs []MatsInput, placed map[string]store.Placement, plan store.Plan,
	ev store.Event, timings store.Timings, now time.Time) forecast.Input {

	day := now
	if first := firstFenced(inputs, now.Location()); !first.IsZero() {
		day = first.In(now.Location())
	}
	clock := func(s string) time.Time {
		t, _ := ParseClock(day, s)
		return t
	}

	out := forecast.Input{
		Now: now,
		Timings: forecast.Timings{
			Match:       time.Duration(timings.Match) * time.Second,
			Changeover:  time.Duration(timings.Changeover) * time.Second,
			BeforeElims: time.Duration(timings.BeforeElims) * time.Second,
			Start:       clock(timings.Start),
			Close:       clock(timings.Close),
		},
		Anomalies: map[string]bool{},
		MatBreaks: matBreaks(plan, day),
		Breaks:    hallBreaks(plan, ev, day),
		Prep:      prepOfAll(plan, inputs),
	}
	for _, k := range plan.Anomalies {
		out.Anomalies[k] = true
	}
	for _, in := range inputs {
		views := in.Snapshot.matchesByID()
		who := whoIs(in)
		for _, w := range PlanItems(in.Slug, in.Snapshot.Tournament, Entrants(in.Snapshot.Competitors, in.Expected)) {
			p, ok := placed[w.ID()]
			if !ok {
				continue
			}
			it := forecastItem(w, p, in, views, who, now.Location())
			it.NotBefore = clock(p.NotBefore)
			// No work of a discipline before it starts (#149).
			if at := clock(plan.Starts[w.Discipline]); at.After(it.NotBefore) {
				it.NotBefore = at
			}
			out.Items = append(out.Items, it)
		}
	}
	return out
}

// firstFenced is when the first match of the event started, or zero.
func firstFenced(inputs []MatsInput, loc *time.Location) time.Time {
	var first time.Time
	for i := range inputs {
		inputs[i].Snapshot.eachMatch(func(m *MatchView) {
			if t := parseAt(m.StartedAt, loc); !t.IsZero() && (first.IsZero() || t.Before(first)) {
				first = t
			}
		})
	}
	return first
}

// whoIs is the person behind each entry of a discipline: who they are across the event or,
// for an entry with no person yet, somebody known only by that entry.
func whoIs(in MatsInput) map[string]string {
	who := map[string]string{}
	for _, c := range in.Snapshot.Competitors {
		who[c.ID] = c.Person
		if who[c.ID] == "" {
			who[c.ID] = in.Slug + "/" + c.ID
		}
	}
	return who
}

// clockSpan is two times of day on a day, when they are that and the second is the later.
func clockSpan(day time.Time, from, to string) (time.Time, time.Time, bool) {
	f, ok1 := ParseClock(day, from)
	t, ok2 := ParseClock(day, to)
	return f, t, ok1 && ok2 && t.After(f)
}

// matBreaks are the times each mat is away, by mat (#123). Nil when none are.
func matBreaks(plan store.Plan, day time.Time) map[int][]forecast.Break {
	var out map[int][]forecast.Break
	for i, ms := range plan.MatSettings {
		for _, a := range ms.Away {
			if from, to, ok := clockSpan(day, a.From, a.To); ok {
				if out == nil {
					out = map[int][]forecast.Break{}
				}
				out[i+1] = append(out[i+1], forecast.Break{From: from, To: to})
			}
		}
	}
	return out
}

func hallBreaks(plan store.Plan, ev store.Event, day time.Time) []forecast.Break {
	var out []forecast.Break
	for _, b := range BreaksOf(plan, ev) {
		if from, to, ok := clockSpan(day, b.From, b.To); ok {
			out = append(out, forecast.Break{From: from, To: to, Label: b.Label})
		}
	}
	return out
}

func prepOfAll(plan store.Plan, inputs []MatsInput) map[string]forecast.Prep {
	var out map[string]forecast.Prep
	for _, in := range inputs {
		if out == nil {
			out = map[string]forecast.Prep{}
		}
		p := PrepOf(plan, in.Slug)
		out[in.Slug] = forecast.Prep{GearCheck: time.Duration(p.GearCheck) * time.Second, Briefing: time.Duration(p.Briefing) * time.Second}
	}
	return out
}

// forecastItem is one placed work item as the forecast takes it, without its hold time.
func forecastItem(w WorkItem, p store.Placement, in MatsInput, views map[string]MatchView, who map[string]string, loc *time.Location) forecast.Item {
	fi := forecast.Item{ID: w.ID(), Discipline: w.Discipline, Kind: w.Kind,
		Mat: p.Mat, Seq: p.Seq, Projected: w.Projected, Pinned: p.Pinned,
		Session: in.Session, Held: in.FinalsLast && (w.Kind == ItemFinal || w.Kind == ItemBronze), WithFinal: p.WithFinal}
	people := map[string]bool{}
	for _, id := range w.Matches {
		m := forecast.Match{Key: w.Discipline + "/" + id}
		for _, from := range w.Feeders[id] {
			m.After = append(m.After, w.Discipline+"/"+from)
		}
		if v, ok := views[id]; ok {
			m.Started, m.Ended = parseAt(v.StartedAt, loc), parseAt(v.EndedAt, loc)
			m.Done = v.Status == "complete"
			for _, c := range []string{v.Red, v.Blue} {
				if c != "" {
					people[who[c]] = true
				}
			}
		}
		fi.Matches = append(fi.Matches, m)
	}
	for p := range people {
		fi.People = append(fi.People, p)
	}
	sort.Strings(fi.People)
	return fi
}
