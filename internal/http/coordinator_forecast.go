package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The plan's times (proposal §10, phase 4): the forecast for the board and the hall,
// the template it starts from, and the measured day that template learns from (#64).

// defaultsName is the organizer's template for new events, kept beside the event folder
// -- in the folder their events live in -- so the next event starts from what this one
// measured.
const defaultsName = "porta-timings.json"

func (c *Coordinator) now() time.Time {
	if c.Clock != nil {
		return c.Clock()
	}
	return time.Now()
}

// DefaultTimingsFile is where the template for new events is kept.
func (c *Coordinator) DefaultTimingsFile() string {
	return filepath.Join(filepath.Dir(c.folder.Dir()), defaultsName)
}

// defaults is the template for new events: the organizer's, over the built-in one.
func (c *Coordinator) defaults() store.Timings {
	var t store.Timings
	if b, err := os.ReadFile(c.DefaultTimingsFile()); err == nil {
		_ = json.Unmarshal(b, &t)
	}
	return TimingsOr(t, DefaultTimings)
}

// hallTimes is the forecast, from snapshots already taken and the mats built from them.
type hallTimes struct {
	inputs  []MatsInput
	in      forecast.Input
	result  forecast.Result
	timings store.Timings
}

func (c *Coordinator) timesFrom(snaps []snapped) hallTimes {
	file, _ := c.folder.Read()
	var inputs []MatsInput
	for _, s := range snaps {
		if s.err != nil {
			continue
		}
		inputs = append(inputs, MatsInput{Slug: s.w.slug, Name: s.snap.Instance.Name, Snapshot: s.snap,
			Expected: file.Plan.Expected[s.w.slug]})
	}
	c.withSessions(inputs, file.Plan)
	placed, _ := c.Placements()
	timings := TimingsOr(file.Plan.Timings, c.defaults())
	in := ForecastInput(inputs, placed, file.Plan, file.Event, timings, c.now())
	return hallTimes{inputs: inputs, in: in, result: forecast.Run(in), timings: timings}
}

// Hall is, for a discipline's snapshot, which match each of the event's mats is on when
// it is this discipline's, and when each of its matches still to come is expected.
func (c *Coordinator) Hall(slug string) (map[int]string, map[string]string) {
	view, result := c.hallNow()
	return CurrentFrom(view, slug), EtasFor(result, slug)
}

// hallCache is the hall worked out once per change (#124). A republish asks every
// discipline for its snapshot, and every snapshot asks for the hall: without it, one
// exchange cost every discipline's snapshot once per discipline, and a forecast each.
type hallCache struct {
	mu     sync.Mutex
	key    string
	at     time.Time
	view   MatsView
	result forecast.Result
}

// hallMaxAge is how long the hall is kept even when nothing that writes has changed: what
// it cannot see -- a hand-edited file, the clock moving a live match on -- shows within it.
const hallMaxAge = time.Second

// hallKey is everything the hall is worked out from that can change without a read: every
// write through the event's files and the disciplines' stores, and who holds which match.
func (c *Coordinator) hallKey() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d/%d", c.folder.Revision(), c.rev.Load(), c.now().Unix())
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			fmt.Fprintf(&b, "|%s:-", w.slug)
		} else {
			fmt.Fprintf(&b, "|%s:%d", w.slug, w.srv.store.Revision())
		}
	}
	for mat := 1; mat <= maxMats; mat++ {
		if d, m := c.held(mat); m != "" {
			fmt.Fprintf(&b, "|%d=%s/%s", mat, d, m)
		}
	}
	return b.String()
}

// hallNow is the hall's mats and forecast, from the cache while it is current.
func (c *Coordinator) hallNow() (MatsView, forecast.Result) {
	c.hall.mu.Lock()
	defer c.hall.mu.Unlock()
	key, now := c.hallKey(), time.Now()
	if key == c.hall.key && now.Sub(c.hall.at) < hallMaxAge && !c.hall.at.After(now) {
		return c.hall.view, c.hall.result
	}
	snaps := c.snapshots()
	c.hall.view, c.hall.result = c.matsFrom(snaps), c.timesFrom(snaps).result
	// Keyed on what it was before: a write while it was worked out may not be in it.
	c.hall.key, c.hall.at = key, now
	return c.hall.view, c.hall.result
}

// ForecastNow is the forecast as the board reads it.
func (c *Coordinator) ForecastNow() ForecastView {
	h := c.timesFrom(c.snapshots())
	v := ViewForecast(h.in, h.result, h.inputs, h.timings)
	file, _ := c.folder.Read()
	v.Expected = file.Plan.Expected
	v.Sessions, v.FinalsLast = Sessions(file.Plan, c.slugs()), file.Plan.FinalsLast
	return v
}

func (c *Coordinator) getForecast(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, c.ForecastNow())
}

// changePlan writes a change to the plan and tells every page.
func (c *Coordinator) changePlan(change func(*store.Plan) error) error {
	c.planMu.Lock()
	_, err := c.folder.Update(func(f *event.File) error { return change(&f.Plan) })
	c.planMu.Unlock()
	if err == nil {
		c.republish()
	}
	return err
}

