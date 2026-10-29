//go:build !js

package signup

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
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
