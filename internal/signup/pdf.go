//go:build !js

package signup

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdffont "github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/form"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/primitives"
)

const MaxPDFResponse = 2 << 20

func init() {
	pdfapi.DisableConfigDir()
}

func pdfConfig() *model.Configuration {
	conf := model.NewDefaultConfiguration()
	conf.Offline = true
	return conf
}

func BuildPDF(def Definition) ([]byte, error) {
	if missing := Ready(def); len(missing) > 0 {
		return nil, fmt.Errorf("signup needs %s", strings.Join(missing, ", "))
	}
	pages := map[string]any{}
	var content *primitives.Content
	var y float64
	var contentBottom float64
	var textStartsAtTop bool
	font := &primitives.FormFont{Name: "Helvetica", Size: 13}
	titleFont := &primitives.FormFont{Name: "Helvetica-Bold", Size: 20}
	const controlHeight = 17.6
	const fieldRowHeight = 26
	const fieldGroupPadding = 18
	const rowGap = fieldRowHeight - controlHeight
	const labelWidth = 100
	const labelGap = 8
	const inputX = 40 + labelWidth + labelGap
	const controlBackground = "#f2f4f6"
	controlBorder := &primitives.Border{Width: 1, Color: "#707070"}
	newPage := func() {
		content = &primitives.Content{}
		pages[strconv.Itoa(len(pages)+1)] = map[string]any{"content": content}
		y = 790
		contentBottom = y
	}
	newPage()
	space := func(height float64) {
		if y-height < 45 {
			newPage()
		}
	}
	wrap := func(value string, width float64, textFont *primitives.FormFont) []string {
		var lines []string
		line := ""
		for _, word := range strings.Fields(value) {
			next := strings.TrimSpace(line + " " + word)
			if line != "" && pdffont.TextWidth(next, textFont.Name, textFont.Size) > width {
				lines = append(lines, line)
				line = word
			} else {
				line = next
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
		return lines
	}
	text := func(value string, textFont *primitives.FormFont) {
		lineHeight := float64(textFont.Size) + 4
		if textStartsAtTop {
			y -= pdffont.LineHeight(textFont.Name, textFont.Size)
			textStartsAtTop = false
		}
		lines := wrap(value, 510, textFont)
		space(float64(len(lines))*lineHeight + rowGap)
		for _, line := range lines {
			content.TextBoxes = append(content.TextBoxes, &primitives.TextBox{
				Value: line, Position: [2]float64{40, y}, Width: 510, Font: textFont,
			})
			contentBottom = y
			y -= lineHeight
		}
		y -= rowGap
	}
	field := func(id, label, value string, locked bool) {
		space(fieldRowHeight)
		bottom := y - 24
		labelY := bottom + (controlHeight-pdffont.LineHeight(font.Name, font.Size))/2
		content.TextBoxes = append(content.TextBoxes, &primitives.TextBox{
			Value: label, Position: [2]float64{inputX - labelGap, labelY}, Width: labelWidth, Font: font, Alignment: "Right",
		})
		content.TextFields = append(content.TextFields, &primitives.TextField{
			ID: id, Tip: label, Value: value, Locked: locked,
			Position: [2]float64{inputX, bottom}, Width: 550 - inputX, Height: controlHeight, Font: font,
			BackgroundColor: controlBackground, Border: controlBorder,
		})
		y -= fieldRowHeight
	}
	type fieldSpec struct {
		id, label, value string
		locked           bool
	}
	fieldGroup := func(rows []fieldSpec) {
		y = contentBottom - fieldGroupPadding + 24 - controlHeight
		space(float64(len(rows)-1)*fieldRowHeight + controlHeight + fieldGroupPadding)
		for _, row := range rows {
			field(row.id, row.label, row.value, row.locked)
		}
		contentBottom = y + fieldRowHeight - 24
		y = contentBottom - fieldGroupPadding
		textStartsAtTop = true
	}
	checkbox := func(id, label string, checked bool) {
		const captionX = 40 + controlHeight + labelGap
		lines := wrap(label, 550-captionX, font)
		lineHeight := float64(font.Size) + 4
		height := fieldRowHeight + float64(len(lines)-1)*lineHeight
		space(height)
		bottom := y - 3
		labelY := bottom + (controlHeight-pdffont.LineHeight(font.Name, font.Size))/2
		content.CheckBoxes = append(content.CheckBoxes, &primitives.CheckBox{
			ID: id, Tip: label, Value: checked, Default: checked,
			Position: [2]float64{40, bottom}, Width: controlHeight, BackgroundColor: controlBackground,
		})
		for index, line := range lines {
			content.TextBoxes = append(content.TextBoxes, &primitives.TextBox{
				Value: line, Position: [2]float64{captionX, labelY - float64(index)*lineHeight}, Width: 550 - captionX, Font: font,
			})
		}
		contentBottom = bottom - float64(len(lines)-1)*lineHeight
		y -= height
	}

	text(def.Event.Name, titleFont)
	text(strings.Join(nonempty(def.Event.Venue, def.Event.Date), " - "), font)
	text("Complete the fields and checkboxes, save the PDF, and return the saved file to the organizer. Do not print or flatten it; handwritten annotations cannot be imported.", font)
	fieldGroup([]fieldSpec{
		{"name", "Name", "", false},
		{"club", "Club", "", false},
		{"email", "Email", "", false},
	})
	text("Disciplines to fence in", font)
	for _, discipline := range def.Tournaments {
		label := discipline.Label
		if discipline.StartsAt != "" {
			label += " - from " + discipline.StartsAt
		}
		checkbox("entry-"+discipline.ID, label, false)
	}
	text("Disciplines to help staff (fencing takes priority where both are checked)", font)
	for _, discipline := range def.Tournaments {
		checkbox("staff-"+discipline.ID, discipline.Label, true)
	}
	text("Staff roles", font)
	for _, role := range def.StaffRoles {
		checkbox("role-"+role.ID, role.Label, role.Offered)
	}
	if len(def.Schedule) > 0 {
		text("Programme", font)
		for _, item := range def.Schedule {
			text(strings.Join(nonempty(item.StartsAt, item.EndsAt, item.Label), " - "), font)
		}
	}
	fieldGroup([]fieldSpec{
		{"porta-event", "Event identifier", def.DefinitionID, true},
		{"porta-format", "Signup format", ResponseFormat + "/" + strconv.Itoa(Version), true},
	})

	layout, err := json.Marshal(map[string]any{"paper": "A4", "pages": pages})
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := pdfapi.Create(nil, bytes.NewReader(layout), &out, pdfConfig()); err != nil {
		return nil, fmt.Errorf("create signup PDF: %w", err)
	}
	return out.Bytes(), nil
}

func nonempty(values ...string) []string {
	var out []string
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parsePDF(body []byte) (res Response, err error) {
	defer func() {
		if recover() != nil {
			res, err = Response{}, fmt.Errorf("not readable as a PDF signup form")
		}
	}()
	if len(body) > MaxPDFResponse {
		return Response{}, fmt.Errorf("the PDF exceeds the 2 MB signup limit")
	}
	fields, err := pdfapi.FormFields(bytes.NewReader(body), pdfConfig())
	if err != nil {
		return Response{}, fmt.Errorf("cannot read PDF form fields; return the saved fillable form, not a scan, annotation or printed copy")
	}
	values := map[string]string{}
	res.Format, res.Version = ResponseFormat, Version
	res.Staff = &StaffOffer{}
	for _, field := range fields {
		if _, exists := values[field.Name]; exists {
			return Response{}, fmt.Errorf("duplicate PDF field %q", field.Name)
		}
		values[field.Name] = field.V
		if field.Typ != form.FTCheckBox || field.V != "Yes" {
			continue
		}
		switch {
		case strings.HasPrefix(field.Name, "entry-"):
			res.Entries = append(res.Entries, strings.TrimPrefix(field.Name, "entry-"))
		case strings.HasPrefix(field.Name, "staff-"):
			res.Staff.Tournaments = append(res.Staff.Tournaments, strings.TrimPrefix(field.Name, "staff-"))
		case strings.HasPrefix(field.Name, "role-"):
			res.Staff.Roles = append(res.Staff.Roles, strings.TrimPrefix(field.Name, "role-"))
		}
	}
	if values["porta-format"] != ResponseFormat+"/"+strconv.Itoa(Version) || values["porta-event"] == "" {
		return Response{}, fmt.Errorf("not a supported Porta signup PDF; ask the organizer for a new form")
	}
	res.DefinitionID = values["porta-event"]
	res.Participant = Participant{Name: values["name"], Club: values["club"], Email: values["email"]}
	identity := strings.TrimSpace(res.DefinitionID) + "\x00" + strings.ToLower(strings.TrimSpace(res.Participant.Email))
	res.SubmissionID = fmt.Sprintf("pdf-%x", sha256.Sum256([]byte(identity)))
	encoded, err := json.Marshal(res)
	if err != nil {
		return Response{}, err
	}
	return ParseResponse(encoded)
}
