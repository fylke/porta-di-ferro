package httpapi

import (
	"bytes"
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
	"time"
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

// Disciplines is the list an organizer picks from rather than types out (issue #80). The
// first one is what a run that has not been named yet is offered.
//
// Preloaded rather than configurable: these are the four MSL runs, they are spelled the
// same way on every entry list, and a volunteer typing "Womens longsword" at one event
// and "Women's and underrepresented genders Longsword" at the next makes two disciplines
// out of one. Nothing stops them typing their own -- the list is a starting point, not a
// closed set.
var Disciplines = []string{
	"Open steel Longsword",
	"Women's and underrepresented genders Longsword",
	"Open Sabre",
	"Open foam Longsword",
}

// cleanName is the shared check on a discipline's name: something readable, and short
// enough to sit in a page header beside the mat number.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a discipline needs a name")
	}
	if len([]rune(name)) > 60 {
		return "", errors.New("that name is too long to fit on the screens")
	}
	if strings.TrimSpace(badName.ReplaceAllString(name, "")) == "" {
		return "", errors.New("the name needs some letters or digits in it")
	}
	return name, nil
}

// rename changes what this run of the application is called.
//
// The name reaches a run as a flag, which is fine for a sibling the organizer started
// from here and useless for the first one: it was started by a shortcut, so it had no
// name and every page said "Unnamed" with nowhere to change it (issue #80). So the name
// is kept with the tournament it belongs to, and this writes it there as well as into
// the live instance, which is what makes it survive a restart.
func (s *Server) rename(name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	t, err := s.store.Tournament()
	if err != nil {
		s.writeMu.Unlock()
		return err
	}
	t.Discipline = name
	err = s.store.SaveTournament(t)
	s.writeMu.Unlock()
	if err != nil {
		return err
	}
	s.instances.mu.Lock()
	s.instances.self.Name = name
	s.instances.mu.Unlock()
	// Every page carries the name, so every page has to hear about it.
	s.publishState()
	return nil
}

// renameChild passes a rename on to a sibling this instance started, then records it, so
// the organizer can name all of their disciplines from the one page they are already on.
// A sibling is a separate process with its own tournament directory; it is the one that
// has to write the name down.
func (s *Server) renameChild(port int, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	s.instances.mu.Lock()
	c, ok := s.instances.children[port]
	url := ""
	if ok {
		url = c.URL
	}
	s.instances.mu.Unlock()
	if !ok {
		return fmt.Errorf("no discipline on port %d was started from here", port)
	}

	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%sapi/instances/%d", url, port), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s did not answer: %w", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("%s refused the rename with %d", url, res.StatusCode)
	}

	s.instances.mu.Lock()
	if c, ok := s.instances.children[port]; ok {
		c.Name = name
	}
	s.instances.mu.Unlock()
	return nil
}

// spawn starts a sibling on the next free port, with a data directory beside this one.
func (s *Server) spawn(name string) (Instance, error) {
	name, err := cleanName(name)
	if err != nil {
		return Instance{}, err
	}
	folder := strings.TrimSpace(badName.ReplaceAllString(name, ""))
	exe, err := os.Executable()
	if err != nil {
		return Instance{}, err
	}
	s.instances.mu.Lock()
	defer s.instances.mu.Unlock()
	if strings.EqualFold(s.instances.self.Name, name) {
		return Instance{}, fmt.Errorf("%s is this one", s.instances.self.Name)
	}
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

// self is this run, read under the lock. It is no longer fixed at startup: renaming a
// discipline writes to it while the snapshot builder is reading (issue #80).
func (s *Server) self() Instance {
	s.instances.mu.Lock()
	defer s.instances.mu.Unlock()
	return s.instances.self
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

// patchInstance renames a discipline: this one, or a sibling this one started.
func (s *Server) patchInstance(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.instances.mu.Lock()
	self := s.instances.self.Port == port
	s.instances.mu.Unlock()
	if self {
		err = s.rename(in.Name)
	} else {
		err = s.renameChild(port, in.Name)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.listInstances())
}

// getDisciplines is the preloaded list the organizer picks from.
func (s *Server) getDisciplines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Disciplines)
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
