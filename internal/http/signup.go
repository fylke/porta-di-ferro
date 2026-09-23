package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/web"
)

// Offline signup, the organizer's half (issue #91,
// docs/proposals/offline-signup.md).
//
// Out goes one HTML file with the event baked into it, which the organizer emails or
// copies onto a stick. Back come one JSON file per participant, which the organizer drops
// on the import screen. Nothing in between touches a network, and the decision about
// what an import would do is made by internal/signup, which is pure.
//
// Importing is two calls on purpose. The preview writes nothing and the confirm writes
// exactly the rows the preview called new, so what the organizer approved and what lands
// in the competitor list cannot come apart.

// maxImport caps one import. Forty participants is a big club open; this is room for
// several hundred and a guard against a folder of something else entirely.
const maxImport = 8 << 20

func (s *Server) signupDefinition(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	def := signup.BuildDefinition(t)
	if r.URL.Query().Get("download") != "" {
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename=%q`, definitionFilename(def)))
	}
	writeJSON(w, http.StatusOK, def)
}

// signupReady tells the admin screen what is still missing, so the organizer finds out
// before the file goes out rather than from a participant who cannot use it.
func (s *Server) signupReady(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	def := signup.BuildDefinition(t)
	writeJSON(w, http.StatusOK, map[string]any{
		"missing":     signup.Ready(def),
		"tournaments": def.Tournaments,
		"filename":    signupAppFilename(def),
		"definition":  definitionFilename(def),
	})
}

// signupApp serves the participant's app with this event written into it, so what the
// organizer sends out is a single file that opens from a downloads folder.
//
// The app works without this -- a bare copy asks for a definition file -- but one
// attachment beats two every time, and "open both of these, the second one from inside
// the first" is an instruction people get wrong.
func (s *Server) signupApp(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	def := signup.BuildDefinition(t)

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
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename=%q`, signupAppFilename(def)))
	}
	w.Write(baked)
}

// definitionMarker is the line in the app that carries the event. Kept as an exact
// string so a change to the app that loses it fails a test rather than silently shipping
// a file that asks every participant for a definition they were never sent.
const definitionMarker = `<script id="definition" type="application/json">null</script>`

// BakeSignupApp writes an event into a copy of the participant app, which is what makes
// the download one file instead of two.
func BakeSignupApp(page []byte, def signup.Definition) ([]byte, error) {
	return bakeDefinition(page, def)
}

func bakeDefinition(page []byte, def signup.Definition) ([]byte, error) {
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

func definitionFilename(def signup.Definition) string {
	return "signup-" + eventSlug(def) + ".json"
}

func signupAppFilename(def signup.Definition) string {
	return "signup-" + eventSlug(def) + ".html"
}

func eventSlug(def signup.Definition) string {
	for _, candidate := range []string{def.DefinitionID, def.Event.Name} {
		if slug := signup.Slug(candidate); slug != "" {
			return slug
		}
	}
	return "event"
}

// --- the import -----------------------------------------------------------------------

type importRequest struct {
	Files []struct {
		Source string `json:"source"`
		Body   string `json:"body"`
	} `json:"files"`
}

func (s *Server) readImport(r *http.Request) (signup.Preview, store.Tournament, error) {
	var in importRequest
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxImport)).Decode(&in); err != nil {
		return signup.Preview{}, store.Tournament{}, err
	}
	t, err := s.store.Tournament()
	if err != nil {
		return signup.Preview{}, store.Tournament{}, err
	}
	competitors, err := s.store.Competitors()
	if err != nil {
		return signup.Preview{}, store.Tournament{}, err
	}

	files := make([]signup.File, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, signup.File{Source: f.Source, Body: []byte(f.Body)})
	}
	def := signup.BuildDefinition(t)
	mine := strings.TrimSpace(t.Event.Signup.Tournament)
	return signup.Check(def, mine, files, competitors), t, nil
}

// previewImport says what would happen. It writes nothing, which is the whole point of
// it: an organizer holding a folder of forty files should be able to look before
// deciding.
func (s *Server) previewImport(w http.ResponseWriter, r *http.Request) {
	preview, t, err := s.readImport(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, previewView{Preview: preview, PoolsDrawn: len(t.Pools) > 0})
}

// previewView is the preview plus the one thing about it that is the server's business
// rather than internal/signup's.
//
// Importing after the draw is allowed, because typing a competitor in by hand after the
// draw is allowed and two rules for the same act would be worse than one. But somebody
// imported into a drawn tournament is in no pool until it is drawn again, and that is
// worth saying on the screen before the button is pressed rather than at a mat.
type previewView struct {
	signup.Preview
	PoolsDrawn bool `json:"poolsDrawn"`
}

// confirmImport writes the rows the preview called new.
//
// It runs the check again over the same files rather than trusting a preview posted back
// to it: the competitor list may have moved between the two calls -- another import, a
// name typed in by hand -- and the second check is what keeps the duplicate rule true.
func (s *Server) confirmImport(w http.ResponseWriter, r *http.Request) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	preview, t, err := s.readImport(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	competitors, err := s.store.Competitors()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	updated := signup.Import(preview, NextCompetitorID, competitors)
	if len(updated) != len(competitors) {
		if err := s.store.SaveCompetitors(updated); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		s.publishState()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"added":   len(updated) - len(competitors),
		"preview": previewView{Preview: preview, PoolsDrawn: len(t.Pools) > 0},
	})
}
