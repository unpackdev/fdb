// pkg/transports/tcp/server.go
package tcp

import (
	"context"

	"github.com/unpackdev/fdb/transports"

	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/types"

	"github.com/panjf2000/gnet/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type OnTrafficHandlerFn func(ctx *ConnectionContext, c gnet.Conn) (action gnet.Action)
type OnCloseHandlerFn func(ctx *ConnectionContext, c gnet.Conn)

// Server represents the TCP server.
type Server struct {
	ctx              context.Context
	handlerRegistry  map[types.HandlerType]transports.Handler
	cnf              config.TcpTransport
	stopChan         chan struct{}
	started          chan struct{}
	startedOnce      sync.Once // Ensures the channel is closed only once
	stopOnce         sync.Once // Ensures Stop is called only once
	eng              gnet.Engine
	mu               deadlock.RWMutex
	stateManager     *StateManager // Added StateManager
	onTrafficHandler OnTrafficHandlerFn
	onCloseHandler   OnCloseHandlerFn
	logger           logger.Logger
	observability    *observability.Observability

	wg sync.WaitGroup // WaitGroup to synchronize server shutdown
}

// NewServer creates a new Server instance with the provided configuration.
func NewServer(ctx context.Context, cnf config.TcpTransport, logger logger.Logger, obs *observability.Observability) (*Server, error) {
	server := &Server{
		ctx:             ctx,
		handlerRegistry: make(map[types.HandlerType]transports.Handler),
		cnf:             cnf,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}), // Buffered to prevent blocking
		logger:          logger,
		observability:   obs,
	}

	// Initialize StateManager
	server.stateManager = NewStateManager(logger, obs)
	server.stateManager.SetState(ServerStateType, Uninitialized)

	return server, nil
}

func (s *Server) SetOnTrafficHandler(handler OnTrafficHandlerFn) {
	s.onTrafficHandler = handler
}

func (s *Server) SetOnCloseHandler(handler OnCloseHandlerFn) {
	s.onCloseHandler = handler
}

// Addr returns the TCP address as a string.
func (s *Server) Addr() string {
	return s.cnf.Addr()
}

// Start initiates the TCP server with retry mechanism.
func (s *Server) Start(ctx context.Context) error {
	// Update state to Initializing
	s.stateManager.SetState(ServerStateType, Initializing)

	listenAddr := "tcp://" + s.cnf.Addr()
	s.logger.Info("Starting TCP Server", zap.String("addr", listenAddr))

	// Channel to capture errors from the goroutine
	errChan := make(chan error, 1)

	// Prepare gnet options
	options := []gnet.Option{
		gnet.WithMulticore(true),
		gnet.WithSocketRecvBuffer(1024 * 64),
		gnet.WithLockOSThread(true),
		gnet.WithTicker(true),
		//gnet.WithTCPNoDelay(1),
		gnet.WithTicker(true),
		// Standard TCP behaviour allows TIME_WAIT when stopping the service period...
		// This can be up to a few minutes. If we stop the server and attempt to start it again
		// without this line, it will fail to start the server. This option allows to forcibly bind
		// to a port in use in the TIME_WAIT state, bypassing the wait time...
		gnet.WithReuseAddr(true),
	}

	// If TLS is configured, add the TLS config
	tlsConfig, err := s.cnf.GetTLSConfig()
	if err != nil {
		s.stateManager.SetState(ServerStateType, Failed)
		return errors.Wrap(err, "failed to get TLS config")
	}
	_ = tlsConfig
	/*
	   if tlsConfig != nil {
	       options = append(options, gnet.WithTLSConfig(&tls.Config{
	           InsecureSkipVerify: tlsConfig.Insecure,
	           Certificates:       tlsConfig.Certificates,
	           RootCAs:            tlsConfig.RootCAs,
	       }))
	       s.logger.Info("TLS is enabled for TCP Server")
	   }
	*/

	// Attempt to start the server with retries
	maxRetries := 3
	retryInterval := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.logger.Info("Attempting to start TCP Server", zap.Int("attempt", attempt))

		// Increment WaitGroup before starting the server goroutine
		s.wg.Add(1)
		// Start the server asynchronously
		go func() {
			defer s.wg.Done() // Decrement WaitGroup when goroutine exits

			// Update state to Starting
			s.stateManager.SetState(ServerStateType, Starting)

			err := gnet.Run(
				s,
				listenAddr,
				options...,
			)
			if err != nil {
				errChan <- err
				return
			}
			close(errChan) // No error, close the channel
		}()

		// Wait until OnBoot signals or an error occurs
		select {
		case <-s.started:
			s.logger.Info("TCP Server successfully started", zap.String("addr", listenAddr))
			// Update state to Started
			s.stateManager.SetState(ServerStateType, Started)
			return nil
		case err := <-errChan:
			if err != nil {
				s.stateManager.SetState(ServerStateType, Failed)
				s.logger.Error("Failed to start TCP Server", zap.Error(err))
				// If max retries not reached, wait and retry
				if attempt < maxRetries {
					s.logger.Info("Retrying to start TCP Server", zap.Int("attempt", attempt+1))
					time.Sleep(retryInterval)
					continue
				}
				return errors.Wrap(err, "failed to start TCP server after retries")
			}
			return nil
		case <-time.After(5 * time.Second): // Increased timeout to 5 seconds
			s.stateManager.SetState(ServerStateType, Failed)
			s.logger.Error("TCP server did not start in time", zap.String("addr", listenAddr))
			// If max retries not reached, wait and retry
			if attempt < maxRetries {
				s.logger.Info("Retrying to start TCP Server", zap.Int("attempt", attempt+1))
				time.Sleep(retryInterval)
				continue
			}
			return errors.New("TCP server did not start in time after retries")
		}
	}

	return errors.New("TCP server failed to start")
}

