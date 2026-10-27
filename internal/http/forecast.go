package httpapi

import (
	"fmt"
	"slices"
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

// BreaksOf is the hall's breaks: the plan's (#149), or -- for an event planned before the
// plan held them -- the break rows of the programme the organizer used to type.
func BreaksOf(plan store.Plan, ev store.Event) []store.Break {
	if len(plan.Breaks) > 0 {
		return plan.Breaks
	}
	var out []store.Break
	for _, row := range ev.Schedule {
		if row.Kind == "break" && row.Ends != "" {
			out = append(out, store.Break{Label: row.Label, From: row.At, To: row.Ends})
		}
	}
	return out
}

// maxBreaks caps the hall's breaks: a day with more stops than this is not a plan.
const maxBreaks = 10

// CleanBreaks checks the hall's breaks: times of day, each ending after it starts, in the
// order of the day. A break with no name is lunch, the one every day has.
func CleanBreaks(in []store.Break) ([]store.Break, error) {
	if len(in) > maxBreaks {
		return nil, fmt.Errorf("a day with more than %d breaks is not one anybody plans", maxBreaks)
	}
	now := time.Now()
	out := make([]store.Break, 0, len(in))
	for _, b := range in {
		b.Label, b.From, b.To = strings.TrimSpace(b.Label), strings.TrimSpace(b.From), strings.TrimSpace(b.To)
		from, ok1 := ParseClock(now, b.From)
		to, ok2 := ParseClock(now, b.To)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("a break needs a start and an end, written as 12:00")
		}
		if !to.After(from) {
			return nil, fmt.Errorf("the break from %s should end after it starts", b.From)
		}
		if b.Label == "" {
			b.Label = "Lunch"
		}
		if len(b.Label) > 60 {
			return nil, fmt.Errorf("a break's name is a word or two, not %d characters", len(b.Label))
		}
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out, nil
}

// CleanPrep checks a discipline's gear check and staff briefing: up to four hours each.
func CleanPrep(p store.Prep) (store.Prep, error) {
	if p.GearCheck < 0 || p.GearCheck > 4*3600 || p.Briefing < 0 || p.Briefing > 4*3600 {
		return p, fmt.Errorf("a gear check or a briefing takes between nothing and four hours")
	}
	return p, nil
}

// CleanStart checks when a discipline starts at the earliest: a time of day, or blank for
// whenever the plan reaches it.
func CleanStart(s string) (string, error) {
	s = strings.TrimSpace(s)
	if _, ok := ParseClock(time.Now(), s); s != "" && !ok {
		return s, fmt.Errorf("%q is not a time of day; write it as 09:30", s)
	}
	return s, nil
}

// ForecastInput is what the forecast needs, from the disciplines and the plan. The day is
// the one the first match was fenced on, or today's.
func ForecastInput(inputs []MatsInput, placed map[string]store.Placement, plan store.Plan,
	ev store.Event, timings store.Timings, now time.Time) forecast.Input {

	type item struct {
		w       WorkItem
		place   store.Placement
		views   map[string]MatchView
		who     map[string]string
		session int
		held    bool
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
				items = append(items, item{w: w, place: p, views: views, who: who, session: in.Session,
					held: in.FinalsLast && (w.Kind == ItemFinal || w.Kind == ItemBronze)})
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
	for i, ms := range plan.MatSettings {
		for _, a := range ms.Away {
			from, ok1 := ParseClock(day, a.From)
			to, ok2 := ParseClock(day, a.To)
			if ok1 && ok2 && to.After(from) {
				if out.MatBreaks == nil {
					out.MatBreaks = map[int][]forecast.Break{}
				}
				out.MatBreaks[i+1] = append(out.MatBreaks[i+1], forecast.Break{From: from, To: to})
			}
		}
	}
	for _, b := range BreaksOf(plan, ev) {
		from, ok1 := ParseClock(day, b.From)
		to, ok2 := ParseClock(day, b.To)
		if ok1 && ok2 && to.After(from) {
			out.Breaks = append(out.Breaks, forecast.Break{From: from, To: to, Label: b.Label})
		}
	}
	for slug, p := range plan.Prep {
		if out.Prep == nil {
			out.Prep = map[string]forecast.Prep{}
		}
		out.Prep[slug] = forecast.Prep{GearCheck: time.Duration(p.GearCheck) * time.Second, Briefing: time.Duration(p.Briefing) * time.Second}
	}
	for _, it := range items {
		notBefore := clock(it.place.NotBefore)
		// No work of a discipline before it starts (#149).
		if at := clock(plan.Starts[it.w.Discipline]); at.After(notBefore) {
			notBefore = at
		}
		fi := forecast.Item{ID: it.w.ID(), Discipline: it.w.Discipline, Kind: it.w.Kind,
			Mat: it.place.Mat, Seq: it.place.Seq, Projected: it.w.Projected, Pinned: it.place.Pinned, NotBefore: notBefore,
			Session: it.session, Held: it.held}
		people := map[string]bool{}
		for _, id := range it.w.Matches {
			m := forecast.Match{Key: it.w.Discipline + "/" + id}
			for _, from := range it.w.Feeders[id] {
				m.After = append(m.After, it.w.Discipline+"/"+from)
			}
			if v, ok := it.views[id]; ok {
				m.Started, m.Ended = parseAt(v.StartedAt, now.Location()), parseAt(v.EndedAt, now.Location())
				m.Done = v.Status == "complete"
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

// Sessions is the block of the day each discipline runs in (#136): the organizer's, or,
// for a discipline not given one, a block of its own in the event's order -- one
// discipline after another.
func Sessions(plan store.Plan, slugs []string) map[string]int {
	out := map[string]int{}
	for i, slug := range slugs {
		out[slug] = i + 1
		if n := plan.Sessions[slug]; n > 0 {
			out[slug] = n
		}
	}
	return out
}

// OrderOf is the order of the day for placing work (#136).
func OrderOf(plan store.Plan, slugs []string, started func(WorkItem) bool) PlaceOrder {
	sessions := Sessions(plan, slugs)
	position := map[string]int{}
	for i, s := range slugs {
		position[s] = i
	}
	return PlaceOrder{
		Session:    func(slug string) int { return sessions[slug] },
		Position:   func(slug string) int { return position[slug] },
		FinalsLast: plan.FinalsLast,
		Started:    started,
	}
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

// CleanMat checks one mat's setting from the planning panel.
func CleanMat(m store.MatSetting) (store.MatSetting, error) {
	m.Name = strings.TrimSpace(m.Name)
	if len([]rune(m.Name)) > 30 {
		return m, fmt.Errorf("a mat's name is at most 30 characters")
	}
	day := time.Now()
	for i, a := range m.Away {
		from, ok1 := ParseClock(day, a.From)
		to, ok2 := ParseClock(day, a.To)
		if !ok1 || !ok2 || !to.After(from) {
			return m, fmt.Errorf("away from %q to %q: two times of day, the second later", a.From, a.To)
		}
		m.Away[i] = store.Away{From: strings.TrimSpace(a.From), To: strings.TrimSpace(a.To)}
	}
	return m, nil
}

// SetMat sets one mat's name and away times in the plan.
func SetMat(plan *store.Plan, mat int, m store.MatSetting) {
	for len(plan.MatSettings) < mat {
		plan.MatSettings = append(plan.MatSettings, store.MatSetting{})
	}
	plan.MatSettings[mat-1] = m
	// Nothing set past the last named mat need be kept.
	for len(plan.MatSettings) > 0 {
		last := plan.MatSettings[len(plan.MatSettings)-1]
		if last.Name != "" || len(last.Away) > 0 {
			break
		}
		plan.MatSettings = plan.MatSettings[:len(plan.MatSettings)-1]
	}
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

// ProgrammeRow is one row of the programme, generated from the plan (#149): a discipline's
// gear check, staff briefing, pools, eliminations or final, or a break for the whole hall.
type ProgrammeRow struct {
	// Discipline is empty for a break; Name is then the break's.
	Discipline string `json:"discipline"`
	Name       string `json:"name"`
	// Stage is "gear-check", "briefing", "pools", "eliminations", "final" or "break".
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
	// Sessions are the blocks of the day in force, by slug, and FinalsLast says the
	// finals are held to the end (#136).
	Sessions   map[string]int `json:"sessions,omitempty"`
	FinalsLast bool           `json:"finalsLast,omitempty"`
	// Breaks, Prep and Starts are the plan's, for the planning panel (#149).
	Breaks []store.Break          `json:"breaks"`
	Prep   map[string]store.Prep  `json:"prep,omitempty"`
	Starts map[string]string      `json:"starts,omitempty"`
	// Gaps are what the plan looks to be missing: a lunch break, a discipline's gear
	// check or staff briefing. Pointed out, never required.
	Gaps []PlanGap `json:"gaps"`
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
	v.Prep, v.Starts = plan.Prep, plan.Starts
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
		if plan.Prep[d.Slug].GearCheck == 0 {
			v.Gaps = append(v.Gaps, PlanGap{Kind: "gear-check", Discipline: d.Slug, Name: d.Name})
		}
		if plan.Prep[d.Slug].Briefing == 0 {
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
		for _, m := range it.Matches {
			out.Live = out.Live || m.Done || !m.Started.IsZero()
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
			done = done && (m.Done || !m.Ended.IsZero())
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
	begun := map[string]bool{}
	for _, it := range in.Items {
		for _, m := range it.Matches {
			begun[it.Discipline] = begun[it.Discipline] || m.Done || !m.Started.IsZero()
		}
	}
	for _, d := range inputs {
		// The gear check and the staff briefing end as the first pool starts (#149).
		first := rows[key{d.Slug, "pools"}]
		if first == nil {
			first = rows[key{d.Slug, "eliminations"}]
		}
		if first == nil || first.Start == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339, first.Start)
		if err != nil {
			continue
		}
		p := in.Prep[d.Slug]
		for _, prep := range []struct {
			stage string
			took  time.Duration
		}{{"gear-check", p.GearCheck}, {"briefing", p.Briefing}} {
			if prep.took > 0 {
				out = append(out, ProgrammeRow{Discipline: d.Slug, Name: d.Name, Stage: prep.stage,
					Start: stamp(at.Add(-prep.took)), End: first.Start, Mats: []int{}, Done: begun[d.Slug]})
			}
		}
	}
	for _, k := range order {
		if row := rows[k]; row != nil {
			out = append(out, *row)
		}
	}
	for _, b := range in.Breaks {
		out = append(out, ProgrammeRow{Name: b.Label, Stage: "break", Start: stamp(b.From), End: stamp(b.To), Mats: []int{},
			Done: !in.Now.IsZero() && in.Now.After(b.To)})
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

// SetPrep writes the gear check and staff briefing of the disciplines named, of those the
// event has. Shared with the browser demo.
func SetPrep(plan *store.Plan, in map[string]store.Prep, slugs []string) error {
	for slug, p := range in {
		if !slices.Contains(slugs, slug) {
			return fmt.Errorf("this event has no discipline %q", slug)
		}
		p, err := CleanPrep(p)
		if err != nil {
			return err
		}
		if plan.Prep == nil {
			plan.Prep = map[string]store.Prep{}
		}
		plan.Prep[slug] = p
	}
	return nil
}

// SetStarts writes when the disciplines named start at the earliest; blank takes the time
// off. Shared with the browser demo.
func SetStarts(plan *store.Plan, in map[string]string, slugs []string) error {
	for slug, s := range in {
		if !slices.Contains(slugs, slug) {
			return fmt.Errorf("this event has no discipline %q", slug)
		}
		s, err := CleanStart(s)
		if err != nil {
			return err
		}
		if s == "" {
			delete(plan.Starts, slug)
			continue
		}
		if plan.Starts == nil {
			plan.Starts = map[string]string{}
		}
		plan.Starts[slug] = s
	}
	return nil
}
