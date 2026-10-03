package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
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
	t, err := s.tournament()
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
	t, err := s.tournament()
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
	t, err := s.tournament()
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

func readFiles(r *http.Request) ([]signup.File, error) {
	var in importRequest
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxImport)).Decode(&in); err != nil {
		return nil, err
	}
	files := make([]signup.File, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, signup.File{Source: f.Source, Body: []byte(f.Body)})
	}
	return files, nil
}

func (s *Server) readImport(r *http.Request) (signup.Preview, store.Tournament, error) {
	files, err := readFiles(r)
	if err != nil {
		return signup.Preview{}, store.Tournament{}, err
	}
	return s.checkSignups(files, nil)
}

// checkSignups is signup.Check over this discipline: as the programme row it is, or as
// the one the event says it is when mine is given.
func (s *Server) checkSignups(files []signup.File, mine *string) (signup.Preview, store.Tournament, error) {
	t, err := s.tournament()
	if err != nil {
		return signup.Preview{}, store.Tournament{}, err
	}
	competitors, err := s.store.Competitors()
	if err != nil {
		return signup.Preview{}, store.Tournament{}, err
	}
	def := signup.BuildDefinition(t)
	row := strings.TrimSpace(t.Event.Signup.Tournament)
	if mine != nil {
		row = *mine
	} else if rows, ok := s.event.(SignupRows); ok {
		// The discipline's own import, in an event: the row the event's import would give
		// it. With several disciplines and no row it would take everything (#126).
		var several bool
		row, several = rows.SignupRowFor(s.self().Slug)
		if several && row == "" {
			return signup.Preview{}, t, ErrNoSignupRow
		}
	}
	return signup.Check(def, row, files, competitors, t.Staff), t, nil
}

// SignupRows is the event saying which programme row a discipline takes, and whether the
// event has several disciplines.
type SignupRows interface {
	SignupRowFor(slug string) (row string, several bool)
}

// ErrNoSignupRow is a discipline's own import in an event of several, when no programme row
// is this discipline's: every response would be its own.
var ErrNoSignupRow = errors.New("no row of the programme is this discipline's, so every response would be imported here; " +
	"choose its row on the event's signup panel, or import there for every discipline at once")

// SignupRow is the programme row this discipline said it is, if it said.
func (s *Server) SignupRow() (string, error) {
	t, err := s.store.Tournament()
	return strings.TrimSpace(t.Event.Signup.Tournament), err
}

// SetSignupRow says which programme row this discipline is: whose entries it takes.
func (s *Server) SetSignupRow(id string) error {
	s.writeMu.Lock()
	t, err := s.store.Tournament()
	if err == nil {
		t.Event.Signup.Tournament = strings.TrimSpace(id)
		err = s.store.SaveTournament(t)
	}
	s.writeMu.Unlock()
	if err == nil {
		s.publishState()
	}
	return err
}

// PreviewSignups is what an import would do here, for the event's import.
func (s *Server) PreviewSignups(files []signup.File, mine string) (signup.Preview, bool, error) {
	p, t, err := s.checkSignups(files, &mine)
	return p, len(t.Pools) > 0, err
}

// ImportSignups is this discipline's share of the event's import.
func (s *Server) ImportSignups(files []signup.File, mine string) (added, addedStaff int, err error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.importSignups(files, &mine)
	return res.added, res.addedStaff, err
}

// previewImport says what would happen. It writes nothing, which is the whole point of
// it: an organizer holding a folder of forty files should be able to look before
// deciding.
func (s *Server) previewImport(w http.ResponseWriter, r *http.Request) {
	preview, t, err := s.readImport(r)
	if errors.Is(err, ErrNoSignupRow) {
		writeErr(w, http.StatusConflict, err)
		return
	}
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

// confirmImport writes the rows the preview called new: competitors to the register and
// staff to the tournament.
//
// It runs the check again over the same files rather than trusting a preview posted back
// to it: the competitor list may have moved between the two calls -- another import, a
// name typed in by hand -- and the second check is what keeps the duplicate rule true.
func (s *Server) confirmImport(w http.ResponseWriter, r *http.Request) {
	files, err := readFiles(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeMu.Lock()
	res, err := s.importSignups(files, nil)
	s.writeMu.Unlock()
	if errors.Is(err, ErrNoSignupRow) {
		writeErr(w, http.StatusConflict, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"added":      res.added,
		"addedStaff": res.addedStaff,
		"preview":    previewView{Preview: res.preview, PoolsDrawn: len(res.t.Pools) > 0},
	})
}

type imported struct {
	added, addedStaff int
	preview           signup.Preview
	t                 store.Tournament
}

// importSignups writes the rows the check calls new. The caller holds writeMu.
func (s *Server) importSignups(files []signup.File, mine *string) (imported, error) {
	preview, t, err := s.checkSignups(files, mine)
	if err != nil {
		return imported{}, err
	}
	competitors, err := s.store.Competitors()
	if err != nil {
		return imported{}, err
	}
	updated := signup.Import(preview, NextCompetitorID, competitors)
	// Each new entry is a person of the event's: the same one in every discipline the
	// response entered, by its submission id (phase 3).
	for i, c := range updated {
		if c.Person != "" {
			continue
		}
		if updated[i].Person, err = s.personFor(c.Name, c.Club, c.Signup, ""); err != nil {
			return imported{}, err
		}
	}
	addedStaff := 0
	if s.staff != nil {
		// Offers to work go on the event's staff, where one person is one member however
		// many disciplines they offered.
		if addedStaff, err = s.staff.AddStaff(s.self().Slug, preview.Rows); err != nil {
			return imported{}, err
		}
	} else if staff := signup.ImportStaff(preview, t.Staff); len(staff) > len(t.Staff) {
		addedStaff = len(staff) - len(t.Staff)
		// Saved onto the stored tournament, not the one read for the check, which has the
		// event's day laid over it and must not be written back here.
		stored, err := s.store.Tournament()
		if err != nil {
			return imported{}, err
		}
		stored.Staff = staff
		if err := s.store.SaveTournament(stored); err != nil {
			return imported{}, err
		}
	}
	if len(updated) != len(competitors) {
		if err := s.store.SaveCompetitors(updated); err != nil {
			return imported{}, err
		}
	}
	if len(updated) != len(competitors) || addedStaff > 0 {
		s.publishState()
	}
	return imported{added: len(updated) - len(competitors), addedStaff: addedStaff, preview: preview, t: t}, nil
}

// deleteStaff takes somebody off the staff: an import the organizer did not want, or a
// volunteer who can no longer come. Unlike a competitor they are in no match, so nothing
// else has to change and the draw does not stand in the way.
func (s *Server) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.staff != nil {
		found, err := s.staff.RemoveStaff(s.self().Slug, id)
		switch {
		case err != nil:
			writeErr(w, http.StatusInternalServerError, err)
		case !found:
			writeErr(w, http.StatusNotFound, fmt.Errorf("no staff member %s", id))
		default:
			s.publishState()
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	kept, found := WithoutStaff(t.Staff, id)
	if !found {
		writeErr(w, http.StatusNotFound, fmt.Errorf("no staff member %s", id))
		return
	}
	t.Staff = kept
	if err := s.store.SaveTournament(t); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
