//go:build !windows

package httpapi

import "os/exec"

// hideWindow is a no-op away from Windows, where a spawned process has no console
// window to hide.
func hideWindow(cmd *exec.Cmd) {}
