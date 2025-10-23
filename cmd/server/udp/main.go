package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	// Listen on localhost:5000
	addr, err := net.ResolveUDPAddr("udp", "localhost:5000")
	if err != nil {
		fmt.Printf("Failed to resolve UDP address: %v\n", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Failed to start UDP server: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("UDP server listening on localhost:5000")

	// Buffer for incoming data
	buffer := make([]byte, 1024)

	for {
		// Set read timeout
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read data from the connection
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("UDP read error: %v\n", err)
			continue
		}

		data := string(buffer[:n])
		fmt.Printf("UDP received from %s: %s\n", clientAddr, data)
	}
}
