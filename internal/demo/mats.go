package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The demo's mats (phase 2): the same plan, queues and moves the coordinator runs --
// hall.Place, BuildMats, Move -- over the demo's in-memory disciplines, so the mat board,
// the score keeper and the screens in the demo cannot disagree with a real event.

// keeper is a score keeper registered in the demo's one tab: which mat, which match, and
// when it was last heard, counted in heartbeats.
type keeper struct {
	Mat        int
	Match      string
	Discipline string
	seen       int
}

// place is where a keeper is, whenever it was heard.
func (k keeper) place() keeper {
	k.seen = 0
	return k
}

// placements is the plan, with any item just drawn placed and kept.
func (e *Event) placements() (map[string]store.Placement, int) {
	var items []hall.WorkItem
	var tournaments []store.Tournament
	for _, d := range e.disciplines {
		tournaments = append(tournaments, d.tournament)
		items = append(items, hall.PlanItems(d.slug, d.tournament, hall.Entrants(d.competitors, e.plan.Expected[d.slug]))...)
	}
	mats := hall.MatCount(e.plan, tournaments)
	started := func(it hall.WorkItem) bool {
		d := e.find(it.Discipline)
		if d == nil {
			return false
		}
		for _, id := range it.Matches {
			if len(d.logs[id]) > 0 {
				return true
			}
		}
		return false
	}
	placed, changed := hall.Place(items, e.plan, mats, hall.OrderOf(e.plan, e.slugs(), started))
	if changed {
		e.plan.Items = placed
	}
	return placed, mats
}

// held is the match the score keeper on a mat is holding: the one heard from last, as the
// server's registry has it, when there are two.
func (e *Event) held(mat int) (string, string) {
	var last keeper
	found := false
	for _, k := range e.keepers {
		if k.Mat == mat && (!found || k.seen > last.seen) {
			last, found = k, true
		}
	}
	return last.Discipline, last.Match
}

// mats is the hall, as GET /api/mats answers it.
func (e *Event) mats() hall.MatsView {
	placed, n := e.placements()
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
	view := hall.BuildMats(inputs, placed, n, e.held)
	view.Upcoming = store.UpcomingOf(e.screens)
	view.Event = e.id
	hall.NameMats(&view, e.plan)
	return view
}

func (e *Event) putMat(mat string, body []byte) Response {
	n, err := strconv.Atoi(mat)
	_, mats := e.placements()
	if err != nil || n < 1 || n > mats {
		return fail(404, fmt.Errorf("there is no mat %s", mat))
	}
	var in store.MatSetting
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	if in, err = hall.CleanMat(in); err != nil {
		return fail(400, err)
	}
	hall.SetMat(&e.plan, n, in)
	return changed(e.mats())
}

func (e *Event) putScreens(body []byte) Response {
	var in store.Screens
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	if store.UpcomingOf(in) != in.Upcoming {
		return fail(400, fmt.Errorf("upcoming is one of %v", store.UpcomingChoices))
	}
	e.screens = in
	return changed(e.mats())
}

// withSessions gives every discipline its block of the day (#136).
func (e *Event) withSessions(inputs []hall.MatsInput) {
	sessions := hall.Sessions(e.plan, e.slugs())
	for i := range inputs {
		inputs[i].Session = sessions[inputs[i].Slug]
		inputs[i].FinalsLast = e.plan.FinalsLast
	}
}

// moveItem is the mat board's move, and a discipline's pool controls in the event.
func (e *Event) moveItem(id string, mat, index int, move string) error {
	view := e.mats()
	placed, n := e.placements()
	var (
		next    map[string]store.Placement
		changed bool
		err     error
	)
	switch move {
	case "up", "down":
		next, changed, err = hall.Step(view, placed, id, move == "up", n)
	default:
		if index < 0 {
			index = len(view.Items)
		}
		next, changed, err = hall.Move(view, placed, id, mat, index, n)
	}
	if err == nil && changed {
		e.plan.Items = next
	}
	return err
}

func (e *Event) patchItem(id string, body []byte) Response {
	var in struct {
		Mat   int    `json:"mat"`
		Index *int   `json:"index"`
		Move  string `json:"move"`
		hall.ItemFlags
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	if in.Any() {
		placed, _ := e.placements()
		next, err := hall.SetItemFlags(placed, id, in.ItemFlags)
		if err != nil {
			return fail(400, err)
		}
		e.plan.Items = next
		if in.Move == "" && in.Mat <= 0 {
			return changed(e.mats())
		}
	}
	index := -1
	if in.Index != nil {
		index = *in.Index
	}
	err := e.moveItem(id, in.Mat, index, in.Move)
	var locked hall.ErrNotMovable
	switch {
	case errors.As(err, &locked):
		return fail(409, err)
	case err != nil:
		return fail(400, err)
	}
	res := ok(e.mats())
	res.Changed = true
	return res
}

func (e *Event) putMats(body []byte) Response {
	var in struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	if in.Count < 1 || in.Count > 8 {
		return fail(400, fmt.Errorf("a hall has between 1 and 8 mats"))
	}
	for _, it := range e.mats().Items {
		if it.Mat > in.Count && it.Status == "running" {
			return fail(409, fmt.Errorf("mat %d is running something; it can go once that is done", it.Mat))
		}
	}
	e.plan.Mats = in.Count
	res := ok(e.mats())
	res.Changed = true
	return res
}

// register is a device's heartbeat. The demo keeps only what the mats need from it: the
// match a score keeper is holding, so a finished result stays on the mat until Next match.
func (e *Event) register(id string, body []byte) Response {
	var in struct {
		Role       string `json:"role"`
		Name       string `json:"name"`
		Mat        int    `json:"mat"`
		Match      string `json:"match"`
		Discipline string `json:"discipline"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	if in.Role != "scorekeeper" {
		return ok(map[string]any{"id": id, "role": in.Role, "name": in.Name, "alive": true})
	}
	if e.keepers == nil {
		e.keepers = map[string]keeper{}
	}
	prev, had := e.keepers[id]
	e.beats++
	now := keeper{Mat: in.Mat, Match: in.Match, Discipline: in.Discipline, seen: e.beats}
	e.keepers[id] = now
	res := ok(map[string]any{"id": id, "role": in.Role, "name": in.Name, "mat": in.Mat, "match": in.Match, "alive": true})
	res.Changed = !had || prev.place() != now.place()
	return res
}

func (e *Event) releaseClient(id string) Response {
	delete(e.keepers, id)
	return changed(map[string]bool{"ok": true})
}

// itemPath is the work item named in a plan path: "/api/plan/items/open-sabre/pool-3".
func itemPath(path string) (string, bool) {
	return strings.CutPrefix(path, "/api/plan/items/")
}
