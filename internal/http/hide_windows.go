//go:build windows

package httpapi

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps a spawned sibling from opening a console window of its own. It still
// gets a tray icon, which is where its Quit lives; the console was only ever a thing
// people close by accident.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
