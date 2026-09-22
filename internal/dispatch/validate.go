package dispatch

import (
	"errors"
	"fmt"

	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/telemetry"
)

// OutputSignals identifies a configured output and the signals it accepts,
// for signal-compatibility validation. Name is the output type, used in
// error messages.
type OutputSignals struct {
	Name    string
	Signals []telemetry.Type
}

// ValidateSignalCompat checks that the configured generators and outputs are
// signal-compatible. Every signal a generator emits must have an accepting
// output, and every output must accept a signal some generator emits. Any
// generator signal or output with zero compatible counterpart is an orphan.
//
// It attempts every pairing and aggregates all orphans, both directions,
// into one error, so a run surfaces every misconfiguration at once rather
// than one per re-run. It returns nil only when nothing is orphaned.
func ValidateSignalCompat(genTypes []config.GeneratorType, outputs []OutputSignals) error {
	accepted := make(map[telemetry.Type]bool)
	for _, o := range outputs {
		for _, s := range o.Signals {
			accepted[s] = true
		}
	}
	emitted := make(map[telemetry.Type]bool)
	for _, t := range genTypes {
		for _, s := range SignalsFor(t) {
			emitted[s] = true
		}
	}

	// No signal emitted at all is the degenerate do-nothing config (the
	// packaged default is nop generator + nop output). Nothing to route, so
	// nothing to orphan.
	if len(emitted) == 0 {
		return nil
	}

	var errs []error

	// Orphan generator signals: emitted with no accepting output. Dedupe by
	// signal so two generators of the same signal report it once.
	reported := make(map[telemetry.Type]bool)
	for _, t := range genTypes {
		for _, s := range SignalsFor(t) {
			if !accepted[s] && !reported[s] {
				reported[s] = true
				errs = append(errs, fmt.Errorf("no output accepts %s emitted by a configured generator", s))
			}
		}
	}

	// Orphan outputs: accept no signal any generator emits.
	for _, o := range outputs {
		orphan := true
		for _, s := range o.Signals {
			if emitted[s] {
				orphan = false
				break
			}
		}
		if orphan {
			errs = append(errs, fmt.Errorf("output %q accepts no generated signal", o.Name))
		}
	}

	return errors.Join(errs...)
}
