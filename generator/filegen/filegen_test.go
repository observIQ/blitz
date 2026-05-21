package filegen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockConsumer implements embed.LogConsumer for testing.
type mockConsumer struct {
	mu         sync.Mutex
	records    []embed.LogRecord
	consumeErr error
	delay      time.Duration
}

func newMockConsumer() *mockConsumer {
	return &mockConsumer{
		records: make([]embed.LogRecord, 0),
	}
}

func (m *mockConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.consumeErr != nil {
		return m.consumeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range records {
		m.records = append(m.records, records[i])
	}
	return nil
}

func (m *mockConsumer) getWrites() []embed.LogRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]embed.LogRecord, len(m.records))
	copy(out, m.records)
	return out
}

func (m *mockConsumer) setConsumeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consumeErr = err
}

// Compile-time assertion: the migrated generator satisfies embed.ProducerModule.
var _ embed.ProducerModule = (*FileLogGenerator)(nil)

func TestFileLogGenerator_Name(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := os.CreateTemp("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	gen, err := New(logger, 1, 100*time.Millisecond, tmpfile.Name(), true, 0, newMockConsumer())
	require.NoError(t, err)
	assert.Equal(t, componentName, gen.Name())
}

func TestFileLogGenerator_NilConsumer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 100*time.Millisecond, "/tmp/test.log", true, 0, nil)
	assert.Error(t, err)
	assert.Nil(t, gen)
	assert.Contains(t, err.Error(), "consumer cannot be nil")
}

func TestNewFileLogGenerator(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name    string
		workers int
		rate    time.Duration
		source  string
		wantErr bool
	}{
		{
			name:    "valid config with file",
			workers: 2,
			rate:    100 * time.Millisecond,
			source:  "/tmp/test.log",
			wantErr: false,
		},
		{
			name:    "invalid workers",
			workers: 0,
			rate:    100 * time.Millisecond,
			source:  "/tmp/test.log",
			wantErr: true,
		},
		{
			name:    "invalid rate",
			workers: 1,
			rate:    0,
			source:  "/tmp/test.log",
			wantErr: true,
		},
		{
			name:    "empty source",
			workers: 1,
			rate:    100 * time.Millisecond,
			source:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := New(logger, tt.workers, tt.rate, tt.source, true, 0, newMockConsumer())
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
	tmpfile, err := os.CreateTemp("", "test*.log")
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

	consumer := newMockConsumer()
	gen, err := New(logger, 1, 10*time.Millisecond, tmpfile.Name(), true, 0, consumer)
	require.NoError(t, err)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	// Let it run for a short time
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)

	writes := consumer.getWrites()
	assert.Greater(t, len(writes), 0)
}

func TestFileLogGeneratorDirectoryMode(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a temporary directory with test files
	tmpdir, err := os.MkdirTemp("", "testdir")
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
		err := os.WriteFile(path, []byte(f.data), 0644)
		require.NoError(t, err)
	}

	// Test with directory mode (auto-detected)
	consumer := newMockConsumer()
	gen, err := New(logger, 1, 10*time.Millisecond, tmpdir, true, 0, consumer)
	require.NoError(t, err)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	// Let it run for a short time
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)

	writes := consumer.getWrites()
	assert.Greater(t, len(writes), 0)
}

func TestFileLogGeneratorNonexistentFile(t *testing.T) {
	logger := zaptest.NewLogger(t)

	gen, err := New(logger, 1, 100*time.Millisecond, "/nonexistent/path/file.log", true, 0, newMockConsumer())
	require.NoError(t, err)

	err = gen.Start(context.Background())
	assert.Error(t, err)
}

func TestFileLogGeneratorStop(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := os.CreateTemp("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// Write test data
	_, err = tmpfile.WriteString("Thu Jan 13 15:30:45 2026 test line\n")
	require.NoError(t, err)
	tmpfile.Close()

	gen, err := New(logger, 1, 100*time.Millisecond, tmpfile.Name(), true, 0, newMockConsumer())
	require.NoError(t, err)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = gen.Stop(ctx)
	require.NoError(t, err)
}

func TestFileLogGeneratorWriteError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := os.CreateTemp("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("Thu Jan 13 15:30:45 2026 test line\n")
	require.NoError(t, err)
	tmpfile.Close()

	consumer := newMockConsumer()
	consumer.setConsumeError(errors.New("write failed"))
	gen, err := New(logger, 1, 10*time.Millisecond, tmpfile.Name(), true, 0, consumer)
	require.NoError(t, err)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)
}

func TestFileLogGeneratorMultipleWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := os.CreateTemp("", "test*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	for i := range 10 {
		_, err := tmpfile.WriteString("Thu Jan 13 15:30:45 2026 test line " + string(rune(i)) + "\n")
		require.NoError(t, err)
	}
	tmpfile.Close()

	consumer := newMockConsumer()
	gen, err := New(logger, 3, 10*time.Millisecond, tmpfile.Name(), true, 0, consumer)
	require.NoError(t, err)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = gen.Stop(ctx)
	cancel()
	require.NoError(t, err)

	writes := consumer.getWrites()
	assert.Greater(t, len(writes), 0)
}

// TestTimestampProcessing tests that timestamp directives are properly replaced
func TestTimestampProcessing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Create a temporary file for the generator to use
	tmpFile, err := os.CreateTemp(t.TempDir(), "test*.log")
	require.NoError(t, err)
	tmpFile.Close()

	gen, err := New(logger, 1, 100*time.Millisecond, tmpFile.Name(), true, 0, newMockConsumer())
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

// TestFileLogGeneratorGlobPatterns tests glob pattern support in source paths
func TestFileLogGeneratorGlobPatterns(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp directory with multiple test files
	tmpdir := t.TempDir()

	// Create test files matching different patterns
	testFiles := []string{
		"test_rfc5424_1.log",
		"test_rfc5424_2.log",
		"test_rfc3164.log",
		"other_file.log",
	}

	for _, fname := range testFiles {
		f, err := os.Create(filepath.Join(tmpdir, fname))
		require.NoError(t, err)
		_, err = f.WriteString("test log line\n")
		require.NoError(t, err)
		f.Close()
	}

	testCases := []struct {
		name        string
		pattern     string
		expectedMin int
	}{
		{
			name:        "single_glob_wildcard",
			pattern:     filepath.Join(tmpdir, "test_*.log"),
			expectedMin: 3, // Should match test_rfc5424_1.log, test_rfc5424_2.log, test_rfc3164.log
		},
		{
			name:        "specific_glob_pattern",
			pattern:     filepath.Join(tmpdir, "*rfc5424*.log"),
			expectedMin: 2, // Should match test_rfc5424_1.log, test_rfc5424_2.log
		},
		{
			name:        "all_files_wildcard",
			pattern:     filepath.Join(tmpdir, "*.log"),
			expectedMin: 4, // Should match all files
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			consumer := newMockConsumer()
			gen, err := New(logger, 1, 10*time.Millisecond, tc.pattern, true, 0, consumer)
			require.NoError(t, err)

			err = gen.Start(context.Background())
			require.NoError(t, err)

			// Give enough time for at least one write
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			// Wait a bit then stop
			time.Sleep(50 * time.Millisecond)
			err = gen.Stop(ctx)
			require.NoError(t, err)

			// Should have written at least one line
			writes := consumer.getWrites()
			require.Greater(t, len(writes), 0, "should have written at least one line")
		})
	}
}

// TestFileLogGeneratorGlobDirectories tests glob patterns with directory wildcards
func TestFileLogGeneratorGlobDirectories(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp directory with subdirectories
	tmpdir := t.TempDir()

	// Create subdirectories matching pattern
	dirs := []string{
		filepath.Join(tmpdir, "syslog_generic"),
		filepath.Join(tmpdir, "syslog_custom"),
		filepath.Join(tmpdir, "other_dir"),
	}

	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		// Create files in each directory
		for i := 1; i <= 2; i++ {
			fname := filepath.Join(dir, fmt.Sprintf("test_%d.log", i))
			f, err := os.Create(fname)
			require.NoError(t, err)
			_, err = f.WriteString("test log line\n")
			require.NoError(t, err)
			f.Close()
		}
	}

	testCases := []struct {
		name        string
		pattern     string
		expectedMin int
	}{
		{
			name:        "syslog_directory_glob",
			pattern:     filepath.Join(tmpdir, "syslog_*/*.log"),
			expectedMin: 4, // Should match 2 files each from syslog_generic and syslog_custom
		},
		{
			name:        "all_directories_glob",
			pattern:     filepath.Join(tmpdir, "*_*/*.log"),
			expectedMin: 6, // Should match all 2 files from each of the 3 directories
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			consumer := newMockConsumer()
			gen, err := New(logger, 1, 10*time.Millisecond, tc.pattern, true, 0, consumer)
			require.NoError(t, err)

			err = gen.Start(context.Background())
			require.NoError(t, err)

			// Wait for at least one write to occur (rate is 10ms per line)
			time.Sleep(100 * time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = gen.Stop(ctx)
			require.NoError(t, err)

			// Should have written at least one line
			writes := consumer.getWrites()
			require.Greater(t, len(writes), 0, "should have written at least one line")
		})
	}
}

