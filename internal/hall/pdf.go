package hall

import (
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
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

// BuildPDF lays the snapshot out on A4. The core fonts cover Latin-1, which covers
// Swedish; a name from further afield loses its accents rather than breaking the page.
func BuildPDF(snap Snapshot, swedish bool) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	d := results{pdf: pdf, tr: pdf.UnicodeTranslatorFromDescriptor(""), swedish: swedish, snap: snap}
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	d.title = "Porta di Ferro"
	if snap.Instance.Name != "" {
		d.title += " -- " + snap.Instance.Name
	}
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5, d.tr(fmt.Sprintf("%s  ·  %s  ·  "+d.t("page %d"), d.title, time.Now().Format("2006-01-02 15:04"), pdf.PageNo())), "", 0, "C", false, 0, "")
	})

	for _, p := range snap.Pools {
		d.pool(p)
	}
	if snap.PoolsComplete && len(snap.Overall) > 0 {
		d.overall()
	}
	if snap.Bracket != nil {
		d.bracket()
	}
	if len(snap.Pools) == 0 {
		pdf.AddPage()
		d.h1(d.title)
		d.small(d.t("The pools have not been drawn yet."))
	}
	return pdf
}

// results is the export as it is laid out.
type results struct {
	pdf     *fpdf.Fpdf
	tr      func(string) string
	swedish bool
	snap    Snapshot
	title   string
}

var (
	standingsWidths = []float64{8, 52, 38, 10, 22, 12, 12, 12, 12}
	matchWidths     = []float64{8, 56, 56, 40}
	bracketWidths   = []float64{34, 52, 52, 40}
)

// t is the export's own word for a key. Translation happens before the Latin-1 pass, so the
// Swedish comes out with its letters intact. A key with no entry comes out in English.
func (d results) t(key string) string {
	if d.swedish {
		if v, ok := pdfSwedish[key]; ok {
			return v
		}
	}
	return key
}

func (d results) competitor(id string) (store.Competitor, bool) {
	for _, c := range d.snap.Competitors {
		if c.ID == id {
			return c, true
		}
	}
	return store.Competitor{}, false
}

func (d results) name(id string) string {
	if c, ok := d.competitor(id); ok {
		return c.Name
	}
	return "?"
}

func (d results) club(id string) string {
	c, _ := d.competitor(id)
	return c.Club
}

func (d results) h1(s string) {
	d.pdf.SetFont("Helvetica", "B", 18)
	d.pdf.SetTextColor(0, 0, 0)
	d.pdf.CellFormat(0, 10, d.tr(s), "", 1, "L", false, 0, "")
}

func (d results) h2(s string) {
	d.pdf.Ln(2)
	d.pdf.SetFont("Helvetica", "B", 12)
	d.pdf.SetTextColor(0, 0, 0)
	d.pdf.CellFormat(0, 8, d.tr(s), "", 1, "L", false, 0, "")
}

func (d results) small(s string) {
	d.pdf.SetFont("Helvetica", "", 9)
	d.pdf.SetTextColor(90, 90, 90)
	d.pdf.CellFormat(0, 5, d.tr(s), "", 1, "L", false, 0, "")
}

// cells is one row of a table: the second and third columns are text, the rest numbers.
func (d results) cells(cells []string, widths []float64, fill bool) {
	for i, c := range cells {
		align := "R"
		if i == 1 || i == 2 {
			align = "L"
		}
		d.pdf.CellFormat(widths[i], 6, d.tr(c), "1", 0, align, fill, 0, "")
	}
	d.pdf.Ln(-1)
}

func (d results) head(cols []string, widths []float64) {
	d.pdf.SetFont("Helvetica", "B", 8)
	d.pdf.SetFillColor(235, 235, 235)
	d.pdf.SetTextColor(0, 0, 0)
	d.cells(cols, widths, true)
}

func (d results) row(cells []string, widths []float64, bold bool) {
	style := ""
	if bold {
		style = "B"
	}
	d.pdf.SetFont("Helvetica", style, 9)
	d.pdf.SetTextColor(0, 0, 0)
	d.cells(cells, widths, false)
}

