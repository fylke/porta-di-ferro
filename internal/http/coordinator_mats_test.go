package httpapi_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The event's mats over HTTP (phase 2): one queue per physical mat across disciplines,
// the plan moved from the mat board or from a discipline's own pool controls, and the
// devices at the mats registered with the event.

// draw enters n competitors in a discipline (its API prefix) and draws its pools on the
// given number of its own mats.
func (h *hall) draw(prefix string, n, mats int) {
	h.t.Helper()
	for i := 0; i < n; i++ {
		h.must("POST", prefix+"/competitors", map[string]string{"name": fmt.Sprintf("Fencer %d", i), "club": fmt.Sprintf("Club %d", i%3)}, nil)
	}
	h.must("PUT", prefix+"/tournament", map[string]int{"mats": mats, "minPoolSize": 4, "maxPoolSize": 6}, nil)
	h.must("POST", prefix+"/tournament/pools", nil, nil)
}

func (h *hall) mats() httpapi.MatsView {
	h.t.Helper()
	var v httpapi.MatsView
	h.must("GET", "/api/mats", nil, &v)
	return v
}

func itemOf(v httpapi.MatsView, id string) httpapi.ItemView {
	for _, it := range v.Items {
		if it.ID == id {
			return it
		}
	}
	return httpapi.ItemView{}
}

func TestTheMatsAreTheEvents(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.draw("/api/d/"+ls, 12, 2)

	// One discipline: its mats are the event's, exactly as before.
	v := h.mats()
	if len(v.Mats) != 2 || itemOf(v, ls+"/pool-1").Mat != 1 || itemOf(v, ls+"/pool-2").Mat != 2 {
		t.Fatalf("a discipline on its own should keep its mats: %+v", v.Items)
	}
	if c := v.Mats[0].Current; c == nil || c.Discipline != ls || c.Red == "" {
		t.Errorf("mat 1 should be on the longsword's first match, names and all: %+v", c)
	}

	h.must("POST", "/api/disciplines", map[string]string{"name": "Open Sabre"}, nil)
	h.draw("/api/d/open-sabre", 8, 2)
	v = h.mats()
	sabre := itemOf(v, "open-sabre/pool-1")
	behind := 0
	for _, it := range v.Items {
		// The longsword's drawn work; its bracket, not drawn yet, is planned after Sabre's.
		if it.Mat == 1 && it.Discipline == ls && !it.Projected {
			behind++
		}
	}
	if sabre.Mat != 1 || sabre.Position != behind+1 {
		t.Fatalf("Sabre's first pool should queue behind the longsword's %d on mat 1: %+v", behind, sabre)
	}
	q := v.Mats[0].Queue
	if q[0].Discipline != ls || q[len(q)-1].Discipline != "open-sabre" {
		t.Errorf("mat 1's queue should run the longsword's pool, then Sabre's")
	}

	// The mat board: Sabre's pool to the front of mat 2.
	if code := h.do("PATCH", "/api/plan/items/open-sabre/pool-1", map[string]int{"mat": 2, "index": 0}, &v); code != 200 {
		t.Fatalf("moving an item returned %d", code)
	}
	if it := itemOf(v, "open-sabre/pool-1"); it.Mat != 2 || it.Position != 1 {
		t.Errorf("Sabre's pool should be first on mat 2: %+v", it)
	}
	// Every page of the discipline names the mat its pool is on now.
	var s httpapi.Snapshot
	h.must("GET", "/api/d/open-sabre/state", nil, &s)
	if s.Pools[0].Number != 1 || s.Pools[0].Mat != 2 || s.EventMats != 2 {
		t.Errorf("Sabre's snapshot should say its pool 1 runs on mat 2 of 2: %+v", s.Pools[0])
	}

	// A discipline's own pool controls move the plan too.
	h.must("PATCH", "/api/d/open-sabre/tournament/pools/1", map[string]int{"mat": 1}, nil)
	if it := itemOf(h.mats(), "open-sabre/pool-1"); it.Mat != 1 {
		t.Errorf("the pool's mat picker should move its work item: %+v", it)
	}

	// A third mat, for the afternoon.
	h.must("PUT", "/api/mats", map[string]int{"count": 3}, &v)
	if len(v.Mats) != 3 {
		t.Errorf("the hall should have three mats, got %d", len(v.Mats))
	}
	if code := h.do("PUT", "/api/mats", map[string]int{"count": 99}, nil); code != 400 {
		t.Errorf("a hall of 99 mats should be refused, got %d", code)
	}
}

