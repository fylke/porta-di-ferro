package demo

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// The tournament a visitor arrives in the middle of.
//
// Nothing here is a hand-written snapshot. The competitors are a list of names, and
// everything after that -- the pools, the club spread, the running order, the colours,
// the mat assignment -- is produced by the same tournament.Generate a real event uses,
// and the results are real match logs replayed by the real engine. A fixture written out
// as JSON would have gone stale the first time the draw changed; this cannot, because it
// is not a record of the output, it is the input.
//
// The seed is fixed, so every visitor sees the same tournament and a screenshot in the
// README stays true.
const fixtureSeed = 20260922

// Where the visitor comes in.
//
// Mats 2 and 3 have finished their pools; mat 1 has a match under way and two more to
// come. That is deliberately not "the last few matches of the tournament", which is what
// stopping at the end of the overall running order gives you: the demo's most obvious
// link is Score keeper, the score keeper opens on a mat, and a mat with nothing left on
// it is a dead end. Mat 1 is the one everything points at, so mat 1 is the one still
// fencing.
//
// The eliminations are left undrawn so there is an arc to finish. Play the rest does it
// for anyone who does not want to score three matches by hand.
const liveMat = 1
const unplayedOnLiveMat = 3

// And one still to come on each of the others, so all three mat displays have something
// on them and /score/2 and /score/3 are not dead ends either. A hall of screens saying
// "no match up yet" is a poor advertisement for a thing whose whole job is telling a
// hall what is happening.
const unplayedOnOtherMats = 1

func fixture(rules match.Ruleset, limits tournament.Limits) ([]store.Competitor, store.Tournament, map[string][]match.Event) {
	competitors := roster()

	t := store.Defaults()
	t.Discipline = "Open steel Longsword"
	t.Mats = 3
	t.MinPoolSize = 5
	t.MaxPoolSize = 6
	t.Seed = fixtureSeed

	drawn, err := tournament.Generate(t, competitors, limits)
	if err != nil {
		// The roster and the limits are both in this file; a failure here is a bug in
		// this package rather than anything a visitor can cause.
		panic("demo fixture cannot be drawn: " + err.Error())
	}
	drawn.GeneratedAt = time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	// Each mat's queue, in the order that mat runs it.
	byMat := map[int][]store.Match{}
	var mats []int
	for _, p := range tournament.RunOrder(drawn) {
		if _, seen := byMat[p.Mat]; !seen {
			mats = append(mats, p.Mat)
		}
		byMat[p.Mat] = append(byMat[p.Mat], p.Matches...)
	}
	// Sorted, because the seed walks this list and a map's order is not stable between
	// runs. Without it every reset would draw a different tournament.
	sort.Ints(mats)

	logs := map[string][]match.Event{}
	seed := int64(fixtureSeed)
	for _, mat := range mats {
		queue := byMat[mat]
		leave := unplayedOnOtherMats
		if mat == liveMat {
			leave = unplayedOnLiveMat
		}
		stop := len(queue) - leave
		if stop < 0 {
			stop = 0
		}
		for _, m := range queue[:stop] {
			seed++
			logs[m.ID] = playMatch(rules, m.ID, seed, false)
		}
		// The first of the ones left on the live mat is under way: a couple of exchanges
		// in with the clock running, which is what every display and the score keeper
		// client are built around.
		if mat == liveMat && stop < len(queue) {
			seed++
			logs[queue[stop].ID] = partMatch(rules, queue[stop].ID, seed)
		}
	}

	return competitors, drawn, logs
}

// roster is the field: names and clubs that read as a Nordic HEMA event without being
// anybody in particular. Thirty-two entrants fill five pools of six and one of five at
// the sizes above, which is a plausible club open and enough for a full bracket.
func roster() []store.Competitor {
	entries := []struct{ name, club string }{
		{"Astrid Lindqvist", "MSL Linköping"},
		{"Björn Halvorsen", "Gotlands Fäktskola"},
		{"Clara Wikström", "MSL Linköping"},
		{"Dag Nyström", "Uppsala HEMA"},
		{"Elin Sørensen", "Oslo Fribryterlag"},
		{"Fredrik Ahlberg", "Gotlands Fäktskola"},
		{"Greta Mäkinen", "Helsinki Longsword"},
		{"Henrik Dahl", "Uppsala HEMA"},
		{"Ida Karlsson", "MSL Linköping"},
		{"Jonas Ek", "Malmö Svärdsgille"},
		{"Katrin Olsen", "Oslo Fribryterlag"},
		{"Lars Bergström", "Uppsala HEMA"},
		{"Maja Lund", "Malmö Svärdsgille"},
		{"Nils Åkerlund", "Gotlands Fäktskola"},
		{"Oda Jensen", "Oslo Fribryterlag"},
		{"Petter Sandström", "MSL Linköping"},
		{"Quintin Bergh", "Malmö Svärdsgille"},
		{"Rakel Virtanen", "Helsinki Longsword"},
		{"Sigrid Moen", "Oslo Fribryterlag"},
		{"Tobias Lindgren", "MSL Linköping"},
		{"Ulla Niemi", "Helsinki Longsword"},
		{"Viktor Sten", "Uppsala HEMA"},
		{"Wilma Bäck", "Gotlands Fäktskola"},
		{"Yrsa Thorsen", "Oslo Fribryterlag"},
		{"Zakarias Holm", "Malmö Svärdsgille"},
		{"Anneli Rask", "Helsinki Longsword"},
		{"Bo Kjellberg", "MSL Linköping"},
		{"Cecilia Norberg", "Uppsala HEMA"},
		{"Didrik Aas", "Oslo Fribryterlag"},
		{"Ellen Forsberg", "Gotlands Fäktskola"},
		{"Filip Rantanen", "Helsinki Longsword"},
		{"Gunilla Sjöberg", "Malmö Svärdsgille"},
	}
	out := make([]store.Competitor, len(entries))
	for i, e := range entries {
		out[i] = store.Competitor{ID: fmt.Sprintf("c%d", i+1), Name: e.name, Club: e.club}
	}
	return out
}

