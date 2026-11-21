package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// pushAs sends events stamped the way a score keeper client stamps them.
func (h *testHall) pushAs(path string, events any, client string, epoch int) int {
	h.t.Helper()
	b, _ := json.Marshal(events)
	req, _ := http.NewRequest("POST", h.url+path, bytes.NewReader(b))
	req.Header.Set("X-Porta-Client", client)
	req.Header.Set("X-Porta-Epoch", strconv.Itoa(epoch))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatalf("POST %s: %v", path, err)
	}
	res.Body.Close()
	return res.StatusCode
}

type claimHolder struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Alive bool   `json:"alive"`
}

// Nothing is taken from a device without somebody asking (#150): not from a live one,
// and not from one that has only gone quiet either. Taking over is a deliberate act,
// and whatever the device it was taken from sends afterwards is set aside.
// REQ: FR_TABLET_01
func TestNothingIsTakenWithoutAsking(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev struct {
		Disciplines []struct{ Slug string } `json:"disciplines"`
	}
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.draw("/api/d/"+ls, 8, 1)
	id := h.mats().Mats[0].Queue[0].Match.ID
	claim := "/api/d/" + ls + "/matches/" + id + "/claim"
	events := "/api/d/" + ls + "/matches/" + id + "/events"
	at := time.Date(2026, 11, 14, 9, 0, 0, 0, time.Local)
	timer := func(seq int, action string) []map[string]any {
		return []map[string]any{{"seq": seq, "type": "timer", "at": at.Format(time.RFC3339), "timer": map[string]string{"action": action}}}
	}

	h.must("POST", "/api/clients/tab1", map[string]any{"role": "scorekeeper", "name": "Tablet 1", "mat": 1, "match": id, "discipline": ls}, nil)
	var got struct {
		Epoch        int         `json:"epoch"`
		TookOverFrom string      `json:"tookOverFrom"`
		Holder       claimHolder `json:"holder"`
	}
	h.must("POST", claim, map[string]any{"client": "tab1"}, &got)
	if got.Epoch != 1 {
		t.Fatalf("the first claim should be epoch 1, got %d", got.Epoch)
	}
	if code := h.pushAs(events, timer(1, "start"), "tab1", 1); code != http.StatusOK {
		t.Fatalf("the holder's push was refused: %d", code)
	}

	// A second device opening the mat is asked, and told who has it.
	if code := h.do("POST", claim, map[string]any{"client": "tab2"}, &got); code != http.StatusConflict {
		t.Fatalf("a live holder's match was handed on without asking: %d", code)
	}
	if got.Holder.Name != "Tablet 1" || !got.Holder.Alive {
		t.Errorf("the refusal should name the live holder: %+v", got.Holder)
	}

	// A holder the server has not heard from is asked about just the same: a dark
	// screen looks exactly like a dead battery.
	h.must("POST", "/api/clients/tab3", map[string]any{"role": "scorekeeper", "name": "Tablet 3", "mat": 1}, nil)
	h.must("POST", "/api/clients/tab3/release", nil, nil)
	second := h.mats().Mats[0].Queue[1].Match.ID
	quiet := "/api/d/" + ls + "/matches/" + second + "/claim"
	h.must("POST", quiet, map[string]any{"client": "tab3"}, nil)
	got.Holder = claimHolder{}
	if code := h.do("POST", quiet, map[string]any{"client": "tab2"}, &got); code != http.StatusConflict {
		t.Fatalf("a quiet holder's match was handed on without asking: %d", code)
	}
	if got.Holder.ID != "tab3" || got.Holder.Alive {
		t.Errorf("the refusal should say the holder has gone quiet: %+v", got.Holder)
	}

	// Nobody writes to a held match without a claim.
	if code := h.do("POST", events, timer(2, "stop"), nil); code != http.StatusConflict {
		t.Errorf("an unclaimed push to a held match should be refused, got %d", code)
	}

	// The holder can see that it still holds the match.
	var who struct {
		Client string       `json:"client"`
		Epoch  int          `json:"epoch"`
		Holder *claimHolder `json:"holder"`
	}
	h.must("GET", claim, nil, &who)
	if who.Client != "tab1" || who.Epoch != 1 || who.Holder == nil || who.Holder.Name != "Tablet 1" {
		t.Errorf("the claim should be tab1's, epoch 1: %+v", who)
	}

	// Taking over is asked for, and the holder can see it has lost the match.
	h.must("POST", claim, map[string]any{"client": "tab2", "force": true}, &got)
	if got.Epoch != 2 || got.TookOverFrom != "tab1" {
		t.Errorf("taking over should bump the epoch and name tab1: %+v", got)
	}
	h.must("GET", claim, nil, &who)
	if who.Client != "tab2" || who.Epoch != 2 {
		t.Errorf("after the takeover the claim should be tab2's, epoch 2: %+v", who)
	}
	if code := h.pushAs(events, timer(2, "stop"), "tab1", 1); code != http.StatusConflict {
		t.Errorf("the displaced device's push should be set aside, got %d", code)
	}
	var log []map[string]any
	h.must("GET", events, nil, &log)
	if len(log) != 1 {
		t.Errorf("only the holder's exchange should be in the log, got %d events", len(log))
	}
}

