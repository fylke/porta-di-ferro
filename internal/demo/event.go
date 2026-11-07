package demo

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Event is the demo's event: the day around the fencing, and the disciplines in it
// (docs/proposals/one-event-many-disciplines.md, phase 1).
//
// It is to Demo what httpapi.Coordinator is to httpapi.Server: the event's own endpoints
// are answered here, and /api/d/{slug}/… is handed to the discipline it names with the
// prefix taken off. While there is one discipline the unprefixed paths answer as it, and
// with several they say so, exactly as the server does -- the client cannot tell the two
// apart, which is the rule the demo is built to (docs/demo.md).
type Event struct {
	info        store.Event
	disciplines []*Demo
	// plan is where every discipline's work runs on the event's mats (phase 2).
	plan store.Plan
	// keepers are the score keepers registered in this tab, for the match each holds, and
	// beats counts their heartbeats, for which was heard from last.
	keepers map[string]keeper
	beats   int
	// people is the event's registry of people (phase 3): people.json, in memory.
	people []store.Person
	// defaults is the template the visitor kept for new events (phase 4): the file beside
	// the event folder, in memory.
	defaults store.Timings
	// staff is the event's staff and who works where (phase 5): staff.json, in memory.
	staff store.Staff
	// screens is how the mat screens look (#110).
	screens store.Screens
	// id is the event's identity (event.File.ID): the same for the event a visitor
	// arrives in, a new one for every event that replaces it (#141).
	id string
	// moved are identifiers disciplines used to have (event.File.Aliases). Not saved: only
	// a page open in this tab can have learned one.
	moved map[string]string
}

// NewEvent builds the event a visitor arrives in: the longsword halfway through its pools
// and the sabre drawn for the afternoon.
func NewEvent() *Event {
	e := &Event{}
	e.Reset()
	return e
}

// Reset puts the event back to how a visitor first found it, disciplines added in the
// demo included.
func (e *Event) Reset() {
	longsword := newDiscipline("open-steel-longsword", fixture)
	sabre := newDiscipline("open-sabre", sabreFixture)
	buckler := newDiscipline("sword-and-buckler", bucklerFixture)
	// The day was the longsword's own before events existed; it is the event's now, as a
	// migrated folder's is.
	e.info = longsword.tournament.Event
	longsword.tournament.Event = store.Event{}
	e.disciplines = nil
	e.plan = store.Plan{}
	e.keepers = nil
	e.people = nil
	e.defaults = store.Timings{}
	e.id = "demo"
	e.adopt(longsword)
	e.adopt(sabre)
	e.adopt(buckler)
	// The day in blocks, as the issue that asked for them drew it (#136): the longsword;
	// then the sabre and the sword and buckler side by side; then every final on mat 1.
	e.plan.Sessions = map[string]int{"open-steel-longsword": 1, "open-sabre": 2, "sword-and-buckler": 2}
	e.plan.FinalsLast = true
	// Astrid and Greta signed up for both on one response each, so they are one person in
	// both; Bo was typed in at each desk, so he is two until the organizer says otherwise.
	e.ensurePeople()
	e.startAtFirstMatch()
	// The mats as an organizer would have laid them out: the suggestion, applied.
	e.applyPlanSuggestion()
	// The volunteers, already put to work: the suggestion applied, as an organizer would
	// have done before the doors opened.
	e.seedStaff()
	_, s := e.staffing()
	e.staff.Assignments = s.Assignments
}

// Delete replaces the event with a new one, as the server's DELETE /api/event does: no
// disciplines and nothing else (#149). The template for new events is the visitor's, not
// the event's, and stays.
func (e *Event) Delete() {
	e.info = store.Event{}
	e.disciplines = nil
	e.plan = store.Plan{}
	e.keepers = nil
	e.people = nil
	e.staff = store.Staff{}
	e.screens = store.Screens{}
	e.id = fmt.Sprintf("ev-%x", time.Now().UnixNano())
}

func (e *Event) adopt(d *Demo) {
	d.event = e
	e.disciplines = append(e.disciplines, d)
}

func (e *Event) find(slug string) *Demo {
	if to, ok := e.moved[slug]; ok {
		slug = to
	}
	for _, d := range e.disciplines {
		if d.slug == slug {
			return d
		}
	}
	return nil
}

