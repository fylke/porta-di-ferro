package httpapi_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The plan's times (phase 4), over HTTP: the forecast, the template, pins and holds,
// planning before the entries, estimated times on every match, and the measured day.

func clockOf(s string) string {
	if len(s) < 16 {
		return ""
	}
	return s[11:16]
}

func (h *hall) forecast() httpapi.ForecastView {
	h.t.Helper()
	var v httpapi.ForecastView
	h.must("GET", "/api/forecast", nil, &v)
	return v
}

func timesOf(v httpapi.ForecastView, id string) httpapi.ItemTimes {
	for _, it := range v.Items {
		if it.ID == id {
			return it
		}
	}
	return httpapi.ItemTimes{}
}

func TestTheDayIsForecast(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "event")
	h := openHall(t, dir)
	now := time.Date(2026, 11, 14, 8, 0, 0, 0, time.Local)
	h.c.Clock = func() time.Time { return now }
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug

	h.must("PUT", "/api/plan/timings", map[string]any{"match": 300, "changeover": 60, "beforeElims": 600, "start": "09:00", "close": "10:00"}, nil)
	if code := h.do("PUT", "/api/plan/timings", map[string]any{"start": "after lunch"}, nil); code != 400 {
		t.Errorf("a start that is not a time should be refused, got %d", code)
	}
	h.draw("/api/d/"+ls, 8, 2)

	v := h.forecast()
	p1 := timesOf(v, ls+"/pool-1")
	if clockOf(p1.Start) != "09:00" || p1.PlannedStart != p1.Start {
		t.Errorf("pool 1 should start at nine, as planned: %+v", p1)
	}
	if v.Live || v.Timings.Match != 300 {
		t.Errorf("nothing is fenced yet, and the timings should be the event's: %+v", v)
	}
	final := timesOf(v, ls+"/final")
	if final.Start == "" || final.Start < p1.End {
		t.Errorf("the projected final should be timed after the pools: %+v / %+v", final, p1)
	}
	overrun := false
	for _, w := range v.Warnings {
		overrun = overrun || w.Kind == "overrun"
	}
	if !overrun {
		t.Errorf("eight fencers' pools and a bracket do not fit in an hour; the board should say so: %+v", v.Warnings)
	}
	if len(v.Programme) < 2 || v.Programme[0].Stage != "pools" {
		t.Errorf("the derived programme should list the pools, then what follows: %+v", v.Programme)
	}

	// Every match still to come says when it is expected.
	var s httpapi.Snapshot
	h.must("GET", "/api/d/"+ls+"/state", nil, &s)
	if eta := s.Pools[0].Matches[1].Eta; clockOf(eta) == "" {
		t.Errorf("a pending match should have an expected time: %+v", s.Pools[0].Matches[1])
	}

	// Hold the final to a time, and pin it.
	h.must("PATCH", "/api/plan/items/"+ls+"/final", map[string]any{"notBefore": "09:50", "pinned": true}, nil)
	if f := timesOf(h.forecast(), ls+"/final"); clockOf(f.Start) < "09:50" {
		t.Errorf("the final is held until 09:50: %+v", f)
	}
	if it := itemOf(h.mats(), ls+"/final"); !it.Pinned || it.NotBefore != "09:50" {
		t.Errorf("the card should show the pin and the hold: %+v", it)
	}
	if code := h.do("PATCH", "/api/plan/items/"+ls+"/final", map[string]any{"notBefore": "soon"}, nil); code != 400 {
		t.Errorf("a hold that is not a time should be refused, got %d", code)
	}

	// Fence the first match, timed: the day is live, and the report has it.
	first := h.mats().Mats[0].Current.Match.ID
	start := time.Date(2026, 11, 14, 9, 0, 0, 0, time.Local)
	h.must("POST", "/api/d/"+ls+"/matches/"+first+"/events", []map[string]any{
		{"seq": 1, "type": "timer", "at": start.Format(time.RFC3339), "timer": map[string]string{"action": "start"}},
		{"seq": 2, "type": "end", "at": start.Add(7 * time.Minute).Format(time.RFC3339), "end": map[string]string{"reason": "time"}},
	}, nil)
	now = start.Add(8 * time.Minute)
	h.must("GET", "/api/d/"+ls+"/state", nil, &s)
	if s.Pools[0].Matches[0].StartedAt == "" || s.Pools[0].Matches[0].EndedAt == "" {
		t.Errorf("a fenced match should say when it started and ended: %+v", s.Pools[0].Matches[0])
	}
	if v := h.forecast(); !v.Live {
		t.Error("with a match fenced the day is live")
	}
	var rep httpapi.ReportView
	h.must("GET", "/api/plan/report", nil, &rep)
	if len(rep.Matches) != 1 || rep.Match != 420 || rep.Matches[0].Red == "" {
		t.Fatalf("the report should have the seven-minute match, with its fencers: %+v", rep)
	}
	h.must("POST", "/api/plan/timings/learn", nil, &v)
	if v.Timings.Match != 420 {
		t.Errorf("learning should take the measured seven minutes: %+v", v.Timings)
	}
	h.must("PUT", "/api/plan/anomalies", map[string]any{"key": rep.Matches[0].Key, "anomaly": true}, &rep)
	if !rep.Matches[0].Anomaly || rep.Samples != 0 {
		t.Errorf("an anomaly is listed and not learned from: %+v", rep)
	}
	if code := h.do("POST", "/api/plan/timings/learn", nil, nil); code != 409 {
		t.Errorf("with the only match an anomaly there is nothing to learn from, got %d", code)
	}

	// The template for the next event, beside this one.
	h.must("POST", "/api/plan/timings/default", nil, nil)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(dir), "porta-timings.json"))
	if err != nil || !strings.Contains(string(b), `"match": 420`) {
		t.Errorf("the template for new events should be kept beside the event: %v %s", err, b)
	}
}

// Before anyone is entered, the day can be planned from how many each discipline expects.
func TestPlanningBeforeTheEntries(t *testing.T) {
	h := openHall(t, filepath.Join(t.TempDir(), "event"))
	h.c.Clock = func() time.Time { return time.Date(2026, 11, 14, 8, 0, 0, 0, time.Local) }
	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)
	if v := h.forecast(); len(v.Items) != 0 {
		t.Fatalf("nobody entered and nothing expected is nothing to plan: %+v", v.Items)
	}
	h.must("PUT", "/api/plan/expected", map[string]int{"open-sabre": 14}, nil)
	v := h.forecast()
	if len(v.Items) < 4 || timesOf(v, "open-sabre/pool-1").Start == "" {
		t.Errorf("fourteen expected should plan pools and a bracket: %+v", v.Items)
	}
	if it := itemOf(h.mats(), "open-sabre/pool-1"); !it.Projected || it.Status != "planned" {
		t.Errorf("the planned pool is on the board as planned: %+v", it)
	}
}
