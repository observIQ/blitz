package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

// WARNING: Test certificates are for TESTING ONLY and should NOT be used in production.
// The certificates located in this directory (test_server.key and test_server.crt) are
// self-signed certificates generated for local development and testing purposes only.
const (
	defaultCertFile = "test_server.crt"
	defaultKeyFile  = "test_server.key"
)

func main() {
	var (
		tlsEnabled = flag.Bool("tls", false, "Enable TLS using test_server.crt and test_server.key")
		certFile   = flag.String("cert", defaultCertFile, "Path to TLS certificate file")
		keyFile    = flag.String("key", defaultKeyFile, "Path to TLS private key file")
		port       = flag.String("port", "5000", "Port to listen on")
	)
	flag.Parse()

	var listener net.Listener
	var err error

	if *tlsEnabled {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			fmt.Printf("Failed to load TLS certificate: %v\n", err)
			os.Exit(1)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		// #nosec G102 - This is a test server, binding to any interface is acceptable
		listener, err = tls.Listen("tcp", ":"+*port, tlsConfig)
		if err != nil {
			fmt.Printf("Failed to start TLS TCP server: %v\n", err)
			os.Exit(1)
		}
		defer listener.Close()

		fmt.Printf("TLS TCP server listening on %s (cert: %s, key: %s)\n", *port, *certFile, *keyFile)
	} else {
		// #nosec G102 - This is a test server, binding to any interface is acceptable
		listener, err = net.Listen("tcp", ":"+*port)
		if err != nil {
			fmt.Printf("Failed to start TCP server: %v\n", err)
			os.Exit(1)
		}
		defer listener.Close()

		fmt.Printf("TCP server listening on %s\n", *port)
	}

	for {
		// Accept connections with a timeout
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		// Handle each connection in a goroutine
		go handleTCPConnection(conn)
	}
}

func handleTCPConnection(conn net.Conn) {
	defer conn.Close()

	err := conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		fmt.Printf("Failed to set read deadline: %v\n", err)
		os.Exit(1)
	}

	// Read data from the connection
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		data := scanner.Text()
		fmt.Printf("TCP received: %s\n", data)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("TCP read error: %v\n", err)
	}
}
