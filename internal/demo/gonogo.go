package demo

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fylke/porta-di-ferro/internal/hall"
)

// Go / no-go (#167), as the coordinator answers it: hall.GoNoGo over the demo's views.

func (e *Event) goNoGoInput(slug string) (hall.GoNoGoInput, bool) {
	d := e.find(slug)
	if d == nil {
		return hall.GoNoGoInput{}, false
	}
	_, mats := e.placements()
	sv, _ := e.staffing()
	name := d.tournament.Discipline
	if name == "" {
		name = d.slug
	}
	return hall.GoNoGoInput{Slug: d.slug, Name: name, Tournament: d.tournament, HallMats: mats, Plan: e.plan,
		Slugs: e.slugs(), Forecast: e.forecastView(), Staffing: sv}, true
}

func (e *Event) goNoGoAll() []hall.GoNoGoView {
	out := []hall.GoNoGoView{}
	for _, d := range e.disciplines {
		if in, ok := e.goNoGoInput(d.slug); ok {
			out = append(out, hall.GoNoGo(in))
		}
	}
	return out
}

func (e *Event) goNoGo(slug string) Response {
	in, found := e.goNoGoInput(slug)
	if !found {
		return fail(404, fmt.Errorf("this event has no discipline %q", slug))
	}
	return ok(hall.GoNoGo(in))
}

func (e *Event) signOff(slug string, body []byte) Response {
	var req struct {
		By string `json:"by"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return fail(400, err)
	}
	in, found := e.goNoGoInput(slug)
	if !found {
		return fail(404, fmt.Errorf("this event has no discipline %q", slug))
	}
	if err := hall.SignOff(&e.plan, in, req.By, time.Now()); err != nil {
		return fail(400, err)
	}
	in.Plan = e.plan
	return changed(hall.GoNoGo(in))
}