// #160: a tablet that scored p1m1 before a redraw still holds that log. Pushed again, with
// the draw it was scored in, it is set aside for the organizer -- the new pairing starts at
// 0-0 instead of finished with the old result.
// REQ: FR_TABLET_02
func TestALogFromAReplacedDrawIsSetAside(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev struct {
		Disciplines []struct{ Slug string } `json:"disciplines"`
	}
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.draw("/api/d/"+ls, 4, 1)
	state := func() (draw string, status string) {
		var s struct {
			Pools []struct {
				Matches []struct {
					ID     string `json:"id"`
					Draw   string `json:"draw"`
					Status string `json:"status"`
				} `json:"matches"`
			} `json:"pools"`
		}
		h.must("GET", "/api/d/"+ls+"/state", nil, &s)
		m := s.Pools[0].Matches[0]
		if m.ID != "p1m1" {
			t.Fatalf("expected p1m1 first, got %s", m.ID)
		}
		return m.Draw, m.Status
	}
	old, _ := state()
	if old == "" {
		t.Fatal("every match should say which draw it is from")
	}
	log := []map[string]any{
		{"seq": 1, "type": "timer", "elapsedMs": 0, "timer": map[string]string{"action": "start"}},
		{"seq": 2, "type": "exchange", "elapsedMs": 1000, "exchange": map[string]any{"red": map[string]int{"value": 2}, "blue": map[string]int{"value": 0}}},
		{"seq": 3, "type": "end", "elapsedMs": 2000, "end": map[string]string{"reason": "time"}},
	}
	push := func(draw string) int {
		b, _ := json.Marshal(log)
		req, _ := http.NewRequest("POST", h.url+"/api/d/"+ls+"/matches/p1m1/events", bytes.NewReader(b))
		req.Header.Set("X-Porta-Client", "tab1")
		req.Header.Set("X-Porta-Epoch", "1")
		req.Header.Set("X-Porta-Draw", draw)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	h.must("POST", "/api/d/"+ls+"/matches/p1m1/claim", map[string]any{"client": "tab1"}, nil)
	if code := push(old); code != 200 {
		t.Fatalf("a push in the current draw should land, got %d", code)
	}

	h.must("POST", "/api/d/"+ls+"/tournament/pools", nil, nil)
	now, status := state()
	if now == old || status != "pending" {
		t.Fatalf("the redraw should be a new draw, its p1m1 to fence: draw %q (was %q), %s", now, old, status)
	}
	if code := push(old); code != http.StatusConflict {
		t.Errorf("the old draw's log should be refused, got %d", code)
	}
	if _, status := state(); status != "pending" {
		t.Errorf("the new p1m1 should still be to fence, is %s", status)
	}
	var p struct {
		Quarantined []struct{ Match string } `json:"quarantined"`
	}
	h.must("GET", "/api/presence", nil, &p)
	if len(p.Quarantined) == 0 {
		t.Error("the old log should be set aside for the organizer")
	}
}
