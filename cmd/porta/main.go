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
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
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

	lan := lanURL(*port)
	banner(lan, *dir, *port)

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
	go runTray(lan, fmt.Sprintf("http://localhost:%d/", *port), quit)

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

func banner(lan, dir string, port int) {
	fmt.Println()
	fmt.Println("  Porta di Ferro", version)
	fmt.Println()
	fmt.Println("  Organizer      http://localhost:" + fmt.Sprint(port) + "/")
	if lan != "" {
		fmt.Println("  Score keepers  " + lan + "/score")
		fmt.Println("  Displays       " + lan + "/display/mats")
	} else {
		fmt.Println("  No network address found -- clients on other devices cannot reach this PC.")
	}
	fmt.Println("  Data           " + dir)
	fmt.Println()
	fmt.Println("  The organizer page shows a QR code for the clients. Leave this window open.")
	fmt.Println()
}

// lanURL finds the address a tablet on the venue LAN can actually reach. Preferring a
// private range keeps it off virtual adapters where possible.
func lanURL(port int) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	var fallback string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}
		url := fmt.Sprintf("http://%s:%d", ip.String(), port)
		if ip.IsPrivate() {
			return url
		}
		if fallback == "" {
			fallback = url
		}
	}
	return fallback
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
