package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fylke/porta-di-ferro/internal/event"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The event coordinator (docs/proposals/one-event-many-disciplines.md §5–§7).
//
// One process, one port, one address for the whole hall. Each discipline is a Server
// exactly as it was when it was a run of its own -- its own folder, store, lock, SSE hub
// and presence -- mounted under /api/d/{slug}/. The coordinator owns what is about the
// event rather than any one discipline: event.json, the list of disciplines, the event's
// stream, and the address everything is served from. It calls the disciplines through Go
// methods; there is no protocol between them to get wrong.
//
// A discipline that cannot be loaded is a state, not an error: it is listed as failed,
// with the reason, every other discipline runs, and the landing page shows what it last
// knew of it (proposal §11). A hand-edited file that no longer parses is the most likely
// failure this product has, and it must cost one discipline, never the hall.

// Coordinator is one event and every discipline in it.
type Coordinator struct {
	folder       *event.Folder
	assets       fs.FS
	hub          *hub
	addressCache addressCache
	// matsHub carries the hall's mats -- every queue, and what each mat is running -- to
	// the score keepers and the screens (phase 2).
	matsHub *hub
	// presence is every device in the hall, shared with every discipline so a claim on a
	// match anywhere asks the one registry whether its holder is alive.
	presence *presence
	// planMu serialises changes to the plan.
	planMu sync.Mutex

	mu      sync.Mutex
	order   []string
	workers map[string]*worker

	changed chan struct{}
	stop    chan struct{}
}

// worker is one discipline as the coordinator holds it. Not a process, thread or actor:
// a Server, or the reason there is not one.
type worker struct {
	slug    string
	srv     *Server
	handler http.Handler
	err     error
	// last is the summary from the last time the discipline could be read, so a
	// discipline that breaks mid-event still shows on the landing page, marked stale.
	last     *DisciplineSummary
	unfollow func()
}

// NewCoordinator loads every discipline in the event folder. An event with none gets one,
// unnamed, so a first start is what it always was: one tournament, ready to set up
// (proposal R10).
func NewCoordinator(folder *event.Folder, assets fs.FS) (*Coordinator, error) {
	c := &Coordinator{
		folder:  folder,
		assets:  assets,
		hub:      newHub(),
		matsHub:  newHub(),
		presence: newPresence(),
		workers:  map[string]*worker{},
		changed:  make(chan struct{}, 1),
		stop:     make(chan struct{}),
	}
	slugs, err := folder.Slugs()
	if err != nil {
		return nil, err
	}
	if len(slugs) == 0 {
		slug, err := folder.Create("")
		if err != nil {
			return nil, err
		}
		slugs = []string{slug}
	}
	for _, slug := range slugs {
		c.workers[slug] = c.load(slug)
		c.order = append(c.order, slug)
	}
	c.liftDisplays()
	go c.announce()
	go c.sweep()
	return c, nil
}

// Close stops the background work and every discipline's.
func (c *Coordinator) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.stop:
		return
	default:
	}
	close(c.stop)
	for _, w := range c.workers {
		w.close()
	}
}

// load opens one discipline. Its two files are read once here so a parse error is found
// now, named, and kept to this discipline.
func (c *Coordinator) load(slug string) *worker {
	w := &worker{slug: slug}
	dir := c.folder.DisciplineDir(slug)
	st, err := store.Open(dir)
	if err == nil {
		_, err = st.Tournament()
	}
	if err == nil {
		_, err = st.Competitors()
	}
	if err != nil {
		w.err = fmt.Errorf("the files in %s could not be read: %w", dir, err)
		log.Printf("porta: discipline %s failed to load: %v", slug, err)
		return w
	}
	w.srv = newServer(st, nil, Instance{Slug: slug}, c.presence)
	w.srv.UseEvent(c)
	w.srv.UseMats(c)
	w.handler = w.srv.Handler()
	w.unfollow = c.follow(w.srv, slug)
	return w
}

func (w *worker) close() {
	if w.unfollow != nil {
		w.unfollow()
	}
	if w.srv != nil {
		w.srv.Close()
	}
}

// follow listens to a discipline's stream for as long as it is in the event, and tells
// the event's stream something changed. A subscriber that falls behind is dropped by the
// hub, so this subscribes again rather than going quiet. A log the organizer rewrote is
// passed on to the mats' stream, with its discipline, for the score keeper holding it.
func (c *Coordinator) follow(srv *Server, slug string) func() {
	done := make(chan struct{})
	go func() {
		defer recoverLogged("following a discipline")
		for {
			ch, unsubscribe := srv.Subscribe()
			for open := true; open; {
				select {
				case <-done:
					unsubscribe()
					return
				case u, ok := <-ch:
					open = ok
					if !ok {
						break
					}
					if u.Kind == "log-replaced" {
						c.matsHub.publish(Update{Kind: "log-replaced", Match: u.Match, Data: map[string]string{"discipline": slug}})
					}
					c.poke()
				}
			}
		}
	}()
	return func() { close(done) }
}

