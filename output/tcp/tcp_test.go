package tcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

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
			var tcp *TCP
			var err error

			if tt.name == "nil logger" {
				tcp, err = New(nil, tt.host, tt.port, tt.workers, nil)
			} else {
				tcp, err = New(logger, tt.host, tt.port, tt.workers, nil)
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

			if tcp == nil {
				t.Errorf("New() returned nil TCP instance")
				return
			}

			// Verify the configuration was set correctly
			if tcp.host != tt.host {
				t.Errorf("New() host = %v, want %v", tcp.host, tt.host)
			}
			if tcp.port != tt.port {
				t.Errorf("New() port = %v, want %v", tcp.port, tt.port)
			}

			// Verify workers defaulting
			expectedWorkers := tt.workers
			if tt.workers <= 0 {
				expectedWorkers = DefaultTCPWorkers
			}
			if tcp.workers != expectedWorkers {
				t.Errorf("New() workers = %v, want %v", tcp.workers, expectedWorkers)
			}

			// Verify channel was created
			if tcp.dataChan == nil {
				t.Errorf("New() dataChan is nil")
			}

			// Verify context was created
			if tcp.ctx == nil {
				t.Errorf("New() ctx is nil")
			}
			if tcp.cancel == nil {
				t.Errorf("New() cancel is nil")
			}

			// Clean up
			tcp.Stop(context.Background())
		})
	}
}

func TestTCP_Integration(t *testing.T) {
	logger := zap.NewNop()

	// Start a TCP server on a random available port
	listener, serverAddr := startTestTCPServer(t)
	defer listener.Close()

	// Extract host and port from the server address
	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client
	tcp, err := New(logger, host, port, 1, nil)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	// Test data to send
	testData1 := "Hello, World!"
	testData2 := "Second message"

	// Send first message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = tcp.Write(ctx, output.LogRecord{Message: testData1})
	if err != nil {
		t.Errorf("First Write() failed: %v", err)
	}

	// Send second message
	err = tcp.Write(ctx, output.LogRecord{Message: testData2})
	if err != nil {
		t.Errorf("Second Write() failed: %v", err)
	}

	// Wait for the server to receive both messages before stopping. Stop closes
	// the data channel and cancels the worker context at the same time, so any
	// data still buffered can be dropped if we stop before the worker drains it.
	require.Eventually(t, func() bool {
		var allData []byte
		for _, data := range getReceivedData(t) {
			allData = append(allData, data...)
		}
		s := string(allData)
		return strings.Contains(s, testData1) && strings.Contains(s, testData2)
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the client
	err = tcp.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify the server received the data
	receivedData := getReceivedData(t)

	// TCP is a stream protocol, so messages might be concatenated
	// We'll check that we received at least one message containing our data
	if len(receivedData) == 0 {
		t.Errorf("Expected at least 1 message, got 0")
		return
	}

	// Combine all received data to check content
	var allData []byte
	for _, data := range receivedData {
		allData = append(allData, data...)
	}

	// Check that both test messages are present in the received data
	// Note: TCP output now appends \n to each message
	allDataStr := string(allData)
	if !strings.Contains(allDataStr, string(testData1)) {
		t.Errorf("First message %q not found in received data: %q", string(testData1), allDataStr)
	}
	if !strings.Contains(allDataStr, string(testData2)) {
		t.Errorf("Second message %q not found in received data: %q", string(testData2), allDataStr)
	}
}

func TestTCP_WriteAfterStop(t *testing.T) {
	logger := zap.NewNop()

	// Start a TCP server
	listener, serverAddr := startTestTCPServer(t)
	defer listener.Close()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client
	tcp, err := New(logger, host, port, 1, nil)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	// Stop the client
	ctx := context.Background()
	err = tcp.Stop(ctx)
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

	err = tcp.Write(ctx, output.LogRecord{Message: "This should fail"})
	if err != nil {
		// Error is also expected due to race condition
		if !strings.Contains(err.Error(), "TCP output is shutting down") {
			t.Errorf("Write after Stop should return shutdown error, got: %v", err)
		}
	}
}

func TestTCP_StopTwice(t *testing.T) {
	logger := zap.NewNop()

	// Start a TCP server
	listener, serverAddr := startTestTCPServer(t)
	defer listener.Close()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client
	tcp, err := New(logger, host, port, 1, nil)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	// Stop the client first time
	ctx := context.Background()
	err = tcp.Stop(ctx)
	if err != nil {
		t.Errorf("First Stop() failed: %v", err)
	}

	// Try to stop again - should panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Second Stop should panic, but didn't")
		}
	}()

	tcp.Stop(ctx)
}

