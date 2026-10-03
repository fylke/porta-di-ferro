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
