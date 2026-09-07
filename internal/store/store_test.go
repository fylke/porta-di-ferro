package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

func TestAppendIsIdempotentOnSequence(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	batch := []match.Event{
		{Seq: 1, Type: match.TypeTimer, Timer: &match.Timer{Action: match.TimerStart}},
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 5000, Exchange: &match.Exchange{}},
	}
	written, err := s.Append("p1m1", batch)
	if err != nil || len(written) != 2 {
		t.Fatalf("first append wrote %d events, err %v", len(written), err)
	}
	written, err = s.Append("p1m1", batch)
	if err != nil || len(written) != 0 {
		t.Fatalf("a retry wrote %d events, err %v", len(written), err)
	}
	events, err := s.Events("p1m1", 0)
	if err != nil || len(events) != 2 {
		t.Fatalf("log holds %d events after a duplicate push, err %v", len(events), err)
	}
	for _, e := range events {
		if e.At == "" {
			t.Errorf("event %d was stored without a timestamp", e.Seq)
		}
	}
}

func TestEventsAfterSequence(t *testing.T) {
	s, _ := store.Open(t.TempDir())
	s.Append("p1m1", []match.Event{{Seq: 1, Type: match.TypeExchange, Exchange: &match.Exchange{}}})
	s.Append("p1m1", []match.Event{{Seq: 2, Type: match.TypeExchange, Exchange: &match.Exchange{}}})
	got, err := s.Events("p1m1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Seq != 2 {
		t.Errorf(`"everything after sequence 1" returned %+v`, got)
	}
}

// TestTruncatedFinalLineCostsOneEvent is why the log is newline-delimited: a crash
// mid-write loses the last line, not the file.
func TestTruncatedFinalLineCostsOneEvent(t *testing.T) {
	dir := t.TempDir()
	s, _ := store.Open(dir)
	s.Append("p1m1", []match.Event{
		{Seq: 1, Type: match.TypeExchange, ElapsedMS: 1000, Exchange: &match.Exchange{}},
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 2000, Exchange: &match.Exchange{}},
	})

	path := filepath.Join(dir, "matches", "p1m1.ndjson")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Chop the last line in half, as an interrupted write would.
	if err := os.WriteFile(path, raw[:len(raw)-20], 0o644); err != nil {
		t.Fatal(err)
	}

	fresh, _ := store.Open(dir)
	got, err := fresh.Events("p1m1", 0)
	if err != nil {
		t.Fatalf("a truncated line should not fail the read: %v", err)
	}
	if len(got) != 1 || got[0].Seq != 1 {
		t.Errorf("expected the first event to survive alone, got %+v", got)
	}
}

func TestMissingFilesAreAnEmptyTournament(t *testing.T) {
	s, _ := store.Open(t.TempDir())
	competitors, err := s.Competitors()
	if err != nil || len(competitors) != 0 {
		t.Errorf("a fresh directory should hold no competitors, got %v (%v)", competitors, err)
	}
	tour, err := s.Tournament()
	if err != nil {
		t.Fatal(err)
	}
	if tour.Mats != 2 || tour.MaxPoolSize != 7 {
		t.Errorf("a fresh directory should fall back to the MVP defaults, got %+v", tour)
	}
}

func TestMatchIDsCannotEscapeTheDirectory(t *testing.T) {
	s, _ := store.Open(t.TempDir())
	for _, bad := range []string{"", "../secret", "a/b", `a\b`, "a.b"} {
		if _, err := s.Append(bad, []match.Event{{Seq: 1, Type: match.TypeExchange}}); err == nil {
			t.Errorf("match id %q became a file name", bad)
		}
	}
}
