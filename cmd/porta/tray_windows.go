//go:build windows

package main

import (
	"os/exec"

	"fyne.io/systray"
)

// runTray puts the server in the notification area with Open and Quit.
//
// Small, and worth having: without it the organizer's server is a console window, and a
// console window is a thing people close. Listed in docs/tech-stack.md §2 as part of the
// mitigation for "the organizer gets a browser tab, not an application".
func runTray(lan, local string, quit chan<- struct{}) {
	onReady := func() {
		systray.SetTitle("Porta di Ferro")
		systray.SetTooltip("Porta di Ferro is running")
		open := systray.AddMenuItem("Open", "Open the organizer view")
		var clients *systray.MenuItem
		if lan != "" {
			clients = systray.AddMenuItem("Copy client address", lan)
		}
		systray.AddSeparator()
		stop := systray.AddMenuItem("Quit", "Stop the server")

		for {
			var clientsClicked <-chan struct{}
			if clients != nil {
				clientsClicked = clients.ClickedCh
			}
			select {
			case <-open.ClickedCh:
				openBrowser(local)
			case <-clientsClicked:
				// clip is on every Windows install, which beats pulling in a clipboard
				// library for one menu item.
				cmd := exec.Command("cmd", "/c", "echo "+lan+"|clip")
				_ = cmd.Run()
			case <-stop.ClickedCh:
				systray.Quit()
				close(quit)
				return
			}
		}
	}
	systray.Run(onReady, func() {})
}
