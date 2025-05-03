// pkg/protocols/rpc/pool.go
package rpc

import (
	"github.com/sasha-s/go-deadlock"
	"net"
	"sync"
)

// ConnectionPool manages a pool of reusable TCP connections.
type ConnectionPool struct {
	addr        string
	pool        chan net.Conn
	maxSize     int
	mu          deadlock.Mutex
	currentSize int
	once        sync.Once // Ensures Close is called only once
}

// NewConnectionPool creates a new ConnectionPool.
func NewConnectionPool(addr string, maxSize int) *ConnectionPool {
	return &ConnectionPool{
		addr:    addr,
		pool:    make(chan net.Conn, maxSize),
		maxSize: maxSize,
	}
}

// Get retrieves a connection from the pool or creates a new one.
func (cp *ConnectionPool) Get() (net.Conn, error) {
	select {
	case conn := <-cp.pool:
		return conn, nil
	default:
		cp.mu.Lock()
		defer cp.mu.Unlock()
		if cp.currentSize < cp.maxSize {
			conn, err := net.Dial("tcp", cp.addr)
			if err != nil {
				return nil, err
			}
			cp.currentSize++
			return conn, nil
		}
		// Wait for a connection to be available
		conn := <-cp.pool
		return conn, nil
	}
}

// Put returns a connection to the pool.
func (cp *ConnectionPool) Put(conn net.Conn) {
	select {
	case cp.pool <- conn:
		// Connection returned to pool
	default:
		// Pool is full, close the connection
		conn.Close()
		cp.mu.Lock()
		cp.currentSize--
		cp.mu.Unlock()
	}
}

// Close closes all connections in the pool.
func (cp *ConnectionPool) Close() {
	cp.once.Do(func() { // Ensures this block runs only once
		cp.mu.Lock()
		defer cp.mu.Unlock()
		close(cp.pool)
		for conn := range cp.pool {
			conn.Close()
		}
		cp.currentSize = 0
	})
}
