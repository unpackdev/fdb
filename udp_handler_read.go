package fdb

import (
	"log"
	"net"
)

// ReadHandler ...
func ReadHandler(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Example: Read data from the connection
	response := []byte("Read action handled")
	_, err := conn.WriteToUDP(response, addr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}