func (d results) score(m MatchView) string {
	if m.Status == "pending" {
		return "-"
	}
	note := ""
	switch m.State.Reason {
	case match.ReasonForfeit:
		note = d.t(" (forfeit)")
	case match.ReasonPenalty:
		note = d.t(" (penalty)")
	}
	return fmt.Sprintf("%d-%d%s", m.State.Red.Score, m.State.Blue.Score, note)
}

// standings is a table of standings, the rows down to top in bold.
func (d results) standings(rows []tournament.Standing, top int) {
	d.head([]string{"#", d.t("Competitor"), d.t("Club"), d.t("M"), d.t("W-D-L"), "MPI", "VI", "SI", "RI"}, standingsWidths)
	index := func(f float64) string { return fmt.Sprintf("%.2f", f) }
	for _, st := range rows {
		d.row([]string{
			fmt.Sprint(st.Rank), st.Name, st.Club, fmt.Sprint(st.Completed),
			fmt.Sprintf("%d-%d-%d", st.Wins, st.Draws, st.Losses),
			index(st.MatchPointIndex), index(st.VictoryIndex), index(st.ScoreIndex), index(st.ReceptionIndex),
		}, standingsWidths, st.Rank <= top)
	}
}

func (d results) pool(p PoolView) {
	d.pdf.AddPage()
	d.h1(fmt.Sprintf(d.t("Pool %d"), p.Number))
	status := d.t("in progress")
	if p.Complete {
		status = d.t("complete")
	}
	d.small(fmt.Sprintf(d.t("Mat %d  ·  %d competitors  ·  %s"), p.Mat, len(p.Competitors), status))

	d.h2(d.t("Standings"))
	d.standings(p.Standings, 1)
	d.small(d.t("MPI match point index, VI victory index, SI score index, RI reception index (lowest wins); all per match completed."))

	d.h2(d.t("Matches"))
	d.head([]string{"#", d.t("Red"), d.t("Blue"), d.t("Result")}, matchWidths)
	for _, m := range p.Matches {
		d.row([]string{fmt.Sprint(m.Order), d.name(m.Red), d.name(m.Blue), d.score(m)}, matchWidths, false)
	}
}

func (d results) overall() {
	d.pdf.AddPage()
	d.h1(d.t("Overall ranking"))
	d.small(d.t("Everyone across the pools, by the same chain as the pool tables. The seeding for the eliminations."))
	d.pdf.Ln(2)
	d.standings(d.snap.Overall, 8)
}

func (d results) bracket() {
	d.pdf.AddPage()
	d.h1(d.t("Eliminations"))
	roundName := map[string]string{
		tournament.RoundQuarter: d.t("Quarter-final"), tournament.RoundSemi: d.t("Semi-final"),
		tournament.RoundBronze: d.t("Bronze match"), tournament.RoundFinal: d.t("Final"),
	}
	d.head([]string{d.t("Round"), d.t("Red"), d.t("Blue"), d.t("Result")}, bracketWidths)
	or := func(id string) string {
		if id == "" {
			return "-"
		}
		return d.name(id)
	}
	for _, m := range d.snap.Bracket.Matches {
		label := roundName[m.Round]
		if m.Round == tournament.RoundQuarter || m.Round == tournament.RoundSemi {
			label = fmt.Sprintf("%s %d", label, m.Slot)
		}
		d.row([]string{label, or(m.Red), or(m.Blue), d.score(m)}, bracketWidths, false)
	}
	podium := d.snap.Bracket.Podium
	if podium.First == "" {
		return
	}
	d.h2(d.t("Podium"))
	d.pdf.SetFont("Helvetica", "", 11)
	d.pdf.SetTextColor(0, 0, 0)
	for i, id := range []string{podium.First, podium.Second, podium.Third} {
		if id == "" {
			continue
		}
		d.pdf.CellFormat(0, 7, d.tr(fmt.Sprintf("%d.  %s  (%s)", i+1, d.name(id), d.club(id))), "", 1, "L", false, 0, "")
	}
}
