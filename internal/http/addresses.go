package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/fylke/porta-di-ferro/internal/lan"
)

// addresses lists the addresses on this PC that a score keeper's device could open.
//
// The organizer view needs this because its own browser is on http://localhost, and that
// is the one address no other device can reach. The page composes the client URL from an
// address here and its own port, so nothing has to be configured and nothing has to be
// read out loud.
//
// The organizer page polls it so the list follows the PC between networks without a
// reload -- joining the wrong wifi first is a reasonable thing to have happen -- and the
// answer is memoised for a few seconds because finding the wifi names means spawning
// netsh, which need not happen on every poll from every open organizer tab.
func (s *Server) addresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.addressCache.get())
}

type addressCache struct {
	mu    sync.Mutex
	at    time.Time
	addrs []lan.Address
}

func (c *addressCache) get() []lan.Address {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.addrs == nil || time.Since(c.at) > 3*time.Second {
		c.addrs = lan.Addresses()
		c.at = time.Now()
	}
	return c.addrs
}
