package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// nextCompetitorID keeps ids short and readable -- c1, c2 -- because the organizer is
// expected to open competitors.json in a text editor, and that escape hatch is only real
// if what they find in there is legible. Numbering continues past the highest id ever
// used, so removing someone never hands their id to the next entrant.
func nextCompetitorID(existing []store.Competitor) string {
	highest := 0
	for _, c := range existing {
		if n, err := strconv.Atoi(strings.TrimPrefix(c.ID, "c")); err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("c%d", highest+1)
}

func (s *Server) addCompetitor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
		Club string `json:"club"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("a competitor needs a name"))
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	competitors, err := s.store.Competitors()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c := store.Competitor{
		ID:   nextCompetitorID(competitors),
		Name: in.Name,
		Club: strings.TrimSpace(in.Club),
	}
	competitors = append(competitors, c)
	if err := s.store.SaveCompetitors(competitors); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) patchCompetitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Name      *string `json:"name"`
		Club      *string `json:"club"`
		Withdrawn *bool   `json:"withdrawn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	competitors, err := s.store.Competitors()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	found := false
	for i := range competitors {
		if competitors[i].ID != id {
			continue
		}
		found = true
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" {
				// POST rejects an empty name, so PATCH has to as well. A blank one is
				// not a harmless record: it reaches the scoreboard and the printed pool
				// sheets, where nobody can tell who is meant to be fencing.
				writeErr(w, http.StatusBadRequest, fmt.Errorf("a competitor needs a name"))
				return
			}
			competitors[i].Name = name
		}
		if in.Club != nil {
			competitors[i].Club = strings.TrimSpace(*in.Club)
		}
		if in.Withdrawn != nil {
			// Withdrawal voids this competitor's results as though they never entered.
			// Nothing is deleted: the ranking stops counting them, and everyone else's
			// indices recompute because they divide by matches completed.
			competitors[i].Withdrawn = *in.Withdrawn
		}
	}
	if !found {
		writeErr(w, http.StatusNotFound, fmt.Errorf("no competitor %s", id))
		return
	}
	if err := s.store.SaveCompetitors(competitors); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) deleteCompetitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	competitors, err := s.store.Competitors()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(t.Pools) > 0 {
		// Once the draw exists, removing a competitor would leave matches pointing at
		// nobody. Withdrawal is the mechanism from here on, and it is the one the rules
		// describe anyway.
		writeErr(w, http.StatusConflict,
			fmt.Errorf("pools are drawn; withdraw the competitor instead of removing them"))
		return
	}
	out := make([]store.Competitor, 0, len(competitors))
	found := false
	for _, c := range competitors {
		if c.ID == id {
			found = true
			continue
		}
		out = append(out, c)
	}
	if !found {
		// Rewriting the same list and reporting success would hide a client bug, and it
		// disagrees with PATCH, which already 404s on an unknown id.
		writeErr(w, http.StatusNotFound, fmt.Errorf("no competitor %s", id))
		return
	}
	if err := s.store.SaveCompetitors(out); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
