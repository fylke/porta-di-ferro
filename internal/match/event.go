// Package match is the match engine: the hardcoded MSL ruleset from docs/design.md §5,
// as pure functions over an append-only event log.
//
// It is mirrored in TypeScript at web/src/lib/match/ (design decision 13). The two
// implementations are held together by the shared vectors in testdata/vectors/, which
// both test suites run. A vector that passes here and fails there fails the build.
package match

// Side identifies a competitor within a match. Red is always on the left of the score
// keeper view and blue always on the right; that mapping mirrors the mat and never moves.
type Side string

const (
	Red  Side = "red"
	Blue Side = "blue"
)

// EventType tags a log record. Every record a match can hold is one of these.
type EventType string

const (
	// TypeExchange is one confirmed exchange. Points and warnings alike arrive only
	// through a confirmation, so a single record carries both: atomic per-exchange
	// commit is a design property (design §4), not an implementation detail.
	TypeExchange EventType = "exchange"
	// TypeTimer is a clock action: start, stop for a time-out, or resume.
	TypeTimer EventType = "timer"
	// TypeUndo voids an earlier record by sequence number. History is never mutated;
	// a correction is appended, which is what makes full history editing a UI change
	// later rather than a data migration.
	TypeUndo EventType = "undo"
	// TypeEnd closes the match.
	TypeEnd EventType = "end"
)

// Reason records why a match ended.
type Reason string

const (
	ReasonNone     Reason = ""
	ReasonTime     Reason = "time"
	ReasonPointCap Reason = "point_cap"
	ReasonPenalty  Reason = "penalty"
	ReasonForfeit  Reason = "forfeit"
)

// Assessment is what the head referee announced for one competitor in one exchange.
//
// Value is the raw assessed value, not the resulting score: the log stays raw and the
// net is derived (design decision 5), so a match recorded under differential scoring
// stays fully interpretable under additive scoring with no data migration.
//
// Penalty is how many levels of penalty this exchange applies to that competitor.
// MVP only ever emits 0 or 1; immediate escalation (Milestone 2) is the same field
// carrying 2 or 3, which is why no separate penalty concept is needed.
type Assessment struct {
	Value   int `json:"value"`
	Penalty int `json:"penalty"`
}

// Exchange carries both competitors' assessments for one confirmation. Confirming with
// nothing selected yields a zero Exchange, which is a real no-score event and is logged.
type Exchange struct {
	Red  Assessment `json:"red"`
	Blue Assessment `json:"blue"`
}

// TimerAction is one of the three clock actions.
type TimerAction string

const (
	TimerStart  TimerAction = "start"
	TimerStop   TimerAction = "stop"
	TimerResume TimerAction = "resume"
)

// Timer is a clock action.
type Timer struct {
	Action TimerAction `json:"action"`
}

// Undo names the sequence number whose effect is voided.
type Undo struct {
	Seq int `json:"seq"`
}

// End closes the match. Forfeiter is set only when Reason is ReasonForfeit.
type End struct {
	Reason    Reason `json:"reason"`
	Forfeiter Side   `json:"forfeiter,omitempty"`
}

// Event is one line of a match's .ndjson log.
//
// The primary key is (Match, Seq), which is what makes a retried push idempotent and
// needs no deduplication logic anywhere (design §3).
//
// ElapsedMS is the match clock at the moment of the event. Storing it rather than
// recomputing from wall-clock timestamps is what keeps replay deterministic: derived
// state depends only on the log, never on when it is replayed.
type Event struct {
	Match     string    `json:"match"`
	Seq       int       `json:"seq"`
	Type      EventType `json:"type"`
	At        string    `json:"at"`
	ElapsedMS int64     `json:"elapsedMs"`
	Exchange  *Exchange `json:"exchange,omitempty"`
	Timer     *Timer    `json:"timer,omitempty"`
	Undo      *Undo     `json:"undo,omitempty"`
	End       *End      `json:"end,omitempty"`
}
