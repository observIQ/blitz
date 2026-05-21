package embed

import "context"

// Runner is the embedded blitz lifecycle handle returned by New.
//
// Implementations are constructed via New and are not instantiated by
// hosts directly. Start dispatches every configured ProducerModule's
// output through the Host's consumers; Stop terminates module execution
// and flushes pending records to the bound consumers.
type Runner interface {
	// Start begins running every configured module. Returns once all
	// modules have signaled successful startup, or on the first start
	// error.
	Start(ctx context.Context, host Host) error

	// Stop terminates module execution and flushes pending records.
	// Stop honors ctx.Done() to bound shutdown time.
	Stop(ctx context.Context) error
}

// BackpressureMode selects how a Runner handles consumer slowness on
// each signal channel.
type BackpressureMode int

const (
	// BackpressureBlock blocks the producing module until the consumer
	// accepts the batch. Default.
	BackpressureBlock BackpressureMode = iota

	// BackpressureDrop drops the batch when the consumer is not ready.
	// Dropped batches are counted in a records_dropped metric.
	BackpressureDrop

	// BackpressureBuffer queues batches in memory up to BufferSize. When
	// the buffer is full, behavior reverts to Block.
	BackpressureBuffer
)

// ConsumerBackpressure configures backpressure for one signal channel.
type ConsumerBackpressure struct {
	Mode       BackpressureMode
	BufferSize int
}

// Config configures a Runner returned by New.
//
// Modules is typed []ProducerModule so that the compiler rejects any
// EffectorModule at the call site — embed.New cannot accept modules
// whose effects land outside blitz's process.
type Config struct {
	// Modules lists the ProducerModules the Runner will operate.
	Modules []ProducerModule

	// Logs, Metrics, Traces configure backpressure per signal channel.
	// Zero values mean BackpressureBlock.
	Logs    ConsumerBackpressure
	Metrics ConsumerBackpressure
	Traces  ConsumerBackpressure
}
