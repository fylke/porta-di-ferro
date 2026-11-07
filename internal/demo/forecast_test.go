package demo_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fylke/porta-di-ferro/internal/demo"
	"github.com/fylke/porta-di-ferro/internal/hall"
)

func TestDemoUsesTypicalPoolAndEliminationTimings(t *testing.T) {
	e := demo.NewEvent()
	for _, slug := range []string{"open-steel-longsword", "open-sabre", "sword-and-buckler"} {
		if response := e.Request("DELETE", "/api/disciplines/"+slug, nil); response.Status != 200 {
			t.Fatalf("removing fixture discipline: %d %s", response.Status, response.Body)
		}
	}
	for _, request := range []struct{ method, path, body string }{
		{"POST", "/api/disciplines", `{"name":"Timing"}`},
		{"PUT", "/api/d/timing/tournament", `{"mats":2,"minPoolSize":4,"maxPoolSize":6}`},
		{"PUT", "/api/plan/expected", `{"timing":8}`},
	} {
		if response := e.Request(request.method, request.path, []byte(request.body)); response.Status >= 300 {
			t.Fatalf("%s: %d %s", request.path, response.Status, response.Body)
		}
	}
	response := e.Request("GET", "/api/forecast", nil)
	var view hall.ForecastView
	if err := json.Unmarshal([]byte(response.Body), &view); err != nil || response.Status != 200 {
		t.Fatalf("forecast: %d %v", response.Status, err)
	}
	var first, final time.Time
	for _, span := range view.Items {
		start, _ := time.Parse(time.RFC3339, span.PlannedStart)
		end, _ := time.Parse(time.RFC3339, span.PlannedEnd)
		if strings.HasPrefix(span.ID, "timing/pool-") && end.Sub(start) != 40*time.Minute {
			t.Errorf("projected four-person pool: %s", end.Sub(start))
		}
		if strings.HasPrefix(span.ID, "timing/elim-") && (first.IsZero() || start.Before(first)) {
			first = start
		}
		if span.ID == "timing/final" {
			final = end
		}
	}
	if got := final.Sub(first); first.IsZero() || got != 40*time.Minute {
		t.Errorf("projected eight-person bracket on two mats: %s", got)
	}
}

// The demo's day has a past -- the fixture's matches carry the times they were fenced at --
// so its forecast has a pace to learn and its pages have times to show (phase 4).
func TestTheDemoForecastsItsDay(t *testing.T) {
	e := demo.NewEvent()
	res := e.Request("GET", "/api/forecast", nil)
	var v hall.ForecastView
	if err := json.Unmarshal([]byte(res.Body), &v); err != nil || res.Status != 200 {
		t.Fatalf("GET /api/forecast: %d %v %s", res.Status, err, res.Body)
	}
	if !v.Live || len(v.Items) == 0 || v.End == "" {
		t.Fatalf("the demo is halfway through its pools, so its day is live and forecast: %+v", v)
	}
	learned := false
	for _, p := range v.Pace {
		learned = learned || p.Samples > 0
	}
	if !learned {
		t.Errorf("the fixture's fenced matches should give the mats a measured pace: %+v", v.Pace)
	}
	if len(v.Programme) == 0 {
		t.Error("the derived programme should list the fencing")
	}
	if v.Timings.Start == "" || v.Timings.Start == "09:00" && len(v.Items) == 0 {
		t.Errorf("the demo's day should start when its first match did: %+v", v.Timings)
	}

	s := snapIn(t, e, "open-sabre")
	if s.Pools[0].Matches[0].Eta == "" {
		t.Errorf("the sabre's matches, all to come, should have expected times: %+v", s.Pools[0].Matches[0])
	}

	res = e.Request("POST", "/api/plan/suggest", nil)
	var sug hall.SuggestionView
	if err := json.Unmarshal([]byte(res.Body), &sug); err != nil || res.Status != 200 || sug.Signature == "" {
		t.Fatalf("POST /api/plan/suggest: %d %s", res.Status, res.Body)
	}
	b, _ := json.Marshal(map[string]string{"signature": sug.Signature})
	if res := e.Request("POST", "/api/plan/apply", b); res.Status != 200 {
		t.Errorf("applying the suggestion: %d %s", res.Status, res.Body)
	}
	var rep hall.ReportView
	_ = json.Unmarshal([]byte(e.Request("GET", "/api/plan/report", nil).Body), &rep)
	if rep.Samples == 0 || len(rep.Matches) == 0 {
		t.Errorf("the measured day should have the fixture's matches: %+v", rep)
	}
}

