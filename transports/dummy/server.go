package transport_dummy

import (
	"context"
	"github.com/panjf2000/gnet"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"time"
)

type DummyHandler func(c gnet.Conn, frame []byte)

type Server struct {
	*gnet.EventServer
	ctx             context.Context
	cnf             config.DummyTransport
	handlerRegistry map[types.HandlerType]DummyHandler
	stopChan        chan struct{}
	started         chan struct{}
}

func NewDummyServer(ctx context.Context, cnf config.DummyTransport) (*Server, error) {
	server := &Server{
		ctx:             ctx,
		cnf:             cnf,
		handlerRegistry: make(map[types.HandlerType]DummyHandler),
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the UDS socket path as a string
func (s *Server) Addr() string {
	return s.cnf.Addr()
}

// Start starts the UDS server
func (s *Server) Start(ctx context.Context) error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}, 1) // Initialize the started channel

	listenAddr := "udp://" + s.cnf.Addr()
	defer zap.L().Info("Dummy transport started", zap.String("addr", listenAddr))

	// Create an error channel to capture errors from the goroutine
	errChan := make(chan error, 1)

	// Start the server asynchronously
	go func() {
		err := gnet.Serve(
			s, listenAddr,
			gnet.WithMulticore(true),
			gnet.WithReusePort(true),
			gnet.WithSocketRecvBuffer(1024*64),
			gnet.WithLockOSThread(true),
			gnet.WithTicker(true),
		)
		if err != nil {
			errChan <- err // Send error to the channel
			return
		}
		close(errChan) // No error, so close the channel
	}()

	// Wait until OnInitComplete sends a signal or an error occurs
	select {
	case <-s.started:
		close(s.started)
		// Server started successfully
		return nil
	case err := <-errChan:
		if err != nil {
			return errors.Wrap(err, "failed to start dummy server")
		}
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("dummy server did not start in time")
	}
}

// Tick is called periodically by gnet
func (s *Server) Tick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop stops the UDS server
func (s *Server) Stop() error {
	close(s.stopChan)
	return nil
}

func (s *Server) WaitStarted() <-chan struct{} {
	return s.started
}

// OnInitComplete is called when the server starts
func (s *Server) OnInitComplete(server gnet.Server) (action gnet.Action) {
	zap.L().Info(
		"Dummy transport initialization completed (listening)",
		zap.String("addr", server.Addr.String()),
	)
	s.started <- struct{}{} // Signal that the server has started
	return
}

// React handles incoming data
func (s *Server) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
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
func (s *Server) parseActionType(frame []byte) (types.HandlerType, error) {
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
func (s *Server) RegisterHandler(actionType types.HandlerType, handler DummyHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *Server) DeregisterHandler(actionType types.HandlerType) {
	delete(s.handlerRegistry, actionType)
}
