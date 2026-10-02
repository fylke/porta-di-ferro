package httpapi

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The forecast, as the event's pages see it (proposal §10, phase 4). internal/forecast
// does the timing; this builds its input from the disciplines' snapshots and the plan,
// and turns its answer into what the pages read. Shared with the browser demo.
//
// Times go out as RFC 3339 in the server's own zone, and the pages print the clock as it
// is written there: the hall's time of day is the organizer's PC's, whatever zone a phone
// in the hall thinks it is in.

// DefaultTimings is the built-in template, in the plan's terms.
var DefaultTimings = store.Timings{
	Match:       int(forecast.Defaults.Match / time.Second),
	Changeover:  int(forecast.Defaults.Changeover / time.Second),
	BeforeElims: int(forecast.Defaults.BeforeElims / time.Second),
	Start:       "09:00",
}

// TimingsOr fills what t leaves out from fallback: the event's own timings, over the
// organizer's default for new events, over the built-in template.
func TimingsOr(t, fallback store.Timings) store.Timings {
	if t.Match <= 0 {
		t.Match = fallback.Match
	}
	if t.Changeover <= 0 {
		t.Changeover = fallback.Changeover
	}
	if t.BeforeElims <= 0 {
		t.BeforeElims = fallback.BeforeElims
	}
	if t.Start == "" {
		t.Start = fallback.Start
	}
	if t.Close == "" {
		t.Close = fallback.Close
	}
	return t
}

// CleanTimings checks what the planning panel sent.
func CleanTimings(t store.Timings) (store.Timings, error) {
	t.Start, t.Close = strings.TrimSpace(t.Start), strings.TrimSpace(t.Close)
	for _, s := range []string{t.Start, t.Close} {
		if _, ok := ParseClock(time.Now(), s); s != "" && !ok {
			return t, fmt.Errorf("%q is not a time of day; write it as 09:30", s)
		}
	}
	if t.Match < 0 || t.Match > 3600 || t.Changeover < 0 || t.Changeover > 3600 || t.BeforeElims < 0 || t.BeforeElims > 4*3600 {
		return t, fmt.Errorf("those timings are out of range")
	}
	return t, nil
}

// ParseClock is a time of day, "09:30", on the given day. Anything else -- "after the
// pools" is a legitimate programme row -- is not a time.
func ParseClock(day time.Time, s string) (time.Time, bool) {
	var h, m int
	if n, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil || n != 2 || h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, false
	}
	y, mo, d := day.Date()
	return time.Date(y, mo, d, h, m, 0, 0, day.Location()), true
}

// ForecastInput is what the forecast needs, from the disciplines and the plan. The day is
// the one the first match was fenced on, or today's.
func ForecastInput(inputs []MatsInput, placed map[string]store.Placement, plan store.Plan,
	ev store.Event, timings store.Timings, now time.Time) forecast.Input {

	type item struct {
		w     WorkItem
		place store.Placement
		views map[string]MatchView
		who   map[string]string
	}
	var items []item
	var first time.Time
	for _, in := range inputs {
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
		for _, v := range views {
			if t := parseAt(v.StartedAt, now.Location()); !t.IsZero() && (first.IsZero() || t.Before(first)) {
				first = t
			}
		}
		who := map[string]string{}
		for _, c := range in.Snapshot.Competitors {
			who[c.ID] = c.Person
			if who[c.ID] == "" {
				who[c.ID] = in.Slug + "/" + c.ID
			}
		}
		for _, w := range PlanItems(in.Slug, in.Snapshot.Tournament, Entrants(in.Snapshot.Competitors, in.Expected)) {
			if p, ok := placed[w.ID()]; ok {
				items = append(items, item{w: w, place: p, views: views, who: who})
			}
		}
	}
	day := now
	if !first.IsZero() {
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
	}
	for _, k := range plan.Anomalies {
		out.Anomalies[k] = true
	}
	for _, row := range ev.Schedule {
		if row.Kind != "break" {
			continue
		}
		from, ok1 := ParseClock(day, row.At)
		to, ok2 := ParseClock(day, row.Ends)
		if ok1 && ok2 && to.After(from) {
			out.Breaks = append(out.Breaks, forecast.Break{From: from, To: to})
		}
	}
	for _, it := range items {
		fi := forecast.Item{ID: it.w.ID(), Discipline: it.w.Discipline, Kind: it.w.Kind,
			Mat: it.place.Mat, Seq: it.place.Seq, Projected: it.w.Projected, Pinned: it.place.Pinned, NotBefore: clock(it.place.NotBefore)}
		people := map[string]bool{}
		for _, id := range it.w.Matches {
			m := forecast.Match{Key: it.w.Discipline + "/" + id}
			if v, ok := it.views[id]; ok {
				m.Started, m.Ended = parseAt(v.StartedAt, now.Location()), parseAt(v.EndedAt, now.Location())
				for _, c := range []string{v.Red, v.Blue} {
					if c != "" {
						people[it.who[c]] = true
					}
				}
			}
			fi.Matches = append(fi.Matches, m)
		}
		for p := range people {
			fi.People = append(fi.People, p)
		}
		sort.Strings(fi.People)
		out.Items = append(out.Items, fi)
	}
	return out
}

