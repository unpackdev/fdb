package fdb

import (
	"context"
	"github.com/panjf2000/gnet"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"log"
	"time"
)

type DummyHandler func(c gnet.Conn, frame []byte)

// DummyServer struct represents the Unix Domain Socket (UDS) server using gnet
type DummyServer struct {
	*gnet.EventServer
	ctx             context.Context
	cnf             config.DummyTransport
	handlerRegistry map[types.HandlerType]DummyHandler
	addr            string
	stopChan        chan struct{}
	started         chan struct{}
}

// NewUDSServer creates a new UDSServer instance
func NewUDSServer(ctx context.Context, cnf config.DummyTransport) (*DummyServer, error) {
	server := &DummyServer{
		ctx:             ctx,
		cnf:             cnf,
		handlerRegistry: make(map[types.HandlerType]DummyHandler),
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the UDS socket path as a string
func (s *DummyServer) Addr() string {
	return s.cnf.Addr()
}

// Start starts the UDS server
func (s *DummyServer) Start() error {
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
func (s *DummyServer) Tick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop stops the UDS server
func (s *DummyServer) Stop() {
	close(s.stopChan)
}

func (s *DummyServer) WaitStarted() <-chan struct{} {
	return s.started
}

// OnInitComplete is called when the server starts
func (s *DummyServer) OnInitComplete(server gnet.Server) (action gnet.Action) {
	log.Printf("Dummy Server is listening on %s", server.Addr.String())
	close(s.started) // Signal that the server has started
	return
}

// React handles incoming data
func (s *DummyServer) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
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
func (s *DummyServer) parseActionType(frame []byte) (types.HandlerType, error) {
	if len(frame) < 1 {
		return 0, errors.New("invalid action: frame too short")
	}

	var actionType types.HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *DummyServer) RegisterHandler(actionType types.HandlerType, handler DummyHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *DummyServer) DeregisterHandler(actionType types.HandlerType) {
	delete(s.handlerRegistry, actionType)
}
