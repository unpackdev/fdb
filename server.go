package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

var pool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 1024) // Pre-allocate a 1KB buffer for each connection
	},
}

func startServer() error {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		return fmt.Errorf("could not start server: %v", err)
	}
	defer listener.Close()

	log.Println("Server started on :8080")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := pool.Get().([]byte)
	defer pool.Put(buffer)

	// Use bufio for efficient reading
	reader := bufio.NewReader(conn)

	for {
		// Set a timeout to prevent long-lived idle connections
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		n, err := reader.Read(buffer)
		if err != nil {
			log.Printf("Error reading from connection: %v", err)
			return
		}

		// Echo the received data back to the client
		_, err = conn.Write(buffer[:n])
		if err != nil {
			log.Printf("Error writing to connection: %v", err)
			return
		}
	}
}