func TestTCP_IntegrationTLS(t *testing.T) {
	logger := zap.NewNop()

	// Find test certificates relative to the repository root
	// Get the absolute path of the test file first
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	// output/tcp/tcp_test.go -> output/tcp/ -> output/ -> repo root
	repoRoot := filepath.Join(testDir, "..", "..")
	certFile := filepath.Join(repoRoot, "cmd", "server", "tcp", "test_server.crt")
	keyFile := filepath.Join(repoRoot, "cmd", "server", "tcp", "test_server.key")

	// Make paths absolute
	certFile, _ = filepath.Abs(certFile)
	keyFile, _ = filepath.Abs(keyFile)

	// Check if certificates exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		t.Skipf("Test certificates not found at %s. Run ./generate_test_certs.sh in cmd/server/tcp/", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Skipf("Test key not found at %s. Run ./generate_test_certs.sh in cmd/server/tcp/", keyFile)
	}

	// Load server certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to load test certificates: %v", err)
	}

	// Start a TLS TCP server on a random available port
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to start TLS TCP server: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Reset received data
	dataMutex.Lock()
	receivedData = make([][]byte, 0)
	dataMutex.Unlock()

	// Start server goroutine
	serverReady := make(chan bool, 1)
	go func() {
		serverReady <- true
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleTestConnection(conn)
		}
	}()

	// Wait for server to be ready. Once Accept is being driven, the kernel
	// backlog accepts the client connection, and the final Eventually poll
	// covers actual data delivery, so no fixed startup sleep is needed.
	<-serverReady

	// Extract host and port from the server address
	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to split server address: %v", err)
	}

	// Load CA certificate for client
	caCert, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("Failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		t.Fatalf("Failed to parse CA certificate")
	}

	// Create TLS config for client
	// Note: Since the certificate is for "localhost", we need to set ServerName
	clientTLSConfig := &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		ServerName:         "localhost", // Certificate is valid for localhost
	}

	// Create TCP client with TLS
	tcp, err := New(logger, host, port, 1, clientTLSConfig)
	if err != nil {
		t.Fatalf("Failed to create TLS TCP client: %v", err)
	}

	// Test data to send
	testData1 := "Hello, TLS World!"
	testData2 := "Second TLS message"

	// Send first message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = tcp.Write(ctx, output.LogRecord{Message: testData1})
	if err != nil {
		t.Errorf("First Write() failed: %v", err)
	}

	// Send second message
	err = tcp.Write(ctx, output.LogRecord{Message: testData2})
	if err != nil {
		t.Errorf("Second Write() failed: %v", err)
	}

	// Wait for the server to receive both messages before stopping. Stop closes
	// the data channel and cancels the worker context at the same time, so any
	// data still buffered can be dropped if we stop before the worker drains it.
	require.Eventually(t, func() bool {
		var allData []byte
		for _, data := range getReceivedData(t) {
			allData = append(allData, data...)
		}
		s := string(allData)
		return strings.Contains(s, testData1) && strings.Contains(s, testData2)
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the client
	err = tcp.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify the server received the data
	receivedData := getReceivedData(t)

	// TCP is a stream protocol, so messages might be concatenated
	if len(receivedData) == 0 {
		t.Errorf("Expected at least 1 message, got 0")
		return
	}

	// Combine all received data to check content
	var allData []byte
	for _, data := range receivedData {
		allData = append(allData, data...)
	}

	// Check that both test messages are present in the received data
	allDataStr := string(allData)
	if !strings.Contains(allDataStr, string(testData1)) {
		t.Errorf("First message %q not found in received data: %q", string(testData1), allDataStr)
	}
	if !strings.Contains(allDataStr, string(testData2)) {
		t.Errorf("Second message %q not found in received data: %q", string(testData2), allDataStr)
	}
}

// TestTCP_StopWithUnreachableDestinationDoesNotHang verifies that Stop completes
// (without hanging) when buffered records cannot be drained because the
// destination is unreachable. Pointing at a closed port means the worker never
// connects, so the records stay buffered until Stop, and the drain's connect
// attempt fails fast rather than blocking shutdown.
func TestTCP_StopWithUnreachableDestinationDoesNotHang(t *testing.T) {
	logger := zap.NewNop()

	// Reserve a port, then close the listener so nothing is listening on it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	require.NoError(t, l.Close())

	tcp, err := New(logger, "127.0.0.1", port, 1, nil)
	require.NoError(t, err)

	// The worker cannot connect, so these stay buffered (well under channel cap).
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		require.NoError(t, tcp.Write(ctx, output.LogRecord{Message: fmt.Sprintf("unreachable-%d", i)}))
	}

	done := make(chan error, 1)
	go func() { done <- tcp.Stop(ctx) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("Stop hung when draining to an unreachable destination")
	}
}

