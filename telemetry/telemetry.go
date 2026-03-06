// Package telemetry defines the telemetry types supported by Blitz
// generators and outputs.
package telemetry

import (
	"fmt"
	"slices"
)

// Type represents a telemetry signal type.
type Type string

const (
	// Logs represents log telemetry.
	Logs Type = "logs"
	// Metrics represents metric telemetry.
	Metrics Type = "metrics"
)

// Valid returns true if the telemetry type is a known type.
func (t Type) Valid() bool {
	switch t {
	case Logs, Metrics:
		return true
	default:
		return false
	}
}

// Supports returns true if the given type is present in the slice of supported types.
func Supports(supported []Type, t Type) bool {
	return slices.Contains(supported, t)
}

// Compatible returns the set of telemetry types that are supported by both
// the generator and the output. If there are no common types, an error is returned.
func Compatible(generatorTypes, outputTypes []Type) ([]Type, error) {
	var common []Type
	for _, gt := range generatorTypes {
		if Supports(outputTypes, gt) {
			common = append(common, gt)
		}
	}
	if len(common) == 0 {
		return nil, fmt.Errorf(
			"generator and output have no compatible telemetry types: generator supports %v, output supports %v",
			generatorTypes, outputTypes,
		)
	}
	return common, nil
}
