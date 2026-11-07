package httpapi

import (
	"net/http"

	"github.com/fylke/porta-di-ferro/internal/hall"
)

func (s *Server) exportInfoPDF(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// The address a phone should open. The organizer picks it in the admin view and
	// passes it here, because only the browser knows which of this PC's networks the
	// tablets are actually on.
	var programme []hall.ProgrammeRow
	if s.event != nil {
		programme = s.event.Programme()
	}
	doc := hall.BuildInfoPDF(snap, programme, r.URL.Query().Get("url"), r.URL.Query().Get("lang") == "sv")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro-info.pdf"`)
	if err := doc.Output(w); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
	}
}
