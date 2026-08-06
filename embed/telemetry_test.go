package embed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNopTelemetry(t *testing.T) {
	tel := NopTelemetry()

	// Every provider and the logger must be non-nil so callers can use the
	// bundle without nil checks.
	require.NotNil(t, tel.Logger)
	require.NotNil(t, tel.MeterProvider)
	require.NotNil(t, tel.TracerProvider)

	// Per-batch spans default off.
	require.False(t, tel.PerBatchSpans)
}

func TestTelemetrySettings_zeroValueFieldsAreNil(t *testing.T) {
	// A zero-value bundle leaves providers nil; components and NewMetrics are
	// responsible for the nil->global fallback. This documents that contract.
	var tel TelemetrySettings

	require.Nil(t, tel.Logger)
	require.Nil(t, tel.MeterProvider)
	require.Nil(t, tel.TracerProvider)
}
