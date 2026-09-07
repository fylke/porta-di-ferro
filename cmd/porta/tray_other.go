//go:build !windows

package main

// runTray is a no-op away from Windows. The tray icon exists to stop the organizer's
// server from being a console window they might close by accident, which is a Windows
// problem: the shipped artifact is a Windows installer.
func runTray(lan, local string, quit chan<- struct{}) {}
