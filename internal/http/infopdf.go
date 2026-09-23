package httpapi

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The info sheet as a printed page (issue #98).
//
// This one is a poster before it is a web page. It goes on the door of the hall, and the
// spectator reading it has no network yet -- that is the whole point of it -- so it has
// to work on paper with nothing behind it. Everything a phone needs is on it twice: once
// as a code to scan, once as text to type when the camera will not focus in a badly lit
// entrance.
//
// A4 landscape, matching the sketch on the issue: the welcome message with the codes
// beside it, and the day's schedule underneath.

func (s *Server) exportInfoPDF(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// The address a phone should open. The organizer picks it in the admin view and
	// passes it here, because only the browser knows which of this PC's networks the
	// tablets are actually on.
	doc := BuildInfoPDF(snap, r.URL.Query().Get("url"), r.URL.Query().Get("lang") == "sv")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="porta-di-ferro-info.pdf"`)
	if err := doc.Output(w); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
	}
}

var infoSwedish = map[string]string{
	"Welcome":                            "Välkommen",
	"Programme":                          "Program",
	"Join the wifi":                      "Anslut till wifi",
	"Then open":                          "Öppna sedan",
	"Network":                            "Nätverk",
	"Password":                           "Lösenord",
	"Address":                            "Adress",
	"Scan, or type it in":                "Skanna, eller skriv in",
	"No network configured":              "Inget nätverk angivet",
	"Ask the organizer":                  "Fråga arrangören",
	"Results and schedule on your phone": "Resultat och schema i mobilen",
}

// BuildInfoPDF lays the sheet out. Exported for the same reason BuildPDF is: the browser
// demo prints the same document, and one of these is enough.
func BuildInfoPDF(snap Snapshot, landing string, swedish bool) *fpdf.Fpdf {
	tr := func(s string) string {
		if swedish {
			if v, ok := infoSwedish[s]; ok {
				return v
			}
		}
		return s
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(18, 16, 18)
	pdf.SetAutoPageBreak(true, 16)
	pdf.AddPage()
	// The built-in fonts are Latin-1, and the names here are Swedish.
	latin := pdf.UnicodeTranslatorFromDescriptor("")

	event := snap.Tournament.Event

	// The heading: the discipline this run is, or the application's name when it has not
	// been named. A sheet that says "Unnamed" on a door helps nobody.
	title := strings.TrimSpace(snap.Instance.Name)
	if title == "" {
		title = "Porta di Ferro"
	}
	pdf.SetFont("Helvetica", "B", 30)
	pdf.CellFormat(0, 14, latin(title), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 12)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 7, latin(tr("Results and schedule on your phone")), "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(6)

	// Two columns: the welcome on the left, the way in on the right.
	const rightWidth = 95.0
	left, top := pdf.GetX(), pdf.GetY()
	pageW, _ := pdf.GetPageSize()
	leftWidth := pageW - 36 - rightWidth - 10

	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(leftWidth, 8, latin(tr("Welcome")), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 12)
	if welcome := strings.TrimSpace(event.Welcome); welcome != "" {
		pdf.MultiCell(leftWidth, 6.2, latin(welcome), "", "L", false)
	}
	leftBottom := pdf.GetY()

	// The right column, starting level with the left.
	pdf.SetXY(left+leftWidth+10, top)
	drawWayIn(pdf, latin, tr, event.Wifi, landing, rightWidth)
	rightBottom := pdf.GetY()

	y := leftBottom
	if rightBottom > y {
		y = rightBottom
	}
	pdf.SetXY(left, y+8)

	// The schedule, full width underneath, as the sketch has it -- in two columns once
	// there are enough items that one would push the sheet onto a second page. This goes
	// on a door; a poster that is two pages is a poster somebody has to staple.
	if len(event.Schedule) > 0 {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(0, 8, latin(tr("Programme")), "", 1, "L", false, 0, "")
		pdf.Ln(1)

		usable := pageW - 36
		perColumn := len(event.Schedule)
		columnWidth := usable
		if len(event.Schedule) > 5 {
			perColumn = (len(event.Schedule) + 1) / 2
			columnWidth = usable/2 - 6
		}

		top := pdf.GetY()
		for i, item := range event.Schedule {
			column, row := i/perColumn, i%perColumn
			pdf.SetXY(left+float64(column)*(columnWidth+12), top+float64(row)*7.2)

			style := ""
			if item.Kind == "discipline" {
				style = "B"
			}
			if item.Kind == "break" {
				pdf.SetTextColor(110, 110, 110)
			}
			pdf.SetFont("Helvetica", style, 12)
			pdf.CellFormat(24, 7.2, latin(item.At), "", 0, "L", false, 0, "")
			pdf.CellFormat(columnWidth-24, 7.2, latin(item.Label), "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		}
	}

	return pdf
}

// drawWayIn is the right-hand column: how to get on the network and where to go once on
// it, each as a code and as text.
func drawWayIn(pdf *fpdf.Fpdf, latin func(string) string, tr func(string) string,
	wifi store.Wifi, landing string, rightWidth float64) {

	x := pdf.GetX()
	const qrSize = 34.0

	if payload := WifiQR(wifi); payload != "" {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.CellFormat(0, 8, latin(tr("Join the wifi")), "", 1, "L", false, 0, "")
		pdf.SetX(x)
		y := pdf.GetY()
		drawQR(pdf, payload, x, y, qrSize, "wifi")
		pdf.SetXY(x+qrSize+6, y+3)
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(0, 5.5, latin(tr("Network")), "", 2, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 6, latin(wifi.SSID), "", 2, "L", false, 0, "")
		if wifi.Security != "nopass" {
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(90, 90, 90)
			pdf.CellFormat(0, 5.5, latin(tr("Password")), "", 2, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Courier", "B", 12)
			pdf.CellFormat(0, 6, latin(wifi.Password), "", 2, "L", false, 0, "")
		}
		pdf.SetXY(x, y+qrSize+6)
	}

	if landing == "" {
		return
	}
	pdf.SetX(x)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(0, 8, latin(tr("Then open")), "", 1, "L", false, 0, "")
	pdf.SetX(x)
	y := pdf.GetY()
	drawQR(pdf, landing, x, y, qrSize, "landing")
	pdf.SetXY(x+qrSize+6, y+4)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(90, 90, 90)
	pdf.MultiCell(rightWidth-qrSize-6, 5, latin(tr("Scan, or type it in")), "", "L", false)
	pdf.SetTextColor(0, 0, 0)
	// Under the code and across the column: beside it there is room for about eighteen
	// characters, and a venue address is longer than that.
	pdf.SetXY(x, y+qrSize+2)
	pdf.SetFont("Courier", "B", 12)
	pdf.CellFormat(rightWidth, 6, latin(landing), "", 1, "L", false, 0, "")
	pdf.SetXY(x, pdf.GetY()+1)
}

// drawQR renders a code straight into the document. A failure is left blank rather than
// failing the sheet: the address and the password are printed beside it in text, so a
// sheet with one code missing is still a usable sheet.
func drawQR(pdf *fpdf.Fpdf, payload string, x, y, size float64, name string) {
	png, err := qrcode.Encode(payload, qrcode.Medium, 512)
	if err != nil {
		return
	}
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
	pdf.ImageOptions(name, x, y, size, size, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}
