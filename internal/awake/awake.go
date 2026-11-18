// Package awake keeps the organizer's PC from going to sleep while the server runs.
//
// The whole hall depends on that one PC: when it sleeps, every tablet, screen and phone
// loses the server mid-match (fylke, #165). Windows lets a program ask for the system to
// stay up -- the screen may still turn off -- which is what Keep does there. Elsewhere it
// does nothing. Closing a laptop's lid can still sleep it if the power settings say so.
package awake

import "time"

// every is how often the request is renewed. Windows's idle timer is minutes long; asking
// once a minute keeps well inside it.
const every = time.Minute

// Keep asks the system to stay awake until stop is closed.
func Keep(stop <-chan struct{}) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			stayUp()
			select {
			case <-stop:
				return
			case <-t.C:
			}
		}
	}()
}