// OnBoot is called when the server starts.
func (s *Server) OnBoot(eng gnet.Engine) (action gnet.Action) {
	s.eng = eng // Store the engine

	s.logger.Info("TCP Server is listening", zap.String("addr", s.cnf.Addr()))

	// Signal that the server has started by closing the channel
	s.startedOnce.Do(func() {
		close(s.started)
	})

	return gnet.None
}

// OnShutdown is called when the server is shutting down.
func (s *Server) OnShutdown(eng gnet.Engine) {
	s.logger.Info("TCP Server is shutting down", zap.String("addr", s.cnf.Addr()))
	// Update state to Stopping
	s.stateManager.SetState(ServerStateType, Stopping)
	// Further shutdown logic if necessary
}

// OnOpen is called when a new connection is opened.
func (s *Server) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	s.logger.Debug("Connection opened", zap.String("remote_addr", c.RemoteAddr().String()))

	// Initialize connection context with a cancellable context
	ctx, cancel := context.WithCancel(s.ctx)
	connCtx := &ConnectionContext{
		Buffer: make([]byte, 0),
		Conn:   NewTCPConnection(c),
		Ctx:    ctx,
		Cancel: cancel,
	}
	c.SetContext(connCtx)

	return nil, gnet.None
}

// OnClose is called when a connection is closed.
func (s *Server) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) ||
			strings.Contains(err.Error(), "closed") ||
			strings.Contains(err.Error(), "connection reset by peer") {
			s.logger.Debug("Connection closed by client", zap.String("addr", c.RemoteAddr().String()))
		} else {
			s.logger.Error(
				"Connection closed with error",
				zap.Error(err),
				zap.String("addr", c.RemoteAddr().String()),
			)
		}
	} else {
		s.logger.Info("Connection closed", zap.String("addr", c.RemoteAddr().String()))
	}

	// Retrieve the connection context
	ctx, ok := c.Context().(*ConnectionContext)
	if ok && s.onCloseHandler != nil {
		s.onCloseHandler(ctx, c)
	}

	// Cancel the connection's context
	if ctx != nil {
		ctx.Cancel()
	}

	return gnet.None
}

