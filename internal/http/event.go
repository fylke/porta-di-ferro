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

func (s *Server) putEvent(w http.ResponseWriter, r *http.Request) {
	var in store.Event
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	in.Welcome = strings.TrimSpace(in.Welcome)
	if len([]rune(in.Welcome)) > maxWelcome {
		writeErr(w, http.StatusBadRequest,
			fmt.Errorf("the welcome message is longer than %d characters", maxWelcome))
		return
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
		writeErr(w, http.StatusBadRequest,
			fmt.Errorf("a schedule of more than %d items is not one anybody reads off a wall", maxSchedule))
		return
	}
	in.Schedule = kept

	in.Wifi.SSID = strings.TrimSpace(in.Wifi.SSID)
	if in.Wifi.Security != "WEP" && in.Wifi.Security != "nopass" {
		in.Wifi.Security = "WPA"
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	t, err := s.store.Tournament()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	t.Event = in
	if err := s.store.SaveTournament(t); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.publishState()
	writeJSON(w, http.StatusOK, t.Event)
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
