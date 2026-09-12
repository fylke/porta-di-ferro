package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/fylke/porta-di-ferro/internal/match"
)

// PDF export (design §7 item 12): the tournament laid out to be printed and pinned up at
// the venue, which is what distinguishes it from the JSON export. Final standings per
// pool with the indices that produced them, and every match and its result.
//
// Made on the server with a pure-Go library rather than by printing the page from a
// browser, because the export is the thing an organizer files and forwards, and it should
// come out the same from a curl as from a click.

func (s *Server) exportPDF(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	doc := buildPDF(snap)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro.pdf"`)
	if err := doc.Output(w); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
	}
}

// buildPDF lays the snapshot out on A4. The core fonts cover Latin-1, which covers
// Swedish; a name from further afield loses its accents rather than breaking the page.
func buildPDF(snap Snapshot) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	title := "Porta di Ferro"
	if snap.Instance.Name != "" {
		title += " -- " + snap.Instance.Name
	}
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("%s  ·  %s  ·  page %d", title, time.Now().Format("2006-01-02 15:04"), pdf.PageNo())), "", 0, "C", false, 0, "")
	})

	name := func(id string) string {
		for _, c := range snap.Competitors {
			if c.ID == id {
				return c.Name
			}
		}
		return "?"
	}
	h1 := func(s string) {
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(0, 10, tr(s), "", 1, "L", false, 0, "")
	}
	h2 := func(s string) {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(0, 8, tr(s), "", 1, "L", false, 0, "")
	}
	small := func(s string) {
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(0, 5, tr(s), "", 1, "L", false, 0, "")
	}
	head := func(cols []string, widths []float64) {
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(235, 235, 235)
		pdf.SetTextColor(0, 0, 0)
		for i, c := range cols {
			align := "R"
			if i == 1 || i == 2 {
				align = "L"
			}
			pdf.CellFormat(widths[i], 6, tr(c), "1", 0, align, true, 0, "")
		}
		pdf.Ln(-1)
	}
	row := func(cells []string, widths []float64, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		pdf.SetFont("Helvetica", style, 9)
		pdf.SetTextColor(0, 0, 0)
		for i, c := range cells {
			align := "R"
			if i == 1 || i == 2 {
				align = "L"
			}
			pdf.CellFormat(widths[i], 6, tr(c), "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}
	fmtIdx := func(f float64) string { return fmt.Sprintf("%.2f", f) }
	score := func(m MatchView) string {
		if m.Status == "pending" {
			return "-"
		}
		note := ""
		switch m.State.Reason {
		case match.ReasonForfeit:
			note = " (forfeit)"
		case match.ReasonPenalty:
			note = " (penalty)"
		}
		return fmt.Sprintf("%d-%d%s", m.State.Red.Score, m.State.Blue.Score, note)
	}

	standingsWidths := []float64{8, 52, 38, 10, 22, 12, 12, 12, 12}
	standingsCols := []string{"#", "Competitor", "Club", "M", "W-D-L", "MPI", "VI", "SI", "RI"}
	matchWidths := []float64{8, 56, 56, 40}

	for _, p := range snap.Pools {
		pdf.AddPage()
		h1(fmt.Sprintf("Pool %d", p.Number))
		status := "in progress"
		if p.Complete {
			status = "complete"
		}
		small(fmt.Sprintf("Mat %d  ·  %d competitors  ·  %s", p.Mat, len(p.Competitors), status))

		h2("Standings")
		head(standingsCols, standingsWidths)
		for _, st := range p.Standings {
			row([]string{
				fmt.Sprint(st.Rank), st.Name, st.Club, fmt.Sprint(st.Completed),
				fmt.Sprintf("%d-%d-%d", st.Wins, st.Draws, st.Losses),
				fmtIdx(st.MatchPointIndex), fmtIdx(st.VictoryIndex), fmtIdx(st.ScoreIndex), fmtIdx(st.ReceptionIndex),
			}, standingsWidths, st.Rank == 1)
		}
		small("MPI match point index, VI victory index, SI score index, RI reception index (lowest wins); all per match completed.")

		h2("Matches")
		head([]string{"#", "Red", "Blue", "Result"}, matchWidths)
		for _, m := range p.Matches {
			row([]string{fmt.Sprint(m.Order), name(m.Red), name(m.Blue), score(m)}, matchWidths, false)
		}
	}

	if len(snap.Pools) == 0 {
		pdf.AddPage()
		h1(title)
		small("The pools have not been drawn yet.")
	}
	return pdf
}
