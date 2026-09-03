// Package httpapi serves the API, the Server-Sent Events streams and the embedded web
// app. Everything a client writes is a plain POST; everything the server pushes is SSE
// (design decision 15). There is no WebSocket: nothing a client renders depends on a
// live connection, so there is no work for a bidirectional transport to do.
package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Update is one message on an SSE stream.
type Update struct {
	Kind  string `json:"kind"`
	Match string `json:"match,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// hub fans updates out to every open stream. Subscribers that fall behind are dropped
// rather than allowed to block the writer: SSE reconnects itself, and a client that
// reconnects asks for everything after the last sequence it saw.
type hub struct {
	mu   sync.Mutex
	subs map[chan Update]struct{}
}

func newHub() *hub { return &hub{subs: map[chan Update]struct{}{}} }

func (h *hub) subscribe() chan Update {
	ch := make(chan Update, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan Update) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *hub) publish(u Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- u:
		default:
			// Full. Dropping the slow subscriber is safe because reconnecting is a
			// resume, not a replay of what it missed from memory.
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// stream is the SSE handler. One stream carries every kind of update; the client filters
// by what it is showing, which keeps the server from having to know what each display is.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	// A comment line every 20 seconds keeps proxies and sleeping phones from deciding the
	// connection is dead.
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case u, open := <-ch:
			if !open {
				return
			}
			b, err := json.Marshal(u)
			if err != nil {
				continue
			}
			if _, err := w.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
