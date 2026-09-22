package demo_test

import (
	"encoding/json"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/demo"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/match"
)

// The demo is compiled to WebAssembly and driven from a browser, which is an awkward
// place to find out that the tournament it serves is wrong. It is plain Go underneath,
// so it is tested here instead: everything below runs in `go test ./...` on any platform,
// and the wasm build is only the forty lines of glue in cmd/demo-wasm.

func state(t *testing.T, d *demo.Demo) httpapi.Snapshot {
	t.Helper()
	res := d.Request("GET", "/api/state", nil)
	if res.Status != 200 {
		t.Fatalf("GET /api/state returned %d: %s", res.Status, res.Body)
	}
	var snap httpapi.Snapshot
	if err := json.Unmarshal([]byte(res.Body), &snap); err != nil {
		t.Fatalf("decoding the snapshot: %v", err)
	}
	return snap
}

// A visitor arrives in the middle of a tournament that has actually been fenced: pools
// drawn by the real generator, results replayed from real logs, standings that are a
// consequence of them.
func TestTheVisitorArrivesInARunningTournament(t *testing.T) {
	d := demo.New()
	snap := state(t, d)

	if len(snap.Competitors) != 32 {
		t.Errorf("the field should be 32 entrants, got %d", len(snap.Competitors))
	}
	if len(snap.Pools) == 0 {
		t.Fatal("the pools should be drawn before anyone opens the page")
	}
	if snap.Tournament.Mats != 3 {
		t.Errorf("the demo runs on 3 mats, got %d", snap.Tournament.Mats)
	}
	if snap.Dir == "" {
		t.Error("the organizer view shows where the data lives and would render an empty line")
	}

	var complete, running, pending int
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			switch m.Status {
			case "complete":
				complete++
			case "running":
				running++
			default:
				pending++
			}
		}
	}
	if complete < 20 {
		t.Errorf("most of the pools should be fenced, got %d complete", complete)
	}
	if running != 1 {
		t.Errorf("exactly one match should be under way, for the displays to show a live clock; got %d", running)
	}
	if pending == 0 {
		t.Error("something should be left to do, or there is nothing for a visitor to try")
	}

	// Every mat has a match up. The demo's most obvious link is Score keeper, which opens
	// on a mat, and the mat displays show all three at once: a visitor who lands on a mat
	// with nothing on it has hit a dead end on their first click.
	for mat := 1; mat <= snap.Tournament.Mats; mat++ {
		if snap.Mats[mat] == "" {
			t.Errorf("mat %d has nothing up, so its display and its score keeper are both empty", mat)
		}
	}
	// The one that is actually under way is on mat 1, which is where Score keeper lands.
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			if m.Status == "running" && m.Mat != 1 {
				t.Errorf("the live match is on mat %d, not the mat the demo points at", m.Mat)
			}
		}
	}

	// The standings are derived, not stored: every competitor who has fenced has a line
	// with a rank on it, and the indices come out of the same chain a real event uses.
	scored := false
	for _, p := range snap.Pools {
		if len(p.Standings) == 0 {
			t.Errorf("pool %d has no standings", p.Number)
			continue
		}
		for i, s := range p.Standings {
			if s.Rank != i+1 {
				t.Errorf("pool %d standing %d has rank %d", p.Number, i, s.Rank)
			}
			if s.Name == "" {
				t.Errorf("pool %d standing %d has no name", p.Number, i)
			}
			if s.Completed > 0 {
				scored = true
			}
		}
	}
	if !scored {
		t.Error("nobody has completed a match, so every table would read as zeroes")
	}

	// A live match has a clock the displays can count on from.
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			if m.Status == "running" && m.State.Running && m.SinceMS <= 0 {
				t.Errorf("%s is running but carries no elapsed time, so a display would freeze it", m.ID)
			}
		}
	}

	// The eliminations are not drawn yet: there is a tournament left to finish.
	if snap.Bracket != nil {
		t.Error("the bracket should not be drawn on arrival")
	}
	if snap.PoolsComplete {
		t.Error("the pools should not be complete on arrival")
	}
}

