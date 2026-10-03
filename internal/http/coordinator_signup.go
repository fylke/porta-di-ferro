package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/web"
)

// Signup for the whole event (proposal §8, phase 3): one file out, one folder back, and
// every discipline gets its share. Each share is decided by the same signup.Check a
// discipline runs on its own, so the rules are the ones #91 settled; what this adds is
// asking every discipline at once, and saying in one table where each response goes.
//
// Which discipline takes which programme row is the discipline's own setting. One that
// never set it takes the row named like it -- "Open Sabre" takes the row labelled Open
// Sabre -- so an organizer who names things the same twice never has to think about it.
// In an event of one discipline an unset row still means "everything", as it always did.

// SignupShare is one discipline's part in the event's signup.
type SignupShare struct {
	Discipline string `json:"discipline"`
	Name       string `json:"name"`
	// Tournament is the programme row it takes: its own setting, or the row named like it.
	Tournament string `json:"tournament"`
	// Chosen says the discipline set the row itself rather than having it matched by name.
	Chosen      bool     `json:"chosen"`
	Adding      int      `json:"adding"`
	AddingStaff int      `json:"addingStaff"`
	Capacity    []string `json:"capacity,omitempty"`
	PoolsDrawn  bool     `json:"poolsDrawn,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// EventSignupRow is one response, and where it goes.
type EventSignupRow struct {
	signup.Row
	// Into are the disciplines it adds an entry or a staff member to.
	Into []RowShare `json:"into,omitempty"`
}

// RowShare is a response's place in one discipline.
type RowShare struct {
	Discipline string         `json:"discipline"`
	Name       string         `json:"name"`
	Verdict    signup.Verdict `json:"verdict"`
}

// EventSignupPreview is the whole event's answer, nothing having happened.
type EventSignupPreview struct {
	Rows        []EventSignupRow `json:"rows"`
	Disciplines []SignupShare    `json:"disciplines"`
	Adding      int              `json:"adding"`
	AddingStaff int              `json:"addingStaff"`
}

// eventDefinition is the event's signup definition, from event.json.
func (c *Coordinator) eventDefinition() (signup.Definition, error) {
	ev, err := c.EventInfo()
	if err != nil {
		return signup.Definition{}, err
	}
	return signup.BuildDefinition(store.Tournament{Discipline: c.View().Name, Event: ev}), nil
}

// SignupRowFor is the programme row a discipline takes, as the event's import would give it,
// and whether the event has several disciplines (SignupRows, for a discipline's own
// import). Read from event.json alone, never from a discipline's snapshot: the asking
// discipline holds its own lock.
func (c *Coordinator) SignupRowFor(slug string) (string, bool) {
	ev, err := c.EventInfo()
	if err != nil {
		return "", false
	}
	def := signup.BuildDefinition(store.Tournament{Event: ev})
	shares, workers := c.shares(def)
	for _, sh := range shares {
		if sh.Discipline == slug {
			return sh.Tournament, len(workers) > 1
		}
	}
	return "", len(workers) > 1
}

// shares is which discipline takes which programme row. A discipline that cannot be read
// is listed with why, and takes nothing.
func (c *Coordinator) shares(def signup.Definition) ([]SignupShare, []*worker) {
	workers := c.snapshotWorkers()
	several := len(workers) > 1
	out := make([]SignupShare, 0, len(workers))
	srvs := make([]*worker, 0, len(workers))
	for _, w := range workers {
		sh := SignupShare{Discipline: w.slug, Name: w.slug}
		if w.srv == nil {
			sh.Error = w.err.Error()
			out, srvs = append(out, sh), append(srvs, w)
			continue
		}
		sh.Name = w.srv.Self().Name
		own, err := w.srv.SignupRow()
		if err != nil {
			sh.Error = err.Error()
		}
		sh.Tournament, sh.Chosen = RowFor(def, own, sh.Name, w.slug, several)
		out, srvs = append(out, sh), append(srvs, w)
	}
	return out, srvs
}

// RowFor is the programme row a discipline takes: the one it chose, else -- in an event
// of several -- the one named like it. Shared with the browser demo.
func RowFor(def signup.Definition, own, name, slug string, several bool) (row string, chosen bool) {
	if own != "" {
		return own, true
	}
	if !several {
		return "", false
	}
	return rowNamed(def, name, slug), false
}

// rowNamed is the programme row named like a discipline, or "".
func rowNamed(def signup.Definition, name, slug string) string {
	want := signup.Slug(name)
	for _, t := range def.Tournaments {
		if want != "" && (t.ID == want || signup.Slug(t.Label) == want) {
			return t.ID
		}
	}
	for _, t := range def.Tournaments {
		if t.ID == slug {
			return t.ID
		}
	}
	return ""
}

// Takes says whether a discipline is in the import: it has a row, or it is the event's
// only discipline and takes everything.
func Takes(sh SignupShare, several bool) bool {
	return sh.Error == "" && (sh.Tournament != "" || !several)
}

// Unclaimed are the programme rows nobody takes: whoever enters them is imported nowhere.
func Unclaimed(def signup.Definition, shares []SignupShare) []signup.TournamentInfo {
	taken := map[string]bool{}
	for _, sh := range shares {
		taken[sh.Tournament] = true
	}
	out := []signup.TournamentInfo{}
	for _, t := range def.Tournaments {
		if !taken[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

func (c *Coordinator) signupReady(w http.ResponseWriter, r *http.Request) {
	def, err := c.eventDefinition()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	shares, _ := c.shares(def)
	writeJSON(w, http.StatusOK, SignupReadyView(def, shares))
}

// SignupReadyView is what the event's signup panel is told before anything goes out.
func SignupReadyView(def signup.Definition, shares []SignupShare) map[string]any {
	return map[string]any{
		"missing":     signup.Ready(def),
		"tournaments": def.Tournaments,
		"filename":    signupAppFilename(def),
		"definition":  definitionFilename(def),
		"disciplines": shares,
		"unclaimed":   Unclaimed(def, shares),
	}
}

func (c *Coordinator) signupDefinition(w http.ResponseWriter, r *http.Request) {
	def, err := c.eventDefinition()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if r.URL.Query().Get("download") != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, definitionFilename(def)))
	}
	writeJSON(w, http.StatusOK, def)
}

func (c *Coordinator) signupApp(w http.ResponseWriter, r *http.Request) {
	def, err := c.eventDefinition()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	page, err := web.SignupApp()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	baked, err := bakeDefinition(page, def)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if r.URL.Query().Get("download") != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, signupAppFilename(def)))
	}
	w.Write(baked)
}

// putSignupRows sets which programme row each discipline takes.
func (c *Coordinator) putSignupRows(w http.ResponseWriter, r *http.Request) {
	var in map[string]string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	for slug, row := range in {
		c.mu.Lock()
		wk := c.workers[slug]
		c.mu.Unlock()
		if wk == nil || wk.srv == nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("this event has no discipline %q to set", slug))
			return
		}
		if err := wk.srv.SetSignupRow(row); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	c.poke()
	c.signupReady(w, r)
}

// PreviewSignups is what importing these files would do, across the event.
func (c *Coordinator) PreviewSignups(files []signup.File) (EventSignupPreview, error) {
	def, err := c.eventDefinition()
	if err != nil {
		return EventSignupPreview{}, err
	}
	shares, workers := c.shares(def)
	previews := map[int]SharePreview{}
	for i, sh := range shares {
		if !Takes(sh, len(workers) > 1) {
			continue
		}
		p, drawn, err := workers[i].srv.PreviewSignups(files, sh.Tournament)
		if err != nil {
			shares[i].Error = err.Error()
			continue
		}
		previews[i] = SharePreview{Preview: p, PoolsDrawn: drawn}
	}
	return CombineSignups(def, files, shares, previews), nil
}

// SharePreview is one discipline's check of an import.
type SharePreview struct {
	Preview    signup.Preview
	PoolsDrawn bool
}

// CombineSignups is the disciplines' checks as one answer: per response, where it goes.
// previews are by index into shares. Shared with the browser demo.
func CombineSignups(def signup.Definition, files []signup.File, shares []SignupShare, previews map[int]SharePreview) EventSignupPreview {
	out := EventSignupPreview{Rows: []EventSignupRow{}, Disciplines: shares}
	var each [][]signup.Row
	var who []int
	for i := range shares {
		sp, ok := previews[i]
		if !ok {
			continue
		}
		p := sp.Preview
		out.Disciplines[i].Adding, out.Disciplines[i].AddingStaff = p.Adding, p.AddingStaff
		out.Disciplines[i].Capacity, out.Disciplines[i].PoolsDrawn = p.Capacity, sp.PoolsDrawn
		out.Adding += p.Adding
		out.AddingStaff += p.AddingStaff
		each, who = append(each, p.Rows), append(who, i)
	}
	if len(each) == 0 {
		// Nobody takes anything; the files are still worth reading back to the organizer.
		p := signup.Check(def, "", files, nil, nil)
		for _, row := range p.Rows {
			if row.Verdict == signup.New || row.Verdict == signup.Staff {
				row.Verdict = signup.NotHere
			}
			out.Rows = append(out.Rows, EventSignupRow{Row: row})
		}
		return out
	}
	// Check gives one row per file, in order, so the disciplines' rows line up.
	for f := range each[0] {
		row := EventSignupRow{Row: each[0][f]}
		var repeat, already bool
		for k, rows := range each {
			v := rows[f].Verdict
			switch v {
			case signup.New, signup.Staff:
				row.Into = append(row.Into, RowShare{Discipline: shares[who[k]].Discipline, Name: shares[who[k]].Name, Verdict: v})
			case signup.Repeat:
				repeat = true
			case signup.Already:
				already = true
			}
		}
		switch {
		case len(row.Into) > 0:
			row.Verdict = signup.Staff
			for _, in := range row.Into {
				if in.Verdict == signup.New {
					row.Verdict = signup.New
				}
			}
		case repeat:
			row.Verdict = signup.Repeat
		case already:
			row.Verdict = signup.Already
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}

func (c *Coordinator) previewSignups(w http.ResponseWriter, r *http.Request) {
	files, err := readFiles(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p, err := c.PreviewSignups(files)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// importSignups gives every discipline its share. Each discipline re-checks its own share
// as it writes, as its own import always has, so a preview posted back is never trusted.
func (c *Coordinator) importSignups(w http.ResponseWriter, r *http.Request) {
	files, err := readFiles(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	def, err := c.eventDefinition()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	shares, workers := c.shares(def)
	added, addedStaff := 0, 0
	var failed []string
	for i, sh := range shares {
		if !Takes(sh, len(workers) > 1) {
			continue
		}
		a, s, err := workers[i].srv.ImportSignups(files, sh.Tournament)
		if err != nil {
			failed = append(failed, sh.Name+": "+err.Error())
			continue
		}
		added += a
		addedStaff += s
	}
	p, err := c.PreviewSignups(files)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.poke()
	out := map[string]any{"added": added, "addedStaff": addedStaff, "preview": p}
	if len(failed) > 0 {
		out["error"] = strings.Join(failed, "; ")
	}
	writeJSON(w, http.StatusOK, out)
}
