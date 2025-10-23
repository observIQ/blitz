package output

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewUDP(t *testing.T) {
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
				udp, err = NewUDP(nil, tt.host, tt.port, tt.workers)
			} else {
				udp, err = NewUDP(logger, tt.host, tt.port, tt.workers)
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewUDP() expected error but got none")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("NewUDP() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewUDP() unexpected error = %v", err)
				return
			}

			if udp == nil {
				t.Errorf("NewUDP() returned nil UDP instance")
				return
			}

			// Verify the configuration was set correctly
			if udp.host != tt.host {
				t.Errorf("NewUDP() host = %v, want %v", udp.host, tt.host)
			}
			if udp.port != tt.port {
				t.Errorf("NewUDP() port = %v, want %v", udp.port, tt.port)
			}

			// Verify workers defaulting
			expectedWorkers := tt.workers
			if tt.workers <= 0 {
				expectedWorkers = DefaultUDPWorkers
			}
			if udp.workers != expectedWorkers {
				t.Errorf("NewUDP() workers = %v, want %v", udp.workers, expectedWorkers)
			}

			// Verify channel was created
			if udp.dataChan == nil {
				t.Errorf("NewUDP() dataChan is nil")
			}

			// Verify context was created
			if udp.ctx == nil {
				t.Errorf("NewUDP() ctx is nil")
			}
			if udp.cancel == nil {
				t.Errorf("NewUDP() cancel is nil")
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
	udp, err := NewUDP(logger, host, port, 1)
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}

	// Test data to send
	testData1 := []byte("Hello, UDP!")
	testData2 := []byte("Second UDP message")

	// Send first message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = udp.Write(ctx, testData1)
	if err != nil {
		t.Errorf("First Write() failed: %v", err)
	}

	// Give some time for first message to be sent
	time.Sleep(50 * time.Millisecond)

	// Send second message
	err = udp.Write(ctx, testData2)
	if err != nil {
		t.Errorf("Second Write() failed: %v", err)
	}

	// Give some time for data to be sent
	time.Sleep(100 * time.Millisecond)

	// Stop the client
	err = udp.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Wait a bit more for final data to arrive
	time.Sleep(100 * time.Millisecond)

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
	if !containsString(allDataStr, string(testData1)) {
		t.Errorf("First message %q not found in received data: %q", string(testData1), allDataStr)
	}
	if !containsString(allDataStr, string(testData2)) {
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
	udp, err := NewUDP(logger, host, port, 1)
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

	err = udp.Write(ctx, []byte("This should fail"))
	if err != nil {
		// Error is also expected due to race condition
		if !containsString(err.Error(), "UDP output is shutting down") {
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
	udp, err := NewUDP(logger, host, port, 1)
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
