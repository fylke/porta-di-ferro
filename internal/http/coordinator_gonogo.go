package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	"github.com/fylke/porta-di-ferro/internal/hall"
)

// Go / no-go (#167): each discipline's summary of what could go wrong, and the tournament
// manager's sign-off. The summary is hall.GoNoGo's, from the views the planning and
// staffing panels already show.

// goNoGoInput is what one discipline's summary is made from, or false when there is no
// such discipline, or its files cannot be read.
func (c *Coordinator) goNoGoInput(slug string, fv hall.ForecastView, sv hall.StaffingView) (hall.GoNoGoInput, bool) {
	c.mu.Lock()
	wk := c.workers[slug]
	c.mu.Unlock()
	if wk == nil || wk.srv == nil {
		return hall.GoNoGoInput{}, false
	}
	t, err := wk.srv.store.Tournament()
	if err != nil {
		return hall.GoNoGoInput{}, false
	}
	name := slug
	for _, d := range c.View().Disciplines {
		if d.Slug == slug && d.Name != "" {
			name = d.Name
		}
	}
	file, _ := c.folder.Read()
	_, mats := c.Placements()
	return hall.GoNoGoInput{Slug: slug, Name: name, Tournament: t, HallMats: mats, Plan: file.Plan,
		Slugs: c.slugs(), Forecast: fv, Staffing: sv}, true
}

// goNoGoAll is every discipline's summary, in the event's order.
func (c *Coordinator) goNoGoAll() ([]hall.GoNoGoView, error) {
	sv, _, _, err := c.StaffingNow()
	if err != nil {
		return nil, err
	}
	fv := c.ForecastNow()
	out := []hall.GoNoGoView{}
	for _, slug := range c.slugs() {
		if in, ok := c.goNoGoInput(slug, fv, sv); ok {
			out = append(out, hall.GoNoGo(in))
		}
	}
	return out, nil
}

func (c *Coordinator) getGoNoGo(w http.ResponseWriter, r *http.Request) {
	all, err := c.goNoGoAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if slug := r.PathValue("d"); slug != "" {
		for _, v := range all {
			if v.Discipline == slug {
				writeJSON(w, http.StatusOK, v)
				return
			}
		}
		writeErr(w, http.StatusNotFound, fmt.Errorf("this event has no discipline %q", slug))
		return
	}
	writeJSON(w, http.StatusOK, all)
}

// signOff is the manager's go for a discipline as it stands: {"by": "Name"}.
func (c *Coordinator) signOff(w http.ResponseWriter, r *http.Request) {
	var in struct {
		By string `json:"by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	sv, _, _, err := c.StaffingNow()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	gi, ok := c.goNoGoInput(r.PathValue("d"), c.ForecastNow(), sv)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("this event has no discipline %q", r.PathValue("d")))
		return
	}
	c.planMu.Lock()
	_, err = c.folder.Update(func(f *event.File) error { return hall.SignOff(&f.Plan, gi, in.By, time.Now()) })
	c.planMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	c.republish()
	gi.Plan.Signoffs = nil
	if file, err := c.folder.Read(); err == nil {
		gi.Plan = file.Plan
	}
	writeJSON(w, http.StatusOK, hall.GoNoGo(gi))
}
