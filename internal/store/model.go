// Package store is the on-disk database: local JSON files the organizer owns and can
// read (design decision 8). One directory per tournament.
//
//	competitors.json          registration and status
//	tournament.json           mats, pool constraints, generated pools and assignments
//	matches/<match_id>.ndjson the append-only exchange log, one event per line
//
// The two JSON files are written by atomic replace, never in place. The log is
// newline-delimited because appending a line is the cheapest durable write there is, and
// a truncated final line after a crash costs one event rather than the file.
//
// Derived state -- scores, standings, rankings -- is never stored here, only computed.
package store

// Competitor is a person entered in the tournament. Name and club only: picture, phone
// number and club crest are Milestone 3 (design §9, issue #1).
type Competitor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Club string `json:"club"`
	// Withdrawn voids this competitor's results as though they never entered. The
	// ranking divides by matches completed, which is what makes that work retroactively.
	Withdrawn bool `json:"withdrawn"`
}

// Match is one competitor against another, fixed at pool creation along with the colours.
type Match struct {
	ID   string `json:"id"`
	Pool int    `json:"pool"`
	// Order is the position within the pool's running order.
	Order int    `json:"order"`
	Mat   int    `json:"mat"`
	Red   string `json:"red"`
	Blue  string `json:"blue"`
}

// Pool is a group of competitors who each fence all the others once.
type Pool struct {
	Number      int      `json:"number"`
	Mat         int      `json:"mat"`
	Competitors []string `json:"competitors"`
	Matches     []Match  `json:"matches"`
}

// Tournament is the setup and the generated draw. Name, logo and discipline linkage are
// Milestone 3 (design §9, issue #2): one run of the application is one tournament under
// one hardcoded ruleset.
type Tournament struct {
	Mats        int `json:"mats"`
	MinPoolSize int `json:"minPoolSize"`
	MaxPoolSize int `json:"maxPoolSize"`
	// Seed makes the random-draw tie-break reproducible, so a standing can be explained
	// after the fact rather than being a fresh coin toss on every page load.
	Seed        int64  `json:"seed"`
	Pools       []Pool `json:"pools"`
	GeneratedAt string `json:"generatedAt,omitempty"`
	// Violations are the ordering constraints the generator could not satisfy. Reported
	// rather than guaranteed away (design §6 item 9).
	Violations []string `json:"violations,omitempty"`
}

// Defaults returns the MVP tournament setup: two mats, pools of four to seven.
func Defaults() Tournament {
	return Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7}
}
