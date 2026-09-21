package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
)

type client struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	Name string `json:"name"`
	Mat  int    `json:"mat"`
	// Match is the score keeper's; Target the display's.
	Match  string `json:"match"`
	Target string `json:"target"`
	Alive  bool   `json:"alive"`
}

type presence struct {
	Clients     []client `json:"clients"`
	Quarantined []struct {
		Match  string      `json:"match"`
		Client string      `json:"client"`
		Event  match.Event `json:"event"`
	} `json:"quarantined"`
}

// pushAs is a score keeper client's push: stamped with who it is and which epoch it holds.
func (s *server) pushAs(t *testing.T, id, clientID string, epoch int, events []match.Event) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(events)
	req, _ := http.NewRequest("POST", s.base+"/api/matches/"+id+"/events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Porta-Client", clientID)
	req.Header.Set("X-Porta-Epoch", fmt.Sprint(epoch))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (s *server) heartbeat(t *testing.T, id, role string, mat int, matchID string) client {
	t.Helper()
	var c client
	s.mustDo(t, "POST", "/api/clients/"+id,
		map[string]any{"role": role, "name": "Device " + id, "mat": mat, "match": matchID}, &c)
	return c
}

func (s *server) claim(t *testing.T, id, clientID string, force bool) (int, map[string]any) {
	t.Helper()
	var out map[string]any
	code := s.do(t, "POST", "/api/matches/"+id+"/claim", map[string]any{"client": clientID, "force": force}, &out)
	return code, out
}

func exchangeAt(seq int, elapsed int64) match.Event {
	return match.Event{Seq: seq, Type: match.TypeExchange, ElapsedMS: elapsed,
		Exchange: &match.Exchange{Red: match.Assessment{Value: 1}}}
}

// TestHandoverQuarantinesTheOldDevice is design §7 item 10 against the real binary: a
// contested claim is refused, a forced one bumps the epoch, and what the displaced device
// sends afterwards is set aside for the organizer rather than appended or dropped.
func TestHandoverQuarantinesTheOldDevice(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)

	s.heartbeat(t, "A", "scorekeeper", 1, id)
	if code, out := s.claim(t, id, "A", false); code != 200 || out["epoch"].(float64) != 1 {
		t.Fatalf("first claim should be epoch 1, got %d %v", code, out)
	}
	if code, _ := s.pushAs(t, id, "A", 1, []match.Event{exchangeAt(1, 1000)}); code != 200 {
		t.Fatalf("the holder's push was refused with %d", code)
	}

	// B wants the mat while A is alive: refused, and told who has it.
	s.heartbeat(t, "B", "scorekeeper", 1, id)
	if code, out := s.claim(t, id, "B", false); code != http.StatusConflict {
		t.Fatalf("a contested claim should be refused, got %d %v", code, out)
	} else if holder, _ := out["holder"].(map[string]any); holder["id"] != "A" {
		t.Errorf("the refusal should name A as the holder, got %v", out)
	}

	// B takes over explicitly. The epoch moves on.
	code, out := s.claim(t, id, "B", true)
	if code != 200 || out["epoch"].(float64) != 2 || out["tookOverFrom"] != "A" {
		t.Fatalf("a forced claim should be epoch 2 taken from A, got %d %v", code, out)
	}
	if code, _ := s.pushAs(t, id, "B", 2, []match.Event{exchangeAt(2, 2000)}); code != 200 {
		t.Fatalf("the new holder's push was refused with %d", code)
	}

	// A comes back with its old epoch. Set aside, not appended, not dropped.
	code, out = s.pushAs(t, id, "A", 1, []match.Event{exchangeAt(3, 3000), exchangeAt(4, 4000)})
	if code != http.StatusConflict || out["quarantined"].(float64) != 2 {
		t.Fatalf("a stale push should be quarantined, got %d %v", code, out)
	}
	var log []match.Event
	s.mustDo(t, "GET", "/api/matches/"+id+"/events", nil, &log)
	if len(log) != 2 {
		t.Errorf("the log should hold the two admitted events only, has %d", len(log))
	}
	var p presence
	s.mustDo(t, "GET", "/api/presence", nil, &p)
	if len(p.Quarantined) != 2 || p.Quarantined[0].Client != "A" || p.Quarantined[0].Event.Seq != 3 {
		t.Errorf("the organizer should see A's two events set aside, got %+v", p.Quarantined)
	}
	s.mustDo(t, "DELETE", "/api/quarantine/"+id, nil, nil)
	s.mustDo(t, "GET", "/api/presence", nil, &p)
	if len(p.Quarantined) != 0 {
		t.Errorf("discarding should empty the quarantine, got %d", len(p.Quarantined))
	}

	// An anonymous push -- no epoch -- is the paper-entry path and still works.
	s.mustDo(t, "POST", "/api/matches/"+id+"/events", []match.Event{exchangeAt(5, 5000)}, nil)
}

