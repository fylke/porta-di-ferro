package demo_test

import (
	"encoding/json"
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
