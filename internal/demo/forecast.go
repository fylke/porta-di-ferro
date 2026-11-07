package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The demo's plan and forecast (phase 4): the coordinator's own functions --
// hall.ForecastInput, forecast.Run, forecast.Suggest, ApplySuggestion -- over the demo's
// in-memory event, so the board, the planning panel and every "about 10:40" in the demo say
// what a real event would.

// timings is the template in force: the event's, over what the visitor kept for new events
// in this tab, over the built-in one.
func (e *Event) timings() store.Timings {
	return hall.TimingsOr(e.plan.Timings, hall.TimingsOr(e.defaults, hall.DefaultTimings))
}

func (e *Event) times() (forecast.Input, forecast.Result, []hall.MatsInput) {
	placed, _ := e.placements()
	var inputs []hall.MatsInput
	for _, d := range e.disciplines {
		snap, err := d.placedSnapshot()
		if err != nil {
			continue
		}
		inputs = append(inputs, hall.MatsInput{Slug: d.slug, Name: d.tournament.Discipline, Snapshot: snap,
			Expected: e.plan.Expected[d.slug]})
	}
	e.withSessions(inputs)
	in := hall.ForecastInput(inputs, placed, e.plan, e.info, e.timings(), time.Now())
	return in, forecast.Run(in), inputs
}

func (e *Event) forecastView() hall.ForecastView {
	in, r, inputs := e.times()
	v := hall.ViewForecast(in, r, inputs, e.timings())
	v.Expected = e.plan.Expected
	v.Sessions, v.FinalsLast = hall.Sessions(e.plan, e.slugs()), e.plan.FinalsLast
	hall.ViewPlan(&v, e.plan, e.info, inputs)
	return v
}

// applyPlanSuggestion lays the mats out as the suggestion would.
func (e *Event) applyPlanSuggestion() {
	in, _, _ := e.times()
	placed, mats := e.placements()
	e.plan.Items = hall.ApplySuggestion(placed, forecast.Suggest(in, mats))
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

// planRoutes are the plan's times.
func (e *Event) planRoutes() []route {
	view := func(call) Response { return ok(e.forecastView()) }
	return []route{
		on("GET /api/forecast", view),
		on("PUT /api/plan/timings", func(c call) Response {
			var in store.Timings
			if err := json.Unmarshal(c.body, &in); err != nil {
				return fail(400, err)
			}
			in, err := hall.CleanTimings(in)
			if err != nil {
				return fail(400, err)
			}
			e.plan.Timings = in
			return changed(e.forecastView())
		}),
		on("POST /api/plan/timings/learn", func(call) Response {
			in, _, _ := e.times()
			rep := forecast.Measure(in)
			if rep.Samples == 0 {
				return fail(409, errors.New("nothing has been fenced yet to learn from"))
			}
			hall.LearnTimings(&e.plan.Timings, rep)
			return changed(e.forecastView())
		}),
		on("POST /api/plan/timings/default", func(call) Response {
			keep := e.timings()
			keep.Close = ""
			e.defaults = keep
			return changed(map[string]any{"file": "this browser", "timings": keep})
		}),
		on("PUT /api/plan/expected", e.changePlan(func(c call) error {
			var in map[string]int
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			return hall.SetExpected(&e.plan, in)
		})),
		on("GET /api/plan/report", func(call) Response { return ok(e.report()) }),
		on("PUT /api/plan/anomalies", func(c call) Response {
			var in struct {
				Key     string `json:"key"`
				Anomaly bool   `json:"anomaly"`
			}
			if err := json.Unmarshal(c.body, &in); err != nil || in.Key == "" {
				return fail(400, fmt.Errorf("say which match"))
			}
			e.plan.Anomalies = hall.SetAnomaly(e.plan.Anomalies, in.Key, in.Anomaly)
			return changed(e.report())
		}),
		on("PUT /api/plan/sessions", e.changePlan(func(c call) error {
			var in map[string]int
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			return hall.SetSessions(&e.plan, in, e.slugs())
		})),
		on("PUT /api/plan/finals", e.changePlan(func(c call) error {
			var in struct {
				Last bool `json:"last"`
			}
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			e.plan.FinalsLast = in.Last
			return nil
		})),
		on("PUT /api/plan/breaks", e.changePlan(func(c call) error {
			var in []store.Break
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			in, err := hall.CleanBreaks(in)
			if err != nil {
				return err
			}
			e.plan.Breaks = in
			return nil
		})),
		on("PUT /api/plan/prep", e.changePlan(func(c call) error {
			var in map[string]store.Prep
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			return hall.SetPrep(&e.plan, in, e.slugs())
		})),
		on("PUT /api/plan/starts", e.changePlan(func(c call) error {
			var in map[string]string
			if err := json.Unmarshal(c.body, &in); err != nil {
				return err
			}
			return hall.SetStarts(&e.plan, in, e.slugs())
		})),
		on("POST /api/plan/suggest", func(call) Response {
			in, _, _ := e.times()
			_, mats := e.placements()
			return ok(hall.ViewSuggestion(forecast.Suggest(in, mats)))
		}),
		on("POST /api/plan/apply", e.applySuggestion),
	}
}

// changePlan is a change to the plan that answers with the forecast, or a 400.
func (e *Event) changePlan(change func(call) error) func(call) Response {
	return func(c call) Response {
		if err := change(c); err != nil {
			return fail(400, err)
		}
		return changed(e.forecastView())
	}
}

func (e *Event) report() hall.ReportView {
	in, _, inputs := e.times()
	return hall.ViewReport(forecast.Measure(in), inputs)
}

func (e *Event) applySuggestion(c call) Response {
	var want struct {
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(c.body, &want); err != nil {
		return fail(400, err)
	}
	in, _, _ := e.times()
	placed, mats := e.placements()
	s := forecast.Suggest(in, mats)
	if hall.SignatureOf(s) != want.Signature {
		return conflict(map[string]any{
			"error":      "the plan has changed since that suggestion; here is a new one",
			"suggestion": hall.ViewSuggestion(s),
		})
	}
	e.plan.Items = hall.ApplySuggestion(placed, s)
	return changed(e.forecastView())
}
