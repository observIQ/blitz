package output

import "context"

// Output is the interface for outputting data.
type Output interface {
	// Write writes the data to the output.
	Write(ctx context.Context, data []byte) error

	// Stop stops the output.
	Stop(ctx context.Context) error
}