// View is the event as its pages see it, built the way the coordinator builds it.
func (e *Event) View() hall.EventView {
	view := hall.EventView{Info: e.info, ID: e.id, Dir: "in this browser", Disciplines: []hall.DisciplineSummary{}}
	mats := e.mats()
	for _, d := range e.disciplines {
		snap, err := d.snapshot()
		if err != nil {
			view.Disciplines = append(view.Disciplines, hall.DisciplineSummary{
				Slug: d.slug, Name: d.tournament.Discipline, Error: err.Error(), Stage: "setup",
				Mats: []hall.MatSummary{}, Entrants: []hall.Entrant{},
			})
			continue
		}
		s := hall.Summarize(d.slug, snap)
		s.Mats = hall.SummaryMats(mats, d.slug)
		view.Disciplines = append(view.Disciplines, s)
	}
	in, r, inputs := e.times()
	view.Programme = hall.ViewForecast(in, r, inputs, e.timings()).Programme
	view.Staff = hall.StaffEntrants(e.staff.Members)
	view.Name = strings.TrimSpace(e.info.Signup.Name)
	if view.Name == "" && len(view.Disciplines) == 1 {
		view.Name = view.Disciplines[0].Name
	}
	return view
}

// Request answers one API call for the whole event.
func (e *Event) Request(method, path string, body []byte) Response {
	if res, found := dispatch(e.routes(), newCall(method, path, body)); found {
		return res
	}
	// An unprefixed path: the one discipline's while there is one, and nobody's after.
	if len(e.disciplines) != 1 {
		return conflict(map[string]any{
			"error":       "this event runs several disciplines: ask for one by its address, /api/d/{discipline}/…",
			"disciplines": e.slugsOrEmpty(),
		})
	}
	return e.disciplines[0].Request(method, path, body)
}

// routes are the event's own paths, as httpapi.Coordinator.Handler registers them, and a
// discipline's by its address.
func (e *Event) routes() []route {
	answer := func(f func() Response) func(call) Response { return func(call) Response { return f() } }
	bodied := func(f func([]byte) Response) func(call) Response { return func(c call) Response { return f(c.body) } }
	return slices.Concat(e.peopleRoutes(), e.planRoutes(), e.staffRoutes(), []route{
		on("GET /api/event", func(call) Response { return ok(e.View()) }),
		on("PUT /api/event/info", bodied(e.putInfo)),
		on("DELETE /api/event", func(call) Response {
			e.Delete()
			return changed(map[string]string{"deleted": "in this browser"})
		}),
		on("GET /api/info.pdf", func(c call) Response { return e.infoPDF(c.query) }),
		on("GET /api/mats", func(call) Response { return ok(e.mats()) }),
		on("GET /api/plan", func(call) Response { return ok(e.mats()) }),
		on("PUT /api/mats", bodied(e.putMats)),
		on("PUT /api/screens", bodied(e.putScreens)),
		on("PUT /api/mats/{mat}", func(c call) Response { return e.putMat(c.param("mat"), c.body) }),
		on("PATCH /api/plan/items/{id...}", func(c call) Response { return e.patchItem(c.param("id"), c.body) }),
		on("GET /api/presence", answer(nobodyPresent)),
		on("POST /api/clients/{id}", func(c call) Response { return e.register(c.param("id"), c.body) }),
		on("POST /api/clients/{id}/release", func(c call) Response { return e.releaseClient(c.param("id")) }),
		on("PUT /api/clients/{id}/target", func(call) Response { return ok(map[string]string{"target": ""}) }),
		// There is no LAN in a browser tab, which is the truth the admin pages show.
		on("GET /api/addresses", answer(noAddresses)),
		on("GET /api/qr.png", func(c call) Response { return qrImage(c.query) }),

		on("GET /api/disciplines", func(call) Response { return ok(e.View().Disciplines) }),
		on("GET /api/disciplines/presets", func(call) Response { return ok(hall.Disciplines) }),
		on("POST /api/disciplines", bodied(e.add)),
		on("PATCH /api/disciplines/{d}", func(c call) Response { return e.rename(c.param("d"), c.body) }),
		on("DELETE /api/disciplines/{d}", func(c call) Response { return e.retire(c.param("d")) }),
		on("PUT /api/disciplines/{d}/identifier", func(c call) Response { return e.setIdentifier(c.param("d"), c.body) }),
		on("POST /api/disciplines/{d}/reload", func(c call) Response { return e.reload(c.param("d")) }),

		// Demo-only, and the adapter is the only caller.
		on("POST /api/demo/reset", func(call) Response {
			e.Reset()
			return changed(map[string]bool{"ok": true})
		}),
		on("POST /api/demo/play", answer(e.playOut)),
		on("GET /api/demo/save", answer(func() Response { return saveResponse(e.Save()) })),
		on("POST /api/demo/load", func(c call) Response { return loadResponse(e.Load(c.body)) }),

		on("/api/d/{d}", e.inDiscipline),
		on("/api/d/{d}/{rest...}", e.inDiscipline),
	})
}

