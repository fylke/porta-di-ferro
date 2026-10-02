package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The demo's mats (phase 2): the same plan, queues and moves the coordinator runs --
// httpapi.Place, BuildMats, Move -- over the demo's in-memory disciplines, so the mat board,
// the score keeper and the screens in the demo cannot disagree with a real event.

// keeper is a score keeper registered in the demo's one tab: which mat, which match.
type keeper struct {
	Mat        int
	Match      string
	Discipline string
}

// placements is the plan, with any item just drawn placed and kept.
func (e *Event) placements() (map[string]store.Placement, int) {
	var items []httpapi.WorkItem
	var tournaments []store.Tournament
	for _, d := range e.disciplines {
		tournaments = append(tournaments, d.tournament)
		items = append(items, httpapi.ItemsOf(d.slug, d.tournament)...)
	}
	mats := httpapi.MatCount(e.plan, tournaments)
	placed, changed := httpapi.Place(items, e.plan, mats)
	if changed {
		e.plan.Items = placed
	}
	return placed, mats
}

func (e *Event) held(mat int) (string, string) {
	for _, k := range e.keepers {
		if k.Mat == mat && k.Match != "" {
			return k.Discipline, k.Match
		}
	}
	return "", ""
}

// mats is the hall, as GET /api/mats answers it.
func (e *Event) mats() httpapi.MatsView {
	placed, n := e.placements()
	var inputs []httpapi.MatsInput
	for _, d := range e.disciplines {
		snap, err := d.placedSnapshot()
		if err != nil {
			continue
		}
		inputs = append(inputs, httpapi.MatsInput{Slug: d.slug, Name: d.tournament.Discipline, Snapshot: snap})
	}
	return httpapi.BuildMats(inputs, placed, n, e.held)
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
		next, changed, err = httpapi.Step(view, placed, id, move == "up", n)
	default:
		if index < 0 {
			index = len(view.Items)
		}
		next, changed, err = httpapi.Move(view, placed, id, mat, index, n)
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
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	index := -1
	if in.Index != nil {
		index = *in.Index
	}
	err := e.moveItem(id, in.Mat, index, in.Move)
	var locked httpapi.ErrNotMovable
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
	now := keeper{Mat: in.Mat, Match: in.Match, Discipline: in.Discipline}
	e.keepers[id] = now
	res := ok(map[string]any{"id": id, "role": in.Role, "name": in.Name, "mat": in.Mat, "match": in.Match, "alive": true})
	res.Changed = !had || prev != now
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
