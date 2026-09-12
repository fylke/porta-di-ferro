//go:build !windows

package lan

// ssids is Windows-only: the shipped product is a Windows executable, and the adapter
// names elsewhere ("wlan0") do not carry the friendly name the Windows lookup keys on.
func ssids() map[string]string { return nil }
