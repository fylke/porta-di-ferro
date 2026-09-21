package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The connected-client registry: which devices are on the LAN right now, what each is
// doing, and whether it is still alive (design §7 items 4 and 10).
//
// A client announces itself with a stable id of its own choosing and then heartbeats;
// a client that stops is dead after a grace period and forgotten after a longer one. The
// registry is in memory only -- who is alive is a fact about now -- and everything that
// has to outlive a restart lives in the store: writer claims, and what each screen shows.
//
// Two things hang off it. Score keeper handover asks it whether the device that holds a
// match is still there before letting another take over. Server-assigned displays are a
// client whose role is "display" and whose target the organizer sets; and the mat's own
// idea of which match is up follows the score keeper registered on it, so the displays
// show a finished match's result for exactly as long as the score keeper holds it.

const (
	// deadAfter is how long without a heartbeat before a client is shown as gone and a
	// match it holds can be taken over without asking. Heartbeats are every five seconds.
	deadAfter = 15 * time.Second
	// forgetAfter is how long a dead client stays in the list, so an organizer can see a
	// screen that has just dropped rather than wonder where it went.
	forgetAfter = 10 * time.Minute
)

// Client is one connected device.
type Client struct {
	ID   string `json:"id"`
	Role string `json:"role"` // "scorekeeper" or "display"
	// Name is what the device calls itself -- "Tablet 7F3A" -- so an organizer can point
	// at a screen and know which row it is.
	Name string `json:"name"`
	// Mat and Match are the score keeper's: which mat it sits at, which match it is on.
	Mat   int    `json:"mat,omitempty"`
	Match string `json:"match,omitempty"`
	// Target is the display's: what it has been told to show, e.g. "mat/1", "mats",
	// "roster", "audience/2". Empty until the organizer assigns it.
	Target   string    `json:"target,omitempty"`
	LastSeen time.Time `json:"lastSeen"`
	Alive    bool      `json:"alive"`
}

type presence struct {
	mu      sync.Mutex
	clients map[string]*Client
}

func newPresence() *presence { return &presence{clients: map[string]*Client{}} }

// touch records a heartbeat, creating the client if it is new. Reports whether anything
// the snapshot depends on -- a score keeper's mat or match -- changed.
func (p *presence) touch(c Client) (changedMat bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	prev, ok := p.clients[c.ID]
	if !ok {
		c.LastSeen = now
		c.Alive = true
		p.clients[c.ID] = &c
		return c.Role == "scorekeeper"
	}
	changedMat = prev.Role == "scorekeeper" && (prev.Mat != c.Mat || prev.Match != c.Match || !prev.Alive)
	prev.Role, prev.Name, prev.Mat, prev.Match = c.Role, c.Name, c.Mat, c.Match
	if c.Target != "" {
		prev.Target = c.Target
	}
	prev.LastSeen = now
	prev.Alive = true
	return changedMat
}

func (p *presence) setTarget(id, target string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.clients[id]
	if !ok {
		return false
	}
	c.Target = target
	return true
}

func (p *presence) remove(id string) *Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.clients[id]
	delete(p.clients, id)
	return c
}

func (p *presence) get(id string) *Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.clients[id]; ok {
		copy := *c
		return &copy
	}
	return nil
}

// alive reports whether a client has been heard from recently enough to hold a match.
func (p *presence) alive(id string) bool {
	c := p.get(id)
	return c != nil && time.Since(c.LastSeen) < deadAfter
}

// scorekeeperOn is the live score keeper registered on a mat, if there is one. Two on
// one mat is a mistake somebody is about to notice; the most recently heard from wins
// until they do.
func (p *presence) scorekeeperOn(mat int) *Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	var best *Client
	for _, c := range p.clients {
		if c.Role != "scorekeeper" || c.Mat != mat || time.Since(c.LastSeen) >= deadAfter {
			continue
		}
		if best == nil || c.LastSeen.After(best.LastSeen) {
			best = c
		}
	}
	if best == nil {
		return nil
	}
	copy := *best
	return &copy
}

