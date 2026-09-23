package httpapi_test

import (
	"strings"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The wifi code on the printed info sheet (issue #98). The format is not ours -- it is
// what every phone camera has implemented for a decade -- and the escaping is the part
// that gets discovered at a venue rather than at a desk.
func TestWifiQRPayload(t *testing.T) {
	cases := []struct {
		name string
		in   store.Wifi
		want string
	}{
		{
			name: "the ordinary case",
			in:   store.Wifi{SSID: "Hall-Guest", Password: "fencing2026"},
			want: `WIFI:T:WPA;S:Hall-Guest;P:fencing2026;;`,
		},
		{
			name: "no network means no code, rather than a code that does nothing",
			in:   store.Wifi{Password: "orphaned"},
			want: "",
		},
		{
			name: "an open network carries no password field at all",
			in:   store.Wifi{SSID: "Hall-Open", Security: "nopass", Password: "ignored"},
			want: `WIFI:T:nopass;S:Hall-Open;;`,
		},
		{
			name: "a hidden network says so",
			in:   store.Wifi{SSID: "Back-Office", Password: "x", Hidden: true},
			want: `WIFI:T:WPA;S:Back-Office;P:x;H:true;;`,
		},
		{
			// The separators of the format are exactly the characters a venue password
			// is most likely to contain. Unescaped, the scanner reads a truncated
			// password and the spectator is told the wifi key is wrong.
			name: "separators in the password are escaped",
			in:   store.Wifi{SSID: "Hall;Guest", Password: `a;b,c:d\e"f`},
			want: `WIFI:T:WPA;S:Hall\;Guest;P:a\;b\,c\:d\\e\"f;;`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := httpapi.WifiQR(c.in); got != c.want {
				t.Errorf("WifiQR() = %q, want %q", got, c.want)
			}
		})
	}
}

// The info sheet is a poster: it has to come out of the printer with everything a
// spectator needs, including when half of it has not been filled in.
func TestTheInfoSheetIsAPrintableDocument(t *testing.T) {
	snap := httpapi.Snapshot{
		Instance: httpapi.Instance{Name: "Open steel Longsword"},
	}
	snap.Tournament.Event = store.Event{
		Welcome:  "Welcome to Stångebroslaget. Fencing starts at nine.",
		Wifi:     store.Wifi{SSID: "Hall-Guest", Password: "fencing2026"},
		Schedule: []store.ScheduleItem{{At: "09:00", Label: "Gear check", Kind: "break"}, {At: "10:00", Label: "Longsword pools", Kind: "discipline"}},
	}

	var buf strings.Builder
	doc := httpapi.BuildInfoPDF(snap, "http://192.168.1.20:8080/", false)
	if err := doc.Output(&writerOnly{&buf}); err != nil {
		t.Fatalf("writing the sheet: %v", err)
	}
	if n := buf.Len(); n < 2000 {
		t.Errorf("the sheet is %d bytes, which is not a printed page with two QR codes on it", n)
	}
	if !strings.HasPrefix(buf.String(), "%PDF-") {
		t.Error("that is not a PDF")
	}

	// An event nobody has configured yet still prints: an organizer testing the printer
	// on the morning should not meet a crash.
	empty := httpapi.BuildInfoPDF(httpapi.Snapshot{}, "", false)
	var bare strings.Builder
	if err := empty.Output(&writerOnly{&bare}); err != nil {
		t.Fatalf("an empty event should still print: %v", err)
	}
	if !strings.HasPrefix(bare.String(), "%PDF-") {
		t.Error("the empty sheet is not a PDF")
	}
}

// fpdf wants an io.Writer; strings.Builder is one, but Output takes it by interface and
// this keeps the intent obvious.
type writerOnly struct{ b *strings.Builder }

func (w *writerOnly) Write(p []byte) (int, error) { return w.b.Write(p) }
