// Package e2e drives the built executable over HTTP, not a dev server or an in-process
// handler. Testing the thing that ships is the point (docs/tech-stack.md §11).
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type server struct {
	base string
	dir  string
	cmd  *exec.Cmd
}

// start builds the binary and runs it against a fresh tournament directory.
func start(t *testing.T) *server {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "porta")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/porta")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}

	port := freePort(t)
	dir := t.TempDir()
	cmd := exec.Command(bin, "-dir", dir, "-port", fmt.Sprint(port), "-no-browser")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the binary: %v", err)
	}

	s := &server{base: fmt.Sprintf("http://127.0.0.1:%d", port), dir: dir, cmd: cmd}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	s.waitReady(t)
	return s
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func (s *server) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(s.base + "/api/state")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the server never became ready")
}

func (s *server) do(t *testing.T, method, path string, body, out any) int {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding request: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, s.base+path, reader)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decoding response: %v", method, path, err)
		}
	}
	return res.StatusCode
}

func (s *server) mustDo(t *testing.T, method, path string, body, out any) {
	t.Helper()
	if code := s.do(t, method, path, body, out); code < 200 || code > 299 {
		t.Fatalf("%s %s returned %d", method, path, code)
	}
}