// list is every client, alive first and then by name, with Alive computed as of now.
func (p *presence) list() []Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Client, 0, len(p.clients))
	for _, c := range p.clients {
		copy := *c
		copy.Alive = time.Since(c.LastSeen) < deadAfter
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Alive != out[j].Alive {
			return out[i].Alive
		}
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// sweep marks clients dead and forgets the long gone. Reports whether anything changed,
// so the caller can publish only then.
func (p *presence) sweep() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	changed := false
	for id, c := range p.clients {
		since := time.Since(c.LastSeen)
		if since >= forgetAfter {
			delete(p.clients, id)
			changed = true
			continue
		}
		if alive := since < deadAfter; alive != c.Alive {
			c.Alive = alive
			changed = true
		}
	}
	return changed
}

// --- handlers ---------------------------------------------------------------------

// Presence is what the organizer view sees: everyone connected, and everything set aside.
type Presence struct {
	Clients     []Client            `json:"clients"`
	Quarantined []store.Quarantined `json:"quarantined"`
}

func (s *Server) presenceView() Presence {
	q, err := s.store.Quarantined()
	if err != nil {
		q = []store.Quarantined{}
	}
	return Presence{Clients: s.presence.list(), Quarantined: q}
}

func (s *Server) publishPresence() {
	s.hub.publish(Update{Kind: "presence", Data: s.presenceView()})
}

// sweepPresence runs for the life of the server, so a device that dies is seen to die
// without anyone asking.
func (s *Server) sweepPresence(stop <-chan struct{}) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if s.presence.sweep() {
				s.publishPresence()
				// A score keeper going quiet changes which match its mat is showing.
				s.publishState()
			}
		}
	}
}

func (s *Server) getPresence(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.presenceView())
}

// register is the heartbeat. A display gets back what it should show, which is the
// whole of server-assigned displays from the device's side: open /display, say hello
// every few seconds, render whatever comes back.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role   string `json:"role"`
		Name   string `json:"name"`
		Mat    int    `json:"mat"`
		Match  string `json:"match"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if in.Role != "scorekeeper" && in.Role != "display" {
		writeErr(w, http.StatusBadRequest, errors.New(`role must be "scorekeeper" or "display"`))
		return
	}
	c := Client{ID: r.PathValue("id"), Role: in.Role, Name: in.Name, Mat: in.Mat, Match: in.Match}
	if in.Role == "display" {
		// The assignment is the server's to remember, not the screen's.
		if displays, err := s.store.Displays(); err == nil {
			c.Target = displays[c.ID]
		}
	}
	changedMat := s.presence.touch(c)
	s.publishPresence()
	if changedMat {
		s.publishState()
	}
	writeJSON(w, http.StatusOK, s.presence.get(c.ID))
}

// release is a client going away on purpose. A score keeper's release also lets go of
// its match, which is the graceful half of handover: the next device claims without
// having to take anything over.
func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c := s.presence.remove(id)
	if c != nil && c.Role == "scorekeeper" {
		s.writeMu.Lock()
		s.releaseAllClaims(id)
		s.writeMu.Unlock()
	}
	s.publishPresence()
	s.publishState()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// assignDisplay is the organizer telling a screen what to show. Persisted, so a hall of
// screens survives the organizer's laptop rebooting.
func (s *Server) assignDisplay(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	id := r.PathValue("id")
	s.writeMu.Lock()
	displays, err := s.store.Displays()
	if err == nil {
		if in.Target == "" {
			delete(displays, id)
		} else {
			displays[id] = in.Target
		}
		err = s.store.SaveDisplays(displays)
	}
	s.writeMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.presence.setTarget(id, in.Target)
	s.publishPresence()
	writeJSON(w, http.StatusOK, map[string]string{"target": in.Target})
}

func (s *Server) discardQuarantine(w http.ResponseWriter, r *http.Request) {
	s.writeMu.Lock()
	err := s.store.DiscardQuarantine(r.PathValue("id"))
	s.writeMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.publishPresence()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