// The logs are real logs. The organizer's history editor opens one and the client engine
// replays it, so what is in there has to be an ordinary match.
func TestTheMatchLogsAreRealLogs(t *testing.T) {
	d := demo.New()
	snap := state(t, d)

	var id string
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			if m.Status == "complete" {
				id = m.ID
			}
		}
	}
	if id == "" {
		t.Fatal("no completed match to read")
	}

	res := d.Request("GET", "/api/matches/"+id+"/events?after=0", nil)
	if res.Status != 200 {
		t.Fatalf("reading the log returned %d", res.Status)
	}
	var events []match.Event
	if err := json.Unmarshal([]byte(res.Body), &events); err != nil {
		t.Fatalf("decoding the log: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("%s has %d events, which is not a match", id, len(events))
	}
	if events[0].Type != match.TypeTimer {
		t.Errorf("a match starts with the clock, got %q", events[0].Type)
	}
	if last := events[len(events)-1]; last.Type != match.TypeEnd {
		t.Errorf("a finished match ends with an end event, got %q", last.Type)
	}
	for i, e := range events {
		if e.Seq != i+1 {
			t.Errorf("event %d has sequence %d; the log should be numbered from 1", i, e.Seq)
		}
		if e.Match != id {
			t.Errorf("event %d says it belongs to %q, not %q", i, e.Match, id)
		}
	}
	// And the state the snapshot reported is what replaying that log produces, rather
	// than anything stored beside it.
	st := match.Replay(match.MSL(), events)
	if !st.Ended {
		t.Error("the log of a complete match should replay to an ended match")
	}
}

