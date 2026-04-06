// Package telemetry defines the supported telemetry signal types for blitz.
package telemetry

import "fmt"

// Type represents a telemetry signal type.
type Type string

const (
	// Logs represents log telemetry.
	Logs Type = "logs"
	// Metrics represents metric telemetry.
	Metrics Type = "metrics"
	// Traces represents trace telemetry.
	Traces Type = "traces"
)

// Valid returns true if the type is a known telemetry type.
func (t Type) Valid() bool {
	switch t {
	case Logs, Metrics, Traces:
		return true
	default:
		return false
	}
}

// Supports returns true if the given type is present in the supported slice.
func Supports(supported []Type, target Type) bool {
	for _, s := range supported {
		if s == target {
			return true
		}
	}
	return false
}

// Validate checks that the generator and output telemetry types have at least
// one overlapping type. Returns an error if there is no overlap.
func Validate(generator, output []Type) error {
	for _, g := range generator {
		for _, o := range output {
			if g == o {
				return nil
			}
		}
	}
	return fmt.Errorf("no compatible telemetry types: generator supports %v, output supports %v", generator, output)
}
