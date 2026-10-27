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

// The generated programme goes on the sheet in the sheet's language (#149): a discipline's
// stages under its name, a nameless break as lunch.
func TestTheInfoSheetPrintsTheGeneratedProgramme(t *testing.T) {
	rows := []httpapi.ProgrammeRow{
		{Discipline: "ls", Name: "Longsword", Stage: "gear-check", Start: "2026-11-14T08:30:00+01:00"},
		{Discipline: "ls", Name: "Longsword", Stage: "pools", Start: "2026-11-14T09:00:00+01:00"},
		{Name: "Lunch", Stage: "break", Start: "2026-11-14T12:00:00+01:00"},
	}
	sv := map[string]string{"Gear check": "Utrustningskontroll", "Pools": "Pooler", "Lunch": "Lunch"}
	got := httpapi.ProgrammeSchedule(rows, func(s string) string { return sv[s] })
	if len(got) != 3 || got[0].At != "08:30" || got[0].Label != "Longsword: Utrustningskontroll" ||
		got[1].Kind != "discipline" || got[2].Kind != "break" || got[2].Label != "Lunch" {
		t.Errorf("the programme should be the sheet's rows: %+v", got)
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
	doc := httpapi.BuildInfoPDF(snap, nil, "http://192.168.1.20:8080/", false)
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
	empty := httpapi.BuildInfoPDF(httpapi.Snapshot{}, nil, "", false)
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
