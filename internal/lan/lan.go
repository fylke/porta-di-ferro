// Package lan finds the addresses on this PC that a device on the venue wifi can actually
// reach.
//
// It exists because the organizer's own browser is on http://localhost, and localhost is
// the one address that is guaranteed not to work from a score keeper's tablet. Putting
// that URL on screen and into the QR code sent every client to a page that could not
// load, which is the worst possible first five minutes for an application whose whole
// promise is that a club can run it themselves (docs/design.md §1).
package lan

import (
	"net"
	"sort"
	"strings"
)

// Address is one place this PC can be reached on.
//
// Interface is the adapter's friendly name -- "Wi-Fi", "Ethernet", "vEthernet (WSL)" on
// Windows -- because on a PC with several networks that name is the only thing that tells
// an organizer which one is the hall.
type Address struct {
	IP        string `json:"ip"`
	Interface string `json:"interface"`
	Private   bool   `json:"private"`
}

// virtual matches the adapters a phone in the hall is never on: hypervisor host-only
// networks, container bridges and overlay VPNs. They are ranked last rather than hidden,
// because "last" is right almost always and "hidden" would be wrong for the organizer who
// really is on one.
var virtual = []string{
	"vethernet", "hyper-v", "virtualbox", "vmware", "vmnet", "docker", "wsl",
	"tailscale", "zerotier", "loopback", "bluetooth", "npcap", "tun", "tap", "utun",
}

// Addresses lists every IPv4 address a client could open, best guess first.
//
// The guess is only ever a default. No heuristic can know which of several networks the
// tablets are on, so the organizer view offers the whole list and remembers the choice;
// what the ordering buys is that the first screen usually shows the right address without
// anyone having to choose at all.
func Addresses() []Address {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []Address{}
	}
	out := []Address{}
	for _, iface := range ifaces {
		// An adapter that is down, or a loopback, cannot carry a score keeper's tablet.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			// IPv4 only. An IPv6 literal has to be bracketed in a URL and read aloud
			// across a table, and every venue LAN this runs on carries IPv4.
			ip := ipnet.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			out = append(out, Address{
				IP:        ip.String(),
				Interface: iface.Name,
				Private:   ip.IsPrivate(),
			})
		}
	}
	sortByPreference(out)
	return out
}

// sortByPreference puts the most likely venue address first. Stable and total, so the
// organizer's list does not reshuffle itself between two loads of the same page.
func sortByPreference(addrs []Address) {
	sort.SliceStable(addrs, func(i, j int) bool {
		a, b := rank(addrs[i]), rank(addrs[j])
		if a != b {
			return a < b
		}
		if addrs[i].Interface != addrs[j].Interface {
			return addrs[i].Interface < addrs[j].Interface
		}
		return addrs[i].IP < addrs[j].IP
	})
}

// rank scores an address, lower being more likely to be the venue LAN.
func rank(a Address) int {
	switch {
	case isLinkLocal(a.IP):
		// 169.254.x.x means DHCP never answered. The adapter is up and has no network.
		return 40
	case isVirtual(a.Interface):
		return 30
	case !a.Private:
		// A public address on an organizer's PC is usually a tethered modem rather than
		// the hall, and handing it out advertises the server beyond the venue.
		return 20
	default:
		return 10
	}
}

func isLinkLocal(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLinkLocalUnicast()
}

func isVirtual(name string) bool {
	lower := strings.ToLower(name)
	for _, v := range virtual {
		if strings.Contains(lower, v) {
			return true
		}
	}
	return false
}
