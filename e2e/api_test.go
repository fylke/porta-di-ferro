package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestCompetitorNameValidationIsConsistent guards the asymmetry a review caught: POST
// rejected an empty name and PATCH accepted one, so a competitor could be blanked after
// entry. A blank name is not a harmless record -- it reaches the scoreboard and the
// printed pool sheets, where nobody can tell who is meant to be fencing.
func TestCompetitorNameValidationIsConsistent(t *testing.T) {
	s := start(t)

	if code := s.do(t, "POST", "/api/competitors", map[string]string{"name": "  "}, nil); code != http.StatusBadRequest {
		t.Errorf("POST with a blank name returned %d, want 400", code)
	}

	var c store.Competitor
	s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": "Ada", "club": "MSL"}, &c)

	for _, blank := range []string{"", "   ", "\t"} {
		code := s.do(t, "PATCH", "/api/competitors/"+c.ID, map[string]string{"name": blank}, nil)
		if code != http.StatusBadRequest {
			t.Errorf("PATCH with name %q returned %d, want 400", blank, code)
		}
	}

	var after []store.Competitor
	s.mustDo(t, "GET", "/api/state", nil, &struct {
		Competitors *[]store.Competitor `json:"competitors"`
	}{&after})
	if len(after) != 1 || after[0].Name != "Ada" {
		t.Errorf("the rejected patches should have left the name alone, got %+v", after)
	}
}

// TestDeletingAnUnknownCompetitorIs404 guards the other half of the same asymmetry: PATCH
// already 404s on an unknown id, while DELETE reported success and quietly rewrote the
// same list, which would hide a client bug rather than surface it.
func TestDeletingAnUnknownCompetitorIs404(t *testing.T) {
	s := start(t)

	if code := s.do(t, "DELETE", "/api/competitors/c999", nil, nil); code != http.StatusNotFound {
		t.Errorf("deleting an id that was never there returned %d, want 404", code)
	}

	var c store.Competitor
	s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": "Bo"}, &c)
	if code := s.do(t, "DELETE", "/api/competitors/"+c.ID, nil, nil); code != http.StatusOK {
		t.Errorf("deleting a real competitor returned %d, want 200", code)
	}
	if code := s.do(t, "DELETE", "/api/competitors/"+c.ID, nil, nil); code != http.StatusNotFound {
		t.Errorf("deleting the same competitor twice returned %d, want 404", code)
	}
}

// TestMissingFilesAre404WhileClientRoutesFallThrough guards the SPA fallthrough. Answering
// a stale hashed asset with 200 and a page of HTML makes the browser fail to parse it as a
// script, which is a miserable thing to debug at a venue.
func TestMissingFilesAre404WhileClientRoutesFallThrough(t *testing.T) {
	s := start(t)

	for _, path := range []string{
		"/assets/index-DOESNOTEXIST.js",
		"/assets/index-DOESNOTEXIST.css",
		"/nope.js",
		"/not-a-favicon.ico",
	} {
		res, err := http.Get(s.base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s returned %d, want 404", path, res.StatusCode)
		}
	}

	// The graphical profile ships in the binary like the bundle does, and a browser asks
	// for /favicon.ico without being told to. Answering that with a page of HTML is what
	// the 404 above is there to prevent, so the icons are checked for rather than assumed.
	for path, want := range map[string]string{
		"/icon.svg":              "image/svg+xml",
		"/icon-maskable.svg":     "image/svg+xml",
		"/favicon.ico":           "image/x-icon",
		"/icon-180.png":          "image/png",
		"/icon-192.png":          "image/png",
		"/icon-512.png":          "image/png",
		"/icon-maskable-512.png": "image/png",
		"/manifest.webmanifest":  "",
	} {
		res, err := http.Get(s.base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s returned %d, want 200", path, res.StatusCode)
			continue
		}
		if want != "" && !strings.HasPrefix(res.Header.Get("Content-Type"), want) {
			t.Errorf("GET %s served %s, want %s", path, res.Header.Get("Content-Type"), want)
		}
	}

	// Client-side routes have no extension and must still reach the app.
	for _, path := range []string{"/", "/score", "/score/1", "/display/mats", "/display/mat/1", "/print/pools", "/brand"} {
		res, err := http.Get(s.base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s returned %d, want 200", path, res.StatusCode)
		}
		if ct := res.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("GET %s served %s, want HTML", path, ct)
		}
	}
}
