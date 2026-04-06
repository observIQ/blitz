//go:build !windows

package wel

import "fmt"

// NewEventWriter returns an error on non-Windows platforms.
// The WEL generator requires Windows to write events to the Event Log.
func NewEventWriter(_ bool) (EventWriter, error) {
	return nil, fmt.Errorf("wel generator requires Windows; run blitz directly on the Windows target")
}
