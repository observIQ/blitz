package winevt

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/internal/generators/winevt/templates"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockWriter implements output.Writer for testing
type mockWriter struct {
	mu     sync.Mutex
	writes [][]byte
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writes: make([][]byte, 0),
	}
}

func (m *mockWriter) Write(ctx context.Context, data output.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, append([]byte(nil), data.Message...))
	return nil
}

func (m *mockWriter) WriteMetric(_ context.Context, _ output.MetricRecord) error {
	return output.ErrUnsupportedTelemetryType
}

func (m *mockWriter) getWrites() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte(nil), m.writes...)
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)
	g, err := New(logger, 2, 50*time.Millisecond)
	assert.NoError(t, err)
	assert.NotNil(t, g)
}

func TestWinevtGenerator_GeneratesAndWrites(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	g, err := New(logger, 2, 20*time.Millisecond)
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

func TestRenderTemplate_RandomSelection(t *testing.T) {
	// Test that random template selection actually works by generating many templates
	// and verifying we get all template types
	exampleCount := 0
	serviceControlManagerCount := 0
	successfulLogonCount := 0

	// Generate 150 templates with random selection
	for range 150 {
		result, err := templates.RenderTemplate(templates.RenderOptions{})
		require.NoError(t, err)

		// Check which template was used by looking for unique identifiers
		if strings.Contains(result, "EventID>4625</EventID") {
			exampleCount++
		} else if strings.Contains(result, "Service Control Manager") {
			serviceControlManagerCount++
		} else if strings.Contains(result, "EventID>4624</EventID") {
			successfulLogonCount++
			// Verify hostname templating worked
			assert.Contains(t, result, "$", "expected hostname with trailing $ in SubjectUserName")
			assert.Contains(t, result, "Account Name:\t\t", "expected account name field")
		}
	}

	// We should have gotten all templates (with high probability)
	// If random selection wasn't working, we'd only get one type
	assert.Greater(t, exampleCount, 0, "expected to see example template at least once")
	assert.Greater(t, serviceControlManagerCount, 0, "expected to see service control manager template at least once")
	assert.Greater(t, successfulLogonCount, 0, "expected to see successful logon template at least once")

	// Log the distribution for debugging
	t.Logf("Template distribution: example=%d, service_control_manager=%d, successful_logon=%d", exampleCount, serviceControlManagerCount, successfulLogonCount)
}

func TestRenderTemplate_SuccessfulLogonHostname(t *testing.T) {
	// Test that the successful logon template properly templates hostnames
	result, err := templates.RenderTemplate(templates.RenderOptions{
		TemplateName: templates.SuccessfulLogonTemplateName,
		Hostnames:    []string{"test-host-123"},
	})
	require.NoError(t, err)

	// Verify hostname appears in uppercase with $ in SubjectUserName
	assert.Contains(t, result, "<Data Name='SubjectUserName'>TEST-HOST-123$</Data>")
	// Verify hostname appears in lowercase in Computer field
	assert.Contains(t, result, "<Computer>test-host-123</Computer>")
	// Verify hostname appears in message
	assert.Contains(t, result, "Account Name:\t\tTEST-HOST-123$")
}
