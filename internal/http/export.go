package httpapi

import (
	"net/http"
	"time"

	"github.com/fylke/porta-di-ferro/internal/hall"
)

// Export is the whole tournament in one file: competitors, the draw, every result and the
// final standings with the indices that produced them. Written from derived state at
// export time, because derived state is never stored.
type Export struct {
	ExportedAt string        `json:"exportedAt"`
	Ruleset    any           `json:"ruleset"`
	Snapshot   hall.Snapshot `json:"tournament"`
}

func (s *Server) exportJSON(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro-results.json"`)
	writeJSON(w, http.StatusOK, Export{
		ExportedAt: time.Now().Format(time.RFC3339),
		Ruleset:    s.rules,
		Snapshot:   snap,
	})
}
