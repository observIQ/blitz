package service

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/output"
	"go.uber.org/zap"
)

type Service struct {
	Logger    *zap.Logger
	Generator generator.Generator
	Output    output.Output
}

func New(logger *zap.Logger, generator generator.Generator, output output.Output) (*Service, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if generator == nil {
		return nil, fmt.Errorf("generator cannot be nil")
	}
	if output == nil {
		return nil, fmt.Errorf("output cannot be nil")
	}

	return &Service{
		Logger:    logger,
		Generator: generator,
		Output:    output,
	}, nil
}

// Start starts the service.
func (s *Service) Start() error {
	return s.Generator.Start(s.Output)
}

// Stop stops the service. Stop will block for up to 30 seconds.
// If the generator or output do not stop within the timeout, an
// error will be returned and the program can exit.
func (s *Service) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Generator.Stop(ctx); err != nil {
		return fmt.Errorf("stop generator: %w", err)
	}

	if err := s.Output.Stop(ctx); err != nil {
		return fmt.Errorf("stop output: %w", err)
	}

	return nil
}
