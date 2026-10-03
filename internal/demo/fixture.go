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

// Where the visitor comes in: about halfway through the pools, with each mat somewhere
// different, because mats never keep pace with each other in a real hall.
//
// Halfway, not nearly done. A tournament down to its last few matches shows standings
// that have stopped moving and fencers with nothing left to do; the middle is where a
// visitor sees what the views are for -- who has two matches left, who is on deck, a
// table half filled in. And every mat has a match up, because the demo's most obvious
// link is Score keeper, it opens on a mat, and a mat with nothing on it is a dead end.
//
// The eliminations are left undrawn so there is an arc to finish. Play the rest does it
// for anyone who does not want to score the rest by hand.
const liveMat = 1

// progress says how far each mat has got: into which of its pools, in running order, and
// what share of that pool's matches are fenced. Everything before that pool is finished.
//
//   - Mat 1, the live one everything points at: two thirds into its first pool, with the
//     next match under way. Its fencers have one or two matches left.
//   - Mat 2: first pool done, halfway into the second, where everyone has two left.
//   - Mat 3: between pools, the first done and the second about to start.
var progress = map[int]struct {
	pool  int
	share float64
}{
	1: {pool: 0, share: 2.0 / 3},
	2: {pool: 1, share: 0.5},
	3: {pool: 1, share: 0},
}

func fixture(rules match.Ruleset, limits tournament.Limits) ([]store.Competitor, store.Tournament, map[string][]match.Event) {
	competitors := roster()

	t := store.Defaults()
	t.Discipline = "Open steel Longsword"
	// The day around the fencing (issue #98). The landing page and the info sheet are
	// most of what a visitor to the demo sees first, and both are empty boxes without
	// this -- which would show the views working and the product looking unfinished.
	t.Event = store.Event{
		Welcome: "Welcome to Stångebroslaget.\n\n" +
			"Gear check opens at 08:30 by the entrance. Pools start at 09:30 on three " +
			"mats. Results and the programme update themselves on this page all day — " +
			"leave it open.",
		Schedule: []store.ScheduleItem{
			{At: "08:30", Label: "Gear check"},
			{At: "09:00", Label: "Staff briefing"},
			// The day in blocks (#136): the longsword, then the sabre and the sword and
			// buckler side by side, then every final, one after another.
			{At: "09:30", Label: "Longsword, pools", Kind: "discipline",
				Tournament: "longsword-pools", Capacity: 32},
			// The same discipline as the pools, so the signup offers the longsword once and
			// the event import has nobody to put in a row of its own (phase 3).
			{At: "11:30", Label: "Longsword, eliminations", Kind: "discipline", Tournament: "longsword-pools"},
			{At: "12:30", Ends: "13:15", Label: "Lunch", Kind: "break"},
			{At: "13:15", Label: "Sabre", Kind: "discipline", Tournament: "sabre-pools", Capacity: 16},
			{At: "13:15", Label: "Sword and buckler", Kind: "discipline", Tournament: "sword-and-buckler", Capacity: 16},
			{At: "16:00", Label: "Finals"},
			{At: "17:00", Label: "Prize giving"},
		},
		// A password with a separator in it, because that is the case the QR escaping
		// gets wrong and the demo is where anybody would notice.
		Wifi: store.Wifi{SSID: "Hall-Guest", Password: "fencing;2026", Security: "WPA"},
		// Offline signup (issue #91): configured, so a visitor can download the file
		// that goes out to participants and see what they would be sent.
		Signup: store.Signup{
			DefinitionID: "stangebroslaget-2026",
			Name:         "Stångebroslaget",
			Venue:        "Linköping",
			Date:         "2026-11-14",
			Tournament:   "longsword-pools",
		},
	}
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

	// Each mat's pools, in the order that mat runs them.
	byMat := map[int][]store.Pool{}
	var mats []int
	for _, p := range tournament.RunOrder(drawn) {
		if _, seen := byMat[p.Mat]; !seen {
			mats = append(mats, p.Mat)
		}
		byMat[p.Mat] = append(byMat[p.Mat], p)
	}
	// Sorted, because the seed walks this list and a map's order is not stable between
	// runs. Without it every reset would draw a different tournament.
	sort.Ints(mats)

	logs := map[string][]match.Event{}
	seed := int64(fixtureSeed)
	for _, mat := range mats {
		var queue []store.Match
		stop := 0
		at := progress[mat]
		for i, p := range byMat[mat] {
			switch {
			case i < at.pool:
				stop += len(p.Matches)
			case i == at.pool:
				stop += int(at.share * float64(len(p.Matches)))
			}
			queue = append(queue, p.Matches...)
		}
		stop = min(stop, len(queue))
		for _, m := range queue[:stop] {
			seed++
			logs[m.ID] = playMatch(rules, m.ID, seed, false)
		}
		// The first of the ones left on the live mat is under way: a couple of exchanges
		// in with the clock running, which is what every display and the score keeper
		// client are built around.
		var fenced []string
		for _, m := range queue[:stop] {
			fenced = append(fenced, m.ID)
		}
		if mat == liveMat && stop < len(queue) {
			seed++
			logs[queue[stop].ID] = partMatch(rules, queue[stop].ID, seed)
			fenced = append(fenced, queue[stop].ID)
		}
		stampMat(logs, fenced, time.Now())
	}

	return competitors, drawn, logs
}

