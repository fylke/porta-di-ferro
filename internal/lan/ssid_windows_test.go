//go:build windows

package lan

import "testing"

// TestParseNetshDoesNotDependOnTheLanguage: the "Name" label is translated on a Swedish
// Windows and SSID is not, so the parser keys on block position and the one stable label.
func TestParseNetshDoesNotDependOnTheLanguage(t *testing.T) {
	swedish := `
Det finns 2 gränssnitt i systemet:

    Namn                   : Wi-Fi
    Beskrivning            : Intel(R) Wi-Fi 6 AX201 160MHz
    GUID                   : 1234
    Fysisk adress          : aa:bb:cc:dd:ee:ff
    Tillstånd              : ansluten
    SSID                   : Hall-Guest
    BSSID                  : 11:22:33:44:55:66
    Nätverkstyp            : Infrastruktur

    Namn                   : Wi-Fi 2
    Beskrivning            : USB adapter
    GUID                   : 5678
    Tillstånd              : frånkopplad

`
	got := parseNetsh(swedish)
	if got["Wi-Fi"] != "Hall-Guest" {
		t.Errorf("Wi-Fi should be on Hall-Guest, got %q", got["Wi-Fi"])
	}
	if _, ok := got["Wi-Fi 2"]; ok {
		t.Errorf("a disconnected adapter has no SSID and should not be listed, got %q", got["Wi-Fi 2"])
	}
}

// TestParseNetshIgnoresBSSID: BSSID contains "SSID" and must not be mistaken for it.
func TestParseNetshIgnoresBSSID(t *testing.T) {
	out := "    Name : Wi-Fi\n    BSSID : 11:22\n    SSID : Club\n"
	if got := parseNetsh(out); got["Wi-Fi"] != "Club" {
		t.Errorf("got %v", got)
	}
}
