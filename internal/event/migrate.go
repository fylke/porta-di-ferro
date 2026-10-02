package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// legacy is everything internal/store keeps in a tournament folder. Finding any of it at
// the top of an event folder means the folder is from before events existed: one run of
// the application, one tournament, with its files where the event's now go.
var legacy = []string{"tournament.json", "competitors.json", "writers.json", "displays.json", "matches"}

// migrate moves a tournament folder from before this layout into it: its files become the
// first discipline's folder, and the day around the tournament (#98) is lifted out of its
// tournament.json into event.json. A move, not a conversion -- the tournament's own files
// come through byte for byte, so nothing about the draw, the logs or the claims can change
// on the way (proposal §7).
//
// It is safe to interrupt. event.json is written first and only when it is absent, and
// each file is a rename of its own, so a second start after a crash finds what is left at
// the top and moves that, rather than starting again or stopping.
func (f *Folder) migrate() error {
	var found []string
	for _, name := range legacy {
		if _, err := os.Stat(filepath.Join(f.dir, name)); err == nil {
			found = append(found, name)
		}
	}
	if len(found) == 0 {
		return nil
	}
	// matches/ is created by store.Open for any folder, so an empty one alone is not a
	// tournament -- it is what a previous start of this same code may have left.
	if len(found) == 1 && found[0] == "matches" && emptyDir(filepath.Join(f.dir, "matches")) {
		return os.Remove(filepath.Join(f.dir, "matches"))
	}

	// Read directly rather than through store.Open, which would create a matches/ here.
	t, terr := store.Defaults(), error(nil)
	if b, err := os.ReadFile(filepath.Join(f.dir, "tournament.json")); err == nil {
		terr = json.Unmarshal(b, &t)
	} else if !errors.Is(err, fs.ErrNotExist) {
		terr = err
	}

	// The slug comes from the name the tournament already had. A tournament.json that does
	// not parse still moves -- it is the organizer's file and it belongs with its logs --
	// and the discipline then reports the parse error where it can be fixed.
	file, err := f.Read()
	if err != nil {
		return err
	}
	slug := ""
	if len(file.Disciplines) > 0 {
		// An interrupted move: carry on into the folder it started.
		slug = file.Disciplines[0]
	} else {
		taken := map[string]bool{}
		if slugs, err := f.Slugs(); err == nil {
			for _, s := range slugs {
				taken[s] = true
			}
		}
		slug = uniqueSlug(t.Discipline, taken)
	}

	if _, err := os.Stat(filepath.Join(f.dir, infoFile)); errors.Is(err, fs.ErrNotExist) {
		lifted := File{Disciplines: []string{slug}}
		if terr == nil {
			lifted.Event = t.Event
		}
		if err := store.WriteJSONAtomic(f.dir, infoFile, lifted); err != nil {
			return err
		}
	}

	dst := f.DisciplineDir(slug)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, name := range found {
		from, to := filepath.Join(f.dir, name), filepath.Join(dst, name)
		if _, err := os.Stat(to); err == nil {
			if name == "matches" && emptyDir(to) {
				if err := os.Remove(to); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("both %s and %s exist; move one of them by hand", from, to)
			}
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("moving %s into the event's first discipline: %w", from, err)
		}
	}

	// The tournament's own copy of the day is now event.json's. Only which programme row
	// this discipline is stays behind, because that is about the discipline.
	if terr == nil {
		moved, err := store.Open(dst)
		if err != nil {
			return err
		}
		t.Event = store.Event{Signup: store.Signup{Tournament: t.Event.Signup.Tournament}}
		if err := moved.SaveTournament(t); err != nil {
			return err
		}
	}
	return nil
}

func emptyDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}
