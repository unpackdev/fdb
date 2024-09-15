package fdb

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"sync"
)

// Handler function type for QUIC
type QuicHandler func(sess quic.Connection, stream quic.Stream)

// QuicServer struct represents the QUIC server
type QuicServer struct {
	handlerRegistry map[HandlerType]QuicHandler
	addr            string
	tlsConfig       *tls.Config
	stopChan        chan struct{}
	started         chan struct{}
	wg              sync.WaitGroup
	listener        *quic.Listener
}

// NewQuicServer creates a new QuicServer instance
func NewQuicServer(ip string, port int, tlsConfig *tls.Config) (*QuicServer, error) {
	listenAddr := fmt.Sprintf("%s:%d", ip, port)

	server := &QuicServer{
		handlerRegistry: make(map[HandlerType]QuicHandler),
		addr:            listenAddr,
		tlsConfig:       tlsConfig,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the server address as a string.
func (s *QuicServer) Addr() string {
	return s.addr
}

// Start starts the QUIC server
func (s *QuicServer) Start() error {
	var err error
	s.listener, err = quic.ListenAddr(s.addr, s.tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to start QUIC server: %w", err)
	}

	// Signal that the server has started
	close(s.started)

	log.Printf("QUIC Server started on %s", s.addr)

	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// acceptConnections continuously accepts new QUIC connections
func (s *QuicServer) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept(context.Background())
			if err != nil {
				log.Printf("Error accepting QUIC connection: %v", err)
				continue
			}

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// handleConnection handles individual QUIC connections
func (s *QuicServer) handleConnection(conn quic.Connection) {
	defer s.wg.Done()

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			log.Printf("Error accepting QUIC stream: %v", err)
			return
		}

		s.wg.Add(1)
		go s.handleStream(conn, stream)
	}
}

// handleStream handles incoming QUIC streams
func (s *QuicServer) handleStream(conn quic.Connection, stream quic.Stream) {
	defer s.wg.Done()

	// Read the first byte to determine the action type
	buffer := make([]byte, 1)
	_, err := stream.Read(buffer)
	if err != nil {
		log.Printf("Error reading action type from stream: %v", err)
		_ = stream.Close() // Close the stream if there's an error
		return
	}

	actionType, err := s.parseActionType(buffer)
	if err != nil {
		log.Printf("Error parsing action type: %v", err)
		_ = stream.Close() // Close the stream if parsing fails
		return
	}

	// Look up the appropriate handler for this action
	handler, exists := s.handlerRegistry[actionType]
	if !exists {
		log.Printf("No handler found for action type %d", actionType)
		_ = stream.Close() // Close the stream if no handler is found
		return
	}

	// Log the action type and key (if available)
	log.Printf("Handling action type: %d", actionType)

	// Call the handler
	handler(conn, stream)

}

// Stop stops the QUIC server
func (s *QuicServer) Stop() {
	close(s.stopChan)
	s.listener.Close()
	s.wg.Wait()
}

// WaitStarted returns a channel that is closed when the server has started
func (s *QuicServer) WaitStarted() <-chan struct{} {
	return s.started
}

// parseActionType parses the action type from the first byte of the stream
func (s *QuicServer) parseActionType(frame []byte) (HandlerType, error) {
	if len(frame) < 1 {
		return 0, fmt.Errorf("invalid action: frame too short")
	}

	var actionType HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *QuicServer) RegisterHandler(actionType HandlerType, handler QuicHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *QuicServer) DeregisterHandler(actionType HandlerType) {
	delete(s.handlerRegistry, actionType)
}
