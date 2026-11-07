//go:build !js

package signup

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/form"

	"github.com/fylke/porta-di-ferro/internal/store"
)

func TestPDFSignupRoundTrip(t *testing.T) {
	def := Definition{
		Format: DefinitionFormat, Version: Version, DefinitionID: "open-2026",
		Event:       DefinitionEvent{Name: "Öppen tävling", Venue: "Linköping"},
		Tournaments: []TournamentInfo{{ID: "longsword", Label: "Långsvärd"}, {ID: "sabre", Label: "Sabre"}},
		StaffRoles:  Roles,
	}
	blank, err := BuildPDF(def)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsePDF(blank); err == nil || !strings.Contains(err.Error(), "no name") {
		t.Fatalf("blank form: %v", err)
	}
	group, err := pdfapi.ExportForm(bytes.NewReader(blank), "signup.pdf", pdfConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range group.Forms[0].TextFields {
		switch field.Name {
		case "name":
			field.Value = "Åsa Öberg"
		case "club":
			field.Value = "Fäktklubben"
		case "email":
			field.Value = " ASA@Example.org "
		}
	}
	for _, field := range group.Forms[0].CheckBoxes {
		if field.Name == "entry-longsword" {
			field.Value = true
		}
	}
	data, err := json.Marshal(group)
	if err != nil {
		t.Fatal(err)
	}
	var saved bytes.Buffer
	if err := pdfapi.FillForm(bytes.NewReader(blank), bytes.NewReader(data), &saved, pdfConfig()); err != nil {
		t.Fatal(err)
	}
	res, err := parsePDF(saved.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if res.Participant.Name != "Åsa Öberg" || res.Participant.Club != "Fäktklubben" || res.Participant.Email != "asa@example.org" {
		t.Fatalf("participant: %+v", res.Participant)
	}
	if len(res.Entries) != 1 || res.Entries[0] != "longsword" || res.Staff == nil || len(res.Staff.Tournaments) != 1 || res.Staff.Tournaments[0] != "sabre" {
		t.Fatalf("choices: %+v", res)
	}
	response, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseResponse(saved.Bytes()); err == nil {
		t.Fatal("the importer must reject PDF bytes")
	}
	files := []File{{Source: "signup.json", Body: response}, {Source: "copy.json", Body: response}}
	preview := Check(def, "longsword", files, nil, nil, nil)
	if preview.Adding != 1 || preview.Rows[1].Verdict != Repeat {
		t.Fatalf("duplicates: %+v", preview)
	}
	roster := Import(preview, func(_ []store.Competitor) string { return "c1" }, nil)
	if next := Check(def, "longsword", files[:1], roster, nil, nil); next.Adding != 0 || next.Rows[0].Verdict != Already {
		t.Fatalf("reimport: %+v", next)
	}
}

// parsePDF reads a filled form the way the organizer's browser does, to prove the form
// BuildPDF makes can be read back.
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
