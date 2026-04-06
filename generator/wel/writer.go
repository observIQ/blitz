package wel

import "context"

// EventWriter writes events to the Windows Event Log.
type EventWriter interface {
	// Setup registers event sources for the configured channels.
	Setup(ctx context.Context, channels []string) error
	// WriteEvent writes a single event to the Windows Event Log.
	WriteEvent(ctx context.Context, event *EventRecord) error
	// Close cleans up resources.
	Close() error
}
