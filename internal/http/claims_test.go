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
func (h *hall) pushAs(path string, events any, client string, epoch int) int {
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
