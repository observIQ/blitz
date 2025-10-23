package output

import (
	"context"
	"net"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// Benchmark UDP server state
var (
	benchUDPServer     net.PacketConn
	benchUDPServerAddr string
	benchUDPServerOnce sync.Once
)

// startBenchmarkUDPServer starts a single UDP server for all benchmarks
func startBenchmarkUDPServer() (net.PacketConn, string) {
	benchUDPServerOnce.Do(func() {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			panic("Failed to start benchmark UDP server: " + err.Error())
		}

		benchUDPServer = conn
		benchUDPServerAddr = conn.LocalAddr().String()

		// Start server goroutine that discards all data
		go func() {
			buffer := make([]byte, 4096)
			for {
				_, _, err := conn.ReadFrom(buffer)
				if err != nil {
					// Connection closed or error, exit
					return
				}
				// Discard data
			}
		}()
	})

	return benchUDPServer, benchUDPServerAddr
}

func BenchmarkUDP_1Worker(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkUDPServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client with 1 worker
	udp, err := NewUDP(logger, host, port, 1)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer udp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := udp.Write(ctx, testData)
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

func BenchmarkUDP_10Workers(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkUDPServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client with 10 workers
	udp, err := NewUDP(logger, host, port, 10)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer udp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := udp.Write(ctx, testData)
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

// BenchmarkUDP_1Worker_Sequential benchmarks 1 worker with sequential writes
func BenchmarkUDP_1Worker_Sequential(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkUDPServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client with 1 worker
	udp, err := NewUDP(logger, host, port, 1)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer udp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := udp.Write(ctx, testData)
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}

// BenchmarkUDP_10Workers_Sequential benchmarks 10 workers with sequential writes
func BenchmarkUDP_10Workers_Sequential(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkUDPServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create UDP client with 10 workers
	udp, err := NewUDP(logger, host, port, 10)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer udp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := udp.Write(ctx, testData)
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}

// BenchmarkUDP_ChannelOnly benchmarks just the channel operations without UDP
func BenchmarkUDP_ChannelOnly(b *testing.B) {
	// Create a channel similar to the UDP implementation
	dataChan := make(chan []byte, DefaultUDPChannelSize)

	// Start a goroutine to consume data
	go func() {
		for range dataChan {
			// Discard data
		}
	}()

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			select {
			case dataChan <- testData:
			default:
				// Channel full, skip
			}
		}
	})

	close(dataChan)
}

// BenchmarkUDP_ChannelOnly_Sequential benchmarks channel operations sequentially
func BenchmarkUDP_ChannelOnly_Sequential(b *testing.B) {
	// Create a channel similar to the UDP implementation
	dataChan := make(chan []byte, DefaultUDPChannelSize)

	// Start a goroutine to consume data
	go func() {
		for range dataChan {
			// Discard data
		}
	}()

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		select {
		case dataChan <- testData:
		default:
			// Channel full, skip
		}
	}

	close(dataChan)
}
