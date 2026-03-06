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

// Compatible returns the single telemetry type that is supported by both
// the generator and the output. An error is returned if there are no common
// types or if more than one type overlaps, since each pipeline must operate
// on exactly one telemetry type.
func Compatible(generatorTypes, outputTypes []Type) (Type, error) {
	var common []Type
	for _, gt := range generatorTypes {
		if Supports(outputTypes, gt) {
			common = append(common, gt)
		}
	}
	switch len(common) {
	case 0:
		return "", fmt.Errorf(
			"generator and output have no compatible telemetry types: generator supports %v, output supports %v",
			generatorTypes, outputTypes,
		)
	case 1:
		return common[0], nil
	default:
		return "", fmt.Errorf(
			"generator and output have multiple compatible telemetry types %v: a single type must be configured",
			common,
		)
	}
}
