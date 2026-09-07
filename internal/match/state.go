package match

// Pending is a decision the score keeper has to make before the match can go on. The
// engine raises it; the score keeper view turns it into a dialog (design §4).
type Pending string

const (
	PendingNone Pending = "none"
	// PendingFinalExchange: the clock is past the match time, so the head referee decides
	// whether this confirmation was the last one. End match / Continue one more exchange.
	PendingFinalExchange Pending = "final_exchange"
	// PendingPointCap: a competitor reached the cap. The rules end it, so the only
	// alternative is that the entry was a mistake. End match / Undo last exchange.
	PendingPointCap Pending = "point_cap"
	// PendingPenaltyCap: a competitor reached the losing penalty level. Same shape.
	PendingPenaltyCap Pending = "penalty_cap"
)

// Competitor is one side's derived state.
type Competitor struct {
	Score int `json:"score"`
	// Penalty is the level, not a count of warnings: 0 clean, 1 warning, 2 point
	// deduction, 3 match lost. Resets each match, which is why it lives here and not
	// on the tournament.
	Penalty int `json:"penalty"`
}

// State is everything derived from a match log. Nothing in it is ever stored — it is
// recomputed by replaying the log (design decision 5).
type State struct {
	Red  Competitor `json:"red"`
	Blue Competitor `json:"blue"`
	// Exchanges counts confirmed exchanges, no-score ones included.
	Exchanges int `json:"exchanges"`
	// ElapsedMS is the match clock as of the last event in the log. A live view adds
	// the wall-clock time since that event when Running is true; replay never does,
	// which is what keeps derived state independent of when it is replayed.
	ElapsedMS int64 `json:"elapsedMs"`
	Running   bool  `json:"running"`
	Ended     bool  `json:"ended"`
	// NoMatchPoints marks a loser who earns nothing rather than the usual 3 — a penalty
	// loss or a forfeit, per design §5.
	NoMatchPoints bool    `json:"noMatchPoints"`
	Reason        Reason  `json:"endReason"`
	Winner        Side    `json:"winner"`
	Pending       Pending `json:"pending"`
	// LastSeq is the highest sequence number seen, undone records included. The next
	// event a client writes uses LastSeq+1.
	LastSeq int `json:"lastSeq"`
	// UndoableSeq is the sequence number of the last exchange that is still standing,
	// or 0 when there is nothing to undo. MVP undo is one step deep.
	UndoableSeq int `json:"undoableSeq"`
}

// Flashing reports whether the clock should be flashing: ten seconds remain, and the
// match has not ended. Live views pass the wall-clock-adjusted elapsed time.
func (r Ruleset) Flashing(elapsedMS int64, ended bool) bool {
	return !ended && elapsedMS >= r.FinalWarningMS
}
