package hall

import (
	"slices"
	"sort"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
)

// The programme, generated from the plan (#149): what the landing pages, the info sheet
// and the signup's planned starts all read.

// Before a discipline's first pool, the gear check ends this long ahead of it and the
// staff briefing this long (#153): the two are not held at once, and the first pool does
// not start the moment the last fencer has been checked.
const (
	gearCheckEndsBefore = 15 * time.Minute
	briefingEndsBefore  = 30 * time.Minute
)

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

// programme is the fencing part of the day, derived from the forecast: each discipline's
// pools, eliminations and final, with when and where.
func programme(in forecast.Input, r forecast.Result, inputs []MatsInput) []ProgrammeRow {
	rows := stageRows(in, r, inputs)
	begun := map[string]bool{}
	live := false
	for _, it := range in.Items {
		if it.Begun() {
			begun[it.Discipline], live = true, true
		}
	}
	out := []ProgrammeRow{}
	for _, d := range inputs {
		out = append(out, prepRows(d, rows, in.Prep[d.Slug], begun[d.Slug])...)
	}
	for _, d := range inputs {
		for _, stage := range []string{"pools", "eliminations", "final"} {
			if row := rows[stageKey{d.Slug, stage}]; row != nil {
				out = append(out, *row)
			}
		}
	}
	for _, b := range in.Breaks {
		// Over once the day is under way and the clock is past it; before the day, a
		// planned lunch is never done.
		out = append(out, ProgrammeRow{Name: b.Label, Stage: "break", Start: stamp(b.From), End: stamp(b.To), Mats: []int{},
			Done: live && in.Now.After(b.To)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// stageKey names one row of the programme's fencing: a stage of a discipline.
type stageKey struct{ disc, stage string }

// stageOf is the stage an item is part of: a bronze match counts with the eliminations.
func stageOf(kind string) string {
	switch kind {
	case forecast.KindPool:
		return "pools"
	case forecast.KindFinal:
		return "final"
	}
	return "eliminations"
}

// stageRows gathers the items into a row per stage: from the first to start to the last to
// end, on every mat any of them runs on, and done when all of them are.
func stageRows(in forecast.Input, r forecast.Result, inputs []MatsInput) map[stageKey]*ProgrammeRow {
	spans := map[string]forecast.Span{}
	for _, s := range r.Items {
		spans[s.ID] = s
	}
	nameOf := map[string]string{}
	for _, d := range inputs {
		nameOf[d.Slug] = d.Name
	}
	rows := map[stageKey]*ProgrammeRow{}
	for _, it := range in.Items {
		stage := stageOf(it.Kind)
		s := spans[it.ID]
		k := stageKey{it.Discipline, stage}
		row := rows[k]
		done := it.Finished()
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
		if !slices.Contains(row.Mats, it.Mat) {
			row.Mats = append(row.Mats, it.Mat)
			sort.Ints(row.Mats)
		}
	}
	return rows
}

// prepRows are a discipline's gear check and then its staff briefing, ahead of its first
// pool (#153). None until the fencing has a time.
func prepRows(d MatsInput, rows map[stageKey]*ProgrammeRow, p forecast.Prep, begun bool) []ProgrammeRow {
	first := rows[stageKey{d.Slug, "pools"}]
	if first == nil {
		first = rows[stageKey{d.Slug, "eliminations"}]
	}
	if first == nil || first.Start == "" {
		return nil
	}
	at, err := time.Parse(time.RFC3339, first.Start)
	if err != nil {
		return nil
	}
	gearEnds := at.Add(-gearCheckEndsBefore)
	gearStarts := gearEnds.Add(-p.GearCheck)
	// A longer gear check moves the briefing ahead of it, rather than on top of it.
	briefingEnds := at.Add(-briefingEndsBefore)
	if p.GearCheck > 0 && gearStarts.Before(briefingEnds) {
		briefingEnds = gearStarts
	}
	var out []ProgrammeRow
	for _, prep := range []struct {
		stage string
		took  time.Duration
		ends  time.Time
	}{{"gear-check", p.GearCheck, gearEnds}, {"briefing", p.Briefing, briefingEnds}} {
		if prep.took > 0 {
			out = append(out, ProgrammeRow{Discipline: d.Slug, Name: d.Name, Stage: prep.stage,
				Start: stamp(prep.ends.Add(-prep.took)), End: stamp(prep.ends), Mats: []int{}, Done: begun})
		}
	}
	return out
}