// stampMat gives a mat's fenced matches the times they would have been logged at,
// counting back from now: the match under way a few seconds since its last exchange, and
// before it, match by match, about three quarters of a minute of stoppages on top of the
// fencing and a little over a minute between pairs. Real logs carry these times, and the
// forecast learns each mat's pace from them (phase 4); a fixture without them would have
// a day with no past.
func stampMat(logs map[string][]match.Event, ids []string, now time.Time) {
	cursor := now.Add(-90 * time.Second)
	if n := len(ids); n > 0 {
		if events := logs[ids[n-1]]; len(events) > 0 && events[len(events)-1].Type != match.TypeEnd {
			cursor = now.Add(-8 * time.Second)
		}
	}
	for i := len(ids) - 1; i >= 0; i-- {
		events := logs[ids[i]]
		if len(events) == 0 {
			continue
		}
		last := time.Duration(events[len(events)-1].ElapsedMS) * time.Millisecond
		ended := events[len(events)-1].Type == match.TypeEnd
		start := cursor.Add(-last)
		end := cursor
		if ended {
			start = cursor.Add(-last - 45*time.Second)
		}
		for j := range events {
			at := start.Add(time.Duration(events[j].ElapsedMS) * time.Millisecond)
			if events[j].Type == match.TypeEnd {
				at = end
			}
			events[j].At = at.UTC().Format("2006-01-02T15:04:05.000Z07:00")
		}
		cursor = start.Add(-70 * time.Second)
	}
}

// sabreFixture is the event's second discipline (#102): Open Sabre, its pools drawn and
// waiting for the afternoon, as the programme says. A demo of one event over several
// disciplines needs a second one there to show, and one that has not started is also the
// honest picture of a day at 11 o'clock.
//
// Three of its fencers are in the longsword as well -- Astrid, Bo and Greta -- because
// somebody entered in two disciplines is the normal case at a club open, and finding a
// name across the event is the thing a single-discipline page could never do. Astrid and
// Greta signed up for both on one response (signedUp), so the event knows each is one
// person; Bo was typed in at both desks, so he is offered to the organizer to merge.
func sabreFixture(rules match.Ruleset, limits tournament.Limits) ([]store.Competitor, store.Tournament, map[string][]match.Event) {
	entries := []struct{ name, club string }{
		{"Astrid Lindqvist", "MSL Linköping"},
		{"Bo Kjellberg", "MSL Linköping"},
		{"Greta Mäkinen", "Helsinki Longsword"},
		{"Hanna Strand", "Oslo Fribryterlag"},
		{"Isak Berg", "Uppsala HEMA"},
		{"Johanna Vik", "Malmö Svärdsgille"},
		{"Kalle Persson", "MSL Linköping"},
		{"Liv Haugen", "Oslo Fribryterlag"},
		{"Mikael Laine", "Helsinki Longsword"},
		{"Nora Ahl", "Gotlands Fäktskola"},
		{"Olof Grahn", "Uppsala HEMA"},
		{"Pia Lehto", "Helsinki Longsword"},
		{"Rasmus Holt", "Malmö Svärdsgille"},
		{"Saga Eriksson", "Gotlands Fäktskola"},
	}
	competitors := make([]store.Competitor, len(entries))
	for i, e := range entries {
		competitors[i] = store.Competitor{ID: fmt.Sprintf("c%d", i+1), Name: e.name, Club: e.club, Signup: signedUp[e.name]}
	}

	t := store.Defaults()
	t.Discipline = "Open Sabre"
	t.Event = store.Event{Signup: store.Signup{Tournament: "sabre-pools"}}
	t.Mats = 2
	t.MinPoolSize = 4
	t.MaxPoolSize = 7
	t.Seed = fixtureSeed + 1
	drawn, err := tournament.Generate(t, competitors, limits)
	if err != nil {
		panic("demo sabre fixture cannot be drawn: " + err.Error())
	}
	drawn.GeneratedAt = time.Now().Add(-90 * time.Minute).Format(time.RFC3339)
	return competitors, drawn, map[string][]match.Event{}
}

