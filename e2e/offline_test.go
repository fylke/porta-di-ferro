package e2e

import (
	"fmt"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestOfflineClientResumes is the offline test from docs/design.md §12, expressed against
// the real binary: a score keeper client scores a run of exchanges while the server never
// hears about them, then hands the whole batch over at once. The server's log must match
// the device's exactly.
//
// This is not a separate mode anyone has to build. Local-first writes already give it,
// and the client simply pushes when it can.
func TestOfflineClientResumes(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)

	// What the device wrote while it was alone: the whole match, in one go.
	offline := []match.Event{
		{Seq: 1, Type: match.TypeTimer, ElapsedMS: 0, Timer: &match.Timer{Action: match.TimerStart}},
	}
	elapsed := int64(0)
	for i := 0; i < 4; i++ {
		elapsed += 15000
		offline = append(offline, match.Event{
			Seq: i + 2, Type: match.TypeExchange, ElapsedMS: elapsed,
			Exchange: &match.Exchange{Red: match.Assessment{Value: 2}},
		})
	}

	var res struct {
		Written int         `json:"written"`
		State   match.State `json:"state"`
	}
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", offline, &res)
	if res.Written != len(offline) {
		t.Fatalf("the server took %d of %d events", res.Written, len(offline))
	}
	if res.State.Red.Score != 8 || res.State.Pending != match.PendingPointCap {
		t.Fatalf("after four clean twos the score should be 8 with the cap dialog up, got %+v", res.State)
	}

	var stored []match.Event
	s.mustDo(t, "GET", "/api/matches/"+id+"/events", nil, &stored)
	if len(stored) != len(offline) {
		t.Fatalf("the server holds %d events, the device wrote %d", len(stored), len(offline))
	}
	for i := range stored {
		if stored[i].Seq != offline[i].Seq || stored[i].Type != offline[i].Type {
			t.Errorf("event %d differs: server %+v, device %+v", i, stored[i], offline[i])
		}
		if stored[i].Match != id {
			t.Errorf("event %d is not stamped with its match", i)
		}
	}
}

// TestRetriesAreIdempotent is what makes the write path safe to call blindly: the primary
// key is (match, seq), so a client that pushes the same batch again -- because it never
// saw the response -- changes nothing.
func TestRetriesAreIdempotent(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)

	batch := []match.Event{
		{Seq: 1, Type: match.TypeTimer, ElapsedMS: 0, Timer: &match.Timer{Action: match.TimerStart}},
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 9000,
			Exchange: &match.Exchange{Red: match.Assessment{Value: 2}, Blue: match.Assessment{Value: 1}}},
	}

	var first, second struct {
		Written int         `json:"written"`
		State   match.State `json:"state"`
	}
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", batch, &first)
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", batch, &second)

	if first.Written != 2 {
		t.Fatalf("the first push should have written 2 events, wrote %d", first.Written)
	}
	if second.Written != 0 {
		t.Errorf("the retry should have written nothing, wrote %d", second.Written)
	}
	if second.State != first.State {
		t.Errorf("the retry changed the state:\nbefore %+v\nafter  %+v", first.State, second.State)
	}

	var stored []match.Event
	s.mustDo(t, "GET", "/api/matches/"+id+"/events", nil, &stored)
	if len(stored) != 2 {
		t.Errorf("the log holds %d events after a duplicate push, want 2", len(stored))
	}

	// "Everything after sequence N" is how a client resumes.
	var after []match.Event
	s.mustDo(t, "GET", "/api/matches/"+id+"/events?after=1", nil, &after)
	if len(after) != 1 || after[0].Seq != 2 {
		t.Errorf("resuming after sequence 1 returned %+v", after)
	}
}

// TestUndoIsAppendedNotMutated checks that a correction leaves the history intact -- the
// property that makes full history editing a UI change in Milestone 2 rather than a data
// migration.
func TestUndoIsAppendedNotMutated(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)

	var res struct {
		State match.State `json:"state"`
	}
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", []match.Event{
		{Seq: 1, Type: match.TypeExchange, ElapsedMS: 5000,
			Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 9000,
			Exchange: &match.Exchange{Blue: match.Assessment{Value: 2}}},
	}, &res)
	if res.State.Blue.Score != 2 {
		t.Fatalf("blue should be on 2 before the undo, got %d", res.State.Blue.Score)
	}

	s.mustDo(t, "POST", "/api/matches/"+id+"/events", []match.Event{
		{Seq: 3, Type: match.TypeUndo, ElapsedMS: 9000, Undo: &match.Undo{Seq: 2}},
	}, &res)
	if res.State.Blue.Score != 0 || res.State.Red.Score != 2 {
		t.Errorf("after undoing blue's exchange the score should be 2-0, got %d-%d",
			res.State.Red.Score, res.State.Blue.Score)
	}
	if res.State.UndoableSeq != 1 {
		t.Errorf("the undoable exchange should now be the first one, got %d", res.State.UndoableSeq)
	}

	var stored []match.Event
	s.mustDo(t, "GET", "/api/matches/"+id+"/events", nil, &stored)
	if len(stored) != 3 {
		t.Errorf("undo should append a fourth record rather than remove one; log holds %d", len(stored))
	}
}

// setupOneMatch draws a minimal tournament and returns the first match's id.
func setupOneMatch(t *testing.T, s *server) string {
	t.Helper()
	for i := 1; i <= 4; i++ {
		s.mustDo(t, "POST", "/api/competitors",
			map[string]string{"name": fmt.Sprintf("Competitor %d", i), "club": "MSL"},
			&store.Competitor{})
	}
	s.mustDo(t, "PUT", "/api/tournament",
		map[string]int{"mats": 1, "minPoolSize": 4, "maxPoolSize": 7}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)

	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if len(snap.Pools) == 0 || len(snap.Pools[0].Matches) == 0 {
		t.Fatal("no matches were drawn")
	}
	return snap.Pools[0].Matches[0].ID
}
