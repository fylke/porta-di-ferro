package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The event's mats, its plan and the devices at the mats (proposal phase 2).
//
// The coordinator owns the plan -- every work item's mat and place in that mat's queue --
// and the registry of devices, because both are about the hall rather than any one
// discipline: a score keeper sits at a physical mat and follows it from a Longsword pool
// to a Sabre pool, and a claim on a match anywhere asks the one registry whether the
// device holding it is still alive.

// maxMats bounds the hall. Four mats per discipline is the draw's own limit; an event of
// several shares more.
const maxMats = 8

// snapped is one discipline's snapshot, or why there is none.
type snapped struct {
	w    *worker
	snap Snapshot
	err  error
}

// snapshots takes every discipline's snapshot once, for everything built from them.
func (c *Coordinator) snapshots() []snapped {
	var out []snapped
	for _, w := range c.snapshotWorkers() {
		s := snapped{w: w}
		if w.srv == nil {
			s.err = w.err
		} else {
			s.snap, s.err = w.srv.PlacedSnapshot()
		}
		out = append(out, s)
	}
	return out
}

// Placements is the plan: every work item's mat and place, and how many mats there are.
// New items -- a draw that just happened -- are placed and written down here, so the next
// call finds them where this one put them.
func (c *Coordinator) Placements() (map[string]store.Placement, int) {
	c.planMu.Lock()
	defer c.planMu.Unlock()
	return c.placementsLocked()
}

func (c *Coordinator) placementsLocked() (map[string]store.Placement, int) {
	file, _ := c.folder.Read()
	var items []WorkItem
	var tournaments []store.Tournament
	var failed, slugs []string
	stores := map[string]*store.Store{}
	for _, w := range c.snapshotWorkers() {
		slugs = append(slugs, w.slug)
		if w.srv == nil {
			failed = append(failed, w.slug+"/")
			continue
		}
		t, err := w.srv.store.Tournament()
		if err != nil {
			failed = append(failed, w.slug+"/")
			continue
		}
		comps, err := w.srv.store.Competitors()
		if err != nil {
			failed = append(failed, w.slug+"/")
			continue
		}
		tournaments = append(tournaments, t)
		stores[w.slug] = w.srv.store
		items = append(items, PlanItems(w.slug, t, Entrants(comps, file.Plan.Expected[w.slug]))...)
	}
	// Under way is any match of the item with a log: nothing goes in front of it.
	started := func(it WorkItem) bool {
		st := stores[it.Discipline]
		if st == nil {
			return false
		}
		for _, id := range it.Matches {
			if evs, err := st.Events(id, 0); err == nil && len(evs) > 0 {
				return true
			}
		}
		return false
	}
	mats := MatCount(file.Plan, tournaments)
	// A discipline that cannot be read keeps its placements for when it can be again,
	// rather than losing every move the organizer made to it: they are set aside from the
	// placing and put back as they were.
	active := store.Plan{Mats: file.Plan.Mats, Items: map[string]store.Placement{}}
	kept := map[string]store.Placement{}
	for id, p := range file.Plan.Items {
		unreadable := false
		for _, prefix := range failed {
			unreadable = unreadable || strings.HasPrefix(id, prefix)
		}
		if unreadable {
			kept[id] = p
		} else {
			active.Items[id] = p
		}
	}
	placed, changed := PlaceIn(items, active, mats, OrderOf(file.Plan, slugs, started))
	for id, p := range kept {
		placed[id] = p
	}
	if changed {
		_, _ = c.folder.Update(func(f *event.File) error {
			f.Plan.Items = placed
			return nil
		})
	}
	return placed, mats
}

// withSessions gives every discipline its block of the day (#136).
func (c *Coordinator) withSessions(inputs []MatsInput, plan store.Plan) {
	sessions := Sessions(plan, c.slugs())
	for i := range inputs {
		inputs[i].Session = sessions[inputs[i].Slug]
		inputs[i].FinalsLast = plan.FinalsLast
	}
}

// held is the match a live score keeper is holding on a mat.
func (c *Coordinator) held(mat int) (string, string) {
	if sk := c.presence.scorekeeperOn(mat); sk != nil {
		return sk.Discipline, sk.Match
	}
	return "", ""
}

// matsFrom lays every readable discipline's items on the mats.
func (c *Coordinator) matsFrom(snaps []snapped) MatsView {
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
	placed, mats := c.Placements()
	view := BuildMats(inputs, placed, mats, c.held)
	view.Upcoming = store.UpcomingOf(file.Screens)
	NameMats(&view, file.Plan)
	return view
}

