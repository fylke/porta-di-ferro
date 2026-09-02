package match_test

import (
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"pgregory.net/rapid"
)

// genLog builds a plausible match log: a start, then a run of exchanges with values and
// penalties inside the ruleset's range. No undo records, so the invariants below can be
// stated over a monotonic history.
func genLog(t *rapid.T) []match.Event {
	n := rapid.IntRange(0, 30).Draw(t, "exchanges")
	events := []match.Event{{
		Seq: 1, Type: match.TypeTimer, ElapsedMS: 0,
		Timer: &match.Timer{Action: match.TimerStart},
	}}
	elapsed := int64(0)
	for i := 0; i < n; i++ {
		elapsed += int64(rapid.IntRange(1000, 20000).Draw(t, "gap"))
		events = append(events, match.Event{
			Seq:       i + 2,
			Type:      match.TypeExchange,
			ElapsedMS: elapsed,
			Exchange: &match.Exchange{
				Red: match.Assessment{
					Value:   rapid.IntRange(0, 2).Draw(t, "redValue"),
					Penalty: rapid.IntRange(0, 1).Draw(t, "redPenalty"),
				},
				Blue: match.Assessment{
					Value:   rapid.IntRange(0, 2).Draw(t, "blueValue"),
					Penalty: rapid.IntRange(0, 1).Draw(t, "bluePenalty"),
				},
			},
		})
	}
	return events
}

// TestNoExchangeMovesAScoreTooFar checks the ruleset's own ceiling: an exchange can add at
// most MaxValue to one competitor, and can only ever take a point off through a penalty.
// The exception is an exchange that reaches the losing penalty level, which rewrites the
// score to the recorded 0-8 by rule.
func TestNoExchangeMovesAScoreTooFar(t *testing.T) {
	rules := match.MSL()
	rapid.Check(t, func(t *rapid.T) {
		events := genLog(t)
		prev := match.Replay(rules, events[:1])
		for i := 2; i <= len(events); i++ {
			got := match.Replay(rules, events[:i])
			if got.Pending == match.PendingPenaltyCap && prev.Pending != match.PendingPenaltyCap {
				prev = got
				continue
			}
			for _, d := range []struct {
				name       string
				was, isNow int
			}{
				{"red", prev.Red.Score, got.Red.Score},
				{"blue", prev.Blue.Score, got.Blue.Score},
			} {
				delta := d.isNow - d.was
				if delta > rules.MaxValue {
					t.Fatalf("%s gained %d in one exchange, ruleset maximum is %d", d.name, delta, rules.MaxValue)
				}
				if delta < -1 {
					t.Fatalf("%s lost %d in one exchange; only a penalty deduction may lower a score, by one", d.name, -delta)
				}
			}
			prev = got
		}
	})
}

// TestPenaltyEscalationIsMonotonic checks that a level never falls. Warnings only ever
// escalate within a match; the count resets between matches, which is a different log.
func TestPenaltyEscalationIsMonotonic(t *testing.T) {
	rules := match.MSL()
	rapid.Check(t, func(t *rapid.T) {
		events := genLog(t)
		prev := match.Replay(rules, events[:1])
		for i := 2; i <= len(events); i++ {
			got := match.Replay(rules, events[:i])
			if got.Red.Penalty < prev.Red.Penalty || got.Blue.Penalty < prev.Blue.Penalty {
				t.Fatalf("penalty level fell: %+v then %+v", prev, got)
			}
			prev = got
		}
	})
}

// TestReplayIsDeterministic is the property the whole append-only design rests on: state
// is a function of the log alone.
func TestReplayIsDeterministic(t *testing.T) {
	rules := match.MSL()
	rapid.Check(t, func(t *rapid.T) {
		events := genLog(t)
		if match.Replay(rules, events) != match.Replay(rules, events) {
			t.Fatal("replaying the same log twice produced different state")
		}
	})
}

// TestScoreNeverGoesNegative guards the deduction floor. Recorded as a property because
// the ranking indices divide by it and a negative would quietly distort the standings.
func TestScoreNeverGoesNegative(t *testing.T) {
	rules := match.MSL()
	rapid.Check(t, func(t *rapid.T) {
		got := match.Replay(rules, genLog(t))
		if got.Red.Score < 0 || got.Blue.Score < 0 {
			t.Fatalf("negative score: %+v", got)
		}
	})
}
