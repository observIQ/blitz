package generator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/internal/winevt/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewWinevtGenerator(t *testing.T) {
	logger := zaptest.NewLogger(t)
	g, err := NewWinevtGenerator(logger, 2, 50*time.Millisecond)
	assert.NoError(t, err)
	assert.NotNil(t, g)
}

func TestWinevtGenerator_GeneratesAndWrites(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	g, err := NewWinevtGenerator(logger, 2, 20*time.Millisecond)
	require.NoError(t, err)

	err = g.Start(writer)
	require.NoError(t, err)

	time.Sleep(120 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = g.Stop(ctx)
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0)

	// Verify the rendered XML includes an IP from our list in both places
	foundBoth := false
	for _, b := range writes {
		out := string(b)
		containsA := false
		containsB := false
		for _, ip := range templates.DefaultIPs {
			if strings.Contains(out, "Source Network Address:\t"+ip) {
				containsA = true
			}
			if strings.Contains(out, "<Data Name='IpAddress'>"+ip+"</Data>") {
				containsB = true
			}
		}
		if containsA && containsB {
			foundBoth = true
			break
		}
	}
	assert.True(t, foundBoth, "expected to find IP address in both message and EventData")
}
