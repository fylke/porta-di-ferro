//go:build windows

package main

import "os/exec"

// openBrowser starts the organizer's default browser. rundll32 is used rather than
// `cmd /c start` so no console window flashes up on top of the server's own.
func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
