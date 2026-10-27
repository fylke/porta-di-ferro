package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/web"
)

// Signup for the whole event (proposal §8, phase 3): one file out, one folder back, and
// every discipline gets its share. Each share is decided by the same signup.Check a
// discipline runs on its own, so the rules are the ones #91 settled; what this adds is
// asking every discipline at once, and saying in one table where each response goes.
//
// What can be entered is the event's disciplines, by name (#142). The file used to list
// rows of the programme, and each discipline had to be matched to one; now a response
// names the discipline itself, by its address in the event, and there is nothing to match.

// SignupShare is one discipline's part in the event's signup.
type SignupShare struct {
	Discipline  string   `json:"discipline"`
	Name        string   `json:"name"`
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

// SignupDefinition is the event's signup definition (SignupEvent, for the disciplines):
// the day from event.json, and every discipline with a name. withTimes adds when each is
// planned to start, from the forecast -- for the file that goes out, never for a check,
// which a discipline makes holding its own lock and the forecast would ask it again.
func (c *Coordinator) SignupDefinition(withTimes bool) (signup.Definition, error) {
	ev, err := c.EventInfo()
	if err != nil {
		return signup.Definition{}, err
	}
	var starts map[string]string
	name := ""
	if withTimes {
		view := c.View()
		starts, name = FirstStarts(view.Programme), view.Name
	}
	var entries []signup.TournamentInfo
	for _, w := range c.snapshotWorkers() {
		if w.srv == nil {
			continue
		}
		t, err := w.srv.store.Tournament()
		if err != nil {
			continue
		}
		entries = append(entries, signup.TournamentInfo{
			ID: w.slug, Label: w.srv.Self().Name, Capacity: t.Capacity, StartsAt: starts[w.slug],
		})
	}
	return signup.BuildDefinition(ev, name, entries), nil
}

// FirstStarts is when each discipline is planned to start, "09:00", by slug. Shared with
// the browser demo.
func FirstStarts(programme []ProgrammeRow) map[string]string {
	first := map[string]time.Time{}
	for _, row := range programme {
		// When the fencing starts: not the gear check before it, nor a break.
		if row.Stage != "pools" && row.Stage != "eliminations" && row.Stage != "final" {
			continue
		}
		at, err := time.Parse(time.RFC3339, row.Start)
		if err != nil {
			continue
		}
		if was, ok := first[row.Discipline]; !ok || at.Before(was) {
			first[row.Discipline] = at
		}
	}
	out := map[string]string{}
	for slug, at := range first {
		out[slug] = at.Local().Format("15:04")
	}
	return out
}

// shares is every discipline's part. A discipline that cannot be read is listed with why,
// and takes nothing.
func (c *Coordinator) shares() ([]SignupShare, []*worker) {
	workers := c.snapshotWorkers()
	out := make([]SignupShare, 0, len(workers))
	for _, w := range workers {
		sh := SignupShare{Discipline: w.slug, Name: w.slug}
		if w.srv == nil {
			sh.Error = w.err.Error()
		} else {
			sh.Name = w.srv.Self().Name
		}
		out = append(out, sh)
	}
	return out, workers
}

func (c *Coordinator) signupReady(w http.ResponseWriter, r *http.Request) {
	def, err := c.SignupDefinition(true)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, SignupReadyView(def))
}

// SignupReadyView is what the signup panel is told before anything goes out.
func SignupReadyView(def signup.Definition) map[string]any {
	return map[string]any{
		"missing":     signup.Ready(def),
		"tournaments": def.Tournaments,
		"filename":    signupAppFilename(def),
		"definition":  definitionFilename(def),
	}
}

func (c *Coordinator) signupDefinition(w http.ResponseWriter, r *http.Request) {
	def, err := c.SignupDefinition(true)
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
	def, err := c.SignupDefinition(true)
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

// PreviewSignups is what importing these files would do, across the event.
func (c *Coordinator) PreviewSignups(files []signup.File) (EventSignupPreview, error) {
	def, err := c.SignupDefinition(false)
	if err != nil {
		return EventSignupPreview{}, err
	}
	shares, workers := c.shares()
	previews := map[int]SharePreview{}
	for i, sh := range shares {
		if sh.Error != "" {
			continue
		}
		p, drawn, err := workers[i].srv.PreviewSignups(files)
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
		p := signup.Check(def, "", files, nil, nil, nil)
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
	shares, workers := c.shares()
	added, addedStaff := 0, 0
	var failed []string
	for i, sh := range shares {
		if sh.Error != "" {
			continue
		}
		a, s, err := workers[i].srv.ImportSignups(files)
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
