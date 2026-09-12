package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// replaceEvents is the organizer's editor saving (design §7 item 1): the whole log,
// rewritten, with the previous version kept as a backup. Scores, standings and the
// bracket recompute from the log as they always do, and a score keeper client holding
// the match is told to reload it, so the correction reaches the mat as well as the
// tables.
func (s *Server) replaceEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var events []match.Event
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeMu.Lock()
	backup, err := s.store.ReplaceEvents(id, events)
	s.writeMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.hub.publish(Update{Kind: "log-replaced", Match: id})
	s.publishState()
	all, err := s.store.Events(id, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"backup": backup,
		"state":  match.Replay(s.rules, all),
	})
}

// backups lists what the editor has kept for a match, so the organizer can see that
// nothing was lost -- and knows what to open by hand if it has to come back.
func (s *Server) backups(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Backups(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}
