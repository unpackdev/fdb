// pkg/shutdown/shutdown.go
package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sasha-s/go-deadlock"

	"github.com/unpackdev/fdb/pkg/logger"
)

// ShutdownTimeout is the maximum time to wait for callbacks to complete
const ShutdownTimeout = 30 * time.Second

// CallbackFn is a function that performs cleanup tasks during shutdown
type CallbackFn func() error

// Manager handles graceful shutdown of the application.
type Manager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	logger        logger.Logger
	wg            sync.WaitGroup
	signals       []os.Signal
	once          sync.Once
	callbacks     []CallbackFn
	mu            deadlock.Mutex
	timeout       time.Duration
	shutdownCause string
	shutdownErr   error
}

// NewManager creates a new ShutdownManager.
// It accepts a parent context, a logger, and optional OS signals to listen for.
func NewManager(ctx context.Context, logger logger.Logger, signals ...os.Signal) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}
	return &Manager{
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		signals:   signals,
		timeout:   ShutdownTimeout,
		callbacks: make([]CallbackFn, 0),
	}
}

// WithTimeout sets a custom timeout for the shutdown process.
func (sm *Manager) WithTimeout(timeout time.Duration) *Manager {
	sm.timeout = timeout
	return sm
}

// Context returns the context associated with the ShutdownManager.
func (sm *Manager) Context() context.Context {
	return sm.ctx
}

// AddShutdownCallback registers a callback function to be called during shutdown.
func (sm *Manager) AddShutdownCallback(callback func() error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.callbacks = append(sm.callbacks, callback)
}

// Trigger manually triggers the shutdown process with a specific cause.
func (sm *Manager) Trigger(cause string) {
	sm.mu.Lock()
	sm.shutdownCause = cause
	sm.mu.Unlock()
	sm.logger.Info("Manually triggered shutdown", "cause", cause)
	sm.cancel() // This will cause handleSignals to exit its select case
}

// Start begins listening for OS signals to initiate shutdown.
func (sm *Manager) Start() {
	sm.wg.Add(1)
	go sm.handleSignals()
}

// handleSignals listens for OS signals and initiates shutdown when received.
func (sm *Manager) handleSignals() {
	defer sm.wg.Done()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sm.signals...)

	select {
	case sig := <-sigChan:
		sm.mu.Lock()
		sm.shutdownCause = fmt.Sprintf("signal: %s", sig.String())
		sm.mu.Unlock()
		sm.logger.Info("Received shutdown signal", "signal", sig.String())
		sm.shutdown()
	case <-sm.ctx.Done():
		sm.mu.Lock()
		if sm.shutdownCause == "" {
			sm.shutdownCause = "context canceled"
		}
		cause := sm.shutdownCause
		sm.mu.Unlock()
		sm.logger.Info("Context canceled, shutting down", "cause", cause)
		sm.shutdown()
	}
}

// shutdown performs the actual shutdown sequence, ensuring it's only executed once.
func (sm *Manager) shutdown() {
	sm.once.Do(func() {
		// Cancel context first to signal all consumers
		sm.cancel()

		// Make a copy of callbacks to avoid holding the lock during execution
		sm.mu.Lock()
		callbacks := make([]CallbackFn, len(sm.callbacks))
		copy(callbacks, sm.callbacks)
		sm.mu.Unlock()

		// Create a timeout context for the entire shutdown process
		timeoutCtx, cancel := context.WithTimeout(context.Background(), sm.timeout)
		defer cancel()

		// Use waitgroup to track completion of all callbacks
		var cbWg sync.WaitGroup
		errs := make([]error, 0)
		errMu := sync.Mutex{}

		// Execute all callbacks concurrently
		for i, callback := range callbacks {
			cbWg.Add(1)
			go func(index int, cb CallbackFn) {
				defer cbWg.Done()

				// Create context with timeout for this specific callback
				cbCtx, cbCancel := context.WithTimeout(timeoutCtx, sm.timeout/2)
				defer cbCancel()

				// Use a channel to track callback completion
				done := make(chan error, 1)
				go func() {
					done <- cb()
				}()

				// Wait for either completion or timeout
				select {
				case err := <-done:
					if err != nil {
						sm.logger.Error("Shutdown callback failed", "index", index, "error", err.Error())
						errMu.Lock()
						errs = append(errs, fmt.Errorf("callback %d: %w", index, err))
						errMu.Unlock()
					}
				case <-cbCtx.Done():
					sm.logger.Error("Shutdown callback timed out", "index", index)
					errMu.Lock()
					errs = append(errs, fmt.Errorf("callback %d: timed out", index))
					errMu.Unlock()
				}
			}(i, callback)
		}

		// Wait for all callbacks with a timeout
		cbDone := make(chan struct{})
		go func() {
			cbWg.Wait()
			close(cbDone)
		}()

		select {
		case <-cbDone:
			sm.logger.Info("All shutdown callbacks completed")
		case <-timeoutCtx.Done():
			sm.logger.Error("Shutdown process timed out")
			errMu.Lock()
			errs = append(errs, fmt.Errorf("shutdown process timed out after %s", sm.timeout))
			errMu.Unlock()
		}

		// Combine all errors
		if len(errs) > 0 {
			combinedErr := fmt.Errorf("shutdown completed with %d errors", len(errs))
			for i, err := range errs {
				combinedErr = fmt.Errorf("%w\n  %d: %v", combinedErr, i, err)
			}
			sm.mu.Lock()
			sm.shutdownErr = combinedErr
			sm.mu.Unlock()
		}
	})
}

// Wait blocks until the shutdown sequence is complete.
func (sm *Manager) Wait() error {
	sm.wg.Wait()

	// Return any errors from the shutdown process
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.shutdownErr
}

// ShutdownCause returns the reason for the shutdown.
func (sm *Manager) ShutdownCause() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.shutdownCause
}