// putTimings is the template, from the planning panel.
func (c *Coordinator) putTimings(w http.ResponseWriter, r *http.Request) {
	var in store.Timings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in, err := CleanTimings(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := c.changePlan(func(p *store.Plan) error { p.Timings = in; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c.ForecastNow())
}

// learnTimings writes what the day measured into the template: #64's templates updated
// from real data.
func (c *Coordinator) learnTimings(w http.ResponseWriter, r *http.Request) {
	h := c.timesFrom(c.snapshots())
	rep := forecast.Measure(h.in)
	if rep.Samples == 0 {
		writeErr(w, http.StatusConflict, errors.New("nothing has been fenced yet to learn from"))
		return
	}
	err := c.changePlan(func(p *store.Plan) error {
		p.Timings.Match = int(rep.Match / time.Second)
		if rep.Changeover > 0 {
			p.Timings.Changeover = int(rep.Changeover / time.Second)
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c.ForecastNow())
}

// keepTimings keeps the timings in force as the template for new events.
func (c *Coordinator) keepTimings(w http.ResponseWriter, r *http.Request) {
	h := c.timesFrom(c.snapshots())
	keep := h.timings
	keep.Close = ""
	if err := store.WriteJSONAtomic(filepath.Dir(c.DefaultTimingsFile()), defaultsName, keep); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": c.DefaultTimingsFile(), "timings": keep})
}

// putSessions is which block of the day each discipline runs in (#136).
func (c *Coordinator) putSessions(w http.ResponseWriter, r *http.Request) {
	var in map[string]int
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	known := map[string]bool{}
	for _, s := range c.slugs() {
		known[s] = true
	}
	err := c.changePlan(func(p *store.Plan) error {
		if p.Sessions == nil {
			p.Sessions = map[string]int{}
		}
		for slug, n := range in {
			if !known[slug] || n < 1 || n > 20 {
				return errors.New("a block is a discipline of this event and a number from 1 to 20")
			}
			p.Sessions[slug] = n
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, c.ForecastNow())
}

// putFinals holds every final to the end of the day, or lets them go (#136).
func (c *Coordinator) putFinals(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Last bool `json:"last"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := c.changePlan(func(p *store.Plan) error { p.FinalsLast = in.Last; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c.ForecastNow())
}

// putExpected is how many each discipline expects, for planning before the entries.
func (c *Coordinator) putExpected(w http.ResponseWriter, r *http.Request) {
	var in map[string]int
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	err := c.changePlan(func(p *store.Plan) error {
		if p.Expected == nil {
			p.Expected = map[string]int{}
		}
		for slug, n := range in {
			if n < 0 || n > 200 {
				return errors.New("expect between 0 and 200")
			}
			if n == 0 {
				delete(p.Expected, slug)
			} else {
				p.Expected[slug] = n
			}
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, c.ForecastNow())
}

func (c *Coordinator) getReport(w http.ResponseWriter, r *http.Request) {
	h := c.timesFrom(c.snapshots())
	writeJSON(w, http.StatusOK, ViewReport(forecast.Measure(h.in), h.inputs))
}

// putAnomaly marks a match's times as not to be learned from, or takes the mark off.
func (c *Coordinator) putAnomaly(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key     string `json:"key"`
		Anomaly bool   `json:"anomaly"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Key == "" {
		writeErr(w, http.StatusBadRequest, errors.New(`say which match: {"key": "open-sabre/p1m2", "anomaly": true}`))
		return
	}
	err := c.changePlan(func(p *store.Plan) error {
		p.Anomalies = SetAnomaly(p.Anomalies, in.Key, in.Anomaly)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.getReport(w, r)
}

// SetAnomaly marks or unmarks one match in the plan's list.
func SetAnomaly(list []string, key string, on bool) []string {
	out := []string{}
	for _, k := range list {
		if k != key {
			out = append(out, k)
		}
	}
	if on {
		out = append(out, key)
	}
	return out
}

// SetItemFlags pins or unpins a work item, or holds it to a time, in the placements.
func SetItemFlags(placed map[string]store.Placement, id string, pinned *bool, notBefore *string) (map[string]store.Placement, error) {
	p, ok := placed[id]
	if !ok {
		return placed, errors.New("no work item " + id)
	}
	if notBefore != nil {
		if _, ok := ParseClock(time.Now(), *notBefore); *notBefore != "" && !ok {
			return placed, errors.New("write the time as 16:30")
		}
		p.NotBefore = *notBefore
	}
	if pinned != nil {
		p.Pinned = *pinned
	}
	next := make(map[string]store.Placement, len(placed))
	for k, v := range placed {
		next[k] = v
	}
	next[id] = p
	return next, nil
}

// flagItem is the card menu's pin, unpin and hold-until.
func (c *Coordinator) flagItem(id string, pinned *bool, notBefore *string) error {
	c.planMu.Lock()
	placed, _ := c.placementsLocked()
	next, err := SetItemFlags(placed, id, pinned, notBefore)
	if err == nil {
		err = c.savePlacementsLocked(next)
	}
	c.planMu.Unlock()
	if err == nil {
		c.republish()
	}
	return err
}

// suggestion is a suggested plan for the hall as it stands.
func (c *Coordinator) suggestion() (forecast.Suggestion, map[string]store.Placement) {
	h := c.timesFrom(c.snapshots())
	placed, mats := c.Placements()
	return forecast.Suggest(h.in, mats), placed
}

// suggest is a suggested plan. It writes nothing, like the signup preview.
func (c *Coordinator) suggest(w http.ResponseWriter, r *http.Request) {
	s, _ := c.suggestion()
	writeJSON(w, http.StatusOK, ViewSuggestion(s))
}

// applySuggestion takes a suggestion: worked out again here rather than trusting what was
// posted, and refused when it is no longer the one the organizer looked at.
func (c *Coordinator) applySuggestion(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s, _ := c.suggestion()
	if SignatureOf(s) != in.Signature {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":      "the plan has changed since that suggestion; here is a new one",
			"suggestion": ViewSuggestion(s),
		})
		return
	}
	c.planMu.Lock()
	placed, _ := c.placementsLocked()
	err := c.savePlacementsLocked(ApplySuggestion(placed, s))
	c.planMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.republish()
	writeJSON(w, http.StatusOK, c.ForecastNow())
}
