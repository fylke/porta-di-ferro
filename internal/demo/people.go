package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/people"
	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/web"
)

// The event's people and its one signup import (phase 3), in memory. The decisions are
// the server's own -- internal/people, httpapi.DecidePerson, httpapi.CombineSignups --
// and only where they are kept differs: a slice here, people.json and each discipline's
// competitors.json there.

// personFor is who a new entry is: httpapi.Coordinator.PersonFor, in memory.
func (e *Event) personFor(name, club, submission, want string) string {
	var id string
	e.people, id = people.For(e.people, name, club, submission, want)
	return id
}

func (e *Event) rosters() ([]people.Roster, map[string]string) {
	rosters := make([]people.Roster, 0, len(e.disciplines))
	names := map[string]string{}
	for _, d := range e.disciplines {
		rosters = append(rosters, people.Roster{Discipline: d.slug, Competitors: d.competitors})
		names[d.slug] = d.tournament.Discipline
	}
	return rosters, names
}

func (e *Event) entries() map[string][]people.Entry {
	rosters, names := e.rosters()
	return people.EntriesOf(e.people, rosters, names)
}

// ensurePeople gives every entry a person, as the coordinator does when it opens.
func (e *Event) ensurePeople() {
	rosters, _ := e.rosters()
	var links people.Links
	e.people, links = people.Ensure(e.people, rosters)
	e.link(links)
}

func (e *Event) link(links people.Links) {
	for slug, byComp := range links {
		d := e.find(slug)
		if d == nil {
			continue
		}
		for i, c := range d.competitors {
			if id, ok := byComp[c.ID]; ok {
				d.competitors[i].Person = id
			}
		}
	}
}

func (e *Event) getPerson(id string) Response {
	v, found := httpapi.ViewPerson(e.people, id, e.entries())
	if !found {
		return fail(404, fmt.Errorf("nobody with id %q is in this event", id))
	}
	return ok(e.withDuties(v))
}

func (e *Event) postPerson(id, action string, body []byte) Response {
	var in httpapi.PersonAction
	if len(body) > 0 {
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err)
		}
	}
	next, links, err := httpapi.DecidePerson(e.people, e.entries(), id, action, in)
	if errors.Is(err, httpapi.ErrNoSuchAction) {
		return fail(404, err)
	}
	if err != nil {
		return fail(400, err)
	}
	e.people = next
	e.link(links)
	return changed(httpapi.ViewPeople(e.people, e.entries()))
}

// --- the event's signup -----------------------------------------------------------------

func (e *Event) definition() signup.Definition {
	return signup.BuildDefinition(store.Tournament{Discipline: e.View().Name, Event: e.info})
}

func (e *Event) shares(def signup.Definition) []httpapi.SignupShare {
	out := make([]httpapi.SignupShare, 0, len(e.disciplines))
	for _, d := range e.disciplines {
		sh := httpapi.SignupShare{Discipline: d.slug, Name: d.tournament.Discipline}
		sh.Tournament, sh.Chosen = httpapi.RowFor(def, strings.TrimSpace(d.tournament.Event.Signup.Tournament),
			sh.Name, d.slug, len(e.disciplines) > 1)
		out = append(out, sh)
	}
	return out
}

func (e *Event) signupApp() Response {
	page, err := web.SignupApp()
	if err != nil {
		return fail(500, err)
	}
	baked, err := httpapi.BakeSignupApp(page, e.definition())
	if err != nil {
		return fail(500, err)
	}
	return Response{Status: 200, ContentType: "text/html; charset=utf-8", Body: string(baked)}
}

func (e *Event) putSignupRows(body []byte) Response {
	var in map[string]string
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	for slug := range in {
		if e.find(slug) == nil {
			return fail(400, fmt.Errorf("this event has no discipline %q to set", slug))
		}
	}
	for slug, row := range in {
		e.find(slug).tournament.Event.Signup.Tournament = strings.TrimSpace(row)
	}
	def := e.definition()
	return changed(httpapi.SignupReadyView(def, e.shares(def)))
}

func (e *Event) previewSignups(files []signup.File) httpapi.EventSignupPreview {
	def := e.definition()
	shares := e.shares(def)
	previews := map[int]httpapi.SharePreview{}
	for i, sh := range shares {
		if !httpapi.Takes(sh, len(shares) > 1) {
			continue
		}
		d := e.disciplines[i]
		previews[i] = httpapi.SharePreview{Preview: d.checkSignups(files, sh.Tournament), PoolsDrawn: len(d.tournament.Pools) > 0}
	}
	return httpapi.CombineSignups(def, files, shares, previews)
}

func (e *Event) signupImport(body []byte, confirm bool) Response {
	files, err := signupFiles(body)
	if err != nil {
		return fail(400, err)
	}
	if !confirm {
		return ok(e.previewSignups(files))
	}
	def := e.definition()
	shares := e.shares(def)
	added, addedStaff := 0, 0
	for i, sh := range shares {
		if !httpapi.Takes(sh, len(shares) > 1) {
			continue
		}
		a, s := e.disciplines[i].importSignups(files, sh.Tournament)
		added += a
		addedStaff += s
	}
	return changed(map[string]any{"added": added, "addedStaff": addedStaff, "preview": e.previewSignups(files)})
}

func signupFiles(body []byte) ([]signup.File, error) {
	var in struct {
		Files []struct {
			Source string `json:"source"`
			Body   string `json:"body"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, err
	}
	files := make([]signup.File, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, signup.File{Source: f.Source, Body: []byte(f.Body)})
	}
	return files, nil
}

// peopleRequest answers the event's people and signup paths, or reports it has none.
func (e *Event) peopleRequest(method, bare string, parts []string, body []byte) (Response, bool) {
	switch {
	case method == "GET" && bare == "/api/people":
		return ok(httpapi.ViewPeople(e.people, e.entries())), true
	case method == "GET" && len(parts) == 3 && parts[1] == "people":
		return e.getPerson(parts[2]), true
	case method == "POST" && len(parts) == 4 && parts[1] == "people":
		return e.postPerson(parts[2], parts[3], body), true

	case method == "GET" && bare == "/api/event/signup/ready":
		def := e.definition()
		return ok(httpapi.SignupReadyView(def, e.shares(def))), true
	case method == "GET" && bare == "/api/event/signup/definition.json":
		return ok(e.definition()), true
	case method == "GET" && bare == "/api/event/signup/app.html":
		return e.signupApp(), true
	case method == "PUT" && bare == "/api/event/signup/rows":
		return e.putSignupRows(body), true
	case method == "POST" && bare == "/api/event/signup/preview":
		return e.signupImport(body, false), true
	case method == "POST" && bare == "/api/event/signup/import":
		return e.signupImport(body, true), true
	}
	return Response{}, false
}
