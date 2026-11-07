// Package hall is what an event knows about itself, with no server in it: each discipline's
// derived picture, the work on the event's mats, the plan's times, its people and staff as
// the pages read them, and the printed sheets.
//
// Everything here is a pure function of its input. That is the boundary: internal/http
// reads these from the disk and serves them, and internal/demo reads them from memory and
// answers the same calls in a browser tab, so the two cannot come to different answers
// (docs/architecture.md §1a, §1b). Nothing here opens a file, starts a goroutine or knows
// what a request is.
package hall

import (
	"errors"
	"regexp"
	"strings"
)

// Instance describes which discipline a server is, so every page can say it.
type Instance struct {
	// Name is the discipline: "Open steel Longsword". Empty on one not named yet.
	Name string `json:"name"`
	// Slug is the discipline's address in the event: /api/d/{slug}/, /d/{slug}/. It never
	// changes once made, whatever the discipline is renamed to.
	Slug string `json:"slug,omitempty"`
	Dir  string `json:"dir"`
	// URL is where the discipline's own pages are, relative to the event's address.
	URL string `json:"url"`
}

var badName = regexp.MustCompile(`[^\p{L}\p{N} _-]+`)

// Disciplines is the list an organizer picks from rather than types out (issue #80). The
// first one is what a discipline that has not been named yet is offered.
//
// Preloaded rather than configurable: these are the four MSL disciplines, they are spelled
// the same way on every entry list, and a volunteer typing "Womens longsword" at one event
// and "Women's and underrepresented genders Longsword" at the next makes two disciplines
// out of one. Nothing stops them typing their own -- the list is a starting point, not a
// closed set.
var Disciplines = []string{
	"Open steel Longsword",
	"Women's and underrepresented genders Longsword",
	"Open Sabre",
	"Open foam Longsword",
}

// CleanName is the shared check on a discipline's name: something readable, and short
// enough to sit in a page header beside the mat number.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a discipline needs a name")
	}
	if len([]rune(name)) > 60 {
		return "", errors.New("that name is too long to fit on the screens")
	}
	if strings.TrimSpace(badName.ReplaceAllString(name, "")) == "" {
		return "", errors.New("the name needs some letters or digits in it")
	}
	return name, nil
}
