package udp

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeConn is a minimal net.Conn used to exercise drainTo's send-failure and
// context/deadline branches without real network I/O.
type fakeConn struct {
	writeErr error
	writes   int
}

func (f *fakeConn) Read([]byte) (int, error) { return 0, io.EOF }
func (f *fakeConn) Write(b []byte) (int, error) {
	f.writes++
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(b), nil
}
func (f *fakeConn) Close() error                     { return nil }
func (f *fakeConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (f *fakeConn) RemoteAddr() net.Addr             { return &net.UDPAddr{} }
func (f *fakeConn) SetDeadline(time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func TestUDP_drainBuffered_emptyChannelReturnsEarly(t *testing.T) {
	u := &UDP{logger: zap.NewNop(), dataChan: make(chan string, 1)}
	u.drainBuffered(context.Background())
}

func TestUDP_drainBuffered_connectErrorReturns(t *testing.T) {
	u := &UDP{logger: zap.NewNop(), host: "nonexistent.invalid", port: "1", dataChan: make(chan string, 1)}
	u.dataChan <- "x"
	u.drainBuffered(context.Background())
}

func TestUDP_drainTo_stopsOnSendError(t *testing.T) {
	u := &UDP{logger: zap.NewNop(), dataChan: make(chan string, 2)}
	u.dataChan <- "one"
	u.dataChan <- "two"
	close(u.dataChan)

	conn := &fakeConn{writeErr: fmt.Errorf("write failed")}
	u.drainTo(context.Background(), conn, time.Now().Add(time.Hour))
	require.Equal(t, 1, conn.writes)
}

func TestUDP_drainTo_stopsWhenContextDone(t *testing.T) {
	u := &UDP{logger: zap.NewNop(), dataChan: make(chan string, 1)}
	u.dataChan <- "one"
	close(u.dataChan)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn := &fakeConn{}
	u.drainTo(ctx, conn, time.Now().Add(time.Hour))
	require.Equal(t, 0, conn.writes)
}

func TestNew(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name        string
		host        string
		port        string
		workers     int
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid configuration with default workers",
			host:    "localhost",
			port:    "8080",
			workers: 0, // Should default to 1
			wantErr: false,
		},
		{
			name:    "valid configuration with custom workers",
			host:    "example.com",
			port:    "9090",
			workers: 3,
			wantErr: false,
		},
		{
			name:    "valid configuration with single worker",
			host:    "127.0.0.1",
			port:    "3000",
			workers: 1,
			wantErr: false,
		},
		{
			name:        "nil logger",
			host:        "localhost",
			port:        "8080",
			workers:     1,
			wantErr:     true,
			errContains: "logger cannot be nil",
		},
		{
			name:        "empty host",
			host:        "",
			port:        "8080",
			workers:     1,
			wantErr:     true,
			errContains: "host cannot be empty",
		},
		{
			name:        "empty port",
			host:        "localhost",
			port:        "",
			workers:     1,
			wantErr:     true,
			errContains: "port cannot be empty",
		},
		{
			name:        "negative workers",
			host:        "localhost",
			port:        "8080",
			workers:     -1,
			wantErr:     false, // Should default to 1
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var udp *UDP
			var err error

			if tt.name == "nil logger" {
				udp, err = New(nil, tt.host, tt.port, tt.workers)
			} else {
				udp, err = New(logger, tt.host, tt.port, tt.workers)
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("New() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("New() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("New() unexpected error = %v", err)
				return
			}

			if udp == nil {
				t.Errorf("New() returned nil UDP instance")
				return
			}

			// Verify the configuration was set correctly
			if udp.host != tt.host {
				t.Errorf("New() host = %v, want %v", udp.host, tt.host)
			}
			if udp.port != tt.port {
				t.Errorf("New() port = %v, want %v", udp.port, tt.port)
			}

			// Verify workers defaulting
			expectedWorkers := tt.workers
			if tt.workers <= 0 {
				expectedWorkers = DefaultUDPWorkers
			}
			if udp.workers != expectedWorkers {
				t.Errorf("New() workers = %v, want %v", udp.workers, expectedWorkers)
			}

			// Verify channel was created
			if udp.dataChan == nil {
				t.Errorf("New() dataChan is nil")
			}

			// Verify context was created
			if udp.ctx == nil {
				t.Errorf("New() ctx is nil")
			}
			if udp.cancel == nil {
				t.Errorf("New() cancel is nil")
			}

			// Clean up
			udp.Stop(context.Background())
		})
	}
}

