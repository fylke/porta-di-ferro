package demo_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/demo"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The demo's day has a past -- the fixture's matches carry the times they were fenced at --
// so its forecast has a pace to learn and its pages have times to show (phase 4).
func TestTheDemoForecastsItsDay(t *testing.T) {
	e := demo.NewEvent()
	res := e.Request("GET", "/api/forecast", nil)
	var v httpapi.ForecastView
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
	var sug httpapi.SuggestionView
	if err := json.Unmarshal([]byte(res.Body), &sug); err != nil || res.Status != 200 || sug.Signature == "" {
		t.Fatalf("POST /api/plan/suggest: %d %s", res.Status, res.Body)
	}
	b, _ := json.Marshal(map[string]string{"signature": sug.Signature})
	if res := e.Request("POST", "/api/plan/apply", b); res.Status != 200 {
		t.Errorf("applying the suggestion: %d %s", res.Status, res.Body)
	}
	var rep httpapi.ReportView
	_ = json.Unmarshal([]byte(e.Request("GET", "/api/plan/report", nil).Body), &rep)
	if rep.Samples == 0 || len(rep.Matches) == 0 {
		t.Errorf("the measured day should have the fixture's matches: %+v", rep)
	}
}

// The demo's day as #136 drew it: the longsword, then the sabre and the sword and buckler
// side by side, then every final on mat 1 -- and nobody due in two places at once.
func TestTheDemoDayRunsInBlocks(t *testing.T) {
	e := demo.NewEvent()
	var v httpapi.ForecastView
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
	var mats httpapi.MatsView
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