// OnTraffic handles incoming data.
func (s *Server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// Retrieve the connection context
	ctx, ok := c.Context().(*ConnectionContext)
	if !ok {
		s.logger.Error("Failed to retrieve connection context")
		return gnet.Close
	}

	//s.logger.Debug("On traffic reached...")

	// In case that on traffic handler is set, bypass the entire packet handling bellow.
	// Bellow custom packet is expected (Protocol+PacketLength+Data) where this WILL NOT
	// be the case with HTTP, WebSocket, etc...
	if s.onTrafficHandler != nil {
		return s.onTrafficHandler(ctx, c)
	}

	// Read available data
	data, err := c.Next(-1)
	if err != nil {
		if err != io.EOF {
			s.logger.Error("Error reading data", zap.Error(err))
			return gnet.Close
		}
		return gnet.None
	}

	// Parse the action type
	handlerType, err := s.parseFrameType(data)
	if err != nil {
		c.AsyncWrite([]byte("ERROR: Invalid action"), nil)
		return gnet.None
	}

	s.logger.Debug("ON TRAFFIC REACHED...", "handler", handlerType, "data", data)

	// Retrieve the handler
	s.mu.RLock()
	handler, exists := s.handlerRegistry[handlerType]
	s.mu.RUnlock()
	if !exists {
		s.logger.Warn("Handler not registered", zap.String("handler_type", handlerType.String()))
		c.AsyncWrite([]byte("ERROR: Handler not registered"), nil)
		return gnet.None
	}

	// Extract the frame (excluding the ProtocolType byte)
	frame := data[1:]

	// Dispatch to the handler
	handler.Handle(ctx.Conn, frame)
	return gnet.None
}

// OnTick is called periodically by gnet.
func (s *Server) OnTick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		// Update state to Stopped
		s.stateManager.SetState(ServerStateType, Stopped)
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop gracefully stops the TCP server with retries.
func (s *Server) Stop() error {
	var stopErr error

	s.stopOnce.Do(func() {
		s.logger.Info("Stopping TCP Server", zap.String("addr", s.cnf.Addr()))
		s.stateManager.SetState(ServerStateType, Stopping)

		maxRetries := 3
		retryInterval := 500 * time.Millisecond

		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Create a new context for stopping the server
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			err := s.eng.Stop(stopCtx)
			if err != nil {
				s.logger.Error("Error stopping TCP server", zap.Error(err))
				// If max retries not reached, wait and retry
				if attempt < maxRetries {
					s.logger.Info("Retrying to stop TCP Server", zap.Int("attempt", attempt+1))
					time.Sleep(retryInterval)
					cancel()
					continue
				}
				// Update state to Failed if stopping fails
				s.stateManager.SetState(ServerStateType, Failed)
				stopErr = err
				cancel()
				break
			}

			// Wait for the server goroutine to finish
			s.wg.Wait()
			cancel()

			// Close the stop channel to indicate the server is stopping
			select {
			case <-s.stopChan:
				// Already closed
			default:
				close(s.stopChan)
			}

			s.logger.Info("TCP Server stopped successfully", zap.String("addr", s.cnf.Addr()))
			// Update state to Stopped
			s.stateManager.SetState(ServerStateType, Stopped)
			return
		}
	})

	return stopErr
}

// WaitStarted returns a channel that is closed when the server starts.
func (s *Server) WaitStarted() <-chan struct{} {
	return s.started
}

// RegisterHandler registers a handler for a specific protocol type.
func (s *Server) RegisterHandler(protocolType types.HandlerType, handler transports.Handler) {
	s.logger.Debug("Registering handler", zap.String("protocol_type", protocolType.String()))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlerRegistry[protocolType] = handler
}

// DeregisterHandler removes a handler for a specific protocol type.
func (s *Server) DeregisterHandler(protocolType types.HandlerType) {
	s.logger.Debug("Deregistering handler", zap.String("protocol_type", protocolType.String()))
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.handlerRegistry, protocolType)
}

// parseActionType parses the action type from the frame
func (s *Server) parseFrameType(frame []byte) (types.HandlerType, error) {
	if len(frame) < 1 {
		return 0, errors.New("invalid frame: frame too short")
	}

	var actionType types.HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}
