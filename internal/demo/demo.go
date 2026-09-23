// Package demo is the application with its server taken away.
//
// The point of the demo (issue #88) is that someone can be shown what this is without
// anyone installing anything, so it has to run entirely in a browser tab, on GitHub
// Pages, with no Go process anywhere. What it must not become is a second, simpler
// implementation that says something different from a real event: standings, tie-breaks
// and the bracket are the parts people ask hard questions about, and a demo that got
// them subtly wrong would be worse than no demo.
//
// So this is the real thing with a different source underneath it. The tournament lives
// in memory instead of in JSON files on the organizer's disk, and every derived answer
// still comes from internal/tournament, internal/match and httpapi.BuildSnapshot -- the
// same code the Go server runs. Compiled to WebAssembly by cmd/demo-wasm, it answers the
// same API calls the client already makes, so the Svelte application is not modified for
// the demo and cannot drift from it.
//
// This package is ordinary portable Go and is tested as such. Only cmd/demo-wasm carries
// the js/wasm build tag, which keeps the whole of this file under `go test ./...`.
package demo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
	"github.com/fylke/porta-di-ferro/web"
)

// Demo is one visitor's tournament: the whole of it, in memory.
//
// There is no locking. A browser tab is single threaded, and the wasm entry point is
// called from that one thread.
type Demo struct {
	rules       match.Ruleset
	limits      tournament.Limits
	competitors []store.Competitor
	tournament  store.Tournament
	logs        map[string][]match.Event
	// lastEvent is what the clock on a display counts on from, and the only thing here
	// that depends on the wall clock.
	lastEvent map[string]time.Time
}

// New builds the demo tournament: the fixture, drawn and mostly played.
func New() *Demo {
	d := &Demo{rules: match.MSL(), limits: tournament.DefaultLimits()}
	d.Reset()
	return d
}

// Reset puts the tournament back to how a visitor first found it. The demo is meant to
// be poked at, which means it has to be possible to undo the poking.
func (d *Demo) Reset() {
	d.competitors, d.tournament, d.logs = fixture(d.rules, d.limits)
	d.lastEvent = map[string]time.Time{}
	// The match still running had its last exchange a few seconds ago, so every display
	// opens with a clock that is already moving. Setting it to this instant would be
	// truthful and look frozen for the first second somebody watched it.
	for id, events := range d.logs {
		if len(events) > 0 && !match.Replay(d.rules, events).Ended {
			d.lastEvent[id] = time.Now().Add(-8 * time.Second)
		}
	}
}

// --- httpapi.Source -----------------------------------------------------------------
//
// The four reads BuildSnapshot needs. *store.Store answers these off the disk; this
// answers them out of memory, and neither knows about the other.

func (d *Demo) Competitors() ([]store.Competitor, error) {
	out := make([]store.Competitor, len(d.competitors))
	copy(out, d.competitors)
	return out, nil
}

func (d *Demo) Tournament() (store.Tournament, error) { return d.tournament, nil }

