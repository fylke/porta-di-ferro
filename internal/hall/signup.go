package hall

// Signup for the whole event, as the pages read it: what the disciplines' checks come to
// together, and the participant's app with the event written into it. The rules about
// importing are internal/signup's.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

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

// SignupReadyView is what the signup panel is told before anything goes out.
func SignupReadyView(def signup.Definition) map[string]any {
	return map[string]any{
		"missing":     signup.Ready(def),
		"tournaments": def.Tournaments,
		"filename":    SignupAppFilename(def),
		"definition":  DefinitionFilename(def),
	}
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

// definitionMarker is the line in the app that carries the event. Kept as an exact
// string so a change to the app that loses it fails a test rather than silently shipping
// a file that asks every participant for a definition they were never sent.
const definitionMarker = `<script id="definition" type="application/json">null</script>`

// BakeSignupApp writes an event into a copy of the participant app, which is what makes
// the download one file instead of two.
func BakeSignupApp(page []byte, def signup.Definition) ([]byte, error) {
	if !bytes.Contains(page, []byte(definitionMarker)) {
		return nil, fmt.Errorf("the signup app has no definition slot to fill")
	}
	body, err := json.Marshal(def)
	if err != nil {
		return nil, err
	}
	// The JSON lands inside a <script> element, so the one sequence that could end that
	// element early has to go. Escaping the slash keeps it valid JSON and keeps "</script>"
	// from ever appearing in the text.
	body = bytes.ReplaceAll(body, []byte("</"), []byte(`<\/`))
	replacement := `<script id="definition" type="application/json">` + string(body) + `</script>`
	return bytes.Replace(page, []byte(definitionMarker), []byte(replacement), 1), nil
}

func DefinitionFilename(def signup.Definition) string {
	return "signup-" + EventSlug(def) + ".json"
}

func SignupAppFilename(def signup.Definition) string {
	return "signup-" + EventSlug(def) + ".html"
}

func EventSlug(def signup.Definition) string {
	for _, candidate := range []string{def.DefinitionID, def.Event.Name} {
		if slug := signup.Slug(candidate); slug != "" {
			return slug
		}
	}
	return "event"
}

// WithoutStaff is the staff list less one person, and whether they were in it.
func WithoutStaff(staff []store.StaffMember, id string) ([]store.StaffMember, bool) {
	out := make([]store.StaffMember, 0, len(staff))
	found := false
	for _, m := range staff {
		if m.ID == id {
			found = true
			continue
		}
		out = append(out, m)
	}
	return out, found
}