// What a mat is running cannot be moved, and nothing jumps in front of it.
func TestARunningItemStays(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.draw("/api/d/"+ls, 8, 1)

	v := h.mats()
	first := v.Mats[0].Current.Match.ID
	h.must("POST", "/api/d/"+ls+"/matches/"+first+"/events",
		[]map[string]any{{"seq": 1, "type": "timer", "timer": map[string]string{"action": "start"}}}, nil)

	if code := h.do("PATCH", "/api/plan/items/"+ls+"/pool-1", map[string]int{"mat": 1, "index": 5}, nil); code != 409 {
		t.Errorf("a pool under way should not move; got %d", code)
	}
	h.must("PATCH", "/api/plan/items/"+ls+"/pool-2", map[string]int{"mat": 1, "index": 0}, &v)
	if p1, p2 := itemOf(v, ls+"/pool-1"), itemOf(v, ls+"/pool-2"); p1.Position != 1 || p2.Position != 2 {
		t.Errorf("pool 2 cannot go in front of the pool under way: %d %d", p1.Position, p2.Position)
	}
}

// A score keeper is the event's: it registers at a physical mat, the mat holds the match
// it holds, and leaving lets go of its claim in whichever discipline that was.
func TestAScoreKeeperBelongsToTheMat(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.draw("/api/d/"+ls, 8, 1)
	v := h.mats()
	queue := v.Mats[0].Queue
	second := queue[1].Match.ID

	h.must("POST", "/api/clients/tab1", map[string]any{"role": "scorekeeper", "name": "Tablet", "mat": 1, "match": second, "discipline": ls}, nil)
	if c := h.mats().Mats[0].Current; c == nil || c.Match.ID != second {
		t.Errorf("the mat should be on the match its score keeper holds: %+v", c)
	}
	h.must("POST", "/api/d/"+ls+"/matches/"+second+"/claim", map[string]any{"client": "tab1"}, nil)
	var pres httpapi.EventPresence
	h.must("GET", "/api/presence", nil, &pres)
	if len(pres.Clients) != 1 || pres.Clients[0].Discipline != ls {
		t.Errorf("the event should list the device with its discipline: %+v", pres.Clients)
	}

	h.must("POST", "/api/clients/tab1/release", nil, nil)
	// Released: another device claims without taking anything over.
	var claim struct {
		TookOverFrom string `json:"tookOverFrom"`
	}
	h.must("POST", "/api/d/"+ls+"/matches/"+second+"/claim", map[string]any{"client": "tab2"}, &claim)
	if claim.TookOverFrom != "" {
		t.Errorf("leaving the mat should have let go of the match, but tab2 took it over from %q", claim.TookOverFrom)
	}

	h.must("PUT", "/api/clients/scr1/target", map[string]string{"target": "mat/1"}, nil)
	var me httpapi.Client
	h.must("POST", "/api/clients/scr1", map[string]any{"role": "display", "name": "Screen"}, &me)
	if me.Target != "mat/1" {
		t.Errorf("a screen's assignment is the event's to remember, got %q", me.Target)
	}
}

// The score keepers and the screens follow the mats on one stream.
func TestTheMatsStreamFollowsEveryDiscipline(t *testing.T) {
	h := openHall(t, t.TempDir())
	var ev httpapi.EventView
	h.must("GET", "/api/event", nil, &ev)
	ls := ev.Disciplines[0].Slug
	h.draw("/api/d/"+ls, 8, 1)

	res, err := http.Get(h.url + "/api/mats/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	frames := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				frames <- strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)

	first := h.mats().Mats[0].Current.Match.ID
	h.must("POST", "/api/d/"+ls+"/matches/"+first+"/events",
		[]map[string]any{{"seq": 1, "type": "timer", "timer": map[string]string{"action": "start"}}}, nil)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case f := <-frames:
			var u struct {
				Kind string           `json:"kind"`
				Data httpapi.MatsView `json:"data"`
			}
			if json.Unmarshal([]byte(f), &u) != nil || u.Kind != "mats" {
				continue
			}
			if c := u.Data.Mats[0].Current; c != nil && c.Match.Status == "running" {
				return
			}
		case <-deadline:
			t.Fatal("the mats stream never carried the running match")
		}
	}
}
