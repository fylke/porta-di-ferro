package demo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/event"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
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
	// keepers are the score keepers registered in this tab, for the match each holds.
	keepers map[string]keeper
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
	// The day was the longsword's own before events existed; it is the event's now, and
	// the discipline keeps only which programme row it is, as a migrated folder does.
	e.info = longsword.tournament.Event
	e.info.Signup.Tournament = ""
	longsword.tournament.Event = store.Event{Signup: store.Signup{Tournament: "longsword-pools"}}
	e.disciplines = nil
	e.plan = store.Plan{}
	e.keepers = nil
	e.adopt(longsword)
	e.adopt(sabre)
}

func (e *Event) adopt(d *Demo) {
	d.event = e
	e.disciplines = append(e.disciplines, d)
}

func (e *Event) find(slug string) *Demo {
	for _, d := range e.disciplines {
		if d.slug == slug {
			return d
		}
	}
	return nil
}

// View is the event as its pages see it, built the way the coordinator builds it.
func (e *Event) View() httpapi.EventView {
	view := httpapi.EventView{Info: e.info, Dir: "in this browser", Disciplines: []httpapi.DisciplineSummary{}}
	mats := e.mats()
	for _, d := range e.disciplines {
		snap, err := d.snapshot()
		if err != nil {
			view.Disciplines = append(view.Disciplines, httpapi.DisciplineSummary{
				Slug: d.slug, Name: d.tournament.Discipline, Error: err.Error(), Stage: "setup",
				Mats: []httpapi.MatSummary{}, Entrants: []httpapi.Entrant{},
			})
			continue
		}
		s := httpapi.Summarize(d.slug, snap)
		s.Mats = httpapi.SummaryMats(mats, d.slug)
		view.Disciplines = append(view.Disciplines, s)
	}
	view.Name = strings.TrimSpace(e.info.Signup.Name)
	if view.Name == "" && len(view.Disciplines) == 1 {
		view.Name = view.Disciplines[0].Name
	}
	return view
}

// Request answers one API call for the whole event.
func (e *Event) Request(method, path string, body []byte) Response {
	bare, query := path, ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		bare, query = path[:i], path[i+1:]
	}
	bare = strings.TrimSuffix(bare, "/")
	parts := strings.Split(strings.TrimPrefix(bare, "/"), "/")

	switch {
	case method == "GET" && bare == "/api/event":
		return ok(e.View())
	case method == "PUT" && bare == "/api/event/info":
		return e.putInfo(body)
	case method == "GET" && bare == "/api/info.pdf":
		return e.infoPDF(query)
	case method == "GET" && (bare == "/api/mats" || bare == "/api/plan"):
		return ok(e.mats())
	case method == "PUT" && bare == "/api/mats":
		return e.putMats(body)
	case method == "PATCH" && strings.HasPrefix(bare, "/api/plan/items/"):
		id, _ := itemPath(bare)
		return e.patchItem(id, body)
	case method == "GET" && bare == "/api/presence":
		return ok(map[string]any{"clients": []any{}, "quarantined": []any{}})
	case method == "POST" && len(parts) == 3 && parts[1] == "clients":
		return e.register(parts[2], body)
	case method == "POST" && len(parts) == 4 && parts[1] == "clients" && parts[3] == "release":
		return e.releaseClient(parts[2])
	case method == "PUT" && len(parts) == 4 && parts[1] == "clients" && parts[3] == "target":
		return ok(map[string]string{"target": ""})
	case method == "GET" && bare == "/api/addresses":
		// There is no LAN in a browser tab, which is the truth the admin pages show.
		return ok([]any{})
	case method == "GET" && bare == "/api/qr.png":
		return qr(query)

	case method == "GET" && bare == "/api/disciplines":
		return ok(e.View().Disciplines)
	case method == "GET" && bare == "/api/disciplines/presets":
		return ok(httpapi.Disciplines)
	case method == "POST" && bare == "/api/disciplines":
		return e.add(body)
	case len(parts) == 3 && parts[1] == "disciplines" && method == "PATCH":
		return e.rename(parts[2], body)
	case len(parts) == 3 && parts[1] == "disciplines" && method == "DELETE":
		return e.retire(parts[2])
	case len(parts) == 4 && parts[1] == "disciplines" && parts[3] == "reload" && method == "POST":
		// Nothing in the demo is read from a file, so there is nothing to read again.
		if d := e.find(parts[2]); d != nil {
			return e.summary(d)
		}
		return fail(404, fmt.Errorf("this event has no discipline %q", parts[2]))

	// Demo-only, and the adapter is the only caller.
	case method == "POST" && bare == "/api/demo/reset":
		e.Reset()
		return changed(map[string]bool{"ok": true})
	case method == "POST" && bare == "/api/demo/play":
		// Play the rest finishes every discipline, so the bracket and the podium can be
		// reached in each without scoring them by hand.
		for _, d := range e.disciplines {
			if res := d.playOut(); res.Status != 200 {
				return res
			}
		}
		return changed(map[string]bool{"ok": true})
	case method == "GET" && bare == "/api/demo/save":
		b, err := e.Save()
		if err != nil {
			return fail(500, err)
		}
		return Response{Status: 200, ContentType: "application/json; charset=utf-8", Body: string(b)}
	case method == "POST" && bare == "/api/demo/load":
		if err := e.Load(body); err != nil {
			return fail(400, err)
		}
		return ok(map[string]bool{"ok": true})

	case len(parts) >= 3 && parts[0] == "api" && parts[1] == "d":
		d := e.find(parts[2])
		if d == nil {
			return fail(404, fmt.Errorf("this event has no discipline %q", parts[2]))
		}
		rest := strings.TrimPrefix(path, "/api/d/"+parts[2])
		return d.Request(method, "/api"+rest, body)
	}

	// An unprefixed path: the one discipline's while there is one, and nobody's after.
	if len(e.disciplines) != 1 {
		slugs := make([]string, 0, len(e.disciplines))
		for _, d := range e.disciplines {
			slugs = append(slugs, d.slug)
		}
		b, _ := json.Marshal(map[string]any{
			"error":       "this event runs several disciplines: ask for one by its address, /api/d/{discipline}/…",
			"disciplines": slugs,
		})
		return Response{Status: 409, ContentType: "application/json; charset=utf-8", Body: string(b)}
	}
	return e.disciplines[0].Request(method, path, body)
}