// poke says the event view may have changed. It never blocks a discipline's writer.
func (c *Coordinator) poke() {
	select {
	case c.changed <- struct{}{}:
	default:
	}
}

// announce publishes the event view whenever something changed, coalescing a burst -- a
// score keeper's exchange is a log append and a state push in quick succession -- into
// one update.
func (c *Coordinator) announce() {
	defer recoverLogged("announcing the event")
	for {
		select {
		case <-c.stop:
			return
		case <-c.changed:
		}
		select {
		case <-c.stop:
			return
		case <-time.After(150 * time.Millisecond):
		}
		select {
		case <-c.changed:
		default:
		}
		event, mats := c.hub.subscribers() > 0, c.matsHub.subscribers() > 0
		if !event && !mats {
			continue
		}
		snaps := c.snapshots()
		view := c.matsFrom(snaps)
		if event {
			c.hub.publish(Update{Kind: "event", Data: c.viewFrom(snaps, view)})
			c.hub.publish(Update{Kind: "presence", Data: c.presenceView()})
		}
		if mats {
			c.matsHub.publish(Update{Kind: "mats", Data: view})
		}
	}
}

// recoverLogged is the rule for background goroutines (proposal §11): a panic in one is
// logged and contained rather than taking every discipline down with it.
func recoverLogged(what string) {
	if r := recover(); r != nil {
		log.Printf("porta: recovered from a panic while %s: %v", what, r)
	}
}

// --- EventInfo, for the disciplines -----------------------------------------------------

// EventInfo is the day around the fencing, from event.json.
func (c *Coordinator) EventInfo() (store.Event, error) {
	f, err := c.folder.Read()
	return f.Event, err
}

// SaveEventInfo writes the day around the fencing and tells every page in the hall.
func (c *Coordinator) SaveEventInfo(ev store.Event) error {
	if _, err := c.folder.Update(func(f *event.File) error {
		f.Event = ev
		return nil
	}); err != nil {
		return err
	}
	// Every discipline's snapshot carries the day, so every discipline republishes.
	for _, w := range c.snapshotWorkers() {
		if w.srv != nil {
			w.srv.PublishState()
		}
	}
	c.poke()
	return nil
}

// --- the event view ---------------------------------------------------------------------

func (c *Coordinator) snapshotWorkers() []*worker {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*worker, 0, len(c.order))
	for _, slug := range c.order {
		out = append(out, c.workers[slug])
	}
	return out
}

// View is the event as its pages see it.
func (c *Coordinator) View() EventView {
	snaps := c.snapshots()
	return c.viewFrom(snaps, c.matsFrom(snaps))
}

// viewFrom builds the event view from snapshots already taken, with each discipline's
// mats -- the event's mats it is running right now -- read off the hall's.
func (c *Coordinator) viewFrom(snaps []snapped, mats MatsView) EventView {
	view := EventView{Dir: c.folder.Dir(), Disciplines: []DisciplineSummary{}}
	file, err := c.folder.Read()
	if err != nil {
		view.InfoError = err.Error()
	}
	view.Info = file.Event
	for _, s := range snaps {
		d := c.summarizeFrom(s)
		if s.err == nil {
			d.Mats = summaryMats(mats, s.w.slug)
		}
		view.Disciplines = append(view.Disciplines, d)
	}
	view.Name = strings.TrimSpace(view.Info.Signup.Name)
	if view.Name == "" && len(view.Disciplines) == 1 {
		view.Name = view.Disciplines[0].Name
	}
	return view
}

func (c *Coordinator) summarize(w *worker) DisciplineSummary {
	s := snapped{w: w}
	if w.srv == nil {
		s.err = w.err
	} else {
		s.snap, s.err = w.srv.Snapshot()
	}
	return c.summarizeFrom(s)
}

func (c *Coordinator) summarizeFrom(sn snapped) DisciplineSummary {
	w := sn.w
	failed := func(err error) DisciplineSummary {
		c.mu.Lock()
		last := w.last
		c.mu.Unlock()
		out := DisciplineSummary{Slug: w.slug, URL: "/d/" + w.slug + "/", Stage: "setup",
			Mats: []MatSummary{}, Entrants: []Entrant{}}
		if last != nil {
			out = *last
			out.Stale = true
		}
		out.Error = err.Error()
		if out.Name == "" {
			out.Name = w.slug
		}
		return out
	}
	if sn.err != nil {
		return failed(sn.err)
	}
	s := Summarize(w.slug, sn.snap)
	c.mu.Lock()
	w.last = &s
	c.mu.Unlock()
	return s
}

