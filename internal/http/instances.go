package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Concurrent disciplines (design §7 item 9). Several disciplines at once are several
// runs of the application at once: each is its own process, port and data directory,
// because one run of the application is one tournament under one ruleset and nothing
// below this line has to know otherwise.
//
// What this adds is the organizer not having to open a terminal to get the second one.
// The first instance -- the one the installer's shortcut starts -- can spawn siblings on
// the next free ports, each with its own directory beside the first, and lists them with
// their addresses. Each sibling shows its name on every page, so an organizer with two
// browser tabs and a hall of screens can tell at a glance which discipline any of them is
// on; the port already keeps their URLs apart.

// Instance describes one running copy of the application.
type Instance struct {
	// Name is the discipline: "Longsword", "Sabre". Empty on an unnamed first instance.
	Name string `json:"name"`
	Port int    `json:"port"`
	Dir  string `json:"dir"`
	// Parent is the URL of the instance that started this one, or empty for the first.
	Parent string `json:"parent,omitempty"`
	// Self marks the instance answering the request, in a list of siblings.
	Self bool   `json:"self"`
	URL  string `json:"url"`
}

// instances is the parent's record of the siblings it started. A child knows only its
// parent, by URL, which is enough for a way back.
type instances struct {
	mu       sync.Mutex
	self     Instance
	children map[int]*child
}

type child struct {
	Instance
	cmd *exec.Cmd
}

func newInstances(self Instance) *instances {
	self.Self = true
	self.URL = fmt.Sprintf("http://localhost:%d/", self.Port)
	return &instances{self: self, children: map[int]*child{}}
}

var badName = regexp.MustCompile(`[^\p{L}\p{N} _-]+`)

// spawn starts a sibling on the next free port, with a data directory beside this one.
func (s *Server) spawn(name string) (Instance, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Instance{}, errors.New("a discipline needs a name")
	}
	folder := strings.TrimSpace(badName.ReplaceAllString(name, ""))
	if folder == "" {
		return Instance{}, errors.New("the name needs some letters or digits in it")
	}
	exe, err := os.Executable()
	if err != nil {
		return Instance{}, err
	}
	s.instances.mu.Lock()
	defer s.instances.mu.Unlock()
	for _, c := range s.instances.children {
		if strings.EqualFold(c.Name, name) {
			return Instance{}, fmt.Errorf("%s is already running on port %d", c.Name, c.Port)
		}
	}
	port, err := freePortAfter(s.instances.self.Port)
	if err != nil {
		return Instance{}, err
	}
	dir := filepath.Join(filepath.Dir(s.instances.self.Dir), folder)
	cmd := exec.Command(exe,
		"-dir", dir,
		"-port", strconv.Itoa(port),
		"-name", name,
		"-parent", s.instances.self.URL,
		"-no-browser",
	)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return Instance{}, err
	}
	inst := Instance{Name: name, Port: port, Dir: dir, Parent: s.instances.self.URL,
		URL: fmt.Sprintf("http://localhost:%d/", port)}
	s.instances.children[port] = &child{Instance: inst, cmd: cmd}
	// Reap it when it exits on its own -- its tray icon's Quit, say -- so the list
	// stops showing it.
	go func() {
		_ = cmd.Wait()
		s.instances.mu.Lock()
		delete(s.instances.children, port)
		s.instances.mu.Unlock()
	}()
	return inst, nil
}

// stop ends a sibling this instance started.
func (s *Server) stopChild(port int) error {
	s.instances.mu.Lock()
	c, ok := s.instances.children[port]
	s.instances.mu.Unlock()
	if !ok {
		return fmt.Errorf("no discipline on port %d was started from here", port)
	}
	return c.cmd.Process.Kill()
}

// StopChildren ends every sibling this instance started. The first instance calls it on
// its way out, so closing the one the organizer started closes the rest.
func (s *Server) StopChildren() {
	s.instances.mu.Lock()
	defer s.instances.mu.Unlock()
	for _, c := range s.instances.children {
		_ = c.cmd.Process.Kill()
	}
}

func (s *Server) listInstances() []Instance {
	s.instances.mu.Lock()
	defer s.instances.mu.Unlock()
	out := []Instance{s.instances.self}
	for _, c := range s.instances.children {
		out = append(out, c.Instance)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

// freePortAfter finds the first port above the given one that nothing is listening on.
func freePortAfter(port int) (int, error) {
	for p := port + 1; p < port+100; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err != nil {
			continue
		}
		l.Close()
		return p, nil
	}
	return 0, fmt.Errorf("no free port found between %d and %d", port+1, port+100)
}

// --- handlers ---------------------------------------------------------------------

func (s *Server) getInstances(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.listInstances())
}

func (s *Server) postInstance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	inst, err := s.spawn(in.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, inst)
}

func (s *Server) deleteInstance(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.stopChild(port); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
