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

// Match is one competitor against another, fixed at pool creation along with the colours
// -- or, in the eliminations, fixed by the results that feed it.
type Match struct {
	ID string `json:"id"`
	// Pool is 0 for a bracket match.
	Pool int `json:"pool"`
	// Order is the position within the pool's running order, or the bracket's.
	Order int    `json:"order"`
	Mat   int    `json:"mat"`
	Red   string `json:"red"`
	Blue  string `json:"blue"`
	// Round and Slot place a bracket match: "quarter", "semi", "bronze" or "final", and
	// the match's number within its round. FeedRed and FeedBlue name the matches whose
	// results fill Red and Blue -- "winner:e-qf1", "loser:e-sf2" -- so a later round's
	// competitors are derived from the log rather than stored, and a corrected
	// quarter-final corrects the semi-final for free.
	Round    string `json:"round,omitempty"`
	Slot     int    `json:"slot,omitempty"`
	FeedRed  string `json:"feedRed,omitempty"`
	FeedBlue string `json:"feedBlue,omitempty"`
}

// Pool is a group of competitors who each fence all the others once.
type Pool struct {
	Number int `json:"number"`
	Mat    int `json:"mat"`
	// Sequence is the pool's place in its mat's queue. The generator sets it to the pool
	// number, so the default order is by number; the organizer's override moves it. A
	// file from before the field existed has zeros throughout, which reads the same way.
	Sequence    int      `json:"sequence,omitempty"`
	Competitors []string `json:"competitors"`
	Matches     []Match  `json:"matches"`
}

// Tournament is the setup and the generated draw. Name, logo and discipline linkage are
// Milestone 3 (design §9, issue #2): one run of the application is one tournament under
// one hardcoded ruleset.
type Tournament struct {
	// Discipline is what this run is called -- "Open steel Longsword", "Open Sabre" --
	// shown on every page so a hall with two runs going can tell them apart. Kept here
	// rather than only in the -name flag, so renaming it from the organizer page sticks
	// across a restart (issue #80).
	Discipline  string `json:"discipline,omitempty"`
	Mats        int    `json:"mats"`
	MinPoolSize int    `json:"minPoolSize"`
	MaxPoolSize int    `json:"maxPoolSize"`
	// ElimMats is how many mats the organizer wants the eliminations run on, or 0 to
	// take the suggestion. The pools spread over everything available because they run
	// for hours; a bracket of seven matches often finishes no sooner on three mats than
	// on two, and the third is a referee and a score keeper staffed for nothing
	// (issue #94). tournament.EliminationMatsFor resolves the two.
	ElimMats int `json:"elimMats,omitempty"`
	// Seed makes the random-draw tie-break reproducible, so a standing can be explained
	// after the fact rather than being a fresh coin toss on every page load.
	Seed        int64  `json:"seed"`
	Pools       []Pool `json:"pools"`
	GeneratedAt string `json:"generatedAt,omitempty"`
	// Violations are the ordering constraints the generator could not satisfy. Reported
	// rather than guaranteed away (design §6 item 9).
	Violations []string `json:"violations,omitempty"`
	// Bracket is the eliminations, in playing order, drawn once the pools are done.
	// First-round competitors are stored; later rounds are filled from results.
	Bracket   []Match `json:"bracket,omitempty"`
	BracketAt string  `json:"bracketAt,omitempty"`
}

// Defaults returns the MVP tournament setup: two mats, pools of four to seven.
func Defaults() Tournament {
	return Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7}
}
