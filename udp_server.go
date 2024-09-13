package fdb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"
)

// Handler function type
type Handler func(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr)

// UdpServer struct represents the UDP server
type UdpServer struct {
	addr            *net.UDPAddr
	conn            *net.UDPConn
	handlerRegistry map[HandlerType]Handler
	pool            sync.Pool
	taskPool        sync.Pool
	taskQueue       chan *Task
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
}

// Task represents a UDP request that needs to be processed
type Task struct {
	conn   *net.UDPConn
	buffer []byte
	addr   *net.UDPAddr
}

// New creates a new UdpServer instance
func New(port int, ip string) (*UdpServer, error) {
	addr := &net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(ip),
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("could not start UDP server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	server := &UdpServer{
		addr:            addr,
		conn:            conn,
		handlerRegistry: make(map[HandlerType]Handler),
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 4096)
			},
		},
		taskPool: sync.Pool{
			New: func() interface{} {
				return new(Task)
			},
		},
		taskQueue: make(chan *Task, 1000),
		ctx:       ctx,
		cancel:    cancel,
	}

	return server, nil
}

func (server *UdpServer) Addr() *net.UDPAddr {
	return server.addr
}

// Start starts the UDP server
func (server *UdpServer) Start() {
	log.Printf("UDP Server started on %v", server.addr)

	// Dynamically determine worker count based on CPU
	workerCount := runtime.NumCPU()

	// Start worker pool
	for i := 0; i < workerCount; i++ {
		server.wg.Add(1)
		go server.worker()
	}

	// Listen for shutdown in a separate goroutine
	go func() {
		<-server.ctx.Done()
		server.conn.Close()     // Close the listener when context is done
		close(server.taskQueue) // Close the task queue to stop workers
	}()

	server.run()
}

// Stop stops the UDP server
func (server *UdpServer) Stop() {
	server.cancel()  // Cancel the context to stop the server
	server.wg.Wait() // Wait for workers to finish
	server.conn.Close()
}

// RegisterHandler registers a handler for a specific action
func (server *UdpServer) RegisterHandler(actionType HandlerType, handler Handler) {
	server.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (server *UdpServer) DeregisterHandler(actionType HandlerType) {
	delete(server.handlerRegistry, actionType)
}

// Run the server loop to process incoming requests
func (server *UdpServer) run() {
	for {
		select {
		case <-server.ctx.Done():
			return
		default:
			buffer := server.pool.Get().([]byte)

			// Read from UDP connection
			n, clientAddr, err := server.conn.ReadFromUDP(buffer)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("Error reading from UDP connection: %v", err)
				server.pool.Put(buffer) // Return buffer on error
				continue
			}

			// Fetch a task from the pool
			task := server.taskPool.Get().(*Task)
			task.conn = server.conn
			task.buffer = buffer[:n]
			task.addr = clientAddr

			// Send the task to the worker pool via the task queue
			server.taskQueue <- task
		}
	}
}

// Worker function to process tasks from the task queue
func (server *UdpServer) worker() {
	defer server.wg.Done()

	for task := range server.taskQueue {
		if task == nil {
			return
		}
		// Process the request based on the action
		server.processRequest(task.conn, task.buffer, task.addr)
		// Return the buffer and task to their respective pools after processing
		server.pool.Put(task.buffer)
		server.taskPool.Put(task)
	}
}

// Process incoming UDP requests and dispatch to the appropriate handler
func (server *UdpServer) processRequest(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Parse the action type from the first byte
	actionType, err := server.parseActionType(buffer)
	if err != nil {
		conn.WriteToUDP([]byte("ERROR: Invalid action"), addr)
		return
	}

	// Look up the handler in the registry
	handler, exists := server.handlerRegistry[actionType]
	if exists {
		handler(conn, buffer, addr)
	} else {
		conn.WriteToUDP([]byte("ERROR: Unknown action"), addr)
	}
}

// ParseActionType parses the first byte of the buffer and returns the corresponding HandlerType
func (server *UdpServer) parseActionType(buffer []byte) (HandlerType, error) {
	if len(buffer) < 1 {
		return 0, errors.New("invalid action: buffer too short")
	}

	var actionType HandlerType
	err := actionType.FromByte(buffer[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}
