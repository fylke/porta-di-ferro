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

// BracketView is the eliminations with every slot that can be filled filled, the
// tournament's podium so far, and the overall ranking the seeds came from.
type BracketView struct {
	Matches  []MatchView       `json:"matches"`
	Podium   tournament.Podium `json:"podium"`
	Complete bool              `json:"complete"`
}

// Snapshot is everything an organizer view or a display needs in one response. Clients
// take this on load and then follow the SSE stream; a display that loses the server shows
// stale data rather than breaking.
type Snapshot struct {
	Competitors []store.Competitor `json:"competitors"`
	Tournament  store.Tournament   `json:"tournament"`
	Pools       []PoolView         `json:"pools"`
	// Overall is everyone ranked across the pools by the pool chain -- the seeding for
	// the eliminations, and meaningful once PoolsComplete.
	Overall       []tournament.Standing `json:"overall"`
	PoolsComplete bool                  `json:"poolsComplete"`
	// Bracket is the eliminations, once drawn.
	Bracket *BracketView  `json:"bracket,omitempty"`
	Ruleset match.Ruleset `json:"ruleset"`
	// Mats maps a mat number to the match it is currently running, or the next one due.
	Mats map[int]string `json:"mats"`
	Dir  string         `json:"dir"`
	// Instance is which run of the application this is, so every page can say which
	// discipline it belongs to when there is more than one.
	Instance Instance `json:"instance"`
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
		Instance:    s.instances.self,
	}

	// Every match's state, pools and bracket alike, replayed once and shared.
	all := map[string]match.State{}
	view := func(m store.Match) (MatchView, error) {
		events, err := s.store.Events(m.ID, 0)
		if err != nil {
			return MatchView{}, err
		}
		st := match.Replay(s.rules, events)
		all[m.ID] = st
		status := "pending"
		switch {
		case st.Ended:
			status = "complete"
		case len(events) > 0:
			status = "running"
		}
		v := MatchView{Match: m, State: st, Status: status, Options: match.OptionsOf(events)}
		if st.Running {
			if at, ok := s.store.LastEventAt(m.ID); ok {
				if since := time.Since(at).Milliseconds(); since > 0 {
					v.SinceMS = since
				}
			}
		}
		return v, nil
	}

	for _, p := range tournament.RunOrder(t) {
		states := map[string]match.State{}
		views := make([]MatchView, 0, len(p.Matches))
		complete := true
		for _, m := range p.Matches {
			v, err := view(m)
			if err != nil {
				return Snapshot{}, err
			}
			states[m.ID] = v.State
			if !v.State.Ended {
				complete = false
			}
			views = append(views, v)
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

	snap.Overall = tournament.Overall(s.rules, t, byID, all)
	snap.PoolsComplete = tournament.PoolsComplete(t, byID, all)

	// The eliminations: later rounds filled from the results that feed them, then
	// replayed like any other match. A slot that is not filled yet is not a match a mat
	// can run, so it stays out of the mat's queue until it is.
	if len(t.Bracket) > 0 {
		bv := &BracketView{Complete: true}
		for _, m := range tournament.Fill(t.Bracket, all) {
			v, err := view(m)
			if err != nil {
				return Snapshot{}, err
			}
			if !v.State.Ended {
				bv.Complete = false
			}
			bv.Matches = append(bv.Matches, v)
		}
		// Fill again now that the bracket's own states are known, so a semi-final
		// decided a moment ago already shows the finalists.
		filled := tournament.Fill(t.Bracket, all)
		for i := range bv.Matches {
			bv.Matches[i].Match = filled[i]
		}
		bv.Podium = tournament.Result(filled, all)
		snap.Bracket = bv
	}

	// A mat runs its pools in queue order, which is the order snap.Pools is already in:
	// when one finishes, that mat picks up its next -- and after the pools, the bracket
	// matches assigned to it, once both their competitors are known. But a live score
	// keeper registered on the mat knows better: it holds a finished match on screen until
	// Next match is pressed, and the displays should show that result for exactly as long.
	// So the score keeper's own match wins whenever there is one -- a bracket match
	// included, once it can be run.
	known := map[string]bool{}
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			known[m.ID] = true
		}
	}
	if snap.Bracket != nil {
		for _, m := range snap.Bracket.Matches {
			if m.Red != "" && m.Blue != "" {
				known[m.ID] = true
			}
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
		if snap.Mats[mat] != "" || snap.Bracket == nil {
			continue
		}
		for _, m := range snap.Bracket.Matches {
			if m.Mat == mat && m.Status != "complete" && m.Red != "" && m.Blue != "" {
				snap.Mats[mat] = m.ID
				break
			}
		}
	}
	return snap, nil
}