// MatsNow is the hall's mats as they stand.
func (c *Coordinator) MatsNow() MatsView { return c.matsFrom(c.snapshots()) }

// MoveItem moves a work item on the event's mats: to a mat at an index (-1 for the end),
// or one step "up" or "down" its own mat. A change republishes every discipline, whose
// snapshots name the mats their pools are on.
func (c *Coordinator) MoveItem(id string, mat, index int, move string) error {
	view := c.MatsNow()
	c.planMu.Lock()
	placed, mats := c.placementsLocked()
	var (
		next    map[string]store.Placement
		changed bool
		err     error
	)
	switch move {
	case "up", "down":
		next, changed, err = Step(view, placed, id, move == "up", mats)
	default:
		if index < 0 {
			index = len(view.Items)
		}
		next, changed, err = Move(view, placed, id, mat, index, mats)
	}
	if err == nil && changed {
		_, err = c.folder.Update(func(f *event.File) error {
			f.Plan.Items = next
			return nil
		})
	}
	c.planMu.Unlock()
	if err != nil {
		return err
	}
	if changed {
		c.republish()
	}
	return nil
}

// republish tells every discipline's pages to redraw, and the event's.
func (c *Coordinator) republish() {
	for _, w := range c.snapshotWorkers() {
		if w.srv != nil {
			w.srv.PublishState()
		}
	}
	c.poke()
}

// SummaryMats is what each mat is running, for the discipline whose match it is.
func SummaryMats(view MatsView, slug string) []MatSummary {
	out := []MatSummary{}
	for _, m := range view.Mats {
		cur := m.Current
		if cur == nil || cur.Discipline != slug {
			continue
		}
		out = append(out, MatSummary{
			Mat: m.Mat, Match: cur.Match.ID, Status: cur.Match.Status, Pool: cur.Match.Pool,
			Round: cur.Match.Round, Red: cur.Red, Blue: cur.Blue,
			RedColour: cur.Match.Options.Red, BlueColour: cur.Match.Options.Blue,
			RedScore: cur.Match.State.Red.Score, BlueScore: cur.Match.State.Blue.Score,
		})
	}
	return out
}

// --- the mats and the plan, over HTTP ---------------------------------------------------

func (c *Coordinator) getMats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, c.MatsNow())
}

// putMats is how many mats the hall has. A mat cannot be taken away while it is running
// something; what was waiting on it moves to the end of the mat its lane maps to.
func (c *Coordinator) putMats(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if in.Count < 1 || in.Count > maxMats {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("a hall has between 1 and %d mats", maxMats))
		return
	}
	for _, it := range c.MatsNow().Items {
		if it.Mat > in.Count && it.Status == "running" {
			writeErr(w, http.StatusConflict, fmt.Errorf("mat %d is running something; it can go once that is done", it.Mat))
			return
		}
	}
	c.planMu.Lock()
	_, err := c.folder.Update(func(f *event.File) error {
		f.Plan.Mats = in.Count
		return nil
	})
	c.planMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.republish()
	writeJSON(w, http.StatusOK, c.MatsNow())
}