// TestGracefulHandoverNeedsNoForce: the outgoing device releases, the next claims without
// ceremony, and the epoch still moves on so the old one can never write again.
func TestGracefulHandoverNeedsNoForce(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)

	s.heartbeat(t, "A", "scorekeeper", 1, id)
	s.claim(t, id, "A", false)
	s.mustDo(t, "POST", "/api/clients/A/release", nil, nil)

	s.heartbeat(t, "B", "scorekeeper", 1, id)
	code, out := s.claim(t, id, "B", false)
	if code != 200 || out["epoch"].(float64) != 2 || out["tookOverFrom"] != nil {
		t.Fatalf("after a release the claim should be plain and on epoch 2, got %d %v", code, out)
	}
	if code, _ := s.pushAs(t, id, "A", 1, []match.Event{exchangeAt(1, 1000)}); code != http.StatusConflict {
		t.Errorf("the released device's old epoch should be stale, got %d", code)
	}
}

// TestADeadDeviceIsTakenOverWithoutAsking: a claim that was never heartbeated -- or
// whose device has gone quiet -- is not contested.
func TestADeadDeviceIsTakenOverWithoutAsking(t *testing.T) {
	s := start(t)
	id := setupOneMatch(t, s)
	s.claim(t, id, "ghost", false) // claimed, never registered: as dead as a device gets
	s.heartbeat(t, "B", "scorekeeper", 1, id)
	code, out := s.claim(t, id, "B", false)
	if code != 200 || out["tookOverFrom"] != "ghost" || out["epoch"].(float64) != 2 {
		t.Fatalf("a dead holder should be displaced without force, got %d %v", code, out)
	}
}

// TestTheMatFollowsItsScoreKeeper: the match a mat is showing is the one the live score
// keeper says it is on, finished or not -- so the displays hold a result for exactly as
// long as the score keeper does.
func TestTheMatFollowsItsScoreKeeper(t *testing.T) {
	s := start(t)
	for i := 1; i <= 4; i++ {
		var c store.Competitor
		s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": fmt.Sprintf("C%d", i), "club": "MSL"}, &c)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 1, "minPoolSize": 4, "maxPoolSize": 4}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)
	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	first := snap.Pools[0].Matches[0].ID
	second := snap.Pools[0].Matches[1].ID

	// Finish the first match. With nobody registered, the mat moves on at once.
	s.mustDo(t, "POST", "/api/matches/"+first+"/events", []match.Event{
		{Seq: 1, Type: match.TypeEnd, ElapsedMS: 0, End: &match.End{Reason: match.ReasonForfeit, Forfeiter: match.Red}},
	}, nil)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Mats["1"] != second {
		t.Fatalf("with no score keeper the mat should be on the next match, is on %q", snap.Mats["1"])
	}

	// A score keeper holding the finished match keeps the mat there.
	s.heartbeat(t, "A", "scorekeeper", 1, first)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Mats["1"] != first {
		t.Errorf("the mat should follow its score keeper onto the finished match, is on %q", snap.Mats["1"])
	}
	// Pressing Next match is a heartbeat with the next match.
	s.heartbeat(t, "A", "scorekeeper", 1, second)
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Mats["1"] != second {
		t.Errorf("the mat should follow its score keeper on, is on %q", snap.Mats["1"])
	}
	// A match the tournament does not have is ignored rather than trusted.
	s.heartbeat(t, "A", "scorekeeper", 1, "nonsense")
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	if snap.Mats["1"] != second {
		t.Errorf("an unknown match from the score keeper should fall back, got %q", snap.Mats["1"])
	}
}

// TestServerAssignedDisplays is design §7 item 4: a screen registers, the organizer tells
// it what to show, and the assignment survives in the store.
func TestServerAssignedDisplays(t *testing.T) {
	s := start(t)
	var c client
	s.mustDo(t, "POST", "/api/clients/screen1", map[string]any{"role": "display", "name": "Screen 1"}, &c)
	if c.Target != "" {
		t.Fatalf("a new screen should have no target, has %q", c.Target)
	}
	s.mustDo(t, "PUT", "/api/clients/screen1/target", map[string]string{"target": "audience/1"}, nil)
	s.mustDo(t, "POST", "/api/clients/screen1", map[string]any{"role": "display", "name": "Screen 1"}, &c)
	if c.Target != "audience/1" {
		t.Errorf("the next heartbeat should carry the assignment, got %q", c.Target)
	}
	var p presence
	s.mustDo(t, "GET", "/api/presence", nil, &p)
	found := false
	for _, x := range p.Clients {
		if x.ID == "screen1" && x.Role == "display" && x.Target == "audience/1" && x.Alive {
			found = true
		}
	}
	if !found {
		t.Errorf("the organizer should see the screen, alive, with its target: %+v", p.Clients)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "displays.json")); err != nil {
		t.Errorf("the assignment should be on disk: %v", err)
	}
	if code := s.do(t, "POST", "/api/clients/x", map[string]any{"role": "toaster"}, nil); code != 400 {
		t.Errorf("an unknown role should be a 400, got %d", code)
	}
}
