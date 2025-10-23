package output

import (
	"context"
	"net"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// Benchmark server state
var (
	benchServer     net.Listener
	benchServerAddr string
	benchServerOnce sync.Once
)

// startBenchmarkServer starts a single TCP server for all benchmarks
func startBenchmarkServer() (net.Listener, string) {
	benchServerOnce.Do(func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic("Failed to start benchmark server: " + err.Error())
		}

		benchServer = listener
		benchServerAddr = listener.Addr().String()

		// Start server goroutine that discards all data
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					// Listener closed, exit
					return
				}

				go func() {
					defer conn.Close()
					// Discard all data by reading into a buffer
					buffer := make([]byte, 4096)
					for {
						_, err := conn.Read(buffer)
						if err != nil {
							return
						}
					}
				}()
			}
		}()
	})

	return benchServer, benchServerAddr
}

func BenchmarkTCP_1Worker(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 1 worker
	tcp, err := NewTCP(logger, host, port, 1)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := tcp.Write(ctx, testData)
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

func BenchmarkTCP_10Workers(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 10 workers
	tcp, err := NewTCP(logger, host, port, 10)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := tcp.Write(ctx, testData)
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

// BenchmarkTCP_1Worker_Sequential benchmarks 1 worker with sequential writes
func BenchmarkTCP_1Worker_Sequential(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 1 worker
	tcp, err := NewTCP(logger, host, port, 1)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := tcp.Write(ctx, testData)
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}

// BenchmarkTCP_10Workers_Sequential benchmarks 10 workers with sequential writes
func BenchmarkTCP_10Workers_Sequential(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr := startBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 10 workers
	tcp, err := NewTCP(logger, host, port, 10)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := []byte("benchmark test data")

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := tcp.Write(ctx, testData)
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}

// BenchmarkTCP_ChannelOnly benchmarks just the channel operations without TCP
func BenchmarkTCP_ChannelOnly(b *testing.B) {
	// Create a channel similar to the TCP implementation
	dataChan := make(chan []byte, DefaultTCPChannelSize)

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

// BenchmarkTCP_ChannelOnly_Sequential benchmarks channel operations sequentially
func BenchmarkTCP_ChannelOnly_Sequential(b *testing.B) {
	// Create a channel similar to the TCP implementation
	dataChan := make(chan []byte, DefaultTCPChannelSize)

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