// The demo's day as #136 drew it: the longsword, then the sabre and the sword and buckler
// side by side, then every final on mat 1 -- and nobody due in two places at once.
func TestTheDemoDayRunsInBlocks(t *testing.T) {
	e := demo.NewEvent()
	var v hall.ForecastView
	_ = json.Unmarshal([]byte(e.Request("GET", "/api/forecast", nil).Body), &v)
	for _, w := range v.Warnings {
		if w.Kind == "overlap" {
			t.Errorf("nobody should be due in two places at once: %+v", w)
		}
	}
	end := func(prefix string) (last string) {
		for _, it := range v.Items {
			if strings.HasPrefix(it.ID, prefix) && it.End > last && !isFinal(it.ID) {
				last = it.End
			}
		}
		return last
	}
	longsword := end("open-steel-longsword/")
	for _, it := range v.Items {
		later := strings.HasPrefix(it.ID, "open-sabre/") || strings.HasPrefix(it.ID, "sword-and-buckler/")
		if later && !isFinal(it.ID) && it.Start < longsword {
			t.Errorf("%s should wait for the longsword to finish: starts %s, longsword ends %s", it.ID, it.Start, longsword)
		}
	}
	var mats hall.MatsView
	_ = json.Unmarshal([]byte(e.Request("GET", "/api/mats", nil).Body), &mats)
	var finals []string
	for _, it := range mats.Items {
		if it.Kind == "final" {
			if it.Mat != 1 {
				t.Errorf("finals are held for mat 1: %s on mat %d", it.ID, it.Mat)
			}
			finals = append(finals, it.ID)
		}
	}
	if len(finals) != 3 || finals[len(finals)-1] != "open-steel-longsword/final" {
		t.Errorf("the longsword's final closes the day: %v", finals)
	}
}

func isFinal(id string) bool {
	return strings.HasSuffix(id, "/final") || strings.HasSuffix(id, "/bronze")
}

// Every mat of the demo started its day at one time, and none is late from the first look
// (#131): a mat that fenced fewer matches was slower, not late to start.
func TestTheDemoMatsStartTogether(t *testing.T) {
	e := demo.NewEvent()
	var v hall.ForecastView
	_ = json.Unmarshal([]byte(e.Request("GET", "/api/forecast", nil).Body), &v)
	first := map[int]hall.ItemTimes{}
	for _, it := range v.Items {
		if !strings.HasPrefix(it.ID, "open-steel-longsword/pool-") {
			continue
		}
		if f, ok := first[it.Mat]; !ok || it.Start < f.Start {
			first[it.Mat] = it
		}
	}
	if len(first) < 2 {
		t.Fatalf("the longsword's pools should be on several mats: %+v", first)
	}
	minutes := func(at string) int {
		when, err := time.Parse(time.RFC3339, at)
		if err != nil {
			t.Fatalf("%q is no time: %v", at, err)
		}
		return when.Hour()*60 + when.Minute()
	}
	for mat, it := range first {
		if d := minutes(it.Start) - minutes(first[1].Start); d < -1 || d > 1 {
			t.Errorf("mat %d started at %s, mat 1 at %s: every mat starts together", mat, it.Start, first[1].Start)
		}
		if late := minutes(it.Start) - minutes(it.PlannedStart); late > 5 {
			t.Errorf("mat %d's first pool is %d minutes late from the outset: %+v", mat, late, it)
		}
	}
}
