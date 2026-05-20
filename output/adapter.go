package output

import (
	"context"

	"github.com/observiq/blitz/embed"
)

// WriterAsLogConsumer wraps a Writer so it can be used in contexts that
// expect an embed.LogConsumer. The adapter pushes each record in the
// batch through Writer.Write in order, returning the first error it
// encounters.
//
// CLI generator wiring uses this adapter to bridge migrated modules
// (which talk to embed.LogConsumer) with the existing Output instances
// (which implement Writer).
func WriterAsLogConsumer(w Writer) embed.LogConsumer {
	return &writerAsLogConsumer{w: w}
}

type writerAsLogConsumer struct {
	w Writer
}

func (a *writerAsLogConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	for i := range records {
		if err := a.w.Write(ctx, records[i]); err != nil {
			return err
		}
	}
	return nil
}
