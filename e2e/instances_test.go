package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type instance struct {
	Name   string `json:"name"`
	Port   int    `json:"port"`
	Dir    string `json:"dir"`
	Parent string `json:"parent"`
	Self   bool   `json:"self"`
	URL    string `json:"url"`
}

// TestAnInstanceStartsASiblingDiscipline is design §7 item 9 against the real binary: the
// first run starts a second on the next free port with its own directory beside it, the
// second knows its name and its parent, and the first can stop it.
func TestAnInstanceStartsASiblingDiscipline(t *testing.T) {
	s := start(t)

	var list []instance
	s.mustDo(t, "GET", "/api/instances", nil, &list)
	if len(list) != 1 || !list[0].Self {
		t.Fatalf("a fresh run should list only itself, got %+v", list)
	}

	var child instance
	s.mustDo(t, "POST", "/api/instances", map[string]string{"name": "Sabre"}, &child)
	// Whatever happens below, the sibling must not outlive the test.
	t.Cleanup(func() { s.do(t, "DELETE", fmt.Sprintf("/api/instances/%d", child.Port), nil, nil) })

	if child.Name != "Sabre" || child.Port <= 0 || child.Parent != fmt.Sprintf("http://localhost:%d/", portOf(s)) {
		t.Fatalf("the sibling should be named, on a port, and know its parent: %+v", child)
	}
	if filepath.Dir(child.Dir) != filepath.Dir(s.dir) || filepath.Base(child.Dir) != "Sabre" {
		t.Errorf("the sibling's directory should sit beside this one, named for it: %s beside %s", child.Dir, s.dir)
	}

	// It comes up on its own, as a full instance that knows which discipline it is.
	deadline := time.Now().Add(20 * time.Second)
	var snap struct {
		Instance instance `json:"instance"`
	}
	for {
		res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/state", child.Port))
		if err == nil {
			ok := res.StatusCode == http.StatusOK
			if ok {
				_ = json.NewDecoder(res.Body).Decode(&snap)
			}
			res.Body.Close()
			if ok {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the sibling never came up")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if snap.Instance.Name != "Sabre" || snap.Instance.Port != child.Port || snap.Instance.Parent == "" {
		t.Errorf("the sibling's snapshot should name it and its parent: %+v", snap.Instance)
	}
	if _, err := os.Stat(child.Dir); err != nil {
		t.Errorf("the sibling should have made its directory: %v", err)
	}

	s.mustDo(t, "GET", "/api/instances", nil, &list)
	if len(list) != 2 || list[1].Name != "Sabre" || list[1].Self {
		t.Errorf("the parent should list itself and the sibling, got %+v", list)
	}
	if code := s.do(t, "POST", "/api/instances", map[string]string{"name": "sabre"}, nil); code != 400 {
		t.Errorf("starting the same discipline twice should be refused, got %d", code)
	}
	if code := s.do(t, "POST", "/api/instances", map[string]string{"name": "  "}, nil); code != 400 {
		t.Errorf("a nameless discipline should be refused, got %d", code)
	}

	s.mustDo(t, "DELETE", fmt.Sprintf("/api/instances/%d", child.Port), nil, nil)
	deadline = time.Now().Add(10 * time.Second)
	for {
		s.mustDo(t, "GET", "/api/instances", nil, &list)
		if len(list) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stopped sibling should have left the list, got %+v", list)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func portOf(s *server) int {
	var port int
	fmt.Sscanf(s.base, "http://127.0.0.1:%d", &port)
	return port
}