// --- routing ----------------------------------------------------------------------------

// Handler is the whole hall's address. The path says what a request is about: the event,
// one discipline by name, or -- while the event has one discipline -- that one, by the
// paths every client used before there were several (proposal §6).
func (c *Coordinator) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/event", c.getEvent)
	mux.HandleFunc("PUT /api/event/info", c.putInfo)
	mux.HandleFunc("GET /api/event/stream", func(w http.ResponseWriter, r *http.Request) {
		serveStream(w, r, c.hub)
	})
	mux.HandleFunc("GET /api/info.pdf", c.infoPDF)
	mux.HandleFunc("GET /api/addresses", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.addressCache.get())
	})
	mux.HandleFunc("GET /api/qr.png", serveQR)

	mux.HandleFunc("GET /api/disciplines", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.View().Disciplines)
	})
	mux.HandleFunc("GET /api/disciplines/presets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, Disciplines)
	})
	mux.HandleFunc("POST /api/disciplines", c.addDiscipline)
	mux.HandleFunc("PATCH /api/disciplines/{d}", c.renameDiscipline)
	mux.HandleFunc("DELETE /api/disciplines/{d}", c.retireDiscipline)
	mux.HandleFunc("POST /api/disciplines/{d}/reload", c.reloadDiscipline)

	mux.HandleFunc("GET /api/mats", c.getMats)
	mux.HandleFunc("PUT /api/mats", c.putMats)
	mux.HandleFunc("GET /api/mats/stream", func(w http.ResponseWriter, r *http.Request) {
		serveStream(w, r, c.matsHub)
	})
	mux.HandleFunc("GET /api/plan", c.getMats)
	mux.HandleFunc("PATCH /api/plan/items/{id...}", c.patchItem)

	// The devices at the mats are the event's, whatever discipline they are scoring.
	mux.HandleFunc("GET /api/presence", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.presenceView())
	})
	mux.HandleFunc("POST /api/clients/{id}", c.register)
	mux.HandleFunc("POST /api/clients/{id}/release", c.release)
	mux.HandleFunc("PUT /api/clients/{id}/target", c.assignDisplay)

	mux.HandleFunc("/api/d/{d}/", c.discipline)
	mux.HandleFunc("/api/", c.alias)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { serveAssets(c.assets, w, r) })
	return mux
}

// discipline hands a request to the discipline it names, with the prefix taken off: what
// it answers is exactly what it answered as a run of its own.
func (c *Coordinator) discipline(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("d")
	c.mu.Lock()
	wk := c.workers[slug]
	c.mu.Unlock()
	if wk == nil {
		// An address the discipline had before it was named still reaches it.
		c.mu.Lock()
		wk = c.workers[c.folder.Resolve(slug)]
		c.mu.Unlock()
	}
	if wk == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("this event has no discipline %q", slug))
		return
	}
	if wk.srv == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": wk.err.Error(), "discipline": slug,
		})
		return
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/api" + strings.TrimPrefix(r.URL.Path, "/api/d/"+slug)
	r2.URL.RawPath = ""
	wk.handler.ServeHTTP(w, r2)
}

// alias answers today's unprefixed paths -- /api/state, /api/matches/… -- as the event's
// one discipline, so an event with one discipline looks exactly as a run of the
// application always did and every bookmark and offline score keeper keeps working
// (proposal §6, Compatibility). With several there is no one discipline to mean, and the
// answer says which there are rather than guessing.
func (c *Coordinator) alias(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	var only *worker
	slugs := append([]string{}, c.order...)
	if len(c.order) == 1 {
		only = c.workers[c.order[0]]
	}
	c.mu.Unlock()
	if only == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":       "this event runs several disciplines: ask for one by its address, /api/d/{discipline}/…",
			"disciplines": slugs,
		})
		return
	}
	if only.srv == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": only.err.Error(), "discipline": only.slug,
		})
		return
	}
	only.handler.ServeHTTP(w, r)
}

// --- the event's own endpoints ----------------------------------------------------------

func (c *Coordinator) getEvent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, c.View())
}

// putInfo is the welcome, the programme, the wifi and the signup settings, typed once for
// the whole event.
func (c *Coordinator) putInfo(w http.ResponseWriter, r *http.Request) {
	var in store.Event
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in, err := CleanEvent(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in.Signup.Tournament = ""
	if err := c.SaveEventInfo(in); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, in)
}

