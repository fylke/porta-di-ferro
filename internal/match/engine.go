package match

// Replay derives a match's state from its log.
//
// Two passes. The first collects the sequence numbers voided by undo records; the second
// applies everything that survives. That is why a correction is an append rather than a
// mutation, and why full history editing (Milestone 2) is a UI change rather than a data
// migration: the engine already skips any voided record, not merely the last one.
func Replay(r Ruleset, events []Event) State {
	undone := map[int]bool{}
	for _, e := range events {
		if e.Type == TypeUndo && e.Undo != nil {
			undone[e.Undo.Seq] = true
		}
	}

	var s State
	s.Pending = PendingNone
	s.Winner = ""
	s.Reason = ReasonNone

	// applied is the stack of exchanges that actually took effect. An exchange that was
	// voided by an undo never reaches it, and neither does one ignored because the match
	// was already over by rule — so the top of the stack is exactly what the Undo control
	// acts on, and it is never an exchange that changed nothing.
	var applied []int

	for _, e := range events {
		if e.Seq > s.LastSeq {
			s.LastSeq = e.Seq
		}
		if undone[e.Seq] {
			continue
		}
		switch e.Type {
		case TypeExchange:
			if applyExchange(r, &s, e) {
				applied = append(applied, e.Seq)
			}
		case TypeTimer:
			applyTimer(&s, e)
		case TypeUndo:
			// The void itself was recorded in the first pass; the record it names is
			// skipped above. All that is left here is to drop the dialog the voided
			// exchange had raised.
			s.Pending = PendingNone
		case TypeEnd:
			applyEnd(r, &s, e)
		}
	}
	if !s.Ended && len(applied) > 0 {
		s.UndoableSeq = applied[len(applied)-1]
	}
	return s
}

// applyExchange resolves one confirmed exchange.
func applyExchange(r Ruleset, s *State, e Event) bool {
	if s.Ended || e.Exchange == nil {
		return false
	}
	// After either cap the match is over by rule, and the only legal next records are
	// the end itself or an undo of the exchange that tripped it. Ignoring further
	// exchanges here keeps a stray write from moving a score off a recorded 0-8.
	// The time condition is different: play may legitimately continue.
	if s.Pending == PendingPointCap || s.Pending == PendingPenaltyCap {
		return false
	}
	s.ElapsedMS = e.ElapsedMS
	s.Exchanges++

	// Differential scoring: the difference between the two assessments is awarded, not
	// both values. A 2 against a 1 gives the winner 1 and the other nothing; a 2 against
	// a 2 gives nobody anything. Afterblows and doubles need no special handling — they
	// net out. Confirming with nothing selected nets zero, which is a real no-score
	// exchange and is counted above.
	net := e.Exchange.Red.Value - e.Exchange.Blue.Value
	if net > 0 {
		s.Red.Score += net
	} else if net < 0 {
		s.Blue.Score -= net
	}

	// Penalties are applied after the points, so a deduction lands on the score this
	// exchange produced.
	redLost := applyPenalty(r, &s.Red, e.Exchange.Red.Penalty)
	blueLost := applyPenalty(r, &s.Blue, e.Exchange.Blue.Penalty)

	// A penalty loss is recorded 0-8 against the warned competitor, whatever the running
	// score was, and earns them no match points.
	switch {
	case redLost && blueLost:
		// Both sides reaching the losing level in one exchange is only reachable through
		// Milestone 2 escalation. Record it as a double loss: nobody wins, nobody scores.
		s.Red.Score, s.Blue.Score = 0, 0
		s.pendEnd(PendingPenaltyCap)
		return true
	case redLost:
		s.Red.Score, s.Blue.Score = 0, r.ForfeitScore
		s.pendEnd(PendingPenaltyCap)
		return true
	case blueLost:
		s.Red.Score, s.Blue.Score = r.ForfeitScore, 0
		s.pendEnd(PendingPenaltyCap)
		return true
	}

	// The cap takes precedence over the time condition when one confirmation trips both.
	if s.Red.Score >= r.PointCap || s.Blue.Score >= r.PointCap {
		s.pendEnd(PendingPointCap)
		return true
	}
	if e.ElapsedMS >= r.MatchTimeMS {
		s.pendEnd(PendingFinalExchange)
		return true
	}
	s.Pending = PendingNone
	return true
}

// applyPenalty advances a competitor's penalty level and reports whether that took them
// to the losing level. The deduction fires when the level crosses the deduction level,
// so an immediate escalation from 0 straight to 2 still costs the point.
func applyPenalty(r Ruleset, c *Competitor, levels int) bool {
	if levels <= 0 {
		return false
	}
	before := c.Penalty
	c.Penalty += levels
	if before < r.PenaltyDeduction && c.Penalty >= r.PenaltyDeduction {
		c.Score--
		// Floored at zero. design §5 says "one point off" and does not say what happens
		// to a competitor who has none, and a negative score would feed straight into
		// two of the four ranking indices. Flagged as an open question; if MSL's answer
		// is that the opponent gains the point instead, it changes here and in the
		// TypeScript mirror, and the vectors move with it.
		if c.Score < 0 {
			c.Score = 0
		}
	}
	return c.Penalty >= r.PenaltyLoss
}

func (s *State) pendEnd(p Pending) { s.Pending = p }

func applyTimer(s *State, e Event) {
	if s.Ended || e.Timer == nil {
		return
	}
	s.ElapsedMS = e.ElapsedMS
	switch e.Timer.Action {
	case TimerStart, TimerResume:
		s.Running = true
	case TimerStop:
		s.Running = false
	}
}

func applyEnd(r Ruleset, s *State, e Event) {
	if e.End == nil {
		return
	}
	s.ElapsedMS = e.ElapsedMS
	s.Ended = true
	s.Running = false
	s.Pending = PendingNone
	s.Reason = e.End.Reason

	if e.End.Reason == ReasonForfeit {
		// A forfeit is recorded 0-8 and earns the forfeiter no match points.
		if e.End.Forfeiter == Red {
			s.Red = Competitor{Score: 0, Penalty: s.Red.Penalty}
			s.Blue.Score = r.ForfeitScore
			s.Winner = Blue
		} else {
			s.Blue = Competitor{Score: 0, Penalty: s.Blue.Penalty}
			s.Red.Score = r.ForfeitScore
			s.Winner = Red
		}
		s.NoMatchPoints = true
		return
	}

	s.NoMatchPoints = e.End.Reason == ReasonPenalty
	switch {
	case s.Red.Score > s.Blue.Score:
		s.Winner = Red
	case s.Blue.Score > s.Red.Score:
		s.Winner = Blue
	default:
		s.Winner = ""
	}
}
