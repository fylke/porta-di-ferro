package httpapi

import (
	"net/http"

	"github.com/fylke/porta-di-ferro/internal/lan"
)

// addresses lists the addresses on this PC that a score keeper's device could open.
//
// The organizer view needs this because its own browser is on http://localhost, and that
// is the one address no other device can reach. The page composes the client URL from an
// address here and its own port, so nothing has to be configured and nothing has to be
// read out loud.
func (s *Server) addresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, lan.Addresses())
}
