package httpapi

import (
	"github.com/fylke/porta-di-ferro/internal/store"
)

// What the event's pages know about each discipline (proposal §12).
//
// The landing page of an event with several disciplines is the one page every phone in
// the hall is pointed at, so it holds one stream, not one per discipline, and that stream
// carries a summary of each rather than every discipline's full snapshot: enough to say
// where each one has got to, what is on its mats right now, and who is entered, so a
// spectator can find a name without knowing which discipline to look in. A discipline's
// own pages still take its full snapshot from its own stream.

// EventView is the whole event as the landing page and the event admin see it.
type EventView struct {
	// Name is what the event is called: the signup's event name when the organizer has
	// given one, or the only discipline's name when there is just one.
	Name string      `json:"name"`
	Info store.Event `json:"info"`
	// InfoError says event.json could not be read. The disciplines run regardless; the
	// day around them is blank until the file is fixed (proposal §11).
	InfoError   string              `json:"infoError,omitempty"`
	Disciplines []DisciplineSummary `json:"disciplines"`
	Dir         string              `json:"dir"`
	// Programme is the fencing part of the day, as the forecast has it (phase 4).
	Programme []ProgrammeRow `json:"programme"`
	// Staff are the event's staff, for the landing page's list of everyone (#135): their
	// names, as every discipline's snapshot already lists them, and their pages.
	Staff []Entrant `json:"staff"`
}

// StaffEntrants is the staff as the landing page lists them.
func StaffEntrants(members []store.StaffMember) []Entrant {
	out := make([]Entrant, 0, len(members))
	for _, m := range members {
		out = append(out, Entrant{ID: m.ID, Name: m.Name, Club: m.Club, Person: m.Person})
	}
	return out
}

// DisciplineSummary is one discipline, compactly.
type DisciplineSummary struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	// URL is where the discipline's own pages are: "/d/open-sabre/".
	URL string `json:"url"`
	// Error says the discipline could not be read, and why. Everything else is then the
	// last summary it had, if it ever had one, and Stale says so.
	Error string `json:"error,omitempty"`
	Stale bool   `json:"stale,omitempty"`
	// Stage is where the discipline has got to: "setup" before the pools are drawn,
	// "pools", "waiting" between the last pool match and the bracket, "eliminations",
	// and "done" once the bracket is fenced.
	Stage        string       `json:"stage"`
	Competitors  int          `json:"competitors"`
	Pools        int          `json:"pools"`
	MatchesDone  int          `json:"matchesDone"`
	MatchesTotal int          `json:"matchesTotal"`
	Podium       *PodiumNames `json:"podium,omitempty"`
	Mats         []MatSummary `json:"mats"`
	Entrants     []Entrant    `json:"entrants"`
}

// MatSummary is what one of a discipline's mats is running or has up next.
type MatSummary struct {
	Mat    int    `json:"mat"`
	Match  string `json:"match,omitempty"`
	Status string `json:"status,omitempty"`
	Pool   int    `json:"pool,omitempty"`
	Round  string `json:"round,omitempty"`
	// Red and Blue are names, with the colour each side is shown in and the score.
	Red        string `json:"red,omitempty"`
	Blue       string `json:"blue,omitempty"`
	RedColour  string `json:"redColour,omitempty"`
	BlueColour string `json:"blueColour,omitempty"`
	RedScore   int    `json:"redScore"`
	BlueScore  int    `json:"blueScore"`
}

// Entrant is somebody entered in a discipline, for finding a name across the event.
type Entrant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Club string `json:"club,omitempty"`
	// Person is who they are across the event: their page is /who/{person}.
	Person string `json:"person,omitempty"`
}

// PodiumNames is a discipline's podium, by name.
type PodiumNames struct {
	First  string `json:"first"`
	Second string `json:"second"`
	Third  string `json:"third"`
}

// Summarize boils a discipline's snapshot down to its summary. Exported for the browser
// demo, which builds the same event view out of memory.
func Summarize(slug string, snap Snapshot) DisciplineSummary {
	out := DisciplineSummary{
		Slug:     slug,
		Name:     snap.Instance.Name,
		URL:      snap.Instance.URL,
		Mats:     []MatSummary{},
		Entrants: []Entrant{},
	}
	names := map[string]string{}
	for _, c := range snap.Competitors {
		names[c.ID] = c.Name
		if c.Withdrawn {
			continue
		}
		out.Competitors++
		out.Entrants = append(out.Entrants, Entrant{ID: c.ID, Name: c.Name, Club: c.Club, Person: c.Person})
	}

	byID := map[string]MatchView{}
	for _, p := range snap.Pools {
		out.Pools++
		for _, m := range p.Matches {
			byID[m.ID] = m
			out.MatchesTotal++
			if m.Status == "complete" {
				out.MatchesDone++
			}
		}
	}
	if snap.Bracket != nil {
		for _, m := range snap.Bracket.Matches {
			byID[m.ID] = m
			out.MatchesTotal++
			if m.Status == "complete" {
				out.MatchesDone++
			}
		}
	}

	switch {
	case len(snap.Pools) == 0:
		out.Stage = "setup"
	case !snap.PoolsComplete:
		out.Stage = "pools"
	case snap.Bracket == nil:
		out.Stage = "waiting"
	case !snap.Bracket.Complete:
		out.Stage = "eliminations"
	default:
		out.Stage = "done"
	}
	if snap.Bracket != nil {
		if p := snap.Bracket.Podium; p.First != "" || p.Second != "" || p.Third != "" {
			out.Podium = &PodiumNames{First: names[p.First], Second: names[p.Second], Third: names[p.Third]}
		}
	}

	for mat := 1; mat <= snap.Tournament.Mats; mat++ {
		ms := MatSummary{Mat: mat}
		if m, ok := byID[snap.Mats[mat]]; ok {
			ms.Match, ms.Status, ms.Pool, ms.Round = m.ID, m.Status, m.Pool, m.Round
			ms.Red, ms.Blue = names[m.Red], names[m.Blue]
			ms.RedColour, ms.BlueColour = m.Options.Red, m.Options.Blue
			ms.RedScore, ms.BlueScore = m.State.Red.Score, m.State.Blue.Score
		}
		out.Mats = append(out.Mats, ms)
	}
	return out
}
