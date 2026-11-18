package awake

import "testing"

// Asking to stay up is harmless, on every system, and stops when asked.
func TestKeepStops(t *testing.T) {
	stop := make(chan struct{})
	Keep(stop)
	close(stop)
}
