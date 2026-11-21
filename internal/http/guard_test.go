package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// #165: who may write, and from where.
// REQ: NFR_SEC_01, NFR_SEC_02
func TestTheGuard(t *testing.T) {
	reached := false
	var read int64
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		n, err := io.Copy(io.Discard, r.Body)
		read = n
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	})
	h := guard(next)
	type req struct {
		name, method, path, remote, host string
		headers                          map[string]string
		want                             bool
	}
	lan, pc := "192.168.1.20:5000", "127.0.0.1:5000"
	for _, c := range []req{
		{"the organizer redraws", "POST", "/api/d/ls/tournament/pools", pc, "localhost:8080", nil, true},
		{"a phone on the wifi redraws", "POST", "/api/d/ls/tournament/pools", lan, "localhost:8080", nil, false},
		{"a phone deletes the event", "DELETE", "/api/event", lan, "localhost:8080", nil, false},
		{"a phone rewrites a match's history", "PUT", "/api/d/ls/matches/p1m1/events", lan, "localhost:8080", nil, false},
		{"a phone reads the standings", "GET", "/api/d/ls/state", lan, "localhost:8080", nil, true},
		{"a score keeper pushes exchanges", "POST", "/api/d/ls/matches/p1m1/events", lan, "localhost:8080", nil, true},
		{"a score keeper on the unprefixed path", "POST", "/api/matches/p1m1/events", lan, "localhost:8080", nil, true},
		{"a score keeper claims a match", "POST", "/api/d/ls/matches/p1m1/claim", lan, "localhost:8080", nil, true},
		{"a score keeper lets a match go", "DELETE", "/api/d/ls/matches/p1m1/claim", lan, "localhost:8080", nil, true},
		{"a device's heartbeat", "POST", "/api/clients/tab1", lan, "localhost:8080", nil, true},
		{"a phone assigns a screen", "PUT", "/api/clients/tab1/target", lan, "localhost:8080", nil, false},
		{"another site's page posts", "POST", "/api/d/ls/tournament/pools", pc, "localhost:8080",
			map[string]string{"Sec-Fetch-Site": "cross-site"}, false},
		{"another port on this PC posts", "POST", "/api/d/ls/tournament/pools", pc, "localhost:8080",
			map[string]string{"Sec-Fetch-Site": "same-site"}, false},
		{"an old browser's cross-origin post", "POST", "/api/d/ls/tournament/pools", pc, "localhost:8080",
			map[string]string{"Origin": "http://evil.example"}, false},
		{"the organizer's own page", "POST", "/api/d/ls/tournament/pools", pc, "localhost:8080",
			map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://localhost:8080"}, true},
		{"a rebound name", "GET", "/api/event", pc, "evil.example:8080", nil, false},
		{"loopback by number", "GET", "/api/event", pc, "127.0.0.1:8080", nil, true},
	} {
		reached = false
		r := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
		r.RemoteAddr, r.Host = c.remote, c.host
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if reached != c.want {
			t.Errorf("%s: reached the handler %v, want %v (status %d)", c.name, reached, c.want, w.Code)
		}
	}

	// A body past the limit is cut off.
	r := httptest.NewRequest("POST", "/api/d/ls/competitors", strings.NewReader(strings.Repeat("x", maxBody+10)))
	r.RemoteAddr, r.Host = pc, "localhost:8080"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if read > maxBody || w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a body past %d bytes should be cut off: read %d, status %d", maxBody, read, w.Code)
	}
}
