//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// openBrowser is a convenience on the platforms that are not the shipped target. The MVP
// installs on Windows; a Linux build arrives with the cloud mirror in Milestone 3.
func openBrowser(url string) {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	_ = exec.Command(cmd, url).Start()
}
