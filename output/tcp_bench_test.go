package output

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// Benchmark server state
var (
	benchServer     net.Listener
	benchServerAddr string
	benchServerOnce sync.Once

	benchTLSServer     net.Listener
	benchTLSServerAddr string
	benchTLSServerOnce sync.Once
	benchTLSConfig     *tls.Config
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

// startTLSBenchmarkServer starts a single TLS TCP server for all TLS benchmarks
func startTLSBenchmarkServer() (net.Listener, string, *tls.Config) {
	benchTLSServerOnce.Do(func() {
		// Find test certificates relative to the repository root
		_, testFile, _, _ := runtime.Caller(0)
		testDir := filepath.Dir(testFile)
		repoRoot := filepath.Join(testDir, "..")
		certFile := filepath.Join(repoRoot, "cmd", "server", "tcp", "test_server.crt")
		keyFile := filepath.Join(repoRoot, "cmd", "server", "tcp", "test_server.key")

		// Make paths absolute
		certFile, _ = filepath.Abs(certFile)
		keyFile, _ = filepath.Abs(keyFile)

		// Check if certificates exist
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			panic("Test certificates not found at " + certFile + ". Run ./generate_test_certs.sh in cmd/server/tcp/")
		}
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			panic("Test key not found at " + keyFile + ". Run ./generate_test_certs.sh in cmd/server/tcp/")
		}

		// Load server certificate
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			panic("Failed to load test certificates: " + err.Error())
		}

		// Start TLS server
		serverTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
		if err != nil {
			panic("Failed to start TLS benchmark server: " + err.Error())
		}

		benchTLSServer = listener
		benchTLSServerAddr = listener.Addr().String()

		// Load CA certificate for client
		caCert, err := os.ReadFile(certFile)
		if err != nil {
			panic("Failed to read CA certificate: " + err.Error())
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			panic("Failed to parse CA certificate")
		}

		// Create TLS config for client
		benchTLSConfig = &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			ServerName:         "localhost",
		}

		// Start server goroutine that discards all data
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go func() {
					defer conn.Close()
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

	return benchTLSServer, benchTLSServerAddr, benchTLSConfig
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
	tcp, err := NewTCP(logger, host, port, 1, nil)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := tcp.Write(ctx, LogRecord{Message: testData})
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
	tcp, err := NewTCP(logger, host, port, 10, nil)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := tcp.Write(ctx, LogRecord{Message: testData})
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
	tcp, err := NewTCP(logger, host, port, 1, nil)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := tcp.Write(ctx, LogRecord{Message: testData})
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
	tcp, err := NewTCP(logger, host, port, 10, nil)
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := tcp.Write(ctx, LogRecord{Message: testData})
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

func BenchmarkTCP_TLS_1Worker(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr, tlsConfig := startTLSBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 1 worker and TLS
	tcp, err := NewTCP(logger, host, port, 1, tlsConfig)
	if err != nil {
		b.Fatalf("Failed to create TLS TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Give time for the worker to establish TLS connection
	// This is done outside the benchmark timer
	time.Sleep(100 * time.Millisecond)

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := tcp.Write(ctx, LogRecord{Message: testData})
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

func BenchmarkTCP_TLS_10Workers(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr, tlsConfig := startTLSBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 10 workers and TLS
	tcp, err := NewTCP(logger, host, port, 10, tlsConfig)
	if err != nil {
		b.Fatalf("Failed to create TLS TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Give time for the workers to establish TLS connections
	time.Sleep(200 * time.Millisecond)

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			err := tcp.Write(ctx, LogRecord{Message: testData})
			if err != nil {
				b.Errorf("Write failed: %v", err)
			}
		}
	})
}

func BenchmarkTCP_TLS_1Worker_Sequential(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr, tlsConfig := startTLSBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 1 worker and TLS
	tcp, err := NewTCP(logger, host, port, 1, tlsConfig)
	if err != nil {
		b.Fatalf("Failed to create TLS TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Give time for the worker to establish TLS connection
	time.Sleep(100 * time.Millisecond)

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := tcp.Write(ctx, LogRecord{Message: testData})
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}

func BenchmarkTCP_TLS_10Workers_Sequential(b *testing.B) {
	logger := zap.NewNop()

	_, serverAddr, tlsConfig := startTLSBenchmarkServer()
	defer func() {
		// Don't close the server as it's shared across benchmarks
	}()

	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		b.Fatalf("Failed to split server address: %v", err)
	}

	// Create TCP client with 10 workers and TLS
	tcp, err := NewTCP(logger, host, port, 10, tlsConfig)
	if err != nil {
		b.Fatalf("Failed to create TLS TCP client: %v", err)
	}
	defer tcp.Stop(context.Background())

	// Give time for the workers to establish TLS connections
	time.Sleep(200 * time.Millisecond)

	// Test data
	testData := "benchmark test data"

	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		err := tcp.Write(ctx, LogRecord{Message: testData})
		if err != nil {
			b.Errorf("Write failed: %v", err)
		}
	}
}
