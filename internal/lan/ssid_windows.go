//go:build windows

package lan

import (
	"bufio"
	"os/exec"
	"strings"
	"syscall"
)

// ssids maps each wireless adapter's name to the network it is on, by asking netsh.
//
// The IP of a Wi-Fi adapter says nothing an organizer can act on; the network's name
// does. A PC that joined the wrong wifi -- the club's office network rather than the
// hall's -- is a reasonable thing to have happen, and "Wi-Fi Hall-Guest" against "Wi-Fi
// Office" is how it gets noticed.
//
// netsh rather than the WLAN API: it is on every Windows install, and one process spawn
// per refresh is nothing. The output is localised, so nothing here depends on the word
// "Name" -- each adapter is a block of "key : value" lines, the first line of a block is
// its name, and SSID is the one label Windows does not translate.
func ssids() map[string]string {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseNetsh(string(out))
}

func parseNetsh(out string) map[string]string {
	found := map[string]string{}
	var name, ssid string
	flush := func() {
		if name != "" && ssid != "" {
			found[name] = ssid
		}
		name, ssid = "", ""
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch {
		case name == "":
			name = value
		case strings.EqualFold(key, "SSID"):
			ssid = value
		}
	}
	flush()
	return found
}
