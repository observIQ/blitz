package filegen

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockWriter implements output.Writer for testing
type mockWriter struct {
	mu       sync.Mutex
	writes   []output.LogRecord
	writeErr error
	delay    time.Duration
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writes: make([]output.LogRecord, 0),
	}
}

func (m *mockWriter) Write(ctx context.Context, record output.LogRecord) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.writeErr != nil {
		return m.writeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, record)
	return nil
}

func (m *mockWriter) Close(ctx context.Context) error {
	return nil
}

func (m *mockWriter) getWrites() []output.LogRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	writes := make([]output.LogRecord, len(m.writes))
	copy(writes, m.writes)
	return writes
}

func TestNewFileLogGenerator(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name    string
		workers int
		rate    time.Duration
		mode    Mode
		source  string
		pattern string
		wantErr bool
	}{
		{
			name:    "valid config",
			workers: 2,
			rate:    100 * time.Millisecond,
			mode:    ModeFile,
			source:  "/tmp/test.log",
			wantErr: false,
		},
		{
			name:    "invalid workers",
			workers: 0,
			rate:    100 * time.Millisecond,
			mode:    ModeFile,
			source:  "/tmp/test.log",
			wantErr: true,
		},
		{
			name:    "invalid rate",
			workers: 1,
			rate:    0,
			mode:    ModeFile,
			source:  "/tmp/test.log",
			wantErr: true,
		},
		{
			name:    "invalid mode",
			workers: 1,
			rate:    100 * time.Millisecond,
			mode:    Mode("invalid"),
			source:  "/tmp/test.log",
			wantErr: true,
		},
		{
			name:    "empty source",
			workers: 1,
			rate:    100 * time.Millisecond,
			mode:    ModeFile,
			source:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := New(logger, tt.workers, tt.rate, tt.mode, tt.source, tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, gen)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gen)
			}
		})
	}
}

func TestFileLogGeneratorFileMode(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a temporary file with test data
	tmpfile, err := ioutil.TempFile("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	testData := []string{
		"Thu Jan 13 15:30:45 2026 test line 1\n",
		"Fri Jan 14 10:20:30 2026 test line 2\n",
		"Mon Jan 17 12:45:00 2026 test line 3\n",
	}

	for _, line := range testData {
		_, err := tmpfile.WriteString(line)
		require.NoError(t, err)
	}
	tmpfile.Close()

	gen, err := New(logger, 1, 10*time.Millisecond, ModeFile, tmpfile.Name(), "")
	require.NoError(t, err)

	writer := newMockWriter()

	err = gen.Start(writer)
	require.NoError(t, err)

	// Let it run for a short time
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0)
}

func TestFileLogGeneratorDirectoryMode(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a temporary directory with test files
	tmpdir, err := ioutil.TempDir("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Create test files
	files := []struct {
		name string
		data string
	}{
		{"file1.log", "Thu Jan 13 15:30:45 2026 line from file1\n"},
		{"file2.log", "Fri Jan 14 10:20:30 2026 line from file2\n"},
		{"other.txt", "Mon Jan 17 12:45:00 2026 line from other\n"},
	}

	for _, f := range files {
		path := filepath.Join(tmpdir, f.name)
		err := ioutil.WriteFile(path, []byte(f.data), 0644)
		require.NoError(t, err)
	}

	// Test with pattern
	gen, err := New(logger, 1, 10*time.Millisecond, ModeDirectory, tmpdir, "*.log")
	require.NoError(t, err)

	writer := newMockWriter()

	err = gen.Start(writer)
	require.NoError(t, err)

	// Let it run for a short time
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0)
}

func TestFileLogGeneratorNonexistentFile(t *testing.T) {
	logger := zaptest.NewLogger(t)

	gen, err := New(logger, 1, 100*time.Millisecond, ModeFile, "/nonexistent/path/file.log", "")
	require.NoError(t, err)

	writer := newMockWriter()

	err = gen.Start(writer)
	assert.Error(t, err)
}

