package logtypes

import (
	"time"
)

// LogType constants for log type selection
const (
	LogTypeDefault = "default"
	LogTypePII     = "pii"
)

// LogData is the interface for all log type data structures
// This allows generators to work with different log types in a type-safe way
type LogData interface {
	// Timestamp returns the log timestamp
	Timestamp() time.Time
	// Level returns the log severity level
	Level() string
	// Message returns the log message
	Message() string
}

// DefaultLogData represents the default log type with standard fields
type DefaultLogData struct {
	TimestampVal   time.Time
	LevelVal       string
	EnvironmentVal string
	LocationVal    string
	MessageVal     string
}

// Timestamp implements LogData interface
func (d *DefaultLogData) Timestamp() time.Time {
	return d.TimestampVal
}

// Level implements LogData interface
func (d *DefaultLogData) Level() string {
	return d.LevelVal
}

// Message implements LogData interface
func (d *DefaultLogData) Message() string {
	return d.MessageVal
}

// PIILogData represents the PII log type with banking/PII fields
type PIILogData struct {
	TimestampVal time.Time
	LevelVal     string
	MessageVal   string
	EventVal     string
	DetailVal    string
	TypeVal      string
	ActionVal    string
	StatusVal    string
	UserIDVal    string
	SSNVal       string
	IBANVal      string
	PhoneVal     string
}

// Timestamp implements LogData interface
func (p *PIILogData) Timestamp() time.Time {
	return p.TimestampVal
}

// Level implements LogData interface
func (p *PIILogData) Level() string {
	return p.LevelVal
}

// Message implements LogData interface
func (p *PIILogData) Message() string {
	return p.MessageVal
}
