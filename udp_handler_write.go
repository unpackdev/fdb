package fdb

import (
	"log"
	"net"
)

// WriteHandler ...
func WriteHandler(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Example: Write data to the connection
	response := []byte("Write action handled")
	_, err := conn.WriteToUDP(response, addr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}
