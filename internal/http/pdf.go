package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// PDF export (design §7 item 12): the tournament laid out to be printed and pinned up at
// the venue, which is what distinguishes it from the JSON export. Final standings per
// pool with the indices that produced them, every match and its result, the overall
// ranking once the pools are done, and the bracket and podium once they exist.
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
	doc := buildPDF(snap, r.URL.Query().Get("lang") == "sv")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro.pdf"`)
	if err := doc.Output(w); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
	}
}

// pdfSwedish is the export's own dictionary, keyed by the English it replaces, the same
// way the web client's is (design §7 item 11). Small enough to live here.
var pdfSwedish = map[string]string{
	"Pool %d":                          "Pool %d",
	"Mat %d  ·  %d competitors  ·  %s": "Matta %d  ·  %d fäktare  ·  %s",
	"in progress":                      "pågår",
	"complete":                         "klar",
	"Standings":                        "Tabell",
	"Matches":                          "Matcher",
	"#":                                "#",
	"Competitor":                       "Fäktare",
	"Club":                             "Klubb",
	"M":                                "M",
	"W-D-L":                            "V-O-F",
	"Red":                              "Röd",
	"Blue":                             "Blå",
	"Result":                           "Resultat",
	"Round":                            "Omgång",
	" (forfeit)":                       " (uppgiven)",
	" (penalty)":                       " (bestraffning)",
	"MPI match point index, VI victory index, SI score index, RI reception index (lowest wins); all per match completed.": "MPI matchpoängindex, VI segerindex, SI punktindex, RI mottaget index (lägst vinner); alla per utkämpad match.",
	"Overall ranking": "Sammanlagd ranking",
	"Everyone across the pools, by the same chain as the pool tables. The seeding for the eliminations.": "Alla fäktare över poolerna, efter samma kedja som pooltabellerna. Seedningen till elimineringarna.",
	"Eliminations":                       "Elimineringar",
	"Quarter-final":                      "Kvartsfinal",
	"Semi-final":                         "Semifinal",
	"Bronze match":                       "Bronsmatch",
	"Final":                              "Final",
	"Podium":                             "Pallen",
	"The pools have not been drawn yet.": "Poolerna är inte lottade ännu.",
	"page %d":                            "sida %d",
}

// buildPDF lays the snapshot out on A4. The core fonts cover Latin-1, which covers
// Swedish; a name from further afield loses its accents rather than breaking the page.
func buildPDF(snap Snapshot, swedish bool) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	// Translation happens before the Latin-1 pass, so the Swedish comes out with its
	// letters intact. A key with no entry comes out in English.
	t := func(key string) string {
		if swedish {
			if v, ok := pdfSwedish[key]; ok {
				return v
			}
		}
		return key
	}
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
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("%s  ·  %s  ·  "+t("page %d"), title, time.Now().Format("2006-01-02 15:04"), pdf.PageNo())), "", 0, "C", false, 0, "")
	})

	name := func(id string) string {
		for _, c := range snap.Competitors {
			if c.ID == id {
				return c.Name
			}
		}
		return "?"
	}
	club := func(id string) string {
		for _, c := range snap.Competitors {
			if c.ID == id {
				return c.Club
			}
		}
		return ""
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
			note = t(" (forfeit)")
		case match.ReasonPenalty:
			note = t(" (penalty)")
		}
		return fmt.Sprintf("%d-%d%s", m.State.Red.Score, m.State.Blue.Score, note)
	}

	standingsWidths := []float64{8, 52, 38, 10, 22, 12, 12, 12, 12}
	standingsCols := []string{"#", t("Competitor"), t("Club"), t("M"), t("W-D-L"), "MPI", "VI", "SI", "RI"}
	matchWidths := []float64{8, 56, 56, 40}

	for _, p := range snap.Pools {
		pdf.AddPage()
		h1(fmt.Sprintf(t("Pool %d"), p.Number))
		status := t("in progress")
		if p.Complete {
			status = t("complete")
		}
		small(fmt.Sprintf(t("Mat %d  ·  %d competitors  ·  %s"), p.Mat, len(p.Competitors), status))

		h2(t("Standings"))
		head(standingsCols, standingsWidths)
		for _, st := range p.Standings {
			row([]string{
				fmt.Sprint(st.Rank), st.Name, st.Club, fmt.Sprint(st.Completed),
				fmt.Sprintf("%d-%d-%d", st.Wins, st.Draws, st.Losses),
				fmtIdx(st.MatchPointIndex), fmtIdx(st.VictoryIndex), fmtIdx(st.ScoreIndex), fmtIdx(st.ReceptionIndex),
			}, standingsWidths, st.Rank == 1)
		}
		small(t("MPI match point index, VI victory index, SI score index, RI reception index (lowest wins); all per match completed."))

		h2(t("Matches"))
		head([]string{"#", t("Red"), t("Blue"), t("Result")}, matchWidths)
		for _, m := range p.Matches {
			row([]string{fmt.Sprint(m.Order), name(m.Red), name(m.Blue), score(m)}, matchWidths, false)
		}
	}

	if snap.PoolsComplete && len(snap.Overall) > 0 {
		pdf.AddPage()
		h1(t("Overall ranking"))
		small(t("Everyone across the pools, by the same chain as the pool tables. The seeding for the eliminations."))
		pdf.Ln(2)
		head(standingsCols, standingsWidths)
		for _, st := range snap.Overall {
			row([]string{
				fmt.Sprint(st.Rank), st.Name, st.Club, fmt.Sprint(st.Completed),
				fmt.Sprintf("%d-%d-%d", st.Wins, st.Draws, st.Losses),
				fmtIdx(st.MatchPointIndex), fmtIdx(st.VictoryIndex), fmtIdx(st.ScoreIndex), fmtIdx(st.ReceptionIndex),
			}, standingsWidths, st.Rank <= 8)
		}
	}

	if snap.Bracket != nil {
		pdf.AddPage()
		h1(t("Eliminations"))
		roundName := map[string]string{
			tournament.RoundQuarter: t("Quarter-final"), tournament.RoundSemi: t("Semi-final"),
			tournament.RoundBronze: t("Bronze match"), tournament.RoundFinal: t("Final"),
		}
		bracketWidths := []float64{34, 52, 52, 40}
		head([]string{t("Round"), t("Red"), t("Blue"), t("Result")}, bracketWidths)
		or := func(id string) string {
			if id == "" {
				return "-"
			}
			return name(id)
		}
		for _, m := range snap.Bracket.Matches {
			label := roundName[m.Round]
			if m.Round == tournament.RoundQuarter || m.Round == tournament.RoundSemi {
				label = fmt.Sprintf("%s %d", label, m.Slot)
			}
			row([]string{label, or(m.Red), or(m.Blue), score(m)}, bracketWidths, false)
		}
		if snap.Bracket.Podium.First != "" {
			h2(t("Podium"))
			pdf.SetFont("Helvetica", "", 11)
			pdf.SetTextColor(0, 0, 0)
			for i, id := range []string{snap.Bracket.Podium.First, snap.Bracket.Podium.Second, snap.Bracket.Podium.Third} {
				if id == "" {
					continue
				}
				pdf.CellFormat(0, 7, tr(fmt.Sprintf("%d.  %s  (%s)", i+1, name(id), club(id))), "", 1, "L", false, 0, "")
			}
		}
	}

	if len(snap.Pools) == 0 {
		pdf.AddPage()
		h1(title)
		small(t("The pools have not been drawn yet."))
	}
	return pdf
}