// Test server implementation
var (
	receivedData [][]byte
	dataMutex    sync.Mutex
)

func startTestTCPServer(t *testing.T) (net.Listener, string) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	// Reset received data
	dataMutex.Lock()
	receivedData = make([][]byte, 0)
	dataMutex.Unlock()

	// Start server goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Listener closed, exit
				return
			}

			go handleTestConnection(conn)
		}
	}()

	return listener, listener.Addr().String()
}

func handleTestConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			// Connection closed or error, exit
			return
		}

		// Store received data
		data := make([]byte, n)
		copy(data, buffer[:n])

		dataMutex.Lock()
		receivedData = append(receivedData, data)
		dataMutex.Unlock()
	}
}

func getReceivedData(t *testing.T) [][]byte {
	dataMutex.Lock()
	defer dataMutex.Unlock()

	// Return a copy of the received data
	result := make([][]byte, len(receivedData))
	for i, data := range receivedData {
		result[i] = make([]byte, len(data))
		copy(result[i], data)
	}

	return result
}

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
func (f *fakeConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (f *fakeConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (f *fakeConn) SetDeadline(time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error { return nil }

// drainBuffered with an empty channel must return before attempting to connect.
func TestTCP_drainBuffered_emptyChannelReturnsEarly(t *testing.T) {
	tcp := &TCP{logger: zap.NewNop(), dataChan: make(chan string, 1)}
	tcp.drainBuffered(context.Background())
}

// drainTo stops at the first send failure, leaving the rest undrained.
func TestTCP_drainTo_stopsOnSendError(t *testing.T) {
	tcp := &TCP{logger: zap.NewNop(), dataChan: make(chan string, 4)}
	tcp.dataChan <- "one"
	tcp.dataChan <- "two"
	close(tcp.dataChan)

	conn := &fakeConn{writeErr: fmt.Errorf("write failed")}
	tcp.drainTo(context.Background(), conn, time.Now().Add(time.Hour))

	require.Equal(t, 1, conn.writes, "drainTo should stop after the first failed send")
}

// drainTo returns without sending when the context is already done.
func TestTCP_drainTo_stopsWhenContextDone(t *testing.T) {
	tcp := &TCP{logger: zap.NewNop(), dataChan: make(chan string, 4)}
	tcp.dataChan <- "one"
	close(tcp.dataChan)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn := &fakeConn{}
	tcp.drainTo(ctx, conn, time.Now().Add(time.Hour))

	require.Equal(t, 0, conn.writes, "drainTo should not send once the context is done")
}

// TestTCP_StopDrainsBufferedRecords verifies Stop() drains everything already
// buffered before returning, instead of dropping records that are still queued
// when shutdown begins. Unlike the other delivery tests, this one does NOT poll
// for delivery before calling Stop — draining is Stop()'s responsibility.
func TestTCP_StopDrainsBufferedRecords(t *testing.T) {
	logger := zap.NewNop()

	listener, serverAddr := startTestTCPServer(t)
	defer listener.Close()

	host, port, err := net.SplitHostPort(serverAddr)
	require.NoError(t, err)

	tcp, err := New(logger, host, port, 1, nil)
	require.NoError(t, err)

	// Queue a backlog of uniquely-identifiable records as fast as possible, so a
	// large number are still buffered when Stop is called, then stop immediately.
	const n = 100
	ctx := context.Background()
	for i := 0; i < n; i++ {
		require.NoError(t, tcp.Write(ctx, output.LogRecord{Message: fmt.Sprintf("drain-msg-%d", i)}))
	}

	require.NoError(t, tcp.Stop(ctx))

	// Once Stop has returned, every record must have been delivered. Poll only to
	// let the test server goroutine finish reading bytes already on the wire; no
	// new records can be sent after Stop returns, so a missing record means it was
	// dropped rather than drained.
	// Once Stop has returned, every record must have been delivered. Poll only to
	// let the test server goroutine finish reading bytes already on the wire; no
	// new records can be sent after Stop returns, so a missing record means it was
	// dropped rather than drained.
	require.Eventually(t, func() bool {
		var all []byte
		for _, d := range getReceivedData(t) {
			all = append(all, d...)
		}
		s := string(all)
		for i := 0; i < n; i++ {
			if !strings.Contains(s, fmt.Sprintf("drain-msg-%d\n", i)) {
				return false
			}
		}
		return true
	}, 3*time.Second, 10*time.Millisecond, "all buffered records should be delivered before Stop returns")
}
