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

	// writeMu serialises writes. One organizer and at most four mats: a single lock is
	// simpler than anything cleverer and cannot be got wrong.
	writeMu sync.Mutex
}

// New builds the server. assets is the embedded web bundle; a nil value serves the API
// alone, which is what the Go tests use.
func New(st *store.Store, assets fs.FS) *Server {
	return &Server{
		store:  st,
		rules:  match.MSL(),
		limits: tournament.MVPLimits(),
		hub:    newHub(),
		assets: assets,
	}
}

// Handler wires the routes. Go 1.22 routing covers this workload; a framework buys
// nothing here.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", s.getState)
	mux.HandleFunc("GET /api/stream", s.stream)
	mux.HandleFunc("GET /api/export.json", s.exportJSON)
	mux.HandleFunc("GET /api/qr.png", s.qr)

	mux.HandleFunc("POST /api/competitors", s.addCompetitor)
	mux.HandleFunc("PATCH /api/competitors/{id}", s.patchCompetitor)
	mux.HandleFunc("DELETE /api/competitors/{id}", s.deleteCompetitor)

	mux.HandleFunc("PUT /api/tournament", s.putTournament)
	mux.HandleFunc("POST /api/tournament/pools", s.generatePools)

	mux.HandleFunc("GET /api/matches/{id}/events", s.getEvents)
	mux.HandleFunc("POST /api/matches/{id}/events", s.postEvents)

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

// publishState pushes the whole snapshot. Cheap at this size, and it means a display never
// has to reconcile a partial update against what it already had.
func (s *Server) publishState() {
	snap, err := s.snapshot()
	if err != nil {
		return
	}
	s.hub.publish(Update{Kind: "state", Data: snap})
}
