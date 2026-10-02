package watchdog_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/fylke/porta-di-ferro/internal/watchdog"
)

// The child, when this test binary is started by the watchdog: it counts its starts in a
// file and exits with the code listed for that start.
func TestMain(m *testing.M) {
	if watchdog.IsChild() {
		counter := os.Getenv("WATCHDOG_TEST_COUNTER")
		n := 0
		if b, err := os.ReadFile(counter); err == nil {
			n, _ = strconv.Atoi(string(b))
		}
		n++
		_ = os.WriteFile(counter, []byte(strconv.Itoa(n)), 0o644)
		codes := []int{2, 1, 0}
		if os.Getenv("WATCHDOG_TEST_ALWAYS") != "" {
			codes = []int{2}
		}
		code := codes[len(codes)-1]
		if n <= len(codes) {
			code = codes[n-1]
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestTheDecision(t *testing.T) {
	d := watchdog.NewDecider(watchdog.Policy{MaxRestarts: 2, Window: time.Minute})
	now := time.Now()
	if d.Restart(0, now) {
		t.Error("a clean exit is the organizer quitting, not a crash")
	}
	if d.Restart(watchdog.ExitNoRestart, now) {
		t.Error("a failure a restart cannot fix should not be restarted")
	}
	if !d.Restart(2, now) || !d.Restart(2, now.Add(time.Second)) {
		t.Error("a panic should be restarted")
	}
	if d.Restart(2, now.Add(2*time.Second)) {
		t.Error("a third crash inside the window is a child that cannot stay up")
	}
	if !d.Restart(2, now.Add(2*time.Minute)) {
		t.Error("crashes outside the window should not count against it")
	}
}

// The real thing: the watchdog starts the child, which panics, then fails, then quits.
// It is started three times and the watchdog ends with the clean exit.
func TestItRestartsACrashedChild(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "starts")
	t.Setenv("WATCHDOG_TEST_COUNTER", counter)
	var lines []string
	logf := func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }

	code := watchdog.Run([]string{"-test.run=^$"}, nil,
		watchdog.Policy{MaxRestarts: 5, Window: time.Minute, Delay: 10 * time.Millisecond}, logf)
	if code != 0 {
		t.Errorf("the watchdog should end with the child's clean exit, got %d (%v)", code, lines)
	}
	b, _ := os.ReadFile(counter)
	if string(b) != "3" {
		t.Errorf("the child should have been started three times, got %s", b)
	}
	if len(lines) != 2 {
		t.Errorf("each restart should be said once, got %v", lines)
	}
}

func TestItGivesUpOnAChildThatCannotStayUp(t *testing.T) {
	t.Setenv("WATCHDOG_TEST_COUNTER", filepath.Join(t.TempDir(), "starts"))
	t.Setenv("WATCHDOG_TEST_ALWAYS", "1")
	code := watchdog.Run([]string{"-test.run=^$"}, nil,
		watchdog.Policy{MaxRestarts: 2, Window: time.Minute, Delay: time.Millisecond}, func(string, ...any) {})
	if code != 2 {
		t.Errorf("the watchdog should give up with the child's code, got %d", code)
	}
}
