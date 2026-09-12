package e2e

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// TestPDFExportIsARealDocument is design §7 item 12 against the real binary: a scored
// tournament comes out as a PDF with a page per pool, and an undrawn one as a PDF that
// says so rather than an error.
func TestPDFExportIsARealDocument(t *testing.T) {
	s := start(t)

	res, err := http.Get(s.base + "/api/export.pdf")
	if err != nil {
		t.Fatal(err)
	}
	empty, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !bytes.HasPrefix(empty, []byte("%PDF-")) {
		t.Fatalf("an undrawn tournament should still export a PDF, got %d %q", res.StatusCode, firstBytes(empty))
	}

	for i := 1; i <= 8; i++ {
		var c store.Competitor
		s.mustDo(t, "POST", "/api/competitors", map[string]string{"name": fmt.Sprintf("Fäktare Öberg %d", i), "club": "MSL Linköping"}, &c)
	}
	s.mustDo(t, "PUT", "/api/tournament", map[string]int{"mats": 2, "minPoolSize": 4, "maxPoolSize": 4}, nil)
	s.mustDo(t, "POST", "/api/tournament/pools", nil, nil)
	var snap snapshot
	s.mustDo(t, "GET", "/api/state", nil, &snap)
	for _, p := range snap.Pools {
		for i, m := range p.Matches {
			scoreMatch(t, s, m.ID, i)
		}
	}

	res, err = http.Get(s.base + "/api/export.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("export returned %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("content type should be application/pdf, is %q", ct)
	}
	if !bytes.HasPrefix(body, []byte("%PDF-")) || !bytes.Contains(body, []byte("%%EOF")) {
		t.Errorf("the body should be a complete PDF, starts %q", firstBytes(body))
	}
	// One page per pool, then the overall ranking once every pool match is in. fpdf
	// writes each page as its own object.
	if pages := bytes.Count(body, []byte("/Type /Page\n")); pages != len(snap.Pools)+1 {
		t.Errorf("two finished pools should be three pages, the document has %d", pages)
	}
	if len(body) < 2000 {
		t.Errorf("a scored two-pool tournament should not fit in %d bytes", len(body))
	}
}

func firstBytes(b []byte) string {
	if len(b) > 16 {
		b = b[:16]
	}
	return string(b)
}
