package e2e

import (
	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// The shapes the API actually returns. Declared here rather than imported from the http
// package so the test is a client of the wire format, the way a browser is.
type matchView struct {
	store.Match
	State  match.State `json:"state"`
	Status string      `json:"status"`
}

type poolView struct {
	Number      int                   `json:"number"`
	Mat         int                   `json:"mat"`
	Competitors []string              `json:"competitors"`
	Matches     []matchView           `json:"matches"`
	Standings   []tournament.Standing `json:"standings"`
	Complete    bool                  `json:"complete"`
}

type snapshot struct {
	Competitors []store.Competitor `json:"competitors"`
	Tournament  store.Tournament   `json:"tournament"`
	Pools       []poolView         `json:"pools"`
	Ruleset     match.Ruleset      `json:"ruleset"`
	Mats        map[string]string  `json:"mats"`
	Dir         string             `json:"dir"`
}
