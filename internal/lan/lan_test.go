package lan

import "testing"

// TestSortByPreferencePutsTheVenueFirst pins the ordering that decides which address the
// organizer view offers first, because getting it wrong is not a cosmetic error: it is a
// QR code that sends every score keeper to a page that will not load.
func TestSortByPreferencePutsTheVenueFirst(t *testing.T) {
	addrs := []Address{
		{IP: "169.254.7.7", Interface: "Ethernet 2", Private: false},
		{IP: "203.0.113.9", Interface: "Mobile Broadband", Private: false},
		{IP: "172.28.144.1", Interface: "vEthernet (WSL)", Private: true},
		{IP: "192.168.1.20", Interface: "Wi-Fi", Private: true},
	}
	sortByPreference(addrs)

	want := []string{"192.168.1.20", "203.0.113.9", "172.28.144.1", "169.254.7.7"}
	for i, ip := range want {
		if addrs[i].IP != ip {
			t.Errorf("position %d is %s (%s), want %s", i, addrs[i].IP, addrs[i].Interface, ip)
		}
	}
}

// TestSortByPreferenceIsStable guards against the list reshuffling between two loads of
// the organizer page, which would make the remembered choice look like it had moved.
func TestSortByPreferenceIsStable(t *testing.T) {
	addrs := []Address{
		{IP: "192.168.1.30", Interface: "Ethernet", Private: true},
		{IP: "192.168.1.20", Interface: "Ethernet", Private: true},
		{IP: "10.0.0.5", Interface: "Wi-Fi", Private: true},
	}
	sortByPreference(addrs)
	first := []Address{}
	first = append(first, addrs...)
	sortByPreference(addrs)

	for i := range first {
		if first[i] != addrs[i] {
			t.Fatalf("sorting twice changed the order at %d: %+v then %+v", i, first[i], addrs[i])
		}
	}
	if addrs[0].IP != "192.168.1.20" || addrs[1].IP != "192.168.1.30" {
		t.Errorf("equal ranks should order by interface then IP, got %+v", addrs)
	}
}

// TestIsVirtualMatchesTheAdaptersNobodyIsOn is the list an organizer with Hyper-V, WSL or
// a VPN installed depends on: those adapters are up and private, and would otherwise
// outrank the wifi.
func TestIsVirtualMatchesTheAdaptersNobodyIsOn(t *testing.T) {
	for _, name := range []string{
		"vEthernet (Default Switch)", "VirtualBox Host-Only Network",
		"VMware Network Adapter VMnet8", "Tailscale", "docker0", "utun3",
	} {
		if !isVirtual(name) {
			t.Errorf("%q should be ranked below a real adapter", name)
		}
	}
	for _, name := range []string{"Wi-Fi", "Ethernet", "Wireless Network Connection", "en0"} {
		if isVirtual(name) {
			t.Errorf("%q is a real adapter and should not be demoted", name)
		}
	}
}

// TestAddressesNeverOffersLoopback is the whole point of the package: whatever this PC
// looks like, the list must not contain the one address a tablet cannot reach.
func TestAddressesNeverOffersLoopback(t *testing.T) {
	for _, a := range Addresses() {
		if a.IP == "127.0.0.1" || a.IP == "::1" {
			t.Errorf("loopback %s reached the client list", a.IP)
		}
	}
}
