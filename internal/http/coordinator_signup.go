package httpapi

import (
	"net/http"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/signup"
)

// Signup for the whole event (proposal §8, phase 3): one file out, one folder back, and
// every discipline gets its share. Each share is decided by the same signup.Check a
// discipline runs on its own, so the rules are the ones #91 settled; what this adds is
// asking every discipline at once, and saying in one table where each response goes.
//
// What can be entered is the event's disciplines, by name (#142). The file used to list
// rows of the programme, and each discipline had to be matched to one; now a response
// names the discipline itself, by its address in the event, and there is nothing to match.

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
		starts, name = hall.FirstStarts(view.Programme), view.Name
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
	def := signup.BuildDefinition(ev, name, entries)
	if file, err := c.folder.Read(); err == nil {
		def.Moved = file.Aliases
	}
	return def, nil
}

// shares is every discipline's part. A discipline that cannot be read is listed with why,
// and takes nothing.
func (c *Coordinator) shares() ([]hall.SignupShare, []*worker) {
	workers := c.snapshotWorkers()
	out := make([]hall.SignupShare, 0, len(workers))
	for _, w := range workers {
		sh := hall.SignupShare{Discipline: w.slug, Name: w.slug}
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
	serveSignupFile(w, r, c.SignupDefinition, writeReady)
}

func (c *Coordinator) signupDefinition(w http.ResponseWriter, r *http.Request) {
	serveSignupFile(w, r, c.SignupDefinition, writeDefinition)
}

func (c *Coordinator) signupApp(w http.ResponseWriter, r *http.Request) {
	serveSignupFile(w, r, c.SignupDefinition, writeSignupApp)
}

func (c *Coordinator) signupPDF(w http.ResponseWriter, r *http.Request) {
	serveSignupFile(w, r, c.SignupDefinition, writeSignupPDF)
}

// PreviewSignups is what importing these files would do, across the event.
func (c *Coordinator) PreviewSignups(files []signup.File) (hall.EventSignupPreview, error) {
	def, err := c.SignupDefinition(false)
	if err != nil {
		return hall.EventSignupPreview{}, err
	}
	shares, workers := c.shares()
	previews := map[int]hall.SharePreview{}
	for i, sh := range shares {
		if sh.Error != "" {
			continue
		}
		p, drawn, err := workers[i].srv.PreviewSignups(files)
		if err != nil {
			shares[i].Error = err.Error()
			continue
		}
		previews[i] = hall.SharePreview{Preview: p, PoolsDrawn: drawn}
	}
	return hall.CombineSignups(def, files, shares, previews), nil
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
