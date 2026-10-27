package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// Score keeper handover (design §7 item 10).
//
// One writer per match still holds; the epoch is what makes that safe across a handover
// rather than an exception to it. A device claims a match before it writes. The claim
// carries an epoch, the device stamps every push with it, and a push stamped with an
// older epoch than the current claim is quarantined and shown to the organizer rather
// than appended -- or dropped.
//
//   - Graceful: the outgoing device flushes, then releases. The next claim finds no
//     writer and takes the match with no ceremony.
//   - Ungraceful: the device died. Its claim is still there, but its heartbeats are not.
//     The next claim is refused all the same, naming the device and saying it has gone
//     quiet, and the new device takes over by saying so, which bumps the epoch. Anything
//     the old device sends later -- it came back to life, or it was only unplugged -- is
//     set aside.
//   - Contested: the other device is alive. The claim is refused with who holds it, and
//     the new device may take over explicitly, which bumps the epoch just the same.
//
// Nothing is taken from a device without somebody asking for it (#150). A device that
// is only quiet -- a tablet whose screen went dark, a phone that put its tab to sleep --
// looks exactly like a dead one, and a match handed on silently because somebody else
// opened the mat would lose the next exchange its score keeper confirmed.
//
// A push with no epoch is an anonymous writer: the tests, a curl. It is accepted while
// no device holds the match and refused while one does. The guarantee is for devices
// that claim, and every score keeper client does.

// Header names the score keeper client stamps its pushes with.
const (
	headerClient = "X-Porta-Client"
	headerEpoch  = "X-Porta-Epoch"
)

type claimResult struct {
	Epoch int `json:"epoch"`
	// TookOverFrom is set when the claim displaced another device.
	TookOverFrom string `json:"tookOverFrom,omitempty"`
}

// claimMatch is POST /api/matches/{id}/claim. Must be called with writeMu held by
// nobody: it takes it.
func (s *Server) claimMatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Client string `json:"client"`
		Force  bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Client == "" {
		writeErr(w, http.StatusBadRequest, errors.New("a client id is required"))
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	writers, err := s.store.Writers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// The epoch only ever goes up, a release included: a device that held epoch 3 and
	// was released must not find the match back on epoch 3 under somebody else.
	current, held := writers[id]
	switch {
	case !held || current.Client == "":
		writers[id] = store.Writer{Client: in.Client, Epoch: current.Epoch + 1}
	case current.Client == in.Client:
		// Already ours: a reload, or a reconnect. Same epoch.
	case in.Force:
		writers[id] = store.Writer{Client: in.Client, Epoch: current.Epoch + 1}
	default:
		holder := s.holder(current.Client)
		msg := "another device is scoring this match"
		if !holder.Alive {
			msg = "another device holds this match and has not been heard from lately"
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": msg, "holder": holder})
		return
	}
	if err := s.store.SaveWriters(writers); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := claimResult{Epoch: writers[id].Epoch}
	if held && current.Client != "" && current.Client != in.Client {
		out.TookOverFrom = current.Client
	}
	writeJSON(w, http.StatusOK, out)
}

// holder is the device behind a claim, as the registry knows it now. A device the
// registry has never heard of -- the server restarted, and it has not said hello since --
// is named by nothing but its id, and is not alive.
func (s *Server) holder(client string) Client {
	c := s.presence.get(client)
	if c == nil {
		return Client{ID: client, Role: "scorekeeper"}
	}
	c.Alive = time.Since(c.LastSeen) < deadAfter
	return *c
}

// Claim is who holds a match, for the device that thinks it does.
type Claim struct {
	Client string `json:"client"`
	Epoch  int    `json:"epoch"`
	// Holder is the device, when one holds the match.
	Holder *Client `json:"holder,omitempty"`
}

// getClaim is GET /api/matches/{id}/claim. A score keeper asks it while the match is
// quiet, so a device that has been taken over stops before its next exchange rather
// than finding out by having it set aside.
func (s *Server) getClaim(w http.ResponseWriter, r *http.Request) {
	writers, err := s.store.Writers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	cur := writers[r.PathValue("id")]
	out := Claim{Client: cur.Client, Epoch: cur.Epoch}
	if cur.Client != "" {
		h := s.holder(cur.Client)
		out.Holder = &h
	}
	writeJSON(w, http.StatusOK, out)
}

// releaseClaim is DELETE /api/matches/{id}/claim: the graceful half.
func (s *Server) releaseClaim(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	client := r.URL.Query().Get("client")
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	writers, err := s.store.Writers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if current, held := writers[id]; held && current.Client == client {
		// Released, not deleted: the epoch stays, so the next claim goes above it.
		writers[id] = store.Writer{Client: "", Epoch: current.Epoch}
		if err := s.store.SaveWriters(writers); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// releaseAllClaims drops every match a client holds. Called with writeMu held.
func (s *Server) releaseAllClaims(client string) {
	writers, err := s.store.Writers()
	if err != nil {
		return
	}
	changed := false
	for id, w := range writers {
		if w.Client == client {
			writers[id] = store.Writer{Client: "", Epoch: w.Epoch}
			changed = true
		}
	}
	if changed {
		_ = s.store.SaveWriters(writers)
	}
}

// errHeld refuses an anonymous push to a match a device is scoring.
var errHeld = errors.New("a device is scoring this match: claim it before writing to it")

// admit decides whether a push may be appended. Called with writeMu held. A stale push
// is quarantined here and reported back as such; the caller only has to stop.
func (s *Server) admit(r *http.Request, id string, events []match.Event) (stale bool, err error) {
	writers, err := s.store.Writers()
	if err != nil {
		return false, err
	}
	current, held := writers[id]
	epochHeader := r.Header.Get(headerEpoch)
	if epochHeader == "" {
		if held && current.Client != "" {
			return false, errHeld
		}
		return false, nil
	}
	epoch, err := strconv.Atoi(epochHeader)
	if err != nil {
		return false, err
	}
	client := r.Header.Get(headerClient)
	if !held || (current.Client == client && current.Epoch == epoch) {
		return false, nil
	}
	if current.Client == "" && current.Epoch == epoch {
		// The device that released it is finishing its flush. Its epoch is still the
		// latest, so nothing has been handed to anyone else in between.
		return false, nil
	}
	// Either an older epoch, or the right epoch from the wrong device -- which cannot
	// happen through the API, but a forged header should not become a way in.
	name := client
	if c := s.presence.get(client); c != nil {
		name = c.Name
	}
	if err := s.store.Quarantine(id, client, name, events); err != nil {
		return false, err
	}
	s.publishPresence()
	return true, nil
}
