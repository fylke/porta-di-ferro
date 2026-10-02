// Package event is the event folder on disk: the day at one venue, with every discipline
// it runs in a folder of its own beneath it (docs/proposals/one-event-many-disciplines.md
// §7).
//
//	<event>/
//	  event.json                 welcome, programme, wifi, signup settings, discipline order
//	  disciplines/
//	    open-steel-longsword/    exactly a tournament folder as internal/store knows it
//	      tournament.json
//	      competitors.json
//	      matches/p1m1.ndjson …
//	    open-sabre/
//	  retired/                   disciplines taken out of the event, kept rather than deleted
//
// Every discipline keeps its own files, ruleset and lock, as #49 made them: the split that
// matters is by data, and this package is where that split is laid out. It knows nothing
// about HTTP, and nothing about how a tournament is run; it owns the folder layout, the
// event's one file, and moving a folder from before this layout into it.
package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/store"
)

const (
	infoFile       = "event.json"
	disciplinesDir = "disciplines"
	retiredDir     = "retired"
)

// File is event.json: what is about the event rather than about one discipline.
//
// The event part is store.Event, the same shape a tournament carried on its own before
// (#98), so every reader of it -- the landing page, the info sheet, the signup definition
// -- reads it unchanged. Its Signup.Tournament is never stored here: which programme row a
// discipline is stays with that discipline.
type File struct {
	store.Event
	// Disciplines is the order the disciplines are listed in, by slug. A folder on disk
	// that is not on the list is still part of the event, listed after the rest: the
	// folders are the truth about what exists, this is only how to show them.
	Disciplines []string `json:"disciplines,omitempty"`
}

// Folder is one event's directory.
type Folder struct {
	dir string
	mu  sync.Mutex
}

// Open prepares an event folder, creating it if it is not there, and moves a tournament
// folder from before events existed into its place as the event's first discipline.
func Open(dir string) (*Folder, error) {
	if err := os.MkdirAll(filepath.Join(dir, disciplinesDir), 0o755); err != nil {
		return nil, err
	}
	f := &Folder{dir: dir}
	if err := f.migrate(); err != nil {
		return nil, err
	}
	return f, nil
}

// Dir is the event folder.
func (f *Folder) Dir() string { return f.dir }

// DisciplineDir is where a discipline's tournament folder is.
func (f *Folder) DisciplineDir(slug string) string {
	return filepath.Join(f.dir, disciplinesDir, slug)
}

// Read is event.json. A missing file is an event with nothing filled in yet; a file that
// does not parse is returned as an error, for the caller to report and carry on without
// -- the disciplines' own data does not depend on it (proposal §11).
func (f *Folder) Read() (File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.readLocked()
}

func (f *Folder) readLocked() (File, error) {
	var out File
	b, err := os.ReadFile(filepath.Join(f.dir, infoFile))
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return File{}, fmt.Errorf("%s: %w", infoFile, err)
	}
	out.Signup.Tournament = ""
	return out, nil
}

// Update reads event.json, lets change edit it, and writes it back, under the folder's
// lock so two edits cannot lose each other.
func (f *Folder) Update(change func(*File) error) (File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, err := f.readLocked()
	if err != nil {
		return File{}, err
	}
	if err := change(&file); err != nil {
		return File{}, err
	}
	file.Signup.Tournament = ""
	if err := store.WriteJSONAtomic(f.dir, infoFile, file); err != nil {
		return File{}, err
	}
	return file, nil
}

// Slugs lists the event's disciplines in their order: the stored order first, then any
// folder that is not on it, alphabetically. A slug on the list with no folder is dropped.
func (f *Folder) Slugs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(f.dir, disciplinesDir))
	if err != nil {
		return nil, err
	}
	exists := map[string]bool{}
	var rest []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		exists[e.Name()] = true
		rest = append(rest, e.Name())
	}
	file, _ := f.Read()
	var out []string
	listed := map[string]bool{}
	for _, slug := range file.Disciplines {
		if exists[slug] && !listed[slug] {
			out = append(out, slug)
			listed[slug] = true
		}
	}
	sort.Strings(rest)
	for _, slug := range rest {
		if !listed[slug] {
			out = append(out, slug)
		}
	}
	return out, nil
}

// Create makes a folder for a new discipline and puts it at the end of the order. The
// slug comes from the name and never changes after, because it is in every address and
// in the folder's name: renaming a discipline renames what the pages say, not where it is.
func (f *Folder) Create(name string) (string, error) {
	slugs, err := f.Slugs()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, s := range slugs {
		taken[s] = true
	}
	slug := uniqueSlug(name, taken)
	if err := os.MkdirAll(f.DisciplineDir(slug), 0o755); err != nil {
		return "", err
	}
	_, err = f.Update(func(file *File) error {
		file.Disciplines = append(withExisting(file.Disciplines, slugs), slug)
		return nil
	})
	return slug, err
}

// Retire takes a discipline out of the event. Its folder is moved under retired/ with
// the time beside its name, not deleted: a discipline retired by mistake, or the results
// of one somebody asks about next month, is a folder to move back by hand.
func (f *Folder) Retire(slug string) (string, error) {
	src := f.DisciplineDir(slug)
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("no discipline %q in this event", slug)
	}
	if err := os.MkdirAll(filepath.Join(f.dir, retiredDir), 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(f.dir, retiredDir, slug+"-"+time.Now().Format("20060102-150405"))
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	_, err := f.Update(func(file *File) error {
		kept := file.Disciplines[:0]
		for _, s := range file.Disciplines {
			if s != slug {
				kept = append(kept, s)
			}
		}
		file.Disciplines = kept
		return nil
	})
	return dst, err
}

// withExisting is the stored order with any unlisted folder added, so writing the order
// back after adding one never drops a discipline that was only on disk.
func withExisting(order, slugs []string) []string {
	listed := map[string]bool{}
	for _, s := range order {
		listed[s] = true
	}
	out := append([]string{}, order...)
	for _, s := range slugs {
		if !listed[s] {
			out = append(out, s)
		}
	}
	return out
}

// Slugify makes the identifier a discipline is addressed by: lowercase letters, digits and
// hyphens, so it can stand in a URL and a folder name on any system without escaping.
func Slugify(name string) string {
	var b strings.Builder
	for _, r := range signup.Slug(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 48 {
		slug = strings.TrimRight(slug[:48], "-")
	}
	return slug
}

func uniqueSlug(name string, taken map[string]bool) string {
	base := Slugify(name)
	if base == "" {
		base = "discipline"
	}
	slug := base
	for n := 2; taken[slug] || reserved[slug]; n++ {
		slug = fmt.Sprintf("%s-%d", base, n)
	}
	return slug
}

// reserved are slugs that would read as something else in an address.
var reserved = map[string]bool{"api": true, "admin": true, "event": true}