// Scoring in the demo moves the standings, because the standings are computed from the
// logs by the same code the server runs. This is the whole reason the demo is Go
// compiled to wasm rather than a folder of fixture JSON.
func TestScoringMovesTheStandings(t *testing.T) {
	d := demo.New()
	before := state(t, d)

	var id string
	var pool int
	for _, p := range before.Pools {
		for _, m := range p.Matches {
			if m.Status == "pending" && id == "" {
				id, pool = m.ID, p.Number
			}
		}
	}
	if id == "" {
		t.Fatal("no pending match to score")
	}

	completedIn := func(snap httpapi.Snapshot, number int) int {
		for _, p := range snap.Pools {
			if p.Number != number {
				continue
			}
			total := 0
			for _, s := range p.Standings {
				total += s.Completed
			}
			return total
		}
		return -1
	}
	was := completedIn(before, pool)

	events := []match.Event{
		{Seq: 1, Type: match.TypeTimer, Timer: &match.Timer{Action: match.TimerStart}},
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 10000, Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
		{Seq: 3, Type: match.TypeExchange, ElapsedMS: 20000, Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
		{Seq: 4, Type: match.TypeExchange, ElapsedMS: 30000, Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
		{Seq: 5, Type: match.TypeExchange, ElapsedMS: 40000, Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
		{Seq: 6, Type: match.TypeEnd, ElapsedMS: 40000, End: &match.End{Reason: match.ReasonPointCap}},
	}
	body, _ := json.Marshal(events)
	res := d.Request("POST", "/api/matches/"+id+"/events", body)
	if res.Status != 200 {
		t.Fatalf("scoring returned %d: %s", res.Status, res.Body)
	}
	if !res.Changed {
		t.Error("scoring changed the tournament and the adapter needs to hear about it")
	}

	after := state(t, d)
	if now := completedIn(after, pool); now != was+2 {
		t.Errorf("pool %d counted %d completed matches before and %d after; both competitors should have gained one", pool, was, now)
	}

	// A retry of the same push is a no-op, which is what makes the offline client safe.
	res = d.Request("POST", "/api/matches/"+id+"/events", body)
	if res.Status != 200 {
		t.Fatalf("the retry returned %d", res.Status)
	}
	var fresh []match.Event
	_ = json.Unmarshal([]byte(res.Body), &fresh)
	if len(fresh) != 0 {
		t.Errorf("the retry wrote %d events; (match, seq) should have made it a no-op", len(fresh))
	}
}

// Play the rest finishes the tournament, which is how a visitor who does not want to
// score forty matches still gets to see the bracket and the podium.
func TestPlayingOutReachesAPodium(t *testing.T) {
	d := demo.New()

	if res := d.Request("POST", "/api/demo/play", nil); res.Status != 200 {
		t.Fatalf("playing out the pools returned %d: %s", res.Status, res.Body)
	}
	snap := state(t, d)
	if !snap.PoolsComplete {
		t.Fatal("every pool match should be in")
	}
	if len(snap.Overall) == 0 {
		t.Fatal("the overall ranking should be populated once the pools are done")
	}

	if res := d.Request("POST", "/api/tournament/bracket", nil); res.Status != 200 {
		t.Fatalf("drawing the bracket returned %d: %s", res.Status, res.Body)
	}
	if res := d.Request("POST", "/api/demo/play", nil); res.Status != 200 {
		t.Fatalf("playing out the bracket returned %d: %s", res.Status, res.Body)
	}

	snap = state(t, d)
	if snap.Bracket == nil {
		t.Fatal("the bracket should be drawn")
	}
	if !snap.Bracket.Complete {
		t.Error("every bracket match should be fenced")
	}
	p := snap.Bracket.Podium
	if p.First == "" || p.Second == "" || p.Third == "" {
		t.Errorf("the podium should be filled, got %+v", p)
	}
	if p.First == p.Second || p.Second == p.Third || p.First == p.Third {
		t.Errorf("the podium has the same competitor twice: %+v", p)
	}

	// The eliminations went to two mats of the three, because a third buys the bracket
	// no time (issue #94), and the demo is running the same rule.
	mats := map[int]bool{}
	for _, m := range snap.Bracket.Matches {
		mats[m.Mat] = true
	}
	if len(mats) != 2 {
		t.Errorf("the bracket should spread over 2 mats, got %d", len(mats))
	}
}

// Resetting puts it back, because a demo that can be broken once is a demo that is
// broken for the next visitor.
func TestResetPutsItBack(t *testing.T) {
	d := demo.New()
	before := state(t, d)

	d.Request("POST", "/api/demo/play", nil)
	d.Request("POST", "/api/competitors", []byte(`{"name":"Someone Else","club":"Nowhere"}`))
	if mid := state(t, d); len(mid.Competitors) != len(before.Competitors)+1 {
		t.Fatal("the entrant was not added, so the reset proves nothing")
	}

	if res := d.Request("POST", "/api/demo/reset", nil); res.Status != 200 {
		t.Fatalf("reset returned %d", res.Status)
	}
	after := state(t, d)
	if len(after.Competitors) != len(before.Competitors) {
		t.Errorf("the field should be back to %d, got %d", len(before.Competitors), len(after.Competitors))
	}
	if after.PoolsComplete {
		t.Error("the tournament should be mid-flight again")
	}
	// Same draw, same seed, same tournament: a visitor arriving after someone else has
	// pressed reset sees what the first one saw.
	if after.Tournament.Seed != before.Tournament.Seed {
		t.Errorf("the draw should be reproducible: seed %d became %d", before.Tournament.Seed, after.Tournament.Seed)
	}
	for i := range after.Pools {
		if got, want := len(after.Pools[i].Competitors), len(before.Pools[i].Competitors); got != want {
			t.Errorf("pool %d came back with %d competitors, was %d", i+1, got, want)
		}
	}
}

// The demo says so when it cannot do something, rather than pretending.
func TestWhatTheDemoCannotDoItRefuses(t *testing.T) {
	d := demo.New()

	res := d.Request("POST", "/api/instances", []byte(`{"name":"Open Sabre"}`))
	if res.Status != 400 {
		t.Errorf("starting a second discipline needs a second process; want 400, got %d", res.Status)
	}
	if res = d.Request("GET", "/api/nonsense", nil); res.Status != 404 {
		t.Errorf("an unknown endpoint should be a 404, got %d", res.Status)
	}
	if res = d.Request("GET", "/api/addresses", nil); res.Status != 200 || res.Body != "[]" {
		t.Errorf("a browser tab is on no LAN; want an empty list, got %d %s", res.Status, res.Body)
	}
}

// The printable pool sheets are the same document the server produces, because it is the
// same function building it.
func TestThePDFExportIsARealDocument(t *testing.T) {
	d := demo.New()
	res := d.Request("GET", "/api/export.pdf", nil)
	if res.Status != 200 {
		t.Fatalf("the PDF export returned %d: %s", res.Status, res.Body)
	}
	if res.ContentType != "application/pdf" {
		t.Errorf("served as %q", res.ContentType)
	}
	if !res.Base64 {
		t.Fatal("the PDF has to come out as bytes, not text")
	}
	if len(res.Body) < 1000 {
		t.Errorf("the document is %d base64 characters, which is not a pool sheet", len(res.Body))
	}
}
