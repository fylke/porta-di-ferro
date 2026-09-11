// Command porta is the local product: the server an organizer runs on their own PC.
//
// It is the whole application. Download one file, run it, and a tournament is running on
// the venue LAN in under five minutes -- that is the acceptance criterion the project
// exists to meet (docs/design.md §1), not packaging polish at the end.
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

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/lan"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/web"
)

// version is stamped by the release workflow with -ldflags. An organizer should be able
// to pin a known-good build and say which one they are running.
var version = "dev"

func main() {
	dir := flag.String("dir", defaultDir(), "tournament data directory")
	port := flag.Int("port", 8080, "port to listen on")
	noBrowser := flag.Bool("no-browser", false, "do not open a browser on start")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("porta-di-ferro", version)
		return
	}

	st, err := store.Open(*dir)
	if err != nil {
		fatal("could not open the tournament directory %s: %v", *dir, err)
	}

	srv := httpapi.New(st, web.Assets())
	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
		// No read timeout: an SSE stream is meant to stay open.
		ReadHeaderTimeout: 10 * time.Second,
	}

	addrs := lan.Addresses()
	clients := clientURL(addrs, *port)
	banner(addrs, *dir, *port)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("could not listen on port %d: %v\n\n"+
				"Another program may already be using it. Try: porta -port 8081", *port, err)
		}
	}()

	if !*noBrowser {
		// Choosing a server over a desktop application means "now open your browser" is
		// part of the install. Opening it ourselves is the mitigation.
		openBrowser(fmt.Sprintf("http://localhost:%d/", *port))
	}

	quit := make(chan struct{})
	go runTray(clients, fmt.Sprintf("http://localhost:%d/", *port), quit)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case <-quit:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	fmt.Println("\nStopped. Your tournament is saved in", *dir)
}

func banner(addrs []lan.Address, dir string, port int) {
	fmt.Println()
	fmt.Println("  Porta di Ferro", version)
	fmt.Println()
	fmt.Println("  Organizer      http://localhost:" + fmt.Sprint(port) + "/   (this PC only)")
	if len(addrs) == 0 {
		fmt.Println()
		fmt.Println("  No network address found -- clients on other devices cannot reach this PC.")
		fmt.Println("  Join this PC to the venue wifi and restart.")
	} else {
		base := url(addrs[0], port)
		fmt.Println("  Score keepers  " + base + "/score   (" + addrs[0].Interface + ")")
		fmt.Println("  Displays       " + base + "/display/mats")
		// Every other network this PC is on, named. An organizer whose PC is on both a
		// wired office LAN and the hall wifi cannot be guessed at from here, and being
		// able to see the alternatives is what makes the choice on the organizer page
		// obvious rather than a shot in the dark.
		if len(addrs) > 1 {
			fmt.Println()
			fmt.Println("  Also reachable on:")
			for _, a := range addrs[1:] {
				fmt.Printf("                 %-24s (%s)\n", url(a, port), a.Interface)
			}
		}
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

// clientURL is the address the tray offers to copy: the best guess, or nothing at all
// rather than a localhost URL that would not work on the device it was pasted into.
func clientURL(addrs []lan.Address, port int) string {
	if len(addrs) == 0 {
		return ""
	}
	return url(addrs[0], port)
}

func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "tournament"
	}
	return filepath.Join(home, "Porta di Ferro", "tournament")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  "+format+"\n\n", args...)
	log.SetFlags(0)
	os.Exit(1)
}
