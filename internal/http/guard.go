package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Who may write (#165, docs/design.md "Who may change what").
//
// The poster by the door puts every phone in the hall on the venue wifi and at this
// server's address, so the address alone cannot be what decides who runs the event.
// Four rules, checked before any handler:
//
//   - A request must name one of this server's own addresses in Host: loopback, an
//     address of one of this PC's networks, or its name. A page elsewhere that points a
//     name of its own at this PC (DNS rebinding) is refused.
//   - A write must come from a page of this server: one the browser says came from
//     another site (Sec-Fetch-Site, or Origin when that is missing) is refused, so a web
//     page open in the organizer's browser cannot redraw the pools behind their back.
//   - A write must come from the organizer's own PC, which reaches the server on
//     loopback, unless it is one of the few the hall's devices make: a score keeper's
//     exchanges and claims, and every device's heartbeat. Reading stays open to the LAN.
//   - A request body is never more than maxBody.
//
// The demo has no server and is not touched by any of this.

// maxBody is the most any request may send: well past the largest real one, a signup
// folder imported in one go (maxImport).
const maxBody = 16 << 20

// hallWrites are the writes any device in the hall may make, after any /api/d/{slug}
// prefix: a score keeper's exchanges and its claim on a match, and every device's
// heartbeat and goodbye.
var hallWrites = regexp.MustCompile(`^/api/(matches/[^/]+/(events|claim)|clients/[^/]+(/release)?)$`)

var disciplinePrefix = regexp.MustCompile(`^/api/d/[^/]+/`)

// guard puts the rules above in front of a handler.
// REQ: NFR_SEC_01, NFR_SEC_02
func guard(next http.Handler) http.Handler {
	own := &ownAddresses{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !own.has(r.Host) {
			writeErr(w, http.StatusMisdirectedRequest, errNotOurs)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if crossSite(r) {
				writeErr(w, http.StatusForbidden, errCrossSite)
				return
			}
			if !fromLoopback(r) && !hallWrite(r) {
				writeErr(w, http.StatusForbidden, errOrganizerOnly)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		next.ServeHTTP(w, r)
	})
}

type guardError string

func (e guardError) Error() string { return string(e) }

const (
	errNotOurs       = guardError("this server does not answer to that address")
	errCrossSite     = guardError("a change has to come from this server's own pages")
	errOrganizerOnly = guardError("only the organizer's own computer can change this: open the admin page on the computer running Porta di Ferro")
)

// hallWrite says a write is one any device in the hall may make.
// REQ: NFR_SEC_01
func hallWrite(r *http.Request) bool {
	p := r.URL.Path
	if loc := disciplinePrefix.FindStringIndex(p); loc != nil {
		p = "/api/" + p[loc[1]:]
	}
	if !hallWrites.MatchString(p) {
		return false
	}
	// A display's screen assignment is the organizer's; its heartbeat is its own.
	return r.Method == http.MethodPost || (r.Method == http.MethodDelete && strings.HasSuffix(p, "/claim"))
}

// fromLoopback says the request came from this PC.
// REQ: NFR_SEC_01
func fromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// crossSite says the browser sent the request from another site's page. A client that is
// not a browser sends neither header, and is not one.
// REQ: NFR_SEC_02
func crossSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "":
	default:
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || !strings.EqualFold(u.Host, r.Host)
}

// ownAddresses is every name this server answers to, looked up again at most every few
// seconds when a request names an address it does not know -- a network joined since.
type ownAddresses struct {
	mu      sync.Mutex
	ips     map[string]bool
	names   map[string]bool
	checked time.Time
}

// REQ: NFR_SEC_02
func (o *ownAddresses) has(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ips[host] || o.names[host] {
		return true
	}
	if time.Since(o.checked) < 5*time.Second {
		return false
	}
	o.refresh()
	return o.ips[host] || o.names[host]
}

func (o *ownAddresses) refresh() {
	o.checked = time.Now()
	o.ips, o.names = map[string]bool{}, map[string]bool{}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				o.ips[n.IP.String()] = true
			}
		}
	}
	if name, err := os.Hostname(); err == nil && name != "" {
		name = strings.ToLower(name)
		o.names[name] = true
		o.names[name+".local"] = true
		o.names[name+".lan"] = true
	}
}
