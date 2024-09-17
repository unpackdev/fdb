package transport_ebpf

/*
import (
	"context"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/unpackdev/fdb/config"
	"log"
)

type EbpfHandler func(frame []byte)

type EbpfServer struct {
	ctx             context.Context
	cnf             config.EbpfTransport
	handlerRegistry map[uint8]EbpfHandler // Register handlers by action type
	stopChan        chan struct{}
	started         chan struct{}
	ringBuffer      *ringbuf.Reader
}

// NewEbpfServer initializes the eBPF server with the config and context
func NewEbpfServer(ctx context.Context, cnf config.EbpfTransport) (*EbpfServer, error) {
	server := &EbpfServer{
		ctx:             ctx,
		cnf:             cnf,
		handlerRegistry: make(map[uint8]EbpfHandler),
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Start begins listening to the eBPF ring buffer for packets
func (s *EbpfServer) Start() error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}, 1)

	// Load the pinned ring buffer map (assuming it's pinned by the eBPF program)
	rb, err := ebpf.LoadPinnedMap(s.cnf.PinnedRingBuf, nil)
	if err != nil {
		return err
	}

	s.ringBuffer, err = ringbuf.NewReader(rb)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-s.stopChan:
				return
			default:
				record, err := s.ringBuffer.Read()
				if err != nil {
					if err == ringbuf.ErrClosed {
						return
					}
					log.Printf("Error reading from ring buffer: %s", err)
					continue
				}

				// Handle the packet by calling the appropriate handler
				actionType := record[0] // Assuming the first byte is the action type
				handler, exists := s.handlerRegistry[actionType]
				if exists {
					handler(record)
				} else {
					log.Printf("Unknown action type: %d", actionType)
				}
			}
		}
	}()

	// Wait for the signal to start processing
	close(s.started)
	return nil
}

// Stop gracefully stops the eBPF server
func (s *EbpfServer) Stop() {
	close(s.stopChan)
	if s.ringBuffer != nil {
		s.ringBuffer.Close()
	}
}

// RegisterHandler adds a new handler for a specific action type
func (s *EbpfServer) RegisterHandler(actionType uint8, handler EbpfHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler removes a handler for a specific action type
func (s *EbpfServer) DeregisterHandler(actionType uint8) {
	delete(s.handlerRegistry, actionType)
}*/
