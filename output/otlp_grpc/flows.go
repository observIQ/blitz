package otlpgrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/embed/otelpdata"
	"github.com/observiq/blitz/output"
)

// ConsumeFlows sends flow records to the OTLP gRPC output as OTLP logs, using
// the same network-flow semantic conventions the collector's netflowreceiver
// emits (via otelpdata.FlowToLogRecord). This makes the OTLP path a first-class
// destination for the flow signal, alongside logs/metrics/traces.
func (o *OTLPGrpc) ConsumeFlows(ctx context.Context, records []embed.FlowRecord) error {
	for i := range records {
		rec := otelpdata.FlowToLogRecord(records[i])
		rec.ObservedTimeUnixNano = output.TimeToUnixNanoUint64(time.Now())
		select {
		case o.dataChan <- rec:
			o.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "flows")
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting to write flow: %w", ctx.Err())
		case <-o.ctx.Done():
			return fmt.Errorf("OTLP gRPC output is shutting down")
		}
	}
	return nil
}