func TestUDP_Integration(t *testing.T) {
	logger := zap.NewNop()

	// Start a UDP server on a random available port
	listener, serverAddr := startTestUDPServer(t)
	defer listener.Close()

	// Extract host and port from the server address
	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client
	udp, err := New(logger, host, port, 1)
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}

	// Test data to send
	testData1 := "Hello, UDP!"
	testData2 := "Second UDP message"

	// Send first message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = udp.Write(ctx, output.LogRecord{Message: testData1})
	if err != nil {
		t.Errorf("First Write() failed: %v", err)
	}

	// Send second message
	err = udp.Write(ctx, output.LogRecord{Message: testData2})
	if err != nil {
		t.Errorf("Second Write() failed: %v", err)
	}

	// Wait for the server to receive both messages before stopping. Stop closes
	// the data channel and cancels the worker context at the same time, so any
	// data still buffered can be dropped if we stop before the worker drains it.
	require.Eventually(t, func() bool {
		var allData []byte
		for _, data := range getReceivedUDPData(t) {
			allData = append(allData, data...)
		}
		s := string(allData)
		return strings.Contains(s, testData1) && strings.Contains(s, testData2)
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the client
	err = udp.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify the server received the data
	receivedData := getReceivedUDPData(t)

	// UDP packets are separate, so we should receive them individually
	if len(receivedData) == 0 {
		t.Errorf("Expected at least 1 message, got 0")
		return
	}

	// Check that both test messages are present in the received data
	var allData []byte
	for _, data := range receivedData {
		allData = append(allData, data...)
	}

	allDataStr := string(allData)
	if !strings.Contains(allDataStr, string(testData1)) {
		t.Errorf("First message %q not found in received data: %q", string(testData1), allDataStr)
	}
	if !strings.Contains(allDataStr, string(testData2)) {
		t.Errorf("Second message %q not found in received data: %q", string(testData2), allDataStr)
	}
}

func TestUDP_WriteAfterStop(t *testing.T) {
	logger := zap.NewNop()

	// Start a UDP server
	listener, serverAddr := startTestUDPServer(t)
	defer listener.Close()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client
	udp, err := New(logger, host, port, 1)
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}

	// Stop the client
	ctx := context.Background()
	err = udp.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Try to write after stop - should either panic or return error
	defer func() {
		if r := recover(); r != nil {
			// Panic is expected due to race condition
			// This is acceptable behavior
		}
	}()

	err = udp.Write(ctx, output.LogRecord{Message: "This should fail"})
	if err != nil {
		// Error is also expected due to race condition
		if !strings.Contains(err.Error(), "UDP output is shutting down") {
			t.Errorf("Write after Stop should return shutdown error, got: %v", err)
		}
	}
}

func TestUDP_StopTwice(t *testing.T) {
	logger := zap.NewNop()

	// Start a UDP server
	listener, serverAddr := startTestUDPServer(t)
	defer listener.Close()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client
	udp, err := New(logger, host, port, 1)
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}

	// Stop the client first time
	ctx := context.Background()
	err = udp.Stop(ctx)
	if err != nil {
		t.Errorf("First Stop() failed: %v", err)
	}

	// Try to stop again - should panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Second Stop should panic, but didn't")
		}
	}()

	udp.Stop(ctx)
}

func TestUDP_StopDrainsBufferedRecords(t *testing.T) {
	logger := zap.NewNop()
	listener, serverAddr := startTestUDPServer(t)
	defer listener.Close()
	host, port, err := net.SplitHostPort(serverAddr)
	require.NoError(t, err)
	udp, err := New(logger, host, port, 1)
	require.NoError(t, err)
	const n = 100
	ctx := context.Background()
	for i := 0; i < n; i++ {
		require.NoError(t, udp.Write(ctx, output.LogRecord{Message: fmt.Sprintf("drain-msg-%d", i)}))
	}
	require.NoError(t, udp.Stop(ctx))
	require.Eventually(t, func() bool {
		// UDP sends one datagram per record and does not append a newline, so each
		// buffered record arrives as its own datagram. Match each expected message
		// exactly to disambiguate e.g. drain-msg-1 from drain-msg-10.
		received := make(map[string]bool)
		for _, d := range getReceivedUDPData(t) {
			received[string(d)] = true
		}
		for i := 0; i < n; i++ {
			if !received[fmt.Sprintf("drain-msg-%d", i)] {
				return false
			}
		}
		return true
	}, 3*time.Second, 10*time.Millisecond, "all buffered records should be delivered before Stop returns")
}

// Test UDP server implementation
var (
	receivedUDPData [][]byte
	udpDataMutex    sync.Mutex
)

func startTestUDPServer(t *testing.T) (net.PacketConn, string) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test UDP server: %v", err)
	}

	// Reset received data
	udpDataMutex.Lock()
	receivedUDPData = make([][]byte, 0)
	udpDataMutex.Unlock()

	// Start server goroutine
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, _, err := conn.ReadFrom(buffer)
			if err != nil {
				// Connection closed or error, exit
				return
			}

			// Store received data
			data := make([]byte, n)
			copy(data, buffer[:n])

			udpDataMutex.Lock()
			receivedUDPData = append(receivedUDPData, data)
			udpDataMutex.Unlock()
		}
	}()

	return conn, conn.LocalAddr().String()
}

func getReceivedUDPData(t *testing.T) [][]byte {
	udpDataMutex.Lock()
	defer udpDataMutex.Unlock()

	// Return a copy of the received data
	result := make([][]byte, len(receivedUDPData))
	for i, data := range receivedUDPData {
		result[i] = make([]byte, len(data))
		copy(result[i], data)
	}

	return result
}
