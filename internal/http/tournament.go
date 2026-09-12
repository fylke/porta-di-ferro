package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

func (s *Server) putTournament(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mats        int `json:"mats"`
		MinPoolSize int `json:"minPoolSize"`
		MaxPoolSize int `json:"maxPoolSize"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	t.Mats, t.MinPoolSize, t.MaxPoolSize = in.Mats, in.MinPoolSize, in.MaxPoolSize
	if err := s.store.SaveTournament(t); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) generatePools(w http.ResponseWriter, r *http.Request) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	competitors, err := s.store.Competitors()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Clearing the seed makes regenerating a genuinely new shuffle rather than the same
	// one again. Once drawn, the seed is kept so the tie-breaks stay explainable.
	t.Seed = 0
	t.Pools = nil
	drawn, err := tournament.Generate(t, competitors, s.limits)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	drawn.GeneratedAt = time.Now().Format(time.RFC3339)
	if err := s.store.SaveTournament(drawn); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, drawn)
}

// drawBracket is the cut: the eliminations drawn from the overall ranking once every
// pool match is in. Drawing again replaces the bracket, which is the same escape hatch
// drawing the pools again is, and carries the same warning in the organizer view.
func (s *Server) drawBracket(w http.ResponseWriter, r *http.Request) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !snap.PoolsComplete {
		writeErr(w, http.StatusBadRequest, errPoolsNotDone)
		return
	}
	t := snap.Tournament
	bracket, err := tournament.Bracket(t, snap.Overall)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	t.Bracket = bracket
	t.BracketAt = time.Now().Format(time.RFC3339)
	if err := s.store.SaveTournament(t); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, t)
}

// patchPool is the organizer's override of the mat assignment (design §7 item 8): move a
// pool to another mat, where it queues last, or step it up or down the queue of the mat it
// is on. Either way every view follows, because they all read the pools in run order.
func (s *Server) patchPool(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var in struct {
		Mat  int    `json:"mat"`
		Move string `json:"move"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	switch {
	case in.Mat > 0:
		t, err = tournament.MovePool(t, number, in.Mat)
	case in.Move == "up" || in.Move == "down":
		t, err = tournament.ReorderPool(t, number, in.Move == "up")
	default:
		err = errNoOverride
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SaveTournament(t); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	events, err := s.store.Events(r.PathValue("id"), after)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// postEvents is the write path. Idempotent on (match, seq): a retry is a no-op, which is
// what lets the client push after every confirmed exchange and never think about it again.
//
// The response carries the server's derived state so a client that has been offline can
// check itself against the source of truth without a second round trip.
func (s *Server) postEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var events []match.Event
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeMu.Lock()
	stale, err := s.admit(r, id, events)
	var written []match.Event
	if err == nil && !stale {
		written, err = s.store.Append(id, events)
	}
	s.writeMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if stale {
		// Another device holds this match now. The events are set aside for the
		// organizer, and the sender is told so it can stop -- and say so on screen.
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":       "another device has taken over this match",
			"quarantined": len(events),
		})
		return
	}
	if len(written) > 0 {
		s.hub.publish(Update{Kind: "events", Match: id, Data: written})
		s.publishState()
	}
	all, err := s.store.Events(id, 0)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	state := match.Replay(s.rules, all)
	writeJSON(w, http.StatusOK, map[string]any{
		"written": len(written),
		"state":   state,
		"lastSeq": state.LastSeq,
	})
}
