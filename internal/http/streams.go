package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// One stream per page (#161). A browser allows six HTTP/1.1 connections to one address,
// and a page used to hold up to three Server-Sent Events streams -- the event's, the mats'
// and one per discipline -- so three organizer tabs used them all, and every tab stopped
// answering. GET /api/streams?topic=event&topic=mats&topic=d/open-sabre carries every
// stream a page follows over one connection, each frame tagged with its topic. The single
// streams stay, for anything that already reads them.

// tagged is one update and the stream it came from.
type tagged struct {
	Topic string `json:"topic"`
	Update
}

// hubFor is the hub behind a topic: "event", "mats", or "d/{slug}" ("d/" for the event's
// only discipline, as the unprefixed /api paths answer). Nil for none.
func (c *Coordinator) hubFor(topic string) *hub {
	switch {
	case topic == "event":
		return c.hub
	case topic == "mats":
		return c.matsHub
	case strings.HasPrefix(topic, "d/"):
		slug := strings.TrimPrefix(topic, "d/")
		c.mu.Lock()
		defer c.mu.Unlock()
		wk := c.workers[slug]
		if wk == nil && slug == "" && len(c.order) == 1 {
			wk = c.workers[c.order[0]]
		}
		if wk == nil || wk.srv == nil {
			return nil
		}
		return wk.srv.hub
	}
	return nil
}

// streams serves several hubs over one Server-Sent Events connection. When any of them
// drops this subscriber -- it fell behind, or its discipline was reloaded -- the whole
// stream ends, and the browser's EventSource opens it again and asks for a fresh picture.
func (c *Coordinator) streams(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	out := make(chan tagged, 64)
	seen := map[string]bool{}
	for _, topic := range r.URL.Query()["topic"] {
		h := c.hubFor(topic)
		if h == nil || seen[topic] {
			continue
		}
		seen[topic] = true
		ch := h.subscribe()
		defer h.unsubscribe(ch)
		go func(topic string, ch chan Update) {
			for {
				select {
				case <-ctx.Done():
					return
				case u, open := <-ch:
					if !open {
						cancel()
						return
					}
					select {
					case out <- tagged{Topic: topic, Update: u}:
					case <-ctx.Done():
						return
					}
				}
			}
		}(topic, ch)
	}

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case t := <-out:
			b, err := json.Marshal(t)
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
