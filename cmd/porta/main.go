// Command porta is the local product: the server an organizer runs on their own PC.
//
// It is the whole application. Download one file, run it, and a tournament is running on
// the venue LAN in under five minutes -- that is the acceptance criterion the project
// exists to meet (docs/design.md §1), not packaging polish at the end.
//
// One run is one event: every discipline of the day, at one address, in one process
// (docs/proposals/one-event-many-disciplines.md). The process is watched by a copy of
// itself that starts it again if it dies, so a crash costs the hall a few seconds rather
// than the organizer finding a closed window.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/lan"
	"github.com/fylke/porta-di-ferro/internal/watchdog"
	"github.com/fylke/porta-di-ferro/web"
)

// version is stamped by the release workflow with -ldflags. An organizer should be able
// to pin a known-good build and say which one they are running.
var version = "dev"

func main() {
	dir := flag.String("dir", defaultDir(), "event data directory")
	port := flag.Int("port", 8080, "port to listen on")
	noBrowser := flag.Bool("no-browser", false, "do not open a browser on start")
	noWatchdog := flag.Bool("no-watchdog", false, "run the server directly, without restarting it if it stops")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("porta-di-ferro", version)
		return
	}

	if !*noWatchdog && !watchdog.IsChild() {
		// The browser opens on the first start only; a restart is meant to go unnoticed.
		os.Exit(watchdog.Run(os.Args[1:], []string{"-no-browser"}, watchdog.Default, log.Printf))
	}

	folder, err := event.Open(*dir)
	if err != nil {
		fatal("could not open the event folder %s: %v", *dir, err)
	}
	coord, err := httpapi.NewCoordinator(folder, web.Assets())
	if err != nil {
		fatal("could not load the event in %s: %v", *dir, err)
	}
	view := coord.View()

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: coord.Handler(),
		// No read timeout: an SSE stream is meant to stay open.
		ReadHeaderTimeout: 10 * time.Second,
	}

	addrs := lan.Addresses()
	clients := clientURL(addrs, *port)
	banner(addrs, *dir, *port, view)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("could not listen on port %d: %v\n\n"+
				"Another program may already be using it. Try: porta -port 8081", *port, err)
		}
	}()

	if !*noBrowser {
		// Choosing a server over a desktop application means "now open your browser" is
		// part of the install. Opening it ourselves is the mitigation.
		openBrowser(fmt.Sprintf("http://localhost:%d/admin", *port))
	}

	quit := make(chan struct{})
	go runTray(clients, fmt.Sprintf("http://localhost:%d/admin", *port), quit)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case <-quit:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	coord.Close()
	fmt.Println("\nStopped. Your event is saved in", *dir)
}

func banner(addrs []lan.Address, dir string, port int, view httpapi.EventView) {
	fmt.Println()
	if view.Name != "" {
		fmt.Println("  Porta di Ferro", version, "--", view.Name)
	} else {
		fmt.Println("  Porta di Ferro", version)
	}
	fmt.Println()
	fmt.Println("  Organizer      http://localhost:" + fmt.Sprint(port) + "/admin   (this PC only)")
	if len(addrs) == 0 {
		fmt.Println()
		fmt.Println("  No network address found -- clients on other devices cannot reach this PC.")
		fmt.Println("  Join this PC to the venue wifi and restart.")
	} else {
		base := url(addrs[0], port)
		fmt.Println("  Everyone       " + base + "/   (" + describe(addrs[0]) + ")")
		fmt.Println("  Score keepers  " + base + "/score")
		fmt.Println("  Displays       " + base + "/display/mats")
		// Every other network this PC is on, named. An organizer whose PC is on both a
		// wired office LAN and the hall wifi cannot be guessed at from here, and being
		// able to see the alternatives is what makes the choice on the organizer page
		// obvious rather than a shot in the dark.
		if len(addrs) > 1 {
			fmt.Println()
			fmt.Println("  Also reachable on:")
			for _, a := range addrs[1:] {
				fmt.Printf("                 %-24s (%s)\n", url(a, port), describe(a))
			}
		}
	}
	fmt.Println()
	for _, d := range view.Disciplines {
		name := d.Name
		if name == "" {
			name = "(not named yet)"
		}
		line := fmt.Sprintf("  Discipline     %s   /d/%s/", name, d.Slug)
		if d.Error != "" {
			line += "   NOT LOADED: " + d.Error
		}
		fmt.Println(line)
	}
	if view.InfoError != "" {
		fmt.Println("  Event file     could not be read: " + view.InfoError)
	}
	fmt.Println("  Data           " + dir)
	fmt.Println()
	fmt.Println("  The organizer page carries a QR code for the clients, and lets you switch")
	fmt.Println("  network if the address above is not the one the tablets are on.")
	fmt.Println("  Leave this window open.")
	fmt.Println()
}

func url(a lan.Address, port int) string {
	return fmt.Sprintf("http://%s:%d", a.IP, port)
}

// describe names a network the way an organizer would: by its wifi name when it has one,
// and by the adapter otherwise.
func describe(a lan.Address) string {
	if a.SSID != "" {
		return "Wi-Fi " + a.SSID
	}
	return a.Interface
}

// clientURL is the address the tray offers to copy: the best guess, or nothing at all
// rather than a localhost URL that would not work on the device it was pasted into.
func clientURL(addrs []lan.Address, port int) string {
	if len(addrs) == 0 {
		return ""
	}
	return url(addrs[0], port)
}

// defaultDir is the folder every install has used. It held one tournament before events
// existed; it now holds the event, and the tournament that was in it becomes the event's
// first discipline the first time this version opens it.
func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "tournament"
	}
	return filepath.Join(home, "Porta di Ferro", "tournament")
}

// fatal is for failures a restart cannot fix, so the watchdog does not try.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  "+format+"\n\n", args...)
	log.SetFlags(0)
	os.Exit(watchdog.ExitNoRestart)
}