// bucklerFixture is the third discipline (#136): Sword and buckler, drawn and run beside
// the sabre in the afternoon block, after the longsword. Its fencers are none of the
// sabre's, so the two can run side by side; the longsword's may be in it, because the
// longsword is done by then.
func bucklerFixture(rules match.Ruleset, limits tournament.Limits) ([]store.Competitor, store.Tournament, map[string][]match.Event) {
	entries := []struct{ name, club string }{
		{"Clara Wikström", "MSL Linköping"},
		{"Dag Nyström", "Uppsala HEMA"},
		{"Elin Sørensen", "Oslo Fribryterlag"},
		{"Fredrik Ahlberg", "Gotlands Fäktskola"},
		{"Ida Karlsson", "MSL Linköping"},
		{"Jonas Ek", "Malmö Svärdsgille"},
		{"Katrin Olsen", "Oslo Fribryterlag"},
		{"Lars Bergström", "Uppsala HEMA"},
		{"Maja Lund", "Malmö Svärdsgille"},
		{"Rakel Virtanen", "Helsinki Longsword"},
	}
	competitors := make([]store.Competitor, len(entries))
	for i, e := range entries {
		competitors[i] = store.Competitor{ID: fmt.Sprintf("c%d", i+1), Name: e.name, Club: e.club, Signup: signedUp[e.name]}
	}
	t := store.Defaults()
	t.Discipline = "Sword and buckler"
	t.Event = store.Event{Signup: store.Signup{Tournament: "sword-and-buckler"}}
	t.Mats = 2
	t.MinPoolSize = 4
	t.MaxPoolSize = 6
	t.Seed = fixtureSeed + 2
	drawn, err := tournament.Generate(t, competitors, limits)
	if err != nil {
		panic("demo sword and buckler fixture cannot be drawn: " + err.Error())
	}
	drawn.GeneratedAt = time.Now().Add(-80 * time.Minute).Format(time.RFC3339)
	return competitors, drawn, map[string][]match.Event{}
}

// emptyFixture is a discipline a visitor adds in the demo: named, and nothing else yet.
func emptyFixture(name string) func(match.Ruleset, tournament.Limits) ([]store.Competitor, store.Tournament, map[string][]match.Event) {
	return func(match.Ruleset, tournament.Limits) ([]store.Competitor, store.Tournament, map[string][]match.Event) {
		t := store.Defaults()
		t.Discipline = name
		return []store.Competitor{}, t, map[string][]match.Event{}
	}
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
		out[i] = store.Competitor{ID: fmt.Sprintf("c%d", i+1), Name: e.name, Club: e.club, Signup: signedUp[e.name]}
	}
	return out
}

// signedUp are the fencers who came in on a signup file, by the response they sent. One
// response entered both disciplines, so the event knows they are one person in both
// (phase 3). Everybody else was typed in at a desk.
var signedUp = map[string]string{
	"Astrid Lindqvist": "demo-signup-astrid",
	"Greta Mäkinen":    "demo-signup-greta",
	// The sword and buckler's fencers all fence the longsword too, and said so on one
	// response each.
	"Clara Wikström":  "demo-signup-clara",
	"Dag Nyström":     "demo-signup-dag",
	"Elin Sørensen":   "demo-signup-elin",
	"Fredrik Ahlberg": "demo-signup-fredrik",
	"Ida Karlsson":    "demo-signup-ida",
	"Jonas Ek":        "demo-signup-jonas",
	"Katrin Olsen":    "demo-signup-katrin",
	"Lars Bergström":  "demo-signup-lars",
	"Maja Lund":       "demo-signup-maja",
	"Rakel Virtanen":  "demo-signup-rakel",
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
