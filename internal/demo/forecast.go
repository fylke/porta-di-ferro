package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The demo's plan and forecast (phase 4): the coordinator's own functions --
// httpapi.ForecastInput, forecast.Run, forecast.Suggest, ApplySuggestion -- over the demo's
// in-memory event, so the board, the planning panel and every "about 10:40" in the demo say
// what a real event would.

// timings is the template in force: the event's, over what the visitor kept for new events
// in this tab, over the built-in one.
func (e *Event) timings() store.Timings {
	return httpapi.TimingsOr(e.plan.Timings, httpapi.TimingsOr(e.defaults, httpapi.DefaultTimings))
}

func (e *Event) times() (forecast.Input, forecast.Result, []httpapi.MatsInput) {
	placed, _ := e.placements()
	var inputs []httpapi.MatsInput
	for _, d := range e.disciplines {
		snap, err := d.placedSnapshot()
		if err != nil {
			continue
		}
		inputs = append(inputs, httpapi.MatsInput{Slug: d.slug, Name: d.tournament.Discipline, Snapshot: snap,
			Expected: e.plan.Expected[d.slug]})
	}
	e.withSessions(inputs)
	in := httpapi.ForecastInput(inputs, placed, e.plan, e.info, e.timings(), time.Now())
	return in, forecast.Run(in), inputs
}

func (e *Event) forecastView() httpapi.ForecastView {
	in, r, inputs := e.times()
	v := httpapi.ViewForecast(in, r, inputs, e.timings())
	v.Expected = e.plan.Expected
	v.Sessions, v.FinalsLast = httpapi.Sessions(e.plan, e.slugs()), e.plan.FinalsLast
	return v
}

// applyPlanSuggestion lays the mats out as the suggestion would.
func (e *Event) applyPlanSuggestion() {
	in, _, _ := e.times()
	placed, mats := e.placements()
	e.plan.Items = httpapi.ApplySuggestion(placed, forecast.Suggest(in, mats))
}

// startAtFirstMatch starts the planned day when the fixture's first match started, so a
// visitor at any hour sees a day that began this morning for them.
func (e *Event) startAtFirstMatch() {
	in, _, _ := e.times()
	var first time.Time
	for _, it := range in.Items {
		for _, m := range it.Matches {
			if !m.Started.IsZero() && (first.IsZero() || m.Started.Before(first)) {
				first = m.Started
			}
		}
	}
	if !first.IsZero() {
		e.plan.Timings.Start = first.Truncate(5 * time.Minute).Format("15:04")
	}
}

// planRequest answers the plan's times, or reports it has none.
func (e *Event) planRequest(method, bare string, body []byte) (Response, bool) {
	switch {
	case method == "GET" && bare == "/api/forecast":
		return ok(e.forecastView()), true
	case method == "PUT" && bare == "/api/plan/timings":
		var in store.Timings
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		in, err := httpapi.CleanTimings(in)
		if err != nil {
			return fail(400, err), true
		}
		e.plan.Timings = in
		return changed(e.forecastView()), true
	case method == "POST" && bare == "/api/plan/timings/learn":
		in, _, _ := e.times()
		rep := forecast.Measure(in)
		if rep.Samples == 0 {
			return fail(409, errors.New("nothing has been fenced yet to learn from")), true
		}
		e.plan.Timings.Match = int(rep.Match / time.Second)
		if rep.Changeover > 0 {
			e.plan.Timings.Changeover = int(rep.Changeover / time.Second)
		}
		return changed(e.forecastView()), true
	case method == "POST" && bare == "/api/plan/timings/default":
		keep := e.timings()
		keep.Close = ""
		e.defaults = keep
		return changed(map[string]any{"file": "this browser", "timings": keep}), true
	case method == "PUT" && bare == "/api/plan/expected":
		var in map[string]int
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		if e.plan.Expected == nil {
			e.plan.Expected = map[string]int{}
		}
		for slug, n := range in {
			if n < 0 || n > 200 {
				return fail(400, errors.New("expect between 0 and 200")), true
			}
			if n == 0 {
				delete(e.plan.Expected, slug)
			} else {
				e.plan.Expected[slug] = n
			}
		}
		return changed(e.forecastView()), true
	case method == "GET" && bare == "/api/plan/report":
		in, _, inputs := e.times()
		return ok(httpapi.ViewReport(forecast.Measure(in), inputs)), true
	case method == "PUT" && bare == "/api/plan/anomalies":
		var in struct {
			Key     string `json:"key"`
			Anomaly bool   `json:"anomaly"`
		}
		if err := json.Unmarshal(body, &in); err != nil || in.Key == "" {
			return fail(400, fmt.Errorf("say which match")), true
		}
		e.plan.Anomalies = httpapi.SetAnomaly(e.plan.Anomalies, in.Key, in.Anomaly)
		fin, _, inputs := e.times()
		return changed(httpapi.ViewReport(forecast.Measure(fin), inputs)), true
	case method == "PUT" && bare == "/api/plan/sessions":
		var in map[string]int
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		if e.plan.Sessions == nil {
			e.plan.Sessions = map[string]int{}
		}
		for slug, n := range in {
			if e.find(slug) == nil || n < 1 || n > 20 {
				return fail(400, errors.New("a block is a discipline of this event and a number from 1 to 20")), true
			}
			e.plan.Sessions[slug] = n
		}
		return changed(e.forecastView()), true
	case method == "PUT" && bare == "/api/plan/finals":
		var in struct {
			Last bool `json:"last"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err), true
		}
		e.plan.FinalsLast = in.Last
		return changed(e.forecastView()), true
	case method == "POST" && bare == "/api/plan/suggest":
		in, _, _ := e.times()
		_, mats := e.placements()
		return ok(httpapi.ViewSuggestion(forecast.Suggest(in, mats))), true
	case method == "POST" && bare == "/api/plan/apply":
		var want struct {
			Signature string `json:"signature"`
		}
		if err := json.Unmarshal(body, &want); err != nil {
			return fail(400, err), true
		}
		in, _, _ := e.times()
		placed, mats := e.placements()
		s := forecast.Suggest(in, mats)
		if httpapi.SignatureOf(s) != want.Signature {
			b, _ := json.Marshal(map[string]any{
				"error":      "the plan has changed since that suggestion; here is a new one",
				"suggestion": httpapi.ViewSuggestion(s),
			})
			return Response{Status: 409, ContentType: "application/json; charset=utf-8", Body: string(b)}, true
		}
		e.plan.Items = httpapi.ApplySuggestion(placed, s)
		return changed(e.forecastView()), true
	}
	return Response{}, false
}
