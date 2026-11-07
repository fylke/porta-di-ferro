package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/fylke/porta-di-ferro/internal/hall"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// EventInfo is the event a discipline is part of: where it reads the welcome, the
// programme, the wifi and the signup settings, and where an edit to them goes. Several
// disciplines share one, so it is typed once for the whole hall (proposal §7).
//
// A server without one -- the Go tests, mostly -- keeps the day in its own
// tournament.json, as every run did before events existed.
type EventInfo interface {
	EventInfo() (store.Event, error)
	SaveEventInfo(store.Event) error
	// Programme is the day as the plan has it, for the info sheet (#149).
	Programme() []hall.ProgrammeRow
}

// UseEvent puts this discipline inside an event. Called once, before it serves anything.
func (s *Server) UseEvent(e EventInfo) { s.event = e }

// tournament is the stored tournament with the event's day laid over it, which is what
// every reader of t.Event wants: the snapshot, the info sheet, the signup definition.
//
// Never save what this returns: a read-modify-write goes through s.store.Tournament(), so
// the event's copy is never written back into the discipline.
func (s *Server) tournament() (store.Tournament, error) {
	t, err := s.store.Tournament()
	if err != nil || s.event == nil {
		return t, err
	}
	// An event.json that does not parse leaves the day blank rather than the discipline
	// unreadable; the event reports the error itself (proposal §11).
	ev, _ := s.event.EventInfo()
	t.Event = ev
	// The staff are the event's (phase 5): this discipline's are those who work it.
	if s.staff != nil {
		if staff, err := s.staff.StaffFor(s.self().Slug); err == nil {
			t.Staff = staff
		}
	}
	return t, nil
}

// source is what snapshots are built from: the store, with the event laid over its
// tournament.
type source struct {
	*store.Store
	s *Server
}

func (src source) Tournament() (store.Tournament, error) { return src.s.tournament() }

func (s *Server) putEvent(w http.ResponseWriter, r *http.Request) {
	var in store.Event
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in, err := hall.CleanEvent(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if s.event == nil {
		s.writeMu.Lock()
		t, err := s.store.Tournament()
		if err == nil {
			t.Event = in
			err = s.store.SaveTournament(t)
		}
		s.writeMu.Unlock()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	if s.event != nil {
		// The admin screens send the whole day, as they always have, and the event takes
		// it. Saving it republishes every discipline, this one included.
		if err := s.event.SaveEventInfo(in); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		s.publishState()
	}
	writeJSON(w, http.StatusOK, in)
}
