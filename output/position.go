package output

import (
	"slices"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// WithOutputPosition returns a MeterProvider whose meters carry position (a
// fan-out list position such as "outputs[1]") as the "output" instrumentation
// scope attribute, so two outputs of one type report distinguishable
// blitz.output.* metrics. A nil mp means the global provider.
func WithOutputPosition(mp metric.MeterProvider, position string) metric.MeterProvider {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}
	return positionedProvider{MeterProvider: mp, pos: attribute.String("output", position)}
}

type positionedProvider struct {
	metric.MeterProvider
	pos attribute.KeyValue
}

func (p positionedProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return p.MeterProvider.Meter(name, append(slices.Clip(opts), metric.WithInstrumentationAttributes(p.pos))...)
}
