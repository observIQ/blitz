package embed

import "context"

// Module is the lifecycle interface every blitz module implements.
//
// A module is further classified at compile time as either a
// ProducerModule (yields records a host can consume) or an EffectorModule
// (causes effects outside blitz's process). The split is a property of
// the module's implementation, not a runtime flag.
type Module interface {
	// Name returns the module's identifier — used in logs, metric labels,
	// and error messages. Must be stable across the module's lifetime.
	Name() string

	// Start begins module execution. Returning a nil error means the
	// module is running; resources must be released by Stop.
	Start(ctx context.Context) error

	// Stop terminates module execution. Implementations honor ctx.Done()
	// to bound shutdown time.
	Stop(ctx context.Context) error
}

// ProducerModule is a Module that yields telemetry records to a host's
// consumer interfaces. ProducerModules are embed-eligible: a host can
// register them with embed.New and receive records in-process.
//
// The isProducer marker is unexported and only satisfiable by embedding
// ProducerMarker, which makes ProducerModule a closed set the compiler
// can enforce.
type ProducerModule interface {
	Module
	isProducer()
}

// EffectorModule is a Module whose effects land outside blitz's process
// (operating-system event log, listening sockets, files on disk). The
// host cannot observe these effects in-process, so EffectorModules are
// NOT embed-eligible — embed.New rejects them at compile time via the
// typed Config.Modules []ProducerModule signature.
type EffectorModule interface {
	Module
	isEffector()
}

// ProducerMarker is the embeddable implementation of the ProducerModule
// marker. A module declares itself a Producer by embedding this type:
//
//	type Generator struct {
//	    embed.ProducerMarker
//	    // other fields
//	}
//
// The embedded marker contributes a no-op isProducer method that lets
// the type satisfy ProducerModule.
type ProducerMarker struct{}

func (ProducerMarker) isProducer() {}

// EffectorMarker is the embeddable implementation of the EffectorModule
// marker. A module declares itself an Effector by embedding this type:
//
//	type Generator struct {
//	    embed.EffectorMarker
//	    // other fields
//	}
//
// The embedded marker contributes a no-op isEffector method that lets
// the type satisfy EffectorModule.
type EffectorMarker struct{}

func (EffectorMarker) isEffector() {}