// inDiscipline hands a call to the discipline it names, with the prefix taken off, as the
// coordinator does.
func (e *Event) inDiscipline(c call) Response {
	d := e.find(c.param("d"))
	if d == nil {
		return fail(404, fmt.Errorf("this event has no discipline %q", c.param("d")))
	}
	rest := strings.TrimPrefix(c.path, "/api/d/"+c.param("d"))
	return d.Request(c.method, "/api"+rest, c.body)
}

// reload reads nothing again: nothing in the demo is read from a file.
func (e *Event) reload(slug string) Response {
	if d := e.find(slug); d != nil {
		return e.summary(d)
	}
	return fail(404, fmt.Errorf("this event has no discipline %q", slug))
}

// playOut finishes every discipline, so the bracket and the podium can be reached in each
// without scoring them by hand.
func (e *Event) playOut() Response {
	for _, d := range e.disciplines {
		if res := d.playOut(); res.Status != 200 {
			return res
		}
	}
	return changed(map[string]bool{"ok": true})
}

func (e *Event) slugsOrEmpty() []string {
	slugs := make([]string, 0, len(e.disciplines))
	for _, d := range e.disciplines {
		slugs = append(slugs, d.slug)
	}
	return slugs
}

func (e *Event) summary(d *Demo) Response {
	snap, err := d.snapshot()
	if err != nil {
		return fail(500, err)
	}
	return ok(hall.Summarize(d.slug, snap))
}

func (e *Event) putInfo(body []byte) Response {
	var in store.Event
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	ev, err := hall.CleanEvent(in)
	if err != nil {
		return fail(400, err)
	}
	e.info = ev
	return changed(ev)
}

func (e *Event) nameTaken(name string, except *Demo) error {
	for _, d := range e.disciplines {
		if d != except && strings.EqualFold(d.tournament.Discipline, name) {
			return fmt.Errorf("%s is already in this event", d.tournament.Discipline)
		}
	}
	return nil
}

// add is a discipline the visitor makes: in memory, like everything else here, and empty.
func (e *Event) add(body []byte) Response {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	name, err := hall.CleanName(in.Name)
	if err == nil {
		err = e.nameTaken(name, nil)
	}
	if err != nil {
		return fail(400, err)
	}
	base := event.Slugify(name)
	if base == "" {
		base = "discipline"
	}
	slug := base
	for n := 2; e.find(slug) != nil; n++ {
		slug = fmt.Sprintf("%s-%d", base, n)
	}
	d := newDiscipline(slug, emptyFixture(name))
	e.adopt(d)
	res := e.summary(d)
	res.Status, res.Changed = 201, true
	return res
}

