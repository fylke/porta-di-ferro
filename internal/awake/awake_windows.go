//go:build windows

package awake

import "syscall"

var setThreadExecutionState = syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// esSystemRequired resets the system's idle timer: it does not sleep for want of a user.
// Without ES_CONTINUOUS it is a one-off, so it does not matter which thread asks.
const esSystemRequired = 0x00000001

func stayUp() {
	if setThreadExecutionState.Find() == nil {
		setThreadExecutionState.Call(esSystemRequired)
	}
}
