package fdb

import (
	"github.com/panjf2000/gnet"
	"github.com/pkg/errors"
	"log"
	"os"
	"time"
)

// Handler function type for UDS
type UDSHandler func(c gnet.Conn, frame []byte)

// UDSServer struct represents the Unix Domain Socket (UDS) server using gnet
type UDSServer struct {
	*gnet.EventServer
	handlerRegistry map[HandlerType]UDSHandler
	addr            string
	stopChan        chan struct{}
	started         chan struct{}
}

// NewUDSServer creates a new UDSServer instance
func NewUDSServer(socketPath string) (*UDSServer, error) {
	// Remove the existing socket file if it exists
	if _, err := os.Stat(socketPath); err == nil {
		if rmErr := os.Remove(socketPath); rmErr != nil {
			return nil, errors.Wrap(rmErr, "failed to remove existing UDS socket file")
		}
	}

	server := &UDSServer{
		handlerRegistry: make(map[HandlerType]UDSHandler),
		addr:            socketPath,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the UDS socket path as a string
func (s *UDSServer) Addr() string {
	return s.addr
}

// Start starts the UDS server
func (s *UDSServer) Start() error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}) // Initialize the started channel
	listenAddr := "unix://" + s.addr
	log.Printf("UDS Server started on %s", listenAddr)

	return gnet.Serve(
		s, listenAddr,
		gnet.WithMulticore(true),
		gnet.WithReusePort(true),
		gnet.WithSocketRecvBuffer(1024*64),
		gnet.WithLockOSThread(true),
		gnet.WithTicker(true),
	)
}

// Tick is called periodically by gnet
func (s *UDSServer) Tick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop stops the UDS server
func (s *UDSServer) Stop() {
	close(s.stopChan)
}

func (s *UDSServer) WaitStarted() <-chan struct{} {
	return s.started
}

// OnInitComplete is called when the server starts
func (s *UDSServer) OnInitComplete(server gnet.Server) (action gnet.Action) {
	log.Printf("UDS Server is listening on %s", server.Addr.String())
	close(s.started) // Signal that the server has started
	return
}

// React handles incoming data
func (s *UDSServer) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	if len(frame) < 1 {
		return []byte("ERROR: Invalid action"), gnet.None
	}

	// Parse the action type
	actionType, err := s.parseActionType(frame)
	if err != nil {
		return []byte("ERROR: Invalid action"), gnet.None
	}

	// Check if the handler exists
	handler, exists := s.handlerRegistry[actionType]
	if !exists {
		return []byte("ERROR: Unknown action"), gnet.None
	}

	// Call the handler
	handler(c, frame)

	return nil, gnet.None
}

// parseActionType parses the action type from the frame
func (s *UDSServer) parseActionType(frame []byte) (HandlerType, error) {
	if len(frame) < 1 {
		return 0, errors.New("invalid action: frame too short")
	}

	var actionType HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *UDSServer) RegisterHandler(actionType HandlerType, handler UDSHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *UDSServer) DeregisterHandler(actionType HandlerType) {
	delete(s.handlerRegistry, actionType)
}
