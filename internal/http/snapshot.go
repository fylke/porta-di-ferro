package httpapi

import (
	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// MatchView is one match with its derived state. Nothing here is stored: it is replayed
// from the log every time it is asked for (design decision 5).
type MatchView struct {
	store.Match
	State  match.State `json:"state"`
	Status string      `json:"status"`
}

// PoolView is a pool with its matches and its live standings.
type PoolView struct {
	Number      int                   `json:"number"`
	Mat         int                   `json:"mat"`
	Competitors []string              `json:"competitors"`
	Matches     []MatchView           `json:"matches"`
	Standings   []tournament.Standing `json:"standings"`
	Complete    bool                  `json:"complete"`
}

// Snapshot is everything an organizer view or a display needs in one response. Clients
// take this on load and then follow the SSE stream; a display that loses the server shows
// stale data rather than breaking.
type Snapshot struct {
	Competitors []store.Competitor `json:"competitors"`
	Tournament  store.Tournament   `json:"tournament"`
	Pools       []PoolView         `json:"pools"`
	Ruleset     match.Ruleset      `json:"ruleset"`
	// Mats maps a mat number to the match it is currently running, or the next one due.
	Mats map[int]string `json:"mats"`
	Dir  string         `json:"dir"`
}

// snapshot builds the whole derived picture. It is deliberately recomputed rather than
// cached: a tournament is at most 56 matches, and a cache is a second source of truth.
func (s *Server) snapshot() (Snapshot, error) {
	competitors, err := s.store.Competitors()
	if err != nil {
		return Snapshot{}, err
	}
	t, err := s.store.Tournament()
	if err != nil {
		return Snapshot{}, err
	}

	byID := make(map[string]store.Competitor, len(competitors))
	for _, c := range competitors {
		byID[c.ID] = c
	}

	snap := Snapshot{
		Competitors: competitors,
		Tournament:  t,
		Ruleset:     s.rules,
		Pools:       make([]PoolView, 0, len(t.Pools)),
		Mats:        map[int]string{},
		Dir:         s.store.Dir(),
	}

	for _, p := range t.Pools {
		states := map[string]match.State{}
		views := make([]MatchView, 0, len(p.Matches))
		complete := true
		for _, m := range p.Matches {
			events, err := s.store.Events(m.ID, 0)
			if err != nil {
				return Snapshot{}, err
			}
			st := match.Replay(s.rules, events)
			states[m.ID] = st
			status := "pending"
			switch {
			case st.Ended:
				status = "complete"
			case len(events) > 0:
				status = "running"
			}
			if !st.Ended {
				complete = false
			}
			views = append(views, MatchView{Match: m, State: st, Status: status})
		}
		snap.Pools = append(snap.Pools, PoolView{
			Number:      p.Number,
			Mat:         p.Mat,
			Competitors: p.Competitors,
			Matches:     views,
			Standings:   tournament.Rank(s.rules, p, byID, states, t.Seed),
			Complete:    complete,
		})
	}

	// A mat runs its pools in order: when one finishes, that mat picks up its next.
	for mat := 1; mat <= t.Mats; mat++ {
		snap.Mats[mat] = ""
		for _, p := range snap.Pools {
			if p.Mat != mat {
				continue
			}
			for _, m := range p.Matches {
				if m.Status != "complete" {
					snap.Mats[mat] = m.ID
					break
				}
			}
			if snap.Mats[mat] != "" {
				break
			}
		}
	}
	return snap, nil
}
