// Package watchdog restarts the server when it dies.
//
// Every discipline of an event runs in one process (docs/proposals/one-event-many-
// disciplines.md §5), so a crash takes them all down at once. The cheapest "let it crash"
// for an application whose whole state is on disk is coarse: start it again and let it
// read its files. The executable starts itself as a child and waits; when the child dies
// it is started again, and score keepers ride through the gap as they ride through a wifi
// drop, resending what they could not deliver. No supervision code inside the application.
//
// What it does not do is restart a child that stopped on purpose -- the organizer quit
// from the tray, or pressed Ctrl+C -- or one that failed in a way starting again cannot
// fix: the port is taken, the folder cannot be opened. The child says which by its exit
// code. And it gives up on a child that keeps dying straight away, rather than looping.
package watchdog

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// childEnv marks the child, so it runs the server instead of watching it.
const childEnv = "PORTA_WATCHED"

// ExitNoRestart is the exit code for a failure a restart cannot fix.
const ExitNoRestart = 3

// IsChild reports whether this process is the one being watched.
func IsChild() bool { return os.Getenv(childEnv) == "1" }

// Policy is when to give up: more than MaxRestarts within Window is a child that cannot
// stay up, and starting it again would only print the same error forever.
type Policy struct {
	MaxRestarts int
	Window      time.Duration
	Delay       time.Duration
}

// Default is five crashes inside a minute, a second apart.
var Default = Policy{MaxRestarts: 5, Window: time.Minute, Delay: time.Second}

// Decider applies a Policy to a run of exits.
type Decider struct {
	policy  Policy
	crashes []time.Time
}

// NewDecider starts counting.
func NewDecider(p Policy) *Decider { return &Decider{policy: p} }

// Restart says whether a child that exited with code at now should be started again.
func (d *Decider) Restart(code int, now time.Time) bool {
	if code == 0 || code == ExitNoRestart {
		return false
	}
	kept := d.crashes[:0]
	for _, at := range d.crashes {
		if now.Sub(at) < d.policy.Window {
			kept = append(kept, at)
		}
	}
	d.crashes = append(kept, now)
	return len(d.crashes) <= d.policy.MaxRestarts
}

// Run watches this executable, started with args, and returns the exit code the watchdog
// should end with. later is added to the arguments of every start after the first -- the
// browser opens once, not on every restart.
func Run(args, later []string, p Policy, logf func(string, ...any)) int {
	exe, err := os.Executable()
	if err != nil {
		logf("porta: cannot find this executable to watch it: %v", err)
		return 1
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	decide := NewDecider(p)
	for start := 0; ; start++ {
		cmdArgs := args
		if start > 0 {
			cmdArgs = append(append([]string{}, args...), later...)
		}
		cmd := exec.Command(exe, cmdArgs...)
		cmd.Env = append(os.Environ(), childEnv+"=1")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			logf("porta: could not start the server: %v", err)
			return 1
		}

		exited := make(chan int, 1)
		go func() { exited <- exitCode(cmd.Wait()) }()

		var code int
		select {
		case code = <-exited:
		case <-stop:
			// The organizer stopping the server. A console Ctrl+C reaches the child too;
			// a signal sent to this process alone is passed on. Either way, wait for the
			// child to save and go, and do not start another.
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				_ = cmd.Process.Kill()
			}
			<-exited
			return 0
		}

		if !decide.Restart(code, time.Now()) {
			if code != 0 && code != ExitNoRestart {
				logf("porta: the server stopped %d times in %s; not starting it again.", p.MaxRestarts+1, p.Window)
			}
			return code
		}
		logf("porta: the server stopped unexpectedly (exit %d). Starting it again; your data is on disk.", code)
		select {
		case <-time.After(p.Delay):
		case <-stop:
			return 0
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}

// Describe is the one-line account of a policy, for the startup banner.
func (p Policy) Describe() string {
	return fmt.Sprintf("restarts the server if it stops unexpectedly (up to %d times a %s)", p.MaxRestarts, p.Window)
}
