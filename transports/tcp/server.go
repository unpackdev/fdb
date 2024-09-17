package transport_tcp

import (
	"context"
	"io"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

// TCPHandler function type for TCP handlers
type TCPHandler func(c gnet.Conn, frame []byte)

// Server struct represents the TCP server using gnet
type Server struct {
	ctx             context.Context
	handlerRegistry map[types.HandlerType]TCPHandler
	cnf             config.TcpTransport
	stopChan        chan struct{}
	started         chan struct{}
	eng             gnet.Engine
}

// NewServer creates a new TCP Server instance using the provided configuration
func NewServer(ctx context.Context, cnf config.TcpTransport) (*Server, error) {
	server := &Server{
		ctx:             ctx,
		handlerRegistry: make(map[types.HandlerType]TCPHandler),
		cnf:             cnf,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the TCP address as a string
func (s *Server) Addr() string {
	return s.cnf.Addr()
}

// Start starts the TCP server using the provided configuration
func (s *Server) Start(ctx context.Context) error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}) // Initialize the started channel
	listenAddr := "tcp://" + s.cnf.Addr()
	zap.L().Info("Starting TCP Server", zap.String("addr", listenAddr))

	// Create an error channel to capture errors from the goroutine
	errChan := make(chan error, 1)

	// Start the server asynchronously
	go func() {
		err := gnet.Run(
			s, listenAddr,
			gnet.WithMulticore(true),
			gnet.WithReusePort(true),
			gnet.WithSocketRecvBuffer(1024*64),
			gnet.WithLockOSThread(true),
			gnet.WithTicker(true),
			gnet.WithTCPNoDelay(gnet.TCPNoDelay),
		)
		if err != nil {
			errChan <- err
			return
		}
		close(errChan) // No error, close the channel
	}()

	// Wait until OnBoot sends a signal or an error occurs
	select {
	case <-s.started:
		close(s.started)
		zap.L().Info("TCP Server successfully started", zap.String("addr", listenAddr))
		return nil
	case err := <-errChan:
		if err != nil {
			return errors.Wrap(err, "failed to start TCP server")
		}
		return nil
	case <-time.After(2 * time.Second): // Wait for up to 2 seconds
		return errors.New("TCP server did not start in time")
	}
}

// OnBoot is called when the server starts
func (s *Server) OnBoot(eng gnet.Engine) (action gnet.Action) {
	s.eng = eng // Store the engine

	zap.L().Info("TCP Server is listening", zap.String("addr", s.cnf.Addr()))

	s.started <- struct{}{} // Signal that the server has started
	return gnet.None
}

// OnShutdown is called when the server is shutting down
func (s *Server) OnShutdown(eng gnet.Engine) {
	zap.L().Info("TCP Server is shutting down", zap.String("addr", s.cnf.Addr()))
}

// OnOpen is called when a new connection is opened
func (s *Server) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	return nil, gnet.None
}

// OnClose is called when a connection is closed
func (s *Server) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if err != nil && !errors.Is(err, io.EOF) {
		zap.L().Error(
			"Connection closed",
			zap.Error(err),
			zap.String("addr", c.RemoteAddr().String()),
		)
	}
	return gnet.None
}

// OnTraffic handles incoming data
func (s *Server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// Read all available data from the connection buffer
	frame, err := c.Next(-1)
	if err != nil {
		zap.L().Error("Error reading data", zap.Error(err))
		return gnet.Close
	}

	if len(frame) < 1 {
		zap.L().Warn("Invalid action received", zap.String("addr", c.RemoteAddr().String()))
		c.AsyncWrite([]byte("ERROR: Invalid action"), nil)
		return gnet.None
	}

	// Parse the action type
	actionType, err := s.parseActionType(frame)
	if err != nil {
		c.AsyncWrite([]byte("ERROR: Invalid action"), nil)
		return gnet.None
	}

	// Check if the handler exists
	handler, exists := s.handlerRegistry[actionType]
	if !exists {
		zap.L().Warn("Unknown action type", zap.Int("action_type", int(actionType)), zap.String("addr", c.RemoteAddr().String()))
		c.AsyncWrite([]byte("ERROR: Unknown action"), nil)
		return gnet.None
	}

	// Call the handler
	handler(c, frame)
	return gnet.None
}

// OnTick is called periodically by gnet
func (s *Server) OnTick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop stops the TCP server
func (s *Server) Stop() error {
	zap.L().Info("Stopping TCP Server", zap.String("addr", s.cnf.Addr()))

	err := s.eng.Stop(s.ctx)
	if err != nil {
		zap.L().Error("Error stopping TCP server", zap.Error(err))
		return err
	}

	zap.L().Info("TCP Server stopped successfully", zap.String("addr", s.cnf.Addr()))
	return nil
}

// WaitStarted returns the started channel for waiting until the server starts
func (s *Server) WaitStarted() <-chan struct{} {
	return s.started
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
func (s *Server) RegisterHandler(actionType types.HandlerType, handler TCPHandler) {
	zap.L().Debug("Registering handler", zap.Int("action_type", int(actionType)))
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *Server) DeregisterHandler(actionType types.HandlerType) {
	zap.L().Debug("Deregistering handler", zap.Int("action_type", int(actionType)))
	delete(s.handlerRegistry, actionType)
}
