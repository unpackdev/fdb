package tcp

import (
	"fmt"
	"net"
	"time"
)

// GetFreePort attempts to find an available port and confirm that it's truly available.
func GetFreePort() (int, error) {
	maxAttempts := 5
	for range maxAttempts {
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		if err != nil {
			return -1, err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return port, nil
	}

	return -1, fmt.Errorf("failed to acquire a free port after %d attempts", maxAttempts)
}
