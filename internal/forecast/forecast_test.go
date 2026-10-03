package forecast_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
)

var day = time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)

func at(hhmm string) time.Time {
	var h, m int
	fmt.Sscanf(hhmm, "%d:%d", &h, &m)
	return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

func item(id, disc, kind string, mat, seq, matches int) forecast.Item {
	it := forecast.Item{ID: id, Discipline: disc, Kind: kind, Mat: mat, Seq: seq}
	for i := 0; i < matches; i++ {
		it.Matches = append(it.Matches, forecast.Match{Key: fmt.Sprintf("%s#%d", id, i+1)})
	}
	return it
}

// Four-minute matches and a minute between them, from nine.
var tpl = forecast.Timings{Match: 4 * time.Minute, Changeover: time.Minute, BeforeElims: 15 * time.Minute, Start: at("09:00")}

func spanOf(r forecast.Result, id string) forecast.Span {
	for _, s := range r.Items {
		if s.ID == id {
			return s
		}
	}
	return forecast.Span{}
}

func hm(t time.Time) string { return t.Format("15:04") }

func TestAPlannedDay(t *testing.T) {
	r := forecast.Run(forecast.Input{Timings: tpl, Items: []forecast.Item{
		item("ls/pool-1", "ls", "pool", 1, 1, 3),
		item("ls/pool-2", "ls", "pool", 2, 1, 6),
		item("ls/final", "ls", "final", 1, 2, 1),
	}})
	// Mat 1: 09:00, 09:05, 09:10, done 09:14. Mat 2: six matches, done 09:29.
	if s := spanOf(r, "ls/pool-1"); hm(s.Start) != "09:00" || hm(s.End) != "09:14" {
		t.Errorf("pool 1 should run 09:00-09:14: %s-%s", hm(s.Start), hm(s.End))
	}
	if s := spanOf(r, "ls/pool-2"); hm(s.End) != "09:29" {
		t.Errorf("pool 2 should end 09:29: %s", hm(s.End))
	}
	// The final waits for every pool, and the pause before the bracket: 09:29 + 15.
	if s := spanOf(r, "ls/final"); hm(s.Start) != "09:45" {
		t.Errorf("the final should start at 09:45, after both pools and the pause: %s", hm(s.Start))
	}
	if s := spanOf(r, "ls/final"); s.PlannedStart != s.Start {
		t.Errorf("with nothing fenced the forecast is the plan: %v %v", s.PlannedStart, s.Start)
	}
	if got := hm(r.Matches["ls/pool-1#2"]); got != "09:05" {
		t.Errorf("the second match of pool 1 should be expected at 09:05: %s", got)
	}
}

func TestLunchStopsTheMats(t *testing.T) {
	in := forecast.Input{Timings: tpl, Breaks: []forecast.Break{{From: at("09:04"), To: at("10:00")}},
		Items: []forecast.Item{item("ls/pool-1", "ls", "pool", 1, 1, 2)}}
	r := forecast.Run(in)
	// The first match runs 09:00-09:04; the second would start at 09:05, inside lunch.
	if got := hm(r.Matches["ls/pool-1#2"]); got != "10:00" {
		t.Errorf("the second match should wait for the end of lunch: %s", got)
	}
}

// A slow mat pushes its own later work and nobody else's.
func TestASlowMatPushesOnlyItself(t *testing.T) {
	slow := item("ls/pool-1", "ls", "pool", 1, 1, 6)
	for i := 0; i < 3; i++ {
		start := at("09:00").Add(time.Duration(i) * 11 * time.Minute)
		slow.Matches[i].Started, slow.Matches[i].Ended = start, start.Add(10*time.Minute)
	}
	// Mat 2 has fenced its whole pool on time, by 09:29.
	fast := item("sa/pool-1", "sa", "pool", 2, 1, 6)
	for i := 0; i < 6; i++ {
		start := at("09:00").Add(time.Duration(i) * 5 * time.Minute)
		fast.Matches[i].Started, fast.Matches[i].Ended = start, start.Add(4*time.Minute)
	}
	r := forecast.Run(forecast.Input{Timings: tpl, Now: at("09:33"), Items: []forecast.Item{slow, fast,
		item("ls/pool-2", "ls", "pool", 1, 2, 3), item("sa/pool-2", "sa", "pool", 2, 2, 3)}})
	if p := r.Pace[1]; p.Match <= 6*time.Minute || p.Samples != 3 {
		t.Errorf("mat 1's matches take ten minutes, and its pace should show it: %+v", p)
	}
	if p := r.Pace[2]; p.Match > 5*time.Minute {
		t.Errorf("mat 2's pace should stay near four minutes: %+v", p)
	}
	idle := forecast.Run(forecast.Input{Timings: tpl, Items: []forecast.Item{slow, item("x/pool-1", "x", "pool", 3, 1, 2)}})
	if p := idle.Pace[3]; p.Samples != 0 || p.Match <= 4*time.Minute {
		t.Errorf("a mat with nothing measured goes at the hall's pace, from no matches of its own: %+v", p)
	}
	ls2, sa2 := spanOf(r, "ls/pool-2"), spanOf(r, "sa/pool-2")
	if !ls2.Start.After(ls2.PlannedStart.Add(30 * time.Minute)) {
		t.Errorf("mat 1 is running late, so its next pool should be well behind plan: %s vs %s", hm(ls2.Start), hm(ls2.PlannedStart))
	}
	if sa2.Start.After(sa2.PlannedStart.Add(15 * time.Minute)) {
		t.Errorf("mat 2 is on time, so its next pool should be near plan: %s vs %s", hm(sa2.Start), hm(sa2.PlannedStart))
	}
	// Nothing still to come starts in the past.
	for k, when := range r.Matches {
		if when.Before(at("09:33")) {
			t.Errorf("%s is expected at %s, before now", k, hm(when))
		}
	}
}

func TestSomebodyInTwoPlacesAtOnce(t *testing.T) {
	a := item("ls/pool-1", "ls", "pool", 1, 1, 6)
	b := item("sa/pool-1", "sa", "pool", 2, 1, 6)
	a.People, b.People = []string{"pr-astrid", "pr-bo"}, []string{"pr-astrid"}
	r := forecast.Run(forecast.Input{Timings: tpl, Items: []forecast.Item{a, b}})
	var found []forecast.Warning
	for _, w := range r.Warnings {
		if w.Kind == "overlap" {
			found = append(found, w)
		}
	}
	if len(found) != 1 || found[0].Person != "pr-astrid" || len(found[0].Items) != 2 {
		t.Errorf("Astrid is in two pools at once, and only she: %+v", found)
	}
}

func TestTheDayOverrunsAndAPlanOutOfOrder(t *testing.T) {
	late := tpl
	late.Close = at("09:20")
	r := forecast.Run(forecast.Input{Timings: late, Items: []forecast.Item{
		// The final before the pool it waits for, on the same mat.
		item("ls/final", "ls", "final", 1, 1, 1),
		item("ls/pool-1", "ls", "pool", 1, 2, 6),
	}})
	kinds := map[string]bool{}
	for _, w := range r.Warnings {
		kinds[w.Kind] = true
	}
	if !kinds["overrun"] || !kinds["dependency"] {
		t.Errorf("the day ends after the venue closes and the final is before its pool: %+v", r.Warnings)
	}
}

func TestTheReportLearnsFromTheDayButNotItsAnomalies(t *testing.T) {
	p := item("ls/pool-1", "ls", "pool", 1, 1, 3)
	for i, mins := range []int{4, 6, 30} {
		start := at("09:00").Add(time.Duration(i) * 40 * time.Minute)
		p.Matches[i].Started, p.Matches[i].Ended = start, start.Add(time.Duration(mins)*time.Minute)
	}
	rep := forecast.Measure(forecast.Input{Items: []forecast.Item{p}, Anomalies: map[string]bool{"ls/pool-1#3": true}})
	if len(rep.Matches) != 3 || rep.Samples != 2 || rep.Match != 5*time.Minute || !rep.Matches[2].Anomaly {
		t.Errorf("three matches measured, the half-hour one left out of the average of five minutes: %+v", rep)
	}
}

// Four pools stacked on one mat of two: the suggestion spreads them, keeps what is under
// way and what is pinned, and finishes the day sooner.
func TestASuggestionSpreadsTheWork(t *testing.T) {
	running := item("ls/pool-1", "ls", "pool", 1, 1, 3)
	running.Matches[0].Started = at("09:00")
	pinned := item("ls/pool-4", "ls", "pool", 1, 4, 3)
	pinned.Pinned = true
	in := forecast.Input{Timings: tpl, Now: at("09:02"), Items: []forecast.Item{
		running,
		item("ls/pool-2", "ls", "pool", 1, 2, 3),
		item("ls/pool-3", "ls", "pool", 1, 3, 3),
		pinned,
		item("ls/final", "ls", "final", 1, 5, 1),
	}}
	s := forecast.Suggest(in, 2)
	if s.Order[1][0] != "ls/pool-1" {
		t.Errorf("the pool under way stays first on its mat: %v", s.Order)
	}
	on := map[string]int{}
	for mat, ids := range s.Order {
		for _, id := range ids {
			on[id] = mat
		}
	}
	if on["ls/pool-4"] != 1 {
		t.Errorf("the pinned pool stays on mat 1: %v", s.Order)
	}
	if len(s.Order[2]) == 0 {
		t.Errorf("mat 2 is idle and should be given work: %v", s.Order)
	}
	if !s.End.Before(s.Before) {
		t.Errorf("spreading the pools should end the day sooner: %s, was %s", hm(s.End), hm(s.Before))
	}
	ids := s.Order[on["ls/final"]]
	if ids[len(ids)-1] != "ls/final" {
		t.Errorf("the final comes after the pools it waits for: %v", s.Order)
	}
	moved := false
	for _, m := range s.Moves {
		if m.ID == "ls/pool-1" {
			t.Errorf("the pool under way must not move: %+v", m)
		}
		moved = moved || (m.ToMat == 2 && !m.Start.IsZero())
	}
	if !moved {
		t.Errorf("the moves should say what goes to mat 2, and when it would start: %+v", s.Moves)
	}
}

// A match finished without times in its log -- events posted without them -- is done: it
// is not fenced again, and the day is live (found when the demo played a day out).
func TestAFinishedMatchWithoutTimesIsDone(t *testing.T) {
	p := item("ls/pool-1", "ls", "pool", 1, 1, 3)
	for i := range p.Matches {
		p.Matches[i].Done = true
	}
	next := item("ls/pool-2", "ls", "pool", 1, 2, 2)
	r := forecast.Run(forecast.Input{Timings: tpl, Now: at("11:00"), Items: []forecast.Item{p, next}})
	if _, again := r.Matches["ls/pool-1#1"]; again {
		t.Error("a finished match must not be expected again")
	}
	if got := hm(r.Matches["ls/pool-2#1"]); got != "11:00" {
		t.Errorf("with the first pool done the day is live, and the next starts now: %s", got)
	}
}

// Blocks of the day (#136): the sabre runs after the longsword is done, not interleaved.
func TestABlockWaitsForTheOneBefore(t *testing.T) {
	ls1, ls2 := item("ls/pool-1", "ls", "pool", 1, 1, 3), item("ls/elim-1", "ls", "eliminations", 1, 2, 2)
	sa := item("sa/pool-1", "sa", "pool", 2, 1, 3)
	ls1.Session, ls2.Session, sa.Session = 1, 1, 2
	r := forecast.Run(forecast.Input{Timings: tpl, Items: []forecast.Item{ls1, ls2, sa}})
	if s, e := spanOf(r, "sa/pool-1"), spanOf(r, "ls/elim-1"); s.Start.Before(e.End) {
		t.Errorf("the sabre's block starts once the longsword's is done: sabre %s, longsword ends %s", hm(s.Start), hm(e.End))
	}
	// The same block runs side by side.
	sa.Session = 1
	r = forecast.Run(forecast.Input{Timings: tpl, Items: []forecast.Item{ls1, ls2, sa}})
	if s := spanOf(r, "sa/pool-1"); hm(s.Start) != "09:00" {
		t.Errorf("in one block the sabre runs beside the longsword: %s", hm(s.Start))
	}
}

// Finals held to the end of the day run after everything else, the later blocks' first,
// on mat 1; and a suggestion never puts a fencer in two places at once.
func TestHeldFinalsAndNoDoubleBooking(t *testing.T) {
	lsPool, saPool := item("ls/pool-1", "ls", "pool", 1, 1, 3), item("sa/pool-1", "sa", "pool", 2, 1, 3)
	lsPool.Session, saPool.Session = 1, 1
	lsPool.People, saPool.People = []string{"astrid"}, []string{"astrid"}
	lsFinal, saFinal := item("ls/final", "ls", "final", 2, 2, 1), item("sa/final", "sa", "final", 1, 2, 1)
	lsFinal.Session, saFinal.Session = 1, 2
	lsFinal.Held, saFinal.Held = true, true
	saPool.Session = 2
	in := forecast.Input{Timings: tpl, Items: []forecast.Item{lsPool, saPool, lsFinal, saFinal}}
	r := forecast.Run(in)
	for _, id := range []string{"ls/final", "sa/final"} {
		if spanOf(r, id).Start.Before(spanOf(r, "sa/pool-1").End) {
			t.Errorf("%s is held until everything else is done", id)
		}
	}
	s := forecast.Suggest(in, 2)
	if got := s.Order[1]; len(got) < 2 || got[len(got)-2] != "sa/final" || got[len(got)-1] != "ls/final" {
		t.Errorf("the finals go last on mat 1, the sabre's then the longsword's: %v", s.Order)
	}

	// Astrid in two pools of one block: the suggestion puts them one after the other.
	saPool.Session = 1
	in.Items = []forecast.Item{lsPool, saPool}
	s = forecast.Suggest(in, 2)
	if got := s.Order[1]; len(got) != 2 || got[0] != "ls/pool-1" || got[1] != "sa/pool-1" {
		t.Errorf("Astrid fences both pools: the sabre's should queue behind the longsword's on its mat, so no timing can overlap them: %v", s.Order)
	}
	applied := in
	applied.Items = []forecast.Item{lsPool, saPool}
	applied.Items[1].Mat, applied.Items[1].Seq = 1, 2
	if r := forecast.Run(applied); len(r.Warnings) != 0 {
		t.Errorf("the suggested plan should forecast no overlap: %+v", r.Warnings)
	}
}

// A semi-final waits for the quarter-finals that feed it, wherever they run (#120): a slow
// mat 2 holds up mat 1's semi-final.
func TestASemiFinalWaitsForItsQuarterFinalsOnAnotherMat(t *testing.T) {
	e1 := item("ls/elim-1", "ls", "eliminations", 1, 1, 0)
	e2 := item("ls/elim-2", "ls", "eliminations", 2, 1, 0)
	e1.Matches = []forecast.Match{{Key: "ls/qf1"}, {Key: "ls/qf3"}, {Key: "ls/sf1", After: []string{"ls/qf1", "ls/qf2"}}}
	e2.Matches = []forecast.Match{{Key: "ls/qf2"}, {Key: "ls/qf4"}, {Key: "ls/sf2", After: []string{"ls/qf3", "ls/qf4"}}}
	e2.NotBefore = at("10:00") // mat 2 starts late
	r := forecast.Run(forecast.Input{Timings: tpl, Items: []forecast.Item{e1, e2}})
	qf2End := at("10:04")
	if got := r.Matches["ls/sf1"]; got.Before(qf2End) {
		t.Errorf("the first semi-final needs the winner of quarter-final 2, which ends at 10:04 on mat 2; it is timed at %s", hm(got))
	}
	if len(r.Warnings) != 0 {
		t.Errorf("the lanes waiting on each other is no plan out of order: %+v", r.Warnings)
	}
}

// A mat away for part of the day waits it out; the others do not (#123).
func TestAMatAwayWaitsItOut(t *testing.T) {
	in := forecast.Input{Timings: tpl, MatBreaks: map[int][]forecast.Break{1: {{From: at("09:00"), To: at("10:00")}}},
		Items: []forecast.Item{item("ls/pool-1", "ls", "pool", 1, 1, 2), item("ls/pool-2", "ls", "pool", 2, 1, 2)}}
	r := forecast.Run(in)
	if s := spanOf(r, "ls/pool-1"); hm(s.Start) != "10:00" {
		t.Errorf("mat 1 is away until ten: %s", hm(s.Start))
	}
	if s := spanOf(r, "ls/pool-2"); hm(s.Start) != "09:00" {
		t.Errorf("mat 2 is there from nine: %s", hm(s.Start))
	}
}
