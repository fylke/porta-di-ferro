package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The day around the tournament (issue #98): the welcome message, the agenda and the
// venue wifi. Written from the admin view, read by the participant landing page and the
// printed info sheet, and touching no result anywhere.

// maxWelcome is a limit on the welcome message rather than a design for one. It is there
// so a paste accident cannot put a megabyte of text through every snapshot to every
// display on the LAN; a few paragraphs is what it is for.
const maxWelcome = 4000

// maxSchedule caps the agenda. A day with more than this many items is not a schedule
// anybody reads off a wall.
const maxSchedule = 40

// EventInfo is the event a discipline is part of: where it reads the welcome, the
// programme, the wifi and the signup settings, and where an edit to them goes. Several
// disciplines share one, so it is typed once for the whole hall (proposal §7).
//
// A server without one -- the Go tests, mostly -- keeps the day in its own
// tournament.json, as every run did before events existed.
type EventInfo interface {
	EventInfo() (store.Event, error)
	SaveEventInfo(store.Event) error
}

// UseEvent puts this discipline inside an event. Called once, before it serves anything.
func (s *Server) UseEvent(e EventInfo) { s.event = e }

// tournament is the stored tournament with the event's day laid over it, which is what
// every reader of t.Event wants: the snapshot, the info sheet, the signup definition.
//
// Only which programme row this discipline is -- Signup.Tournament -- is the
// discipline's own. Never save what this returns: a read-modify-write goes through
// s.store.Tournament(), so the event's copy is never written back into the discipline.
func (s *Server) tournament() (store.Tournament, error) {
	t, err := s.store.Tournament()
	if err != nil || s.event == nil {
		return t, err
	}
	mine := t.Event.Signup.Tournament
	// An event.json that does not parse leaves the day blank rather than the discipline
	// unreadable; the event reports the error itself (proposal §11).
	ev, _ := s.event.EventInfo()
	ev.Signup.Tournament = mine
	t.Event = ev
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
	in, err := CleanEvent(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	s.writeMu.Lock()
	t, err := s.store.Tournament()
	if err != nil {
		s.writeMu.Unlock()
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if s.event == nil {
		t.Event = in
	} else {
		// The admin screens send the whole day, as they always have. The event takes it,
		// and the discipline keeps only which programme row it is.
		t.Event = store.Event{Signup: store.Signup{Tournament: in.Signup.Tournament}}
	}
	err = s.store.SaveTournament(t)
	s.writeMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if s.event != nil {
		shared := in
		shared.Signup.Tournament = ""
		// Saving it republishes every discipline, this one included.
		if err := s.event.SaveEventInfo(shared); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		s.publishState()
	}
	writeJSON(w, http.StatusOK, in)
}

// CleanEvent trims what the admin view sent and holds it to the limits above. Exported
// for internal/demo, which answers the same call out of memory and must not accept
// anything this would refuse.
func CleanEvent(in store.Event) (store.Event, error) {
	in.Welcome = strings.TrimSpace(in.Welcome)
	if len([]rune(in.Welcome)) > maxWelcome {
		return in, fmt.Errorf("the welcome message is longer than %d characters", maxWelcome)
	}

	// Blank rows are how a row is deleted from the editor, so they are dropped rather
	// than refused: the organizer clears the label and it goes.
	kept := make([]store.ScheduleItem, 0, len(in.Schedule))
	for _, item := range in.Schedule {
		item.At = strings.TrimSpace(item.At)
		item.Label = strings.TrimSpace(item.Label)
		if item.Label == "" {
			continue
		}
		if item.Kind != "discipline" && item.Kind != "break" {
			item.Kind = ""
		}
		kept = append(kept, item)
	}
	if len(kept) > maxSchedule {
		return in, fmt.Errorf("a schedule of more than %d items is not one anybody reads off a wall", maxSchedule)
	}
	in.Schedule = kept

	in.Wifi.SSID = strings.TrimSpace(in.Wifi.SSID)
	if in.Wifi.Security != "WEP" && in.Wifi.Security != "nopass" {
		in.Wifi.Security = "WPA"
	}
	return in, nil
}

// WifiQR is the payload a phone's camera understands as "join this network". The format
// is not ours: it is what every QR scanner has implemented since Android shipped it, and
// the escaping rules below are part of it.
//
// Returns "" when there is no network to join, so a caller shows nothing rather than a
// code that does nothing.
func WifiQR(wifi store.Wifi) string {
	if strings.TrimSpace(wifi.SSID) == "" {
		return ""
	}
	security := wifi.Security
	if security == "" {
		security = "WPA"
	}
	// Backslash, semicolon, comma, colon and quote are the separators of the format, so
	// a password containing one has to escape it. A password with a semicolon in it is
	// exactly the kind of thing that gets discovered at a venue.
	escape := func(v string) string {
		r := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)
		return r.Replace(v)
	}
	out := fmt.Sprintf("WIFI:T:%s;S:%s;", security, escape(wifi.SSID))
	if security != "nopass" {
		out += fmt.Sprintf("P:%s;", escape(wifi.Password))
	}
	if wifi.Hidden {
		out += "H:true;"
	}
	return out + ";"
}