// playMatch writes a whole match's log: the clock started, a run of exchanges, and the
// end event the engine's pending state calls for.
//
// It builds a log rather than a result, which is the point. Every score, standing and
// tie-break in the demo is then replayed from it by match.Replay exactly as it would be
// from a log a score keeper wrote at a mat, so a visitor who opens the organizer's
// history editor finds a real match in there.
//
// The engine never ends a match on its own: it raises a Pending, and the score keeper
// answers it. This does what that score keeper would do, which is why the reason on the
// end event is the reason the engine asked about rather than one guessed from the score.
//
// decisive is for the eliminations, where a draw is not a result. Level at the final
// exchange, it keeps fencing -- which is the sudden death the client runs at a real
// event, arrived at the same way.
func playMatch(rules match.Ruleset, id string, seed int64, decisive bool) []match.Event {
	rng := rand.New(rand.NewSource(seed))
	events := []match.Event{{
		Match: id, Seq: 1, Type: match.TypeTimer, ElapsedMS: 0,
		Timer: &match.Timer{Action: match.TimerStart},
	}}

	elapsed := int64(0)
	// A bound rather than a condition: sudden death between two competitors who keep
	// trading doubles could otherwise circle for a long time in a browser tab.
	for exchanges := 0; exchanges < 60; exchanges++ {
		// Between twelve and thirty seconds of circling per exchange, which is about
		// what a longsword pool match looks like.
		elapsed += int64(12000 + rng.Intn(18000))
		events = append(events, match.Event{
			Match: id, Seq: len(events) + 1, Type: match.TypeExchange,
			ElapsedMS: elapsed, Exchange: exchange(rng, decisive),
		})

		st := match.Replay(rules, events)
		reason := match.ReasonNone
		switch st.Pending {
		case match.PendingPointCap:
			reason = match.ReasonPointCap
		case match.PendingPenaltyCap:
			reason = match.ReasonPenalty
		case match.PendingFinalExchange:
			// Sudden death: the head referee sends them out again rather than
			// recording a draw that the bracket cannot use.
			if decisive && st.Red.Score == st.Blue.Score {
				continue
			}
			reason = match.ReasonTime
		default:
			continue
		}
		return append(events, match.Event{
			Match: id, Seq: len(events) + 1, Type: match.TypeEnd,
			ElapsedMS: elapsed, End: &match.End{Reason: reason},
		})
	}

	// Ran out of patience. End it on time; a draw here is a draw, and a pool can hold
	// one.
	return append(events, match.Event{
		Match: id, Seq: len(events) + 1, Type: match.TypeEnd,
		ElapsedMS: elapsed, End: &match.End{Reason: match.ReasonTime},
	})
}

// partMatch is a match in progress: started, a few exchanges in, not ended. Its clock is
// left running, which is what puts a moving number on the mat displays.
func partMatch(rules match.Ruleset, id string, seed int64) []match.Event {
	full := playMatch(rules, id, seed, false)
	// Three exchanges in is far enough that the standings mean something and near enough
	// that a visitor can finish it in under a minute.
	cut := 4
	if len(full) < cut {
		cut = len(full)
	}
	return full[:cut]
}

// exchange is one confirmation: usually a clean hit to one side, sometimes a double,
// sometimes nothing at all, and once in a while a warning. The mix is what makes the
// standings interesting -- if every exchange scored, every match would end at the cap in
// the same four exchanges and the tie-break chain would never be exercised.
func exchange(rng *rand.Rand, decisive bool) *match.Exchange {
	ex := &match.Exchange{}
	n := rng.Intn(100)
	// In sudden death a no-score exchange only prolongs things, and a warning could
	// decide a final on a technicality. Both are real, and neither is what a demo should
	// spend a visitor's patience on.
	if decisive && n >= 76 {
		n = rng.Intn(76)
	}
	switch {
	case n < 38:
		ex.Red.Value = 1 + rng.Intn(2)
	case n < 76:
		ex.Blue.Value = 1 + rng.Intn(2)
	case n < 86:
		// A double: both land, and differential scoring nets them off.
		ex.Red.Value = 1 + rng.Intn(2)
		ex.Blue.Value = 1 + rng.Intn(2)
	case n < 92:
		// A no-score exchange, logged like any other.
	case n < 96:
		ex.Red.Penalty = 1
	default:
		ex.Blue.Penalty = 1
	}
	return ex
}
