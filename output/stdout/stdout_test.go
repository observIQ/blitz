package stdout

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)
	stdout, err := New(logger)

	assert.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.Equal(t, logger.Named("output-stdout"), stdout.logger)
}

func TestNew_NilLogger(t *testing.T) {
	stdout, err := New(nil)

	assert.Error(t, err)
	assert.Nil(t, stdout)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestStdoutOutput_Write(t *testing.T) {
	logger := zaptest.NewLogger(t)
	stdout, err := New(logger)
	require.NoError(t, err)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// Write a log record
	logRecord := output.LogRecord{
		Message: "test log message",
	}

	err = stdout.Write(context.Background(), logRecord)
	require.NoError(t, err)

	// Close write end and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "test log message")
}

func TestStdoutOutput_Stop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	stdout, err := New(logger)
	require.NoError(t, err)

	ctx := context.Background()
	err = stdout.Stop(ctx)
	assert.NoError(t, err)
}
