package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func main() {
	// Listen on localhost:5000
	listener, err := net.Listen("tcp", "localhost:5000")
	if err != nil {
		fmt.Printf("Failed to start TCP server: %v\n", err)
		return
	}
	defer listener.Close()

	fmt.Println("TCP server listening on localhost:5000")

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

	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

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