func (d *Demo) Events(id string, after int) ([]match.Event, error) {
	var out []match.Event
	for _, e := range d.logs[id] {
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out, nil
}

func (d *Demo) LastEventAt(id string) (time.Time, bool) {
	at, ok := d.lastEvent[id]
	return at, ok
}

// Dir is what the organizer view shows as where the data lives. Saying so plainly beats
// inventing a path that does not exist.
func (d *Demo) Dir() string { return "in this browser tab" }

func (d *Demo) snapshot() (httpapi.Snapshot, error) {
	return httpapi.BuildSnapshot(d, d.rules, httpapi.Instance{
		Name: "Open steel Longsword",
		Port: 8080,
		Self: true,
		URL:  "/",
	}, func(int) string {
		// Nobody is connected to a demo: every mat shows the next match it would run.
		return ""
	})
}

// --- the API ------------------------------------------------------------------------

// Response is what the demo adapter turns back into a fetch Response.
type Response struct {
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	// Body is the response body. Base64 says it arrived as bytes rather than text,
	// which is how the PDF export gets out.
	Body   string `json:"body"`
	Base64 bool   `json:"base64,omitempty"`
	// Changed says this call altered the tournament, so the adapter knows to push a
	// fresh snapshot down its stand-in for the event stream.
	Changed bool `json:"changed,omitempty"`
}

func ok(v any) Response {
	b, err := json.Marshal(v)
	if err != nil {
		return fail(500, err)
	}
	return Response{Status: 200, ContentType: "application/json; charset=utf-8", Body: string(b)}
}

func changed(v any) Response {
	r := ok(v)
	r.Changed = true
	return r
}

func fail(status int, err error) Response {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return Response{Status: status, ContentType: "application/json; charset=utf-8", Body: string(b)}
}

// Request answers one API call.
//
// It is deliberately a router rather than a mock: the client is the real client, and it
// asks for exactly what it asks a Go server for. Anything the demo cannot honestly do --
// starting a second discipline, which is a second process -- says so with a status code
// rather than pretending it worked.
func (d *Demo) Request(method, path string, body []byte) Response {
	path = strings.TrimSuffix(path, "/")
	query := ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path, query = path[:i], path[i+1:]
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	switch {
	case method == "GET" && path == "/api/state":
		snap, err := d.snapshot()
		if err != nil {
			return fail(500, err)
		}
		return ok(snap)

	case method == "GET" && path == "/api/export.json":
		snap, err := d.snapshot()
		if err != nil {
			return fail(500, err)
		}
		return ok(snap)

	case method == "GET" && path == "/api/export.pdf":
		return d.exportPDF(strings.Contains(query, "lang=sv"))

	case method == "GET" && path == "/api/addresses":
		// There is no LAN in a browser tab. An empty list is what the organizer view
		// shows when a PC is on no network, which is the truth here.
		return ok([]any{})

	case method == "GET" && path == "/api/info.pdf":
		return d.infoPDF(query)

	case method == "GET" && path == "/api/qr.png":
		return d.qr(query)

	// Offline signup (issue #91). The demo can do all of it: the files never touch a
	// network in the real thing either, so there is nothing here it has to pretend about.
	case method == "GET" && path == "/api/signup/ready":
		return d.signupReady()
	case method == "GET" && path == "/api/signup/definition.json":
		return ok(signup.BuildDefinition(d.tournament))
	case method == "GET" && path == "/api/signup/app.html":
		return d.signupApp()
	case method == "POST" && path == "/api/signup/preview":
		return d.signupImport(body, false)
	case method == "POST" && path == "/api/signup/import":
		return d.signupImport(body, true)

	case method == "GET" && path == "/api/instances":
		snap, _ := d.snapshot()
		return ok([]httpapi.Instance{snap.Instance})

	case method == "GET" && path == "/api/disciplines":
		return ok(httpapi.Disciplines)

	case method == "POST" && path == "/api/instances":
		return fail(400, fmt.Errorf("a second discipline is a second copy of the application running beside this one, which the demo has no way to start"))

	case method == "GET" && path == "/api/presence":
		return ok(map[string]any{"clients": []any{}, "quarantined": []any{}})

	// A demo has no other devices to hand a mat to, but the client registers itself on
	// load and would show an error if these failed.
	case len(parts) >= 2 && parts[1] == "clients":
		return ok(map[string]bool{"ok": true})
	case len(parts) == 3 && parts[1] == "quarantine":
		return ok(map[string]bool{"ok": true})

	case method == "POST" && path == "/api/competitors":
		return d.addCompetitor(body)
	case method == "PATCH" && len(parts) == 3 && parts[1] == "competitors":
		return d.patchCompetitor(parts[2], body)
	case method == "DELETE" && len(parts) == 3 && parts[1] == "competitors":
		return d.deleteCompetitor(parts[2])

	case method == "PUT" && path == "/api/tournament":
		return d.putTournament(body)
	case method == "POST" && path == "/api/tournament/pools":
		return d.generatePools()
	case method == "PATCH" && len(parts) == 4 && parts[2] == "pools":
		return d.patchPool(parts[3], body)
	case method == "POST" && path == "/api/tournament/bracket":
		return d.drawBracket()

	case len(parts) == 4 && parts[1] == "matches" && parts[3] == "events":
		switch method {
		case "GET":
			return d.getEvents(parts[2], query)
		case "POST":
			return d.postEvents(parts[2], body)
		case "PUT":
			return d.replaceEvents(parts[2], body)
		}
	case method == "GET" && len(parts) == 4 && parts[1] == "matches" && parts[3] == "backups":
		// The demo keeps no backups: there is no disk to keep them on, and nothing here
		// outlives the tab.
		return ok([]string{})
	case len(parts) == 4 && parts[1] == "matches" && parts[3] == "claim":
		return ok(map[string]bool{"ok": true})

	// Demo-only. Not part of the application's API, and the adapter is the only caller.
	case method == "POST" && path == "/api/demo/reset":
		d.Reset()
		return changed(map[string]bool{"ok": true})
	case method == "POST" && path == "/api/demo/play":
		return d.playOut()
	}

	return fail(404, fmt.Errorf("the demo does not answer %s %s", method, path))
}

// --- offline signup --------------------------------------------------------------------

func (d *Demo) signupReady() Response {
	def := signup.BuildDefinition(d.tournament)
	return ok(map[string]any{
		"missing":     signup.Ready(def),
		"tournaments": def.Tournaments,
		"filename":    "signup-demo.html",
		"definition":  "signup-demo.json",
	})
}

func (d *Demo) signupApp() Response {
	page, err := web.SignupApp()
	if err != nil {
		return fail(500, err)
	}
	baked, err := httpapi.BakeSignupApp(page, signup.BuildDefinition(d.tournament))
	if err != nil {
		return fail(500, err)
	}
	return Response{Status: 200, ContentType: "text/html; charset=utf-8", Body: string(baked)}
}

// signupImport previews an import, and performs it when confirm is set. Two calls on the
// real server, and two here, because the preview writing nothing is the whole design.
func (d *Demo) signupImport(body []byte, confirm bool) Response {
	var in struct {
		Files []struct {
			Source string `json:"source"`
			Body   string `json:"body"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	files := make([]signup.File, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, signup.File{Source: f.Source, Body: []byte(f.Body)})
	}

	def := signup.BuildDefinition(d.tournament)
	preview := signup.Check(def, d.tournament.Event.Signup.Tournament, files, d.competitors)
	view := map[string]any{
		"rows": preview.Rows, "adding": preview.Adding, "capacity": preview.Capacity,
		"tournament": preview.Tournament, "poolsDrawn": len(d.tournament.Pools) > 0,
	}
	if !confirm {
		return ok(view)
	}

	before := len(d.competitors)
	d.competitors = signup.Import(preview, httpapi.NextCompetitorID, d.competitors)
	return changed(map[string]any{
		"added": len(d.competitors) - before, "preview": view,
	})
}

// qr renders a code for whatever it is given -- the wifi payload and the landing address
// on the info sheet. The server has an endpoint for this and the client asks for it with
// an <img>, so without it here the demo's info sheet shows two broken images, which is
// the first thing a visitor to the demo would see of a page that is mostly two codes.
func (d *Demo) qr(query string) Response {
	target := ""
	for _, kv := range strings.Split(query, "&") {
		if after, found := strings.CutPrefix(kv, "url="); found {
			if decoded, err := url.QueryUnescape(after); err == nil {
				target = decoded
			}
		}
	}
	if target == "" {
		return fail(400, fmt.Errorf("a code needs something to encode"))
	}
	png, err := qrcode.Encode(target, qrcode.Medium, 512)
	if err != nil {
		return fail(500, err)
	}
	return Response{
		Status:      200,
		ContentType: "image/png",
		Body:        base64.StdEncoding.EncodeToString(png),
		Base64:      true,
	}
}

// infoPDF is the sheet for the door. The demo has no LAN address to print on it, so the
// adapter passes the page's own, which in the demo is where the landing page really is.
func (d *Demo) infoPDF(query string) Response {
	snap, err := d.snapshot()
	if err != nil {
		return fail(500, err)
	}
	landing := ""
	for _, kv := range strings.Split(query, "&") {
		if after, found := strings.CutPrefix(kv, "url="); found {
			if decoded, err := url.QueryUnescape(after); err == nil {
				landing = decoded
			}
		}
	}
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

func (d *Demo) exportPDF(swedish bool) Response {
	snap, err := d.snapshot()
	if err != nil {
		return fail(500, err)
	}
	// Base64 rather than text: the adapter turns it back into bytes for the Blob the
	// browser prints from.
	var buf bytes.Buffer
	if err := httpapi.BuildPDF(snap, swedish).Output(&buf); err != nil {
		return fail(500, err)
	}
	return Response{
		Status:      200,
		ContentType: "application/pdf",
		Body:        base64.StdEncoding.EncodeToString(buf.Bytes()),
		Base64:      true,
	}
}

// --- competitors --------------------------------------------------------------------

func (d *Demo) addCompetitor(body []byte) Response {
	var in struct{ Name, Club string }
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	if strings.TrimSpace(in.Name) == "" {
		return fail(400, fmt.Errorf("a competitor needs a name"))
	}
	c := store.Competitor{
		ID:   httpapi.NextCompetitorID(d.competitors),
		Name: strings.TrimSpace(in.Name),
		Club: strings.TrimSpace(in.Club),
	}
	d.competitors = append(d.competitors, c)
	return changed(c)
}

func (d *Demo) patchCompetitor(id string, body []byte) Response {
	var in struct {
		Name      *string `json:"name"`
		Club      *string `json:"club"`
		Withdrawn *bool   `json:"withdrawn"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	for i := range d.competitors {
		if d.competitors[i].ID != id {
			continue
		}
		if in.Name != nil {
			if strings.TrimSpace(*in.Name) == "" {
				return fail(400, fmt.Errorf("a competitor needs a name"))
			}
			d.competitors[i].Name = strings.TrimSpace(*in.Name)
		}
		if in.Club != nil {
			d.competitors[i].Club = strings.TrimSpace(*in.Club)
		}
		if in.Withdrawn != nil {
			d.competitors[i].Withdrawn = *in.Withdrawn
		}
		return changed(map[string]bool{"ok": true})
	}
	return fail(404, fmt.Errorf("no competitor %s", id))
}

func (d *Demo) deleteCompetitor(id string) Response {
	for i := range d.competitors {
		if d.competitors[i].ID == id {
			d.competitors = append(d.competitors[:i], d.competitors[i+1:]...)
			return changed(map[string]bool{"ok": true})
		}
	}
	return fail(404, fmt.Errorf("no competitor %s", id))
}

// --- the draw -----------------------------------------------------------------------

func (d *Demo) putTournament(body []byte) Response {
	var in struct {
		Mats        int  `json:"mats"`
		MinPoolSize int  `json:"minPoolSize"`
		MaxPoolSize int  `json:"maxPoolSize"`
		ElimMats    *int `json:"elimMats"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	d.tournament.Mats, d.tournament.MinPoolSize, d.tournament.MaxPoolSize = in.Mats, in.MinPoolSize, in.MaxPoolSize
	if in.ElimMats != nil {
		d.tournament.ElimMats = *in.ElimMats
		if d.tournament.ElimMats < 0 {
			d.tournament.ElimMats = 0
		}
	}
	return changed(d.tournament)
}

// generatePools is the same restart the server performs: the logs of the draw being
// replaced go out of play with it, because match ids are positional and the new pool 1
// would otherwise inherit the old one's results (issue #93).
func (d *Demo) generatePools() Response {
	for _, p := range d.tournament.Pools {
		for _, m := range p.Matches {
			delete(d.logs, m.ID)
			delete(d.lastEvent, m.ID)
		}
	}
	for _, m := range d.tournament.Bracket {
		delete(d.logs, m.ID)
		delete(d.lastEvent, m.ID)
	}
	t := d.tournament
	t.Seed, t.Pools, t.Bracket, t.BracketAt = 0, nil, nil, ""
	drawn, err := tournament.Generate(t, d.competitors, d.limits)
	if err != nil {
		return fail(400, err)
	}
	drawn.GeneratedAt = time.Now().Format(time.RFC3339)
	d.tournament = drawn
	return changed(drawn)
}

func (d *Demo) patchPool(number string, body []byte) Response {
	n, err := strconv.Atoi(number)
	if err != nil {
		return fail(400, err)
	}
	var in struct {
		Mat  int    `json:"mat"`
		Move string `json:"move"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	var next store.Tournament
	switch {
	case in.Move == "up" || in.Move == "down":
		next, err = tournament.ReorderPool(d.tournament, n, in.Move == "up")
	default:
		next, err = tournament.MovePool(d.tournament, n, in.Mat)
	}
	if err != nil {
		return fail(400, err)
	}
	d.tournament = next
	return changed(next)
}

func (d *Demo) drawBracket() Response {
	snap, err := d.snapshot()
	if err != nil {
		return fail(500, err)
	}
	if !snap.PoolsComplete {
		return fail(400, fmt.Errorf("the eliminations are drawn once every pool match is in"))
	}
	for _, m := range d.tournament.Bracket {
		delete(d.logs, m.ID)
		delete(d.lastEvent, m.ID)
	}
	bracket, err := tournament.Bracket(d.tournament, snap.Overall)
	if err != nil {
		return fail(400, err)
	}
	d.tournament.Bracket = bracket
	d.tournament.BracketAt = time.Now().Format(time.RFC3339)
	return changed(d.tournament)
}

// --- match logs ---------------------------------------------------------------------

func (d *Demo) getEvents(id, query string) Response {
	after := 0
	for _, kv := range strings.Split(query, "&") {
		if strings.HasPrefix(kv, "after=") {
			after, _ = strconv.Atoi(strings.TrimPrefix(kv, "after="))
		}
	}
	events, err := d.Events(id, after)
	if err != nil {
		return fail(400, err)
	}
	if events == nil {
		events = []match.Event{}
	}
	return ok(events)
}

// postEvents appends, skipping any sequence number already in the log. That dedupe is
// the whole of the offline story: the client retries what it could not send, and the
// primary key (match, seq) makes the retry a no-op (design §3).
func (d *Demo) postEvents(id string, body []byte) Response {
	var in []match.Event
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	seen := map[int]bool{}
	for _, e := range d.logs[id] {
		seen[e.Seq] = true
	}
	var fresh []match.Event
	for _, e := range in {
		if e.Seq <= 0 || seen[e.Seq] {
			continue
		}
		e.Match = id
		if e.At == "" {
			e.At = time.Now().Format(time.RFC3339Nano)
		}
		seen[e.Seq] = true
		fresh = append(fresh, e)
	}
	if len(fresh) == 0 {
		return changed([]match.Event{})
	}
	d.logs[id] = append(d.logs[id], fresh...)
	d.lastEvent[id] = time.Now()
	return changed(fresh)
}

func (d *Demo) replaceEvents(id string, body []byte) Response {
	var in []match.Event
	if err := json.Unmarshal(body, &in); err != nil {
		return fail(400, err)
	}
	seqs := map[int]bool{}
	for _, e := range in {
		if e.Seq <= 0 {
			return fail(400, fmt.Errorf("event sequence numbers must be positive, got %d", e.Seq))
		}
		if seqs[e.Seq] {
			return fail(400, fmt.Errorf("sequence number %d appears twice", e.Seq))
		}
		seqs[e.Seq] = true
	}
	sort.Slice(in, func(i, j int) bool { return in[i].Seq < in[j].Seq })
	for i := range in {
		in[i].Match = id
		if in[i].At == "" {
			in[i].At = time.Now().Format(time.RFC3339Nano)
		}
	}
	d.logs[id] = in
	d.lastEvent[id] = time.Now()
	return changed(map[string]any{"ok": true, "backup": ""})
}

// playOut finishes every match that is still open, so a visitor can get to the bracket
// and the podium without scoring forty matches by hand. Demo-only: there is no such
// thing at a real event, and the button that calls it says so.
func (d *Demo) playOut() Response {
	snap, err := d.snapshot()
	if err != nil {
		return fail(500, err)
	}
	seed := int64(len(d.logs)*7919 + 13)
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			if m.State.Ended {
				continue
			}
			seed++
			d.logs[m.ID] = playMatch(d.rules, m.ID, seed, false)
			delete(d.lastEvent, m.ID)
		}
	}
	// The bracket fills a round at a time: a semi-final has no competitors until the
	// quarter-finals that feed it have been decided, so each pass plays what it can and
	// the next pass sees further.
	for round := 0; round < 4; round++ {
		snap, err = d.snapshot()
		if err != nil {
			return fail(500, err)
		}
		if snap.Bracket == nil {
			break
		}
		played := false
		for _, m := range snap.Bracket.Matches {
			if m.State.Ended || m.Red == "" || m.Blue == "" {
				continue
			}
			seed++
			// Decisive: a drawn semi-final leaves the final with nobody in it.
			d.logs[m.ID] = playMatch(d.rules, m.ID, seed, true)
			delete(d.lastEvent, m.ID)
			played = true
		}
		if !played {
			break
		}
	}
	return changed(map[string]bool{"ok": true})
}
