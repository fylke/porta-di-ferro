package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// Server ties the store, the engine and the web app together.
type Server struct {
	store  *store.Store
	rules  match.Ruleset
	limits tournament.Limits
	hub    *hub
	assets fs.FS

	addressCache addressCache
	// presence is the connected-client registry: score keepers and displays, alive or
	// not. In memory; a restart lets everyone register again.
	presence *presence
	stop     chan struct{}
	// identity is which discipline this is: its name, slug and folder.
	identity identity
	// event is the event this discipline is part of, or nil for a server on its own.
	event EventInfo
	// mats is the event's plan, when the discipline's work runs on the event's mats.
	mats EventMats


	// writeMu serialises writes. One organizer and at most four mats: a single lock is
	// simpler than anything cleverer and cannot be got wrong.
	writeMu sync.Mutex
}

// New builds the server for one discipline. assets is the embedded web bundle; a nil
// value serves the API alone, which is what the Go tests and the event coordinator use --
// the coordinator serves the bundle once for every discipline. self says which discipline
// this is; a name left empty is read from its tournament.json, where renaming it from the
// admin page keeps it (issue #80).
func New(st *store.Store, assets fs.FS, self Instance) *Server {
	return newServer(st, assets, self, nil)
}

// newServer is New with the registry of devices given: the event's, shared by every
// discipline in it, which the coordinator sweeps. Nil makes the server its own and starts
// its own sweep. Given here rather than swapped in after, so no goroutine of this server
// ever sees the registry change under it.
func newServer(st *store.Store, assets fs.FS, self Instance, shared *presence) *Server {
	self.Dir = st.Dir()
	if self.Name == "" {
		if t, err := st.Tournament(); err == nil {
			self.Name = t.Discipline
		}
	}
	if self.URL == "" {
		self.URL = "/"
		if self.Slug != "" {
			self.URL = "/d/" + self.Slug + "/"
		}
	}
	s := &Server{
		store:    st,
		rules:    match.MSL(),
		limits:   tournament.DefaultLimits(),
		hub:      newHub(),
		assets:   assets,
		presence: shared,
		stop:     make(chan struct{}),
	}
	s.identity.self = self
	if shared == nil {
		s.presence = newPresence()
		go s.sweepPresence(s.stop)
	}
	return s
}

// Close stops the background work. The tests call it; the product runs until the
// process ends.
func (s *Server) Close() { close(s.stop) }

// Handler wires the routes. Go 1.22 routing covers this workload; a framework buys
// nothing here.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", s.getState)
	mux.HandleFunc("GET /api/stream", s.stream)
	mux.HandleFunc("GET /api/export.json", s.exportJSON)
	mux.HandleFunc("GET /api/export.pdf", s.exportPDF)
	mux.HandleFunc("GET /api/qr.png", s.qr)
	mux.HandleFunc("GET /api/addresses", s.addresses)

	mux.HandleFunc("POST /api/competitors", s.addCompetitor)
	mux.HandleFunc("PATCH /api/competitors/{id}", s.patchCompetitor)
	mux.HandleFunc("DELETE /api/competitors/{id}", s.deleteCompetitor)

	mux.HandleFunc("PUT /api/event", s.putEvent)
	mux.HandleFunc("GET /api/signup/definition.json", s.signupDefinition)
	mux.HandleFunc("GET /api/signup/app.html", s.signupApp)
	mux.HandleFunc("GET /api/signup/ready", s.signupReady)
	mux.HandleFunc("POST /api/signup/preview", s.previewImport)
	mux.HandleFunc("POST /api/signup/import", s.confirmImport)
	mux.HandleFunc("DELETE /api/staff/{id}", s.deleteStaff)
	mux.HandleFunc("GET /api/info.pdf", s.exportInfoPDF)

	mux.HandleFunc("PUT /api/tournament", s.putTournament)
	mux.HandleFunc("POST /api/tournament/pools", s.generatePools)
	mux.HandleFunc("PATCH /api/tournament/pools/{number}", s.patchPool)
	mux.HandleFunc("POST /api/tournament/bracket", s.drawBracket)

	mux.HandleFunc("GET /api/matches/{id}/events", s.getEvents)
	mux.HandleFunc("POST /api/matches/{id}/events", s.postEvents)
	mux.HandleFunc("PUT /api/matches/{id}/events", s.replaceEvents)
	mux.HandleFunc("GET /api/matches/{id}/backups", s.backups)
	mux.HandleFunc("POST /api/matches/{id}/claim", s.claimMatch)
	mux.HandleFunc("DELETE /api/matches/{id}/claim", s.releaseClaim)

	mux.HandleFunc("GET /api/presence", s.getPresence)
	mux.HandleFunc("POST /api/clients/{id}", s.register)
	mux.HandleFunc("POST /api/clients/{id}/release", s.release)
	mux.HandleFunc("PUT /api/clients/{id}/target", s.assignDisplay)
	mux.HandleFunc("DELETE /api/quarantine/{id}", s.discardQuarantine)

	mux.HandleFunc("/", s.serveApp)
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// Snapshot is this discipline's whole derived picture, for the event coordinator.
func (s *Server) Snapshot() (Snapshot, error) { return s.snapshot() }

// Subscribe follows this discipline's updates, for the event coordinator. The channel is
// closed if the subscriber falls behind, as any stream's is; the caller subscribes again.
func (s *Server) Subscribe() (<-chan Update, func()) {
	ch := s.hub.subscribe()
	return ch, func() { s.hub.unsubscribe(ch) }
}

// PublishState tells every page on this discipline to fetch nothing and redraw: the
// event's day changed under it.
func (s *Server) PublishState() { s.publishState() }

// publishState pushes the whole snapshot. Cheap at this size, and it means a display never
// has to reconcile a partial update against what it already had.
func (s *Server) publishState() {
	snap, err := s.snapshot()
	if err != nil {
		return
	}
	s.hub.publish(Update{Kind: "state", Data: snap})
}