func TestFileLogGeneratorStop(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := ioutil.TempFile("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// Write test data
	_, err = tmpfile.WriteString("Thu Jan 13 15:30:45 2026 test line\n")
	require.NoError(t, err)
	tmpfile.Close()

	gen, err := New(logger, 1, 100*time.Millisecond, ModeFile, tmpfile.Name(), "")
	require.NoError(t, err)

	writer := newMockWriter()

	err = gen.Start(writer)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = gen.Stop(ctx)
	require.NoError(t, err)
}

func TestFileLogGeneratorWriteError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := ioutil.TempFile("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("Thu Jan 13 15:30:45 2026 test line\n")
	require.NoError(t, err)
	tmpfile.Close()

	gen, err := New(logger, 1, 10*time.Millisecond, ModeFile, tmpfile.Name(), "")
	require.NoError(t, err)

	writer := newMockWriter()
	writer.writeErr = errors.New("write failed")

	err = gen.Start(writer)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)
}

func TestFileLogGeneratorMultipleWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := ioutil.TempFile("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	for i := 0; i < 10; i++ {
		_, err := tmpfile.WriteString("Thu Jan 13 15:30:45 2026 test line " + string(rune(i)) + "\n")
		require.NoError(t, err)
	}
	tmpfile.Close()

	gen, err := New(logger, 3, 10*time.Millisecond, ModeFile, tmpfile.Name(), "")
	require.NoError(t, err)

	writer := newMockWriter()

	err = gen.Start(writer)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0)
}

// TestTimestampProcessing tests that timestamp directives are properly replaced
func TestTimestampProcessing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Create a temporary file for the generator to use
	tmpFile, err := ioutil.TempFile(t.TempDir(), "test*.log")
	require.NoError(t, err)
	tmpFile.Close()

	gen, err := New(logger, 1, 100*time.Millisecond, ModeFile, tmpFile.Name(), "")
	require.NoError(t, err)

	testCases := []struct {
		name        string
		input       string
		contains    []string // substrings that should appear in output
		notContains []string // substrings that should NOT appear
	}{
		{
			name:        "ISO 8601 UTC directive",
			input:       `<85>1 %Y-%m-%dT%H:%M:%SZ loki.example.com su - ID47 - test`,
			contains:    []string{"<85>1", "loki.example.com", "Z", "20"},
			notContains: []string{"%Y-%m-%dT%H:%M:%SZ"},
		},
		{
			name:        "ctime directive",
			input:       `<134>%c hostname service: alert event`,
			contains:    []string{"<134>", "hostname", "service"},
			notContains: []string{"%c"},
		},
		{
			name:        "BSD format directive",
			input:       `<180>%b %d %H:%M:%S paloalto.firewall threat`,
			contains:    []string{"<180>", "paloalto.firewall", "threat"},
			notContains: []string{"%b", "%d", "%H:%M:%S"},
		},
		{
			name:        "ISO date directive",
			input:       `timestamp=%Y-%m-%d event=test`,
			contains:    []string{"timestamp=", "20", "event=test"},
			notContains: []string{"%Y-%m-%d"},
		},
		{
			name:        "Multiple directives in one line",
			input:       `<134>%c hostname: %Y-%m-%d %H:%M:%S event`,
			contains:    []string{"<134>", "hostname", "20", "event"},
			notContains: []string{"%c", "%Y-%m-%d", "%H:%M:%S"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := gen.processTimestamps(tc.input)

			// Check that directives are replaced
			for _, shouldContain := range tc.contains {
				assert.Contains(t, output, shouldContain, "output should contain %q", shouldContain)
			}

			// Check that directives themselves are not present
			for _, shouldNotContain := range tc.notContains {
				assert.NotContains(t, output, shouldNotContain, "output should not contain %q", shouldNotContain)
			}
		})
	}
}