// TestCacheGetSet tests the Cache Get/Set methods
func TestCacheGetSet(t *testing.T) {
	// Test with cache enabled and no TTL
	cache, err := NewCache(true, 0, 10)
	require.NoError(t, err)

	lines := []string{"line1", "line2", "line3"}

	// Test Set
	cache.Set("key1", lines)

	// Test Get - should find the entry
	retrievedLines, found := cache.Get("key1")
	require.True(t, found, "entry should be found in cache")
	require.Equal(t, lines, retrievedLines)

	// Test Get - non-existent key
	_, found = cache.Get("nonexistent")
	require.False(t, found, "non-existent key should not be found")
}

// TestCacheDisabled tests that cache operations are no-ops when disabled
func TestCacheDisabled(t *testing.T) {
	cache, err := NewCache(false, 0, 10)
	require.NoError(t, err)

	lines := []string{"line1", "line2"}

	// Set should be a no-op
	cache.Set("key1", lines)

	// Get should always return false
	_, found := cache.Get("key1")
	require.False(t, found, "Get should return false when cache is disabled")
}

// TestCacheTTLExpiration tests that entries expire after the TTL
func TestCacheTTLExpiration(t *testing.T) {
	// Create cache with 10ms TTL
	cache, err := NewCache(true, 10*time.Millisecond, 10)
	require.NoError(t, err)

	lines := []string{"line1", "line2"}

	cache.Set("key1", lines)

	// Should be found immediately
	_, found := cache.Get("key1")
	require.True(t, found, "entry should be found before TTL expires")

	// Wait for TTL to expire
	time.Sleep(15 * time.Millisecond)

	// Should not be found after TTL
	_, found = cache.Get("key1")
	require.False(t, found, "entry should not be found after TTL expires")
}

// TestCacheLRUEviction tests that cache respects max size limit
func TestCacheLRUEviction(t *testing.T) {
	// Create cache with max size of 2
	cache, err := NewCache(true, 0, 2)
	require.NoError(t, err)

	lines1 := []string{"1"}
	lines2 := []string{"2"}
	lines3 := []string{"3"}

	cache.Set("key1", lines1)
	cache.Set("key2", lines2)
	cache.Set("key3", lines3) // This should evict key1 (LRU)

	// key1 should be evicted
	_, found := cache.Get("key1")
	require.False(t, found, "key1 should be evicted due to LRU")

	// key2 and key3 should still be present
	_, found = cache.Get("key2")
	require.True(t, found, "key2 should still be in cache")

	_, found = cache.Get("key3")
	require.True(t, found, "key3 should still be in cache")
}

func TestFileLogGenerator_SetCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := os.CreateTemp("", "filegen-tracker-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	for i := 0; i < 10; i++ {
		_, err := fmt.Fprintf(tmpfile, "test log line %d\n", i)
		require.NoError(t, err)
	}
	tmpfile.Close()

	gen, err := New(logger, 1, 50*time.Millisecond, tmpfile.Name(), true, 0, newMockConsumer())
	require.NoError(t, err)

	assert.Nil(t, gen.tracker, "tracker should be nil initially")

	tracker := count.NewTracker(10)
	gen.SetCountTracker(tracker)
	assert.Equal(t, tracker, gen.tracker)
}

func TestFileLogGenerator_CountLimited(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpfile, err := os.CreateTemp("", "filegen-count-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	for i := 0; i < 10; i++ {
		_, err := fmt.Fprintf(tmpfile, "test log line %d\n", i)
		require.NoError(t, err)
	}
	tmpfile.Close()

	consumer := newMockConsumer()

	gen, err := New(logger, 2, 10*time.Millisecond, tmpfile.Name(), true, 0, consumer)
	require.NoError(t, err)

	tracker := count.NewTracker(5)
	gen.SetCountTracker(tracker)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have been exhausted")
	}

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = gen.Stop(ctx)
	require.NoError(t, err)

	writes := consumer.getWrites()
	assert.Equal(t, 5, len(writes), "Expected exactly 5 logs with count tracker")
}
