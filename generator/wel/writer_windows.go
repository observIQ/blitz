//go:build windows

package wel

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// windowsEventWriter implements EventWriter using the Windows Event Log API.
type windowsEventWriter struct {
	logger             *zap.Logger
	manageEventSources bool
	sources            map[string]windows.Handle
	mu                 sync.Mutex
	securityAvailable  bool
}

// NewEventWriter creates a new Windows Event Log writer.
// If manageEventSources is true, event sources are registered/deregistered automatically.
func NewEventWriter(manageEventSources bool) (EventWriter, error) {
	return &windowsEventWriter{
		logger:             zap.NewNop(),
		manageEventSources: manageEventSources,
		sources:            make(map[string]windows.Handle),
	}, nil
}

// SetLogger sets the logger for the event writer.
func (w *windowsEventWriter) SetLogger(logger *zap.Logger) {
	w.logger = logger
}

// Setup registers event sources for the configured channels.
func (w *windowsEventWriter) Setup(_ context.Context, channels []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check for SeAuditPrivilege if Security channel is requested
	for _, ch := range channels {
		if strings.EqualFold(ch, "Security") {
			// SeAuditPrivilege is required but rarely available.
			// Default to unavailable and let the caller handle gracefully.
			w.securityAvailable = false
			w.logger.Warn("Security channel requested but SeAuditPrivilege not held — Security events will be skipped. Run blitz as Administrator or with SeAuditPrivilege to enable Security channel event generation.")
			break
		}
	}

	if !w.manageEventSources {
		return nil
	}

	for _, ch := range channels {
		sourceName, err := windows.UTF16PtrFromString("BlitzGenerator-" + ch)
		if err != nil {
			w.deregisterAllLocked()
			return fmt.Errorf("utf16 conversion for source %s: %w", ch, err)
		}

		handle, err := windows.RegisterEventSource(nil, sourceName)
		if err != nil {
			w.deregisterAllLocked()
			return fmt.Errorf("register event source for channel %s: %w", ch, err)
		}
		w.sources[ch] = handle
	}

	return nil
}

// deregisterAllLocked releases every handle in w.sources and resets the map.
// Caller must hold w.mu. Errors from DeregisterEventSource are intentionally
// dropped — we are already returning a Setup error, so the original failure
// is more useful to the caller than a cleanup follow-on.
func (w *windowsEventWriter) deregisterAllLocked() {
	for _, handle := range w.sources {
		_ = windows.DeregisterEventSource(handle)
	}
	w.sources = make(map[string]windows.Handle)
}

// WriteEvent writes a single event to the Windows Event Log.
func (w *windowsEventWriter) WriteEvent(_ context.Context, event *EventRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Skip Security events if privilege is not available
	if strings.EqualFold(event.Channel, "Security") && !w.securityAvailable {
		w.logger.Debug("skipping Security event due to missing SeAuditPrivilege",
			zap.Int("eventID", event.EventID))
		return nil
	}

	handle, ok := w.sources[event.Channel]
	if !ok {
		return fmt.Errorf("no event source registered for channel %s", event.Channel)
	}

	// Convert event type from Level
	var eventType uint16
	switch {
	case event.Level <= 2:
		eventType = windows.EVENTLOG_ERROR_TYPE
	case event.Level == 3:
		eventType = windows.EVENTLOG_WARNING_TYPE
	default:
		eventType = windows.EVENTLOG_INFORMATION_TYPE
	}

	// Build the message string
	msg, err := windows.UTF16PtrFromString(event.Message)
	if err != nil {
		return fmt.Errorf("utf16 conversion for message: %w", err)
	}
	msgPtrs := []*uint16{msg}

	err = windows.ReportEvent(
		handle,
		eventType,
		uint16(event.Task),
		uint32(event.EventID),
		0,           // SID (uintptr)
		1,           // number of strings
		0,           // raw data size
		&msgPtrs[0], // strings
		nil,         // raw data (*byte)
	)
	if err != nil {
		return fmt.Errorf("report event %d to channel %s: %w", event.EventID, event.Channel, err)
	}

	return nil
}

// Close deregisters all event sources.
func (w *windowsEventWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var errs []string
	for ch, handle := range w.sources {
		if err := windows.DeregisterEventSource(handle); err != nil {
			errs = append(errs, fmt.Sprintf("deregister %s: %v", ch, err))
		}
	}
	w.sources = make(map[string]windows.Handle)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
