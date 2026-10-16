package generator

import "github.com/observiq/blitz/internal/datagen"

// Option configures a log generator at construction.
type Option func(*Options)

// Options holds the construction-time settings shared by the log generators.
type Options struct {
	// Identity is the simulated host every emitted record describes. Nil
	// falls back to the process hostname.
	Identity *datagen.SystemIdentity
	// Appliance is the simulated network device an appliance generator (Palo
	// Alto) describes. Nil falls back to the generator's own device name.
	Appliance *datagen.NetworkSystemIdentity
}

// WithIdentity sets the simulated host identity. A generator resolves its one
// hostname from it at construction and uses that hostname everywhere: in the
// resource and in any hostname it writes inside the message.
func WithIdentity(id *datagen.SystemIdentity) Option {
	return func(o *Options) { o.Identity = id }
}

// WithAppliance sets the simulated network appliance. Its hostname is the one
// name the generator uses, in the resource and inside the message.
func WithAppliance(n *datagen.NetworkSystemIdentity) Option {
	return func(o *Options) { o.Appliance = n }
}

// NewOptions applies opts over the zero Options.
func NewOptions(opts ...Option) Options {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