// infoPDF is one poster for the door, for the whole event.
func (c *Coordinator) infoPDF(w http.ResponseWriter, r *http.Request) {
	view := c.View()
	snap := Snapshot{Tournament: store.Tournament{Event: view.Info}, Instance: Instance{Name: view.Name}}
	doc := BuildInfoPDF(snap, r.URL.Query().Get("url"), r.URL.Query().Get("lang") == "sv")
	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro-info.pdf"`)
	w.Write(buf.Bytes())
}

// --- adding, renaming, retiring and reloading disciplines (#4) --------------------------

func (c *Coordinator) nameTaken(name, except string) error {
	for _, wk := range c.snapshotWorkers() {
		if wk.srv == nil || wk.slug == except {
			continue
		}
		if strings.EqualFold(wk.srv.Self().Name, name) {
			return fmt.Errorf("%s is already in this event", wk.srv.Self().Name)
		}
	}
	return nil
}

// addDiscipline is another folder and another Server in this same process: what used to
// be a second copy of the application on the next free port (#49).
func (c *Coordinator) addDiscipline(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	name, err := CleanName(in.Name)
	if err == nil {
		err = c.nameTaken(name, "")
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	slug, err := c.folder.Create(name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	wk := c.load(slug)
	if wk.srv != nil {
		if err := wk.srv.Rename(name); err != nil {
			wk.close()
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	c.mu.Lock()
	c.workers[slug] = wk
	c.order = append(c.order, slug)
	c.mu.Unlock()
	c.poke()
	writeJSON(w, http.StatusCreated, c.summarize(wk))
}

func (c *Coordinator) renameDiscipline(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("d")
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	c.mu.Lock()
	wk := c.workers[slug]
	c.mu.Unlock()
	if wk == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("this event has no discipline %q", slug))
		return
	}
	if wk.srv == nil {
		writeErr(w, http.StatusConflict, fmt.Errorf("%s cannot be renamed until its files can be read: %w", slug, wk.err))
		return
	}
	name, err := CleanName(in.Name)
	if err == nil {
		err = c.nameTaken(name, slug)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if event.Placeholder(slug) && wk.srv.Self().Name == "" {
		wk, err = c.reslug(wk, name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := wk.srv.Rename(name); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	c.poke()
	writeJSON(w, http.StatusOK, c.summarize(wk))
}

// reslug gives a discipline that was made before it had a name -- the first one of a
// fresh event -- an address from the name it is given, so the hall's pages and QR codes
// say /d/open-sabre/ rather than /d/discipline/. The old address stays an alias for it.
func (c *Coordinator) reslug(old *worker, name string) (*worker, error) {
	old.close()
	slug, err := c.folder.Reslug(old.slug, name)
	if err != nil {
		// Back as it was: the folder did not move.
		again := c.load(old.slug)
		c.mu.Lock()
		c.workers[old.slug] = again
		c.mu.Unlock()
		return nil, err
	}
	wk := c.load(slug)
	if wk.srv == nil {
		return nil, wk.err
	}
	c.mu.Lock()
	delete(c.workers, old.slug)
	c.workers[slug] = wk
	for i, s := range c.order {
		if s == old.slug {
			c.order[i] = slug
		}
	}
	c.mu.Unlock()
	return wk, nil
}

// retireDiscipline takes a discipline out of the event. Its folder is kept under
// retired/, never deleted. The last discipline stays: an event with none has nothing for
// any page to show.
func (c *Coordinator) retireDiscipline(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("d")
	c.mu.Lock()
	wk := c.workers[slug]
	if wk == nil {
		c.mu.Unlock()
		writeErr(w, http.StatusNotFound, fmt.Errorf("this event has no discipline %q", slug))
		return
	}
	if len(c.order) == 1 {
		c.mu.Unlock()
		writeErr(w, http.StatusBadRequest, errors.New("an event needs at least one discipline"))
		return
	}
	delete(c.workers, slug)
	kept := c.order[:0]
	for _, s := range c.order {
		if s != slug {
			kept = append(kept, s)
		}
	}
	c.order = kept
	c.mu.Unlock()

	wk.close()
	where, err := c.folder.Retire(slug)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	c.poke()
	writeJSON(w, http.StatusOK, map[string]string{"retired": where})
}

// reloadDiscipline reads a discipline's files again: after the organizer has fixed the
// hand edit that stopped it loading, without restarting the hall.
func (c *Coordinator) reloadDiscipline(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("d")
	c.mu.Lock()
	old := c.workers[slug]
	c.mu.Unlock()
	if old == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("this event has no discipline %q", slug))
		return
	}
	old.close()
	wk := c.load(slug)
	wk.last = old.last
	c.mu.Lock()
	c.workers[slug] = wk
	c.mu.Unlock()
	c.poke()
	writeJSON(w, http.StatusOK, c.summarize(wk))
}
