package httpapi

import (
	"time"

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
	// SinceMS is how long ago this server saw the last event in the match, and is set
	// only while the clock is running.
	//
	// A display has no writer of its own, so its clock has to be placed rather than
	// started: State.ElapsedMS is the time at the last event, which may have been a
	// minute ago. Adding this to it, and then counting on from the moment the snapshot
	// arrived, is what makes a scoreboard opened mid-match agree with the mat instead of
	// restarting the clock from whenever the page happened to load.
	//
	// A duration rather than a timestamp, so a display whose own clock is wrong -- and a
	// spare screen's clock usually is -- never has to agree with the server about what
	// time it is.
	SinceMS int64 `json:"sinceMs,omitempty"`
	// Options is how the match is presented -- colours and display sides -- read from
	// the log by match.OptionsOf. Always present, defaults filled in.
	Options match.Options `json:"options"`
}

// PoolView is a pool with its matches and its live standings.
type PoolView struct {
	Number int `json:"number"`
	Mat    int `json:"mat"`
	// Sequence is the pool's place in its mat's queue; Overridden says the organizer put
	// it somewhere other than where the default mapping would, so the screens can show
	// that it was deliberate.
	Sequence    int                   `json:"sequence"`
	Overridden  bool                  `json:"overridden"`
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
// cached: a tournament is at most a few hundred matches, and a cache is a second source
// of truth.
//
// Pools come out in run order -- by mat, then by the organizer's queue -- rather than by
// number. Every consumer that filters pools by mat then has the mat's running order for
// free, which is what keeps the score keeper, the displays and the roster agreeing on
// which pool a mat picks up next.
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

	for _, p := range tournament.RunOrder(t) {
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
			view := MatchView{Match: m, State: st, Status: status, Options: match.OptionsOf(events)}
			if st.Running {
				if at, ok := s.store.LastEventAt(m.ID); ok {
					if since := time.Since(at).Milliseconds(); since > 0 {
						view.SinceMS = since
					}
				}
			}
			views = append(views, view)
		}
		snap.Pools = append(snap.Pools, PoolView{
			Number:      p.Number,
			Mat:         p.Mat,
			Sequence:    p.Sequence,
			Overridden:  tournament.Overridden(t, p.Number),
			Competitors: p.Competitors,
			Matches:     views,
			Standings:   tournament.Rank(s.rules, p, byID, states, t.Seed),
			Complete:    complete,
		})
	}

	// A mat runs its pools in queue order, which is the order snap.Pools is already in:
	// when one finishes, that mat picks up its next. But a live score keeper registered on
	// the mat knows better: it holds a finished match on screen until Next match is
	// pressed, and the displays should show that result for exactly as long. So the
	// score keeper's own match wins whenever there is one.
	known := map[string]bool{}
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			known[m.ID] = true
		}
	}
	for mat := 1; mat <= t.Mats; mat++ {
		snap.Mats[mat] = ""
		if sk := s.presence.scorekeeperOn(mat); sk != nil && known[sk.Match] {
			snap.Mats[mat] = sk.Match
			continue
		}
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