// parseAt is a log's timestamp in the hall's zone, or zero.
func parseAt(s string, loc *time.Location) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.In(loc)
}

// --- what the pages read ------------------------------------------------------------------

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

// ProgrammeRow is one stage of a discipline, as the derived programme lists it: its pools,
// its eliminations, its final.
type ProgrammeRow struct {
	Discipline string `json:"discipline"`
	Name       string `json:"name"`
	// Stage is "pools", "eliminations" or "final".
	Stage string `json:"stage"`
	Start string `json:"start"`
	End   string `json:"end"`
	Mats  []int  `json:"mats"`
	Done  bool   `json:"done,omitempty"`
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
		for _, m := range it.Matches {
			out.Live = out.Live || !m.Started.IsZero()
		}
	}
	out.Programme = programme(in, r, inputs)
	return out
}

// programme is the fencing part of the day, derived from the forecast: each discipline's
// pools, eliminations and final, with when and where.
func programme(in forecast.Input, r forecast.Result, inputs []MatsInput) []ProgrammeRow {
	spans := map[string]forecast.Span{}
	for _, s := range r.Items {
		spans[s.ID] = s
	}
	type key struct{ disc, stage string }
	rows := map[key]*ProgrammeRow{}
	var order []key
	for _, d := range inputs {
		for _, stage := range []string{"pools", "eliminations", "final"} {
			order = append(order, key{d.Slug, stage})
		}
	}
	nameOf := map[string]string{}
	for _, d := range inputs {
		nameOf[d.Slug] = d.Name
	}
	for _, it := range in.Items {
		stage := "eliminations"
		switch it.Kind {
		case forecast.KindPool:
			stage = "pools"
		case forecast.KindFinal:
			stage = "final"
		}
		s := spans[it.ID]
		k := key{it.Discipline, stage}
		row := rows[k]
		done := len(it.Matches) > 0
		for _, m := range it.Matches {
			done = done && !m.Ended.IsZero()
		}
		if row == nil {
			row = &ProgrammeRow{Discipline: it.Discipline, Name: nameOf[it.Discipline], Stage: stage,
				Start: stamp(s.Start), End: stamp(s.End), Done: done}
			rows[k] = row
		}
		if st := stamp(s.Start); st != "" && (row.Start == "" || st < row.Start) {
			row.Start = st
		}
		if en := stamp(s.End); en > row.End {
			row.End = en
		}
		row.Done = row.Done && done
		if !containsInt(row.Mats, it.Mat) {
			row.Mats = append(row.Mats, it.Mat)
			sort.Ints(row.Mats)
		}
	}
	out := []ProgrammeRow{}
	for _, k := range order {
		if row := rows[k]; row != nil {
			out = append(out, *row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

func containsInt(xs []int, x int) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
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
	for i := range snap.Pools {
		for j := range snap.Pools[i].Matches {
			m := &snap.Pools[i].Matches[j]
			if m.Status == "pending" {
				m.Eta = etas[m.ID]
			}
		}
	}
	if snap.Bracket != nil {
		for i := range snap.Bracket.Matches {
			m := &snap.Bracket.Matches[i]
			if m.Status == "pending" {
				m.Eta = etas[m.ID]
			}
		}
	}
}

// ReportRow is one fenced match in the measured report, with what the organizer needs to
// recognise it.
type ReportRow struct {
	Key            string `json:"key"`
	Discipline     string `json:"discipline"`
	DisciplineName string `json:"disciplineName"`
	Match          string `json:"match"`
	Red            string `json:"red"`
	Blue           string `json:"blue"`
	Mat            int    `json:"mat"`
	Started        string `json:"started"`
	Seconds        int    `json:"seconds"`
	Changeover     int    `json:"changeover"`
	Anomaly        bool   `json:"anomaly"`
}

// ReportView is the measured day: every fenced match, and the averages a template learned
// from it would have.
type ReportView struct {
	Matches    []ReportRow `json:"matches"`
	Match      int         `json:"match"`
	Changeover int         `json:"changeover"`
	Samples    int         `json:"samples"`
}

// ViewReport is the measured report as the planning panel reads it.
func ViewReport(rep forecast.Report, inputs []MatsInput) ReportView {
	type label struct{ name, red, blue string }
	labels := map[string]label{}
	for _, d := range inputs {
		names := map[string]string{}
		for _, c := range d.Snapshot.Competitors {
			names[c.ID] = c.Name
		}
		add := func(m MatchView) {
			labels[d.Slug+"/"+m.ID] = label{d.Name, names[m.Red], names[m.Blue]}
		}
		for _, p := range d.Snapshot.Pools {
			for _, m := range p.Matches {
				add(m)
			}
		}
		if d.Snapshot.Bracket != nil {
			for _, m := range d.Snapshot.Bracket.Matches {
				add(m)
			}
		}
	}
	out := ReportView{Matches: []ReportRow{}, Match: int(rep.Match / time.Second), Changeover: int(rep.Changeover / time.Second), Samples: rep.Samples}
	for _, m := range rep.Matches {
		disc, id, _ := strings.Cut(m.Key, "/")
		l := labels[m.Key]
		out.Matches = append(out.Matches, ReportRow{Key: m.Key, Discipline: disc, DisciplineName: l.name, Match: id,
			Red: l.red, Blue: l.blue, Mat: m.Mat, Started: stamp(m.Started),
			Seconds: int(m.Ended.Sub(m.Started) / time.Second), Changeover: int(m.Changeover / time.Second), Anomaly: m.Anomaly})
	}
	return out
}

// --- suggestions ------------------------------------------------------------------------

// MoveView is one item a suggestion moves, with where from, where to and when it would
// start there.
type MoveView struct {
	ID           string `json:"id"`
	FromMat      int    `json:"fromMat"`
	FromPosition int    `json:"fromPosition"`
	ToMat        int    `json:"toMat"`
	ToPosition   int    `json:"toPosition"`
	Start        string `json:"start"`
}

// SuggestionView is a suggested plan: what moves, when the day would end with it and
// without it, and a signature that applying it must match -- so a plan that changed since
// the organizer looked is looked at again rather than overwritten.
type SuggestionView struct {
	Moves     []MoveView `json:"moves"`
	End       string     `json:"end"`
	Before    string     `json:"before"`
	Signature string     `json:"signature"`
}

// ViewSuggestion is a suggestion as the board reads it.
func ViewSuggestion(s forecast.Suggestion) SuggestionView {
	out := SuggestionView{Moves: []MoveView{}, End: stamp(s.End), Before: stamp(s.Before), Signature: SignatureOf(s)}
	for _, m := range s.Moves {
		out.Moves = append(out.Moves, MoveView{ID: m.ID, FromMat: m.FromMat, FromPosition: m.FromSeq,
			ToMat: m.ToMat, ToPosition: m.ToSeq, Start: stamp(m.Start)})
	}
	return out
}

// SignatureOf names a suggestion by the order it proposes.
func SignatureOf(s forecast.Suggestion) string {
	var mats []int
	for m := range s.Order {
		mats = append(mats, m)
	}
	sort.Ints(mats)
	var b strings.Builder
	for _, m := range mats {
		fmt.Fprintf(&b, "%d:%s;", m, strings.Join(s.Order[m], ","))
	}
	return b.String()
}

// ApplySuggestion is the placements a suggestion makes. Every item keeps its draw stamp,
// its pin and its hold; its mat and place are the suggestion's, settled as planned.
func ApplySuggestion(placed map[string]store.Placement, s forecast.Suggestion) map[string]store.Placement {
	next := make(map[string]store.Placement, len(placed))
	for k, v := range placed {
		next[k] = v
	}
	for mat, ids := range s.Order {
		for i, id := range ids {
			p := next[id]
			p.Mat, p.Seq, p.Planned = mat, i+1, true
			next[id] = p
		}
	}
	return next
}
