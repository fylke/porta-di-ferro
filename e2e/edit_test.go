package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// TestEditingAMatchLogRewritesItAndKeepsABackup is design §7 item 1 against the real
// binary: the organizer corrects a finished match, the derived state and the standings
// follow, and the version that was replaced is on disk beside the log.
func TestEditingAMatchLogRewritesItAndKeepsABackup(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)

	// A match scored 2-0 to red and ended.
	original := []match.Event{
		{Seq: 1, Type: match.TypeTimer, ElapsedMS: 0, Timer: &match.Timer{Action: match.TimerStart}},
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 10000, Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
		{Seq: 3, Type: match.TypeEnd, ElapsedMS: 20000, End: &match.End{Reason: match.ReasonTime}},
	}
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", original, nil)

	// The organizer decides the 2 was blue's, and adds an exchange that was missed. The
	// editor renumbers by position on save, so the end stays last.
	edited := []match.Event{
		original[0],
		{Seq: 2, Type: match.TypeExchange, ElapsedMS: 10000, Exchange: &match.Exchange{Blue: match.Assessment{Value: 2}}},
		{Seq: 3, Type: match.TypeExchange, ElapsedMS: 15000, Exchange: &match.Exchange{Blue: match.Assessment{Value: 1}}},
		{Seq: 4, Type: match.TypeEnd, ElapsedMS: 20000, End: &match.End{Reason: match.ReasonTime}},
	}
	var res struct {
		Backup string      `json:"backup"`
		State  match.State `json:"state"`
	}
	s.mustDo(t, "PUT", "/api/matches/"+id+"/events", edited, &res)
	if res.State.Blue.Score != 3 || res.State.Red.Score != 0 || res.State.Winner != match.Blue {
		t.Errorf("after the edit blue should have won 3-0, got %+v", res.State)
	}
	if res.Backup == "" {
		t.Fatal("the edit should have named its backup")
	}
	if _, err := os.Stat(filepath.Join(s.dir, "matches", res.Backup)); err != nil {
		t.Errorf("the backup should be on disk: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(s.dir, "matches", res.Backup))
	if !strings.Contains(string(b), `"reason":"time"`) || strings.Contains(string(b), `"seq":4`) {
		t.Errorf("the backup should be the log as it was before the edit, is:\n%s", b)
	}

	// The log reads back in sequence order, and the state everyone sees is the edited one.
	var log []match.Event
	s.mustDo(t, "GET", "/api/matches/"+id+"/events", nil, &log)
	if len(log) != 4 || log[2].Seq != 3 || log[3].Type != match.TypeEnd {
		t.Errorf("the edited log should hold four events with the end last, got %+v", log)
	}
	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	for _, p := range snap.Pools {
		for _, m := range p.Matches {
			if m.ID == id && m.State.Blue.Score != 3 {
				t.Errorf("the snapshot should show the edited state, shows %+v", m.State)
			}
		}
	}
	var backups []string
	s.mustDo(t, "GET", "/api/matches/"+id+"/backups", nil, &backups)
	if len(backups) != 1 || backups[0] != res.Backup {
		t.Errorf("one backup should be listed, got %v", backups)
	}

	// A later push still appends: the idempotency index was rebuilt from the new log.
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", []match.Event{{Seq: 4, Type: match.TypeExchange, ElapsedMS: 15000,
		Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}}}, &res)
	if res.State.Blue.Score != 3 {
		t.Errorf("re-sending seq 4 should be a no-op after the edit, got %+v", res.State)
	}

	// Nonsense is refused, and refused before anything is touched.
	if code := s.do(t, "PUT", "/api/matches/"+id+"/events", []match.Event{{Seq: 1}, {Seq: 1}}, nil); code != 400 {
		t.Errorf("duplicate sequence numbers should be a 400, got %d", code)
	}
	s.mustDo(t, "GET", "/api/matches/"+id+"/backups", nil, &backups)
	if len(backups) != 1 {
		t.Errorf("a refused edit should not have made a backup, got %v", backups)
	}
}