func (e *Event) summary(d *Demo) Response {
	snap, err := d.snapshot()
	if err != nil {
		return fail(500, err)
	}
	return ok(httpapi.Summarize(d.slug, snap))
}

func (e *Event) putInfo(body []byte) Response {
	var in store.Event
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	ev, err := httpapi.CleanEvent(in)
	if err != nil {
		return fail(400, err)
	}
	ev.Signup.Tournament = ""
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
	name, err := httpapi.CleanName(in.Name)
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
	name, err := httpapi.CleanName(in.Name)
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

func (e *Event) retire(slug string) Response {
	if e.find(slug) == nil {
		return fail(404, fmt.Errorf("this event has no discipline %q", slug))
	}
	if len(e.disciplines) == 1 {
		return fail(400, fmt.Errorf("an event needs at least one discipline"))
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
	landing := ""
	for _, kv := range strings.Split(query, "&") {
		if after, found := strings.CutPrefix(kv, "url="); found {
			if decoded, err := url.QueryUnescape(after); err == nil {
				landing = decoded
			}
		}
	}
	view := e.View()
	snap := httpapi.Snapshot{Tournament: store.Tournament{Event: view.Info}, Instance: httpapi.Instance{Name: view.Name}}
	var buf bytes.Buffer
	if err := httpapi.BuildInfoPDF(snap, landing, strings.Contains(query, "lang=sv")).Output(&buf); err != nil {
		return fail(500, err)
	}
	return Response{
		Status:      200,
		ContentType: "application/pdf",
		Body:        base64.StdEncoding.EncodeToString(buf.Bytes()),
		Base64:      true,
	}
}

// --- keeping it between tabs (issue #108) ---------------------------------------------

// eventSaveFormat is the shape of savedEvent. Format 1 was a single tournament, from
// before the demo was an event, and format 2 an event without its mats' plan; a browser
// holding either starts fresh.
const eventSaveFormat = 3

type savedEvent struct {
	Format      int               `json:"format"`
	Info        store.Event       `json:"info"`
	Plan        store.Plan        `json:"plan"`
	Disciplines []savedDiscipline `json:"disciplines"`
}

type savedDiscipline struct {
	Slug  string          `json:"slug"`
	Name  string          `json:"name"`
	State json.RawMessage `json:"state"`
}

// Save is the whole event as JSON: the day, and every discipline's own save.
func (e *Event) Save() ([]byte, error) {
	out := savedEvent{Format: eventSaveFormat, Info: e.info, Plan: e.plan}
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
	e.info, e.plan, e.disciplines, e.keepers = next.info, next.plan, nil, nil
	for _, d := range next.disciplines {
		e.adopt(d)
	}
	return nil
}