// patchItem moves a work item: {"mat": 2, "index": 0} to a place on a mat, or
// {"move": "up"} one step along its own.
func (c *Coordinator) patchItem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mat       int     `json:"mat"`
		Index     *int    `json:"index"`
		Move      string  `json:"move"`
		Pinned    *bool   `json:"pinned"`
		NotBefore *string `json:"notBefore"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// The card menu's pin and hold-until (phase 4), on their own or with a move.
	if in.Pinned != nil || in.NotBefore != nil {
		if err := c.flagItem(r.PathValue("id"), in.Pinned, in.NotBefore); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if in.Move == "" && in.Mat <= 0 {
			writeJSON(w, http.StatusOK, c.MatsNow())
			return
		}
	}
	index := -1
	if in.Index != nil {
		index = *in.Index
	}
	if in.Move == "" && in.Mat <= 0 {
		writeErr(w, http.StatusBadRequest, errors.New(`say where: {"mat": n, "index": i} or {"move": "up"|"down"}`))
		return
	}
	err := c.MoveItem(r.PathValue("id"), in.Mat, index, in.Move)
	var locked ErrNotMovable
	switch {
	case errors.As(err, &locked):
		writeErr(w, http.StatusConflict, err)
	case err != nil:
		writeErr(w, http.StatusBadRequest, err)
	default:
		writeJSON(w, http.StatusOK, c.MatsNow())
	}
}

// putMat is one mat's name and the times it is not available (#123).
func (c *Coordinator) putMat(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("mat"))
	_, mats := c.Placements()
	if err != nil || n < 1 || n > mats {
		writeErr(w, http.StatusNotFound, fmt.Errorf("there is no mat %s", r.PathValue("mat")))
		return
	}
	var in store.MatSetting
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in, err = CleanMat(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := c.changePlan(func(p *store.Plan) error { SetMat(p, n, in); return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c.MatsNow())
}

// putScreens is how every mat screen looks (#110).
func (c *Coordinator) putScreens(w http.ResponseWriter, r *http.Request) {
	var in store.Screens
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if store.UpcomingOf(in) != in.Upcoming {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("upcoming is one of %v", store.UpcomingChoices))
		return
	}
	if _, err := c.folder.Update(func(f *event.File) error {
		f.Screens = in
		return nil
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.poke()
	writeJSON(w, http.StatusOK, c.MatsNow())
}

// --- the devices at the mats ------------------------------------------------------------

// QuarantinedIn is an event set aside, with the discipline its match is in.
type QuarantinedIn struct {
	store.Quarantined
	Discipline string `json:"discipline"`
}

// EventPresence is every device in the hall, and everything set aside in any discipline.
type EventPresence struct {
	Clients     []Client        `json:"clients"`
	Quarantined []QuarantinedIn `json:"quarantined"`
}

func (c *Coordinator) presenceView() EventPresence {
	out := EventPresence{Clients: c.presence.list(), Quarantined: []QuarantinedIn{}}
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			continue
		}
		q, err := w.srv.store.Quarantined()
		if err != nil {
			continue
		}
		for _, e := range q {
			out.Quarantined = append(out.Quarantined, QuarantinedIn{Quarantined: e, Discipline: w.slug})
		}
	}
	return out
}

// sweep notices devices that have gone quiet. A score keeper going quiet changes what its
// mat shows, so the mats are republished too.
func (c *Coordinator) sweep() {
	defer recoverLogged("sweeping the devices")
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			if c.presence.sweep() {
				c.poke()
			}
		}
	}
}

// register is a device's heartbeat, at the event: which mat it is at and, for a score
// keeper, which discipline's match it is holding. A screen gets back what it should show.
func (c *Coordinator) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role       string `json:"role"`
		Name       string `json:"name"`
		Mat        int    `json:"mat"`
		Match      string `json:"match"`
		Discipline string `json:"discipline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if in.Role != "scorekeeper" && in.Role != "display" {
		writeErr(w, http.StatusBadRequest, errors.New(`role must be "scorekeeper" or "display"`))
		return
	}
	id := r.PathValue("id")
	cl := Client{ID: id, Role: in.Role, Name: in.Name, Mat: in.Mat, Match: in.Match, Discipline: in.Discipline}
	if in.Role == "display" {
		if displays, err := c.folder.Displays(); err == nil {
			cl.Target = displays[id]
		}
	}
	isNew := c.presence.get(id) == nil
	if c.presence.touch(cl) || isNew {
		c.poke()
	}
	writeJSON(w, http.StatusOK, c.presence.get(id))
}

// release is a device leaving on purpose. A score keeper's release lets go of whatever
// match it holds, in any discipline.
func (c *Coordinator) release(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cl := c.presence.remove(id)
	if cl != nil && cl.Role == "scorekeeper" {
		for _, wk := range c.snapshotWorkers() {
			if wk.srv != nil {
				wk.srv.ReleaseClaims(id)
			}
		}
	}
	c.republish()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// assignDisplay is the organizer telling a screen what to show: "mat/1", "mats",
// "mats/1,2", or a discipline's page such as "d/open-sabre/roster". Kept in the event's
// displays.json, so a hall of screens survives the organizer's laptop rebooting.
func (c *Coordinator) assignDisplay(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	id := r.PathValue("id")
	displays, err := c.folder.Displays()
	if err == nil {
		if in.Target == "" {
			delete(displays, id)
		} else {
			displays[id] = in.Target
		}
		err = c.folder.SaveDisplays(displays)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.presence.setTarget(id, in.Target)
	c.poke()
	writeJSON(w, http.StatusOK, map[string]string{"target": in.Target})
}

// liftDisplays brings a discipline's screen assignments up to the event the first time:
// before mats were the event's, screens were assigned per discipline, and a migrated
// event's first discipline is where they are.
func (c *Coordinator) liftDisplays() {
	if c.folder.HasDisplays() || len(c.order) == 0 {
		return
	}
	first := c.workers[c.order[0]]
	if first == nil || first.srv == nil {
		return
	}
	if d, err := first.srv.store.Displays(); err == nil && len(d) > 0 {
		_ = c.folder.SaveDisplays(d)
	}
}
