package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// serveApp serves the embedded Svelte bundle, falling through to index.html for anything
// it does not recognise. That fallthrough is what makes the client-side routes work:
// /score/1, /display/mats and /print/pools are all the same bundle.
//
// The binary is the web app. At a venue with no internet this is also how a device that
// has never opened it gets it.
func (s *Server) serveApp(w http.ResponseWriter, r *http.Request) {
	if s.assets == nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	f, err := s.assets.Open(name)
	if err != nil {
		// A request with a file extension asked for a file, so a miss is a miss. Falling
		// through would answer a stale /assets/index-OLD.js -- or a missing /sw.js -- with
		// 200 and a page of HTML, which the browser then fails to parse as a script. That
		// is a miserable thing to debug at a venue, and it is what a 404 makes obvious.
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		// No extension: a client-side route, not a missing file.
		s.serveIndex(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		s.serveIndex(w, r)
		return
	}
	if rs, ok := f.(interface {
		Read([]byte) (int, error)
		Seek(int64, int) (int64, error)
	}); ok {
		// Hashed asset names come from Vite, so anything under assets/ is safe to cache
		// hard. index.html never is.
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, r, name, info.ModTime(), rs)
		return
	}
	s.serveIndex(w, r)
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	b, err := fs.ReadFile(s.assets, "index.html")
	if err != nil {
		http.Error(w, "web app not built into this binary", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(b)
}
