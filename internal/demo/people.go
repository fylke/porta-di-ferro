package demo

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/people"
	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/web"
)

// The event's people and its one signup import (phase 3), in memory. The decisions are
// the server's own -- internal/people, hall.DecidePerson, hall.CombineSignups --
// and only where they are kept differs: a slice here, people.json and each discipline's
// competitors.json there.

// personFor is who a new entry is: httpapi.Coordinator.PersonFor, in memory.
func (e *Event) personFor(name, club, submission, email, want string) string {
	var id string
	e.people, id, _ = people.For(e.people, name, club, submission, email, want)
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
	v, found := hall.ViewPerson(e.people, id, e.entries())
	if !found {
		return fail(404, fmt.Errorf("nobody with id %q is in this event", id))
	}
	return ok(e.withDuties(v))
}

func (e *Event) postPerson(id, action string, body []byte) Response {
	var in hall.PersonAction
	if len(body) > 0 {
		if err := json.Unmarshal(body, &in); err != nil {
			return fail(400, err)
		}
	}
	next, links, err := hall.DecidePerson(e.people, e.entries(), id, action, in)
	if errors.Is(err, hall.ErrNoSuchAction) {
		return fail(404, err)
	}
	if err != nil {
		return fail(400, err)
	}
	e.people = next
	e.link(links)
	return changed(hall.ViewPeople(e.people, e.entries()))
}

// --- the event's signup -----------------------------------------------------------------

// definition is the event's signup definition: every discipline with a name, as
// httpapi.Coordinator.SignupDefinition has it.
func (e *Event) definition(withTimes bool) signup.Definition {
	var starts map[string]string
	name := ""
	if withTimes {
		view := e.View()
		starts, name = hall.FirstStarts(view.Programme), view.Name
	}
	entries := make([]signup.TournamentInfo, 0, len(e.disciplines))
	for _, d := range e.disciplines {
		entries = append(entries, signup.TournamentInfo{
			ID: d.slug, Label: d.tournament.Discipline, Capacity: d.tournament.Capacity, StartsAt: starts[d.slug],
		})
	}
	def := signup.BuildDefinition(e.info, name, entries)
	def.Moved = e.moved
	return def
}

func (e *Event) shares() []hall.SignupShare {
	out := make([]hall.SignupShare, 0, len(e.disciplines))
	for _, d := range e.disciplines {
		out = append(out, hall.SignupShare{Discipline: d.slug, Name: d.tournament.Discipline})
	}
	return out
}

func (e *Event) signupApp() Response {
	page, err := web.SignupApp()
	if err != nil {
		return fail(500, err)
	}
	baked, err := hall.BakeSignupApp(page, e.definition(true))
	if err != nil {
		return fail(500, err)
	}
	return Response{Status: 200, ContentType: "text/html; charset=utf-8", Body: string(baked)}
}

func (e *Event) previewSignups(files []signup.File) hall.EventSignupPreview {
	shares := e.shares()
	previews := map[int]hall.SharePreview{}
	for i, d := range e.disciplines {
		previews[i] = hall.SharePreview{Preview: d.checkSignups(files), PoolsDrawn: len(d.tournament.Pools) > 0}
	}
	return hall.CombineSignups(e.definition(false), files, shares, previews)
}

func (e *Event) signupImport(body []byte, confirm bool) Response {
	files, err := signupFiles(body)
	if err != nil {
		return fail(400, err)
	}
	if !confirm {
		return ok(e.previewSignups(files))
	}
	added, addedStaff := 0, 0
	for _, d := range e.disciplines {
		a, s := d.importSignups(files)
		added += a
		addedStaff += s
	}
	return changed(map[string]any{"added": added, "addedStaff": addedStaff, "preview": e.previewSignups(files)})
}

func signupFiles(body []byte) ([]signup.File, error) {
	var in struct {
		Files []signup.UploadFile `json:"files"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, err
	}
	return signup.DecodeFiles(in.Files)
}

// peopleRequest answers the event's people and signup paths, or reports it has none.
func (e *Event) peopleRequest(method, bare string, parts []string, body []byte) (Response, bool) {
	switch {
	case method == "GET" && bare == "/api/people":
		return ok(hall.ViewPeople(e.people, e.entries())), true
	case method == "GET" && len(parts) == 3 && parts[1] == "people":
		return e.getPerson(parts[2]), true
	case method == "POST" && len(parts) == 4 && parts[1] == "people":
		return e.postPerson(parts[2], parts[3], body), true

	case method == "GET" && bare == "/api/event/signup/ready":
		return ok(hall.SignupReadyView(e.definition(true))), true
	case method == "GET" && bare == "/api/event/signup/definition.json":
		return ok(e.definition(true)), true
	case method == "GET" && bare == "/api/event/signup/app.html":
		return e.signupApp(), true
	case method == "GET" && bare == "/api/event/signup/form.pdf":
		return signupPDF(), true
	case method == "POST" && bare == "/api/event/signup/preview":
		return e.signupImport(body, false), true
	case method == "POST" && bare == "/api/event/signup/import":
		return e.signupImport(body, true), true
	}
	return Response{}, false
}
