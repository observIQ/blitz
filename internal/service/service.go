package service

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/output"
	"go.uber.org/zap"
)

// Service manages generators and an output.
type Service struct {
	Logger     *zap.Logger
	Generators []any
	Output     output.Output
}

// New creates a new service with multiple generators and a single output.
func New(logger *zap.Logger, generators []any, output output.Output) (*Service, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if len(generators) == 0 {
		return nil, fmt.Errorf("generators cannot be empty")
	}
	if output == nil {
		return nil, fmt.Errorf("output cannot be nil")
	}

	return &Service{
		Logger:     logger,
		Generators: generators,
		Output:     output,
	}, nil
}

// Start starts all generators, dispatching each to the appropriate writer
// based on type assertions.
func (s *Service) Start() error {
	for i, gen := range s.Generators {
		// PIPE-975 migration: embed.ProducerModule modules own their
		// consumer wiring internally (configured at construction).
		// Service just calls Start(ctx). This case precedes the legacy
		// generator.* cases because migrated modules satisfy both the
		// new and the old marker; legacy generators do not yet satisfy
		// embed.ProducerModule.
		//
		// Concrete-telemetry legacy cases (MetricGenerator, TraceGenerator)
		// precede the base Generator case so that a metric/trace generator
		// can never be mis-dispatched to the log path if those interfaces
		// ever come to share methods with Generator.
		switch g := gen.(type) {
		case embed.ProducerModule:
			if err := g.Start(context.Background()); err != nil {
				return fmt.Errorf("start producer module %d: %w", i, err)
			}
		case generator.MetricGenerator:
			mw, ok := s.Output.(output.MetricWriter)
			if !ok {
				s.Logger.Warn("Output does not support MetricWriter, skipping metric generator",
					zap.Int("generator_index", i))
				continue
			}
			if err := g.Start(mw); err != nil {
				return fmt.Errorf("start metric generator %d: %w", i, err)
			}
		case generator.TraceGenerator:
			tw, ok := s.Output.(output.TraceWriter)
			if !ok {
				s.Logger.Warn("Output does not support TraceWriter, skipping trace generator",
					zap.Int("generator_index", i))
				continue
			}
			if err := g.Start(tw); err != nil {
				return fmt.Errorf("start trace generator %d: %w", i, err)
			}
		case generator.Generator:
			if err := g.Start(s.Output); err != nil {
				return fmt.Errorf("start log generator %d: %w", i, err)
			}
		default:
			return fmt.Errorf("generator %d has unsupported type %T", i, gen)
		}
	}
	return nil
}

// Stop stops all generators and the output. Stop will block for up to 30 seconds.
func (s *Service) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i, gen := range s.Generators {
		switch g := gen.(type) {
		case embed.ProducerModule:
			if err := g.Stop(ctx); err != nil {
				return fmt.Errorf("stop producer module %d: %w", i, err)
			}
		case generator.MetricGenerator:
			if err := g.Stop(ctx); err != nil {
				return fmt.Errorf("stop metric generator %d: %w", i, err)
			}
		case generator.TraceGenerator:
			if err := g.Stop(ctx); err != nil {
				return fmt.Errorf("stop trace generator %d: %w", i, err)
			}
		case generator.Generator:
			if err := g.Stop(ctx); err != nil {
				return fmt.Errorf("stop log generator %d: %w", i, err)
			}
		}
	}

	if err := s.Output.Stop(ctx); err != nil {
		return fmt.Errorf("stop output: %w", err)
	}

	return nil
}