func (e *Event) rename(slug string, body []byte) Response {
	d := e.find(slug)
	if d == nil {
		return fail(404, fmt.Errorf("this event has no discipline %q", slug))
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	name, err := hall.CleanName(in.Name)
	if err == nil {
		err = e.nameTaken(name, d)
	}
	if err != nil {
		return fail(400, err)
	}
	d.tournament.Discipline = name
	res := e.summary(d)
	res.Changed = true
	return res
}

// setIdentifier is httpapi.Coordinator's: a discipline's address and how the signup files
// name it, the old one still leading there (#153).
func (e *Event) setIdentifier(slug string, body []byte) Response {
	d := e.find(slug)
	if d == nil {
		return fail(404, fmt.Errorf("this event has no discipline %q", slug))
	}
	var in struct {
		Identifier string `json:"identifier"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	want, err := event.Identifier(in.Identifier)
	if err != nil {
		return fail(400, err)
	}
	if want == d.slug {
		return e.summary(d)
	}
	if other := e.find(want); other != nil && other != d {
		return fail(400, event.ErrSlugTaken)
	}
	old := d.slug
	d.slug = want
	if e.moved == nil {
		e.moved = map[string]string{}
	}
	for alias, to := range e.moved {
		if to == old {
			e.moved[alias] = want
		}
	}
	e.moved[old] = want
	delete(e.moved, want)
	e.plan.MoveDiscipline(old, want)
	e.staff.MoveDiscipline(old, want)
	for i := range e.people {
		for j, k := range e.people[i].Moved {
			e.people[i].Moved[j] = store.MoveKey(k, old, want)
		}
	}
	for id, k := range e.keepers {
		if k.Discipline == old {
			k.Discipline = want
			e.keepers[id] = k
		}
	}
	res := e.summary(d)
	res.Changed = true
	return res
}

func (e *Event) retire(slug string) Response {
	if e.find(slug) == nil {
		return fail(404, fmt.Errorf("this event has no discipline %q", slug))
	}
	kept := e.disciplines[:0]
	for _, d := range e.disciplines {
		if d.slug != slug {
			kept = append(kept, d)
		}
	}
	e.disciplines = kept
	return changed(map[string]string{"retired": "nowhere: the demo keeps nothing once it is gone"})
}

// infoPDF is the event's sheet for the door. The adapter passes the page's own address,
// which in the demo is where the landing page really is.
func (e *Event) infoPDF(query string) Response {
	view := e.View()
	snap := hall.Snapshot{Tournament: store.Tournament{Event: view.Info}, Instance: hall.Instance{Name: view.Name}}
	return pdf(hall.BuildInfoPDF(snap, view.Programme, queryURL(query), strings.Contains(query, "lang=sv")))
}

// --- keeping it between tabs (issue #108) ---------------------------------------------

// eventSaveFormat is the shape of savedEvent. Format 1 was a single tournament, from
// before the demo was an event, format 2 an event without its mats' plan, format 3 one
// without its people, and format 4 one without its staff; a browser holding any of them
// starts fresh.
const eventSaveFormat = 5

type savedEvent struct {
	Format      int               `json:"format"`
	Info        store.Event       `json:"info"`
	Plan        store.Plan        `json:"plan"`
	People      []store.Person    `json:"people"`
	Defaults    store.Timings     `json:"defaults,omitempty"`
	Staff       store.Staff       `json:"staff"`
	Screens     store.Screens     `json:"screens,omitempty"`
	ID          string            `json:"id,omitempty"`
	Disciplines []savedDiscipline `json:"disciplines"`
}

type savedDiscipline struct {
	Slug  string          `json:"slug"`
	Name  string          `json:"name"`
	State json.RawMessage `json:"state"`
}

// Save is the whole event as JSON: the day, and every discipline's own save.
func (e *Event) Save() ([]byte, error) {
	out := savedEvent{Format: eventSaveFormat, Info: e.info, Plan: e.plan, People: e.people, Defaults: e.defaults, Staff: e.staff, Screens: e.screens, ID: e.id}
	for _, d := range e.disciplines {
		state, err := d.Save()
		if err != nil {
			return nil, err
		}
		out.Disciplines = append(out.Disciplines, savedDiscipline{Slug: d.slug, Name: d.tournament.Discipline, State: state})
	}
	return json.Marshal(out)
}

// Load replaces the event with one Save produced, or leaves it as it was.
func (e *Event) Load(b []byte) error {
	var in savedEvent
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	if in.Format != eventSaveFormat {
		return fmt.Errorf("a save in format %d, and this demo reads %d", in.Format, eventSaveFormat)
	}
	if len(in.Disciplines) == 0 {
		return fmt.Errorf("a save with no disciplines in it")
	}
	next := &Event{info: in.Info, plan: in.Plan}
	for _, sd := range in.Disciplines {
		build := emptyFixture(sd.Name)
		switch sd.Slug {
		case "open-steel-longsword":
			build = fixture
		case "open-sabre":
			build = sabreFixture
		case "sword-and-buckler":
			build = bucklerFixture
		}
		// Loaded and checked on its own, then put in the event: checked inside a half-built
		// event, its snapshot would place the event's items with the disciplines not yet
		// loaded missing, and drop their places from the plan.
		d := newDiscipline(sd.Slug, build)
		if err := d.Load(sd.State); err != nil {
			return fmt.Errorf("%s: %w", sd.Slug, err)
		}
		next.adopt(d)
	}
	e.info, e.plan, e.disciplines, e.keepers, e.people, e.defaults = next.info, next.plan, nil, nil, in.People, in.Defaults
	e.staff, e.screens = in.Staff, in.Screens
	e.id = in.ID
	if e.id == "" {
		e.id = "demo"
	}
	for _, d := range next.disciplines {
		e.adopt(d)
	}
	e.ensurePeople()
	return nil
}
