package httpapi

import (
	"net/http"

	"github.com/fylke/porta-di-ferro/internal/hall"
)

func (s *Server) exportPDF(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	doc := hall.BuildPDF(snap, r.URL.Query().Get("lang") == "sv")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro.pdf"`)
	if err := doc.Output(w); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
	}
}
