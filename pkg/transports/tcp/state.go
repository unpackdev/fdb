// pkg/transports/tcp/state.go
package tcp

import (
	"context"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"

	"time"

	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// StateType represents a type of service or component state that the StateManager tracks.
type StateType string

func (s StateType) String() string {
	return string(s)
}

// Define service and component state types.
const (
	ServerStateType StateType = "server"
)

// State represents the current state of a particular node component type.
type State int

// Define various states a service or component can be in.
const (
	Uninitialized State = iota // The service has not been initialized yet.
	Initializing               // The service is currently in the process of initialization.
	Initialized                // The service has been successfully initialized.
	Starting                   // The service is in the process of starting.
	Started                    // The service has started successfully and is running.
	Failed                     // The service failed to initialize or start.
	Stopping
	Stopped // The service has been stopped.
)

// String returns a string representation of the service state.
func (s State) String() string {
	switch s {
	case Uninitialized:
		return "uninitialized"
	case Initializing:
		return "initializing"
	case Initialized:
		return "initialized"
	case Starting:
		return "starting"
	case Started:
		return "started"
	case Failed:
		return "failed"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// StateManager manages and tracks the state of services or components.
type StateManager struct {
	mu                deadlock.RWMutex             // Mutex to protect state changes.
	logger            logger.Logger                // Logger for recording state transitions and events.
	obs               *observability.Observability // Observability instance for emitting metrics.
	serviceStates     map[StateType]State          // Map to track the current state of each service or component.
	waitChans         map[StateType]chan struct{}  // Channels to signal state transitions for services.
	serviceStateGauge metric.Int64ObservableGauge  // Observable gauge metric for tracking service states.
}

// NewStateManager creates a new StateManager instance with the provided logger and observability.
func NewStateManager(logger logger.Logger, obs *observability.Observability) *StateManager {
	manager := &StateManager{
		logger:        logger,
		obs:           obs,
		serviceStates: make(map[StateType]State),
		waitChans:     make(map[StateType]chan struct{}),
	}

	// Initialize metrics for the service state manager.
	manager.initMetrics()
	return manager
}

// initMetrics initializes the OpenTelemetry metrics for the StateManager.
func (sm *StateManager) initMetrics() {
	if sm.obs != nil && sm.obs.Meter != nil {
		// Create an Int64ObservableGauge for tracking service states.
		gauge, err := sm.obs.Meter.Int64ObservableGauge(
			"node.service_state",
			metric.WithDescription("Tracks the current state of each service or component"),
		)
		if err != nil {
			sm.logger.Error("Failed to create observable gauge for service states", zap.Error(err))
			return
		}
		sm.serviceStateGauge = gauge

		// Register a callback to update the gauge value for each service state.
		_, err = sm.obs.Meter.RegisterCallback(
			sm.recordServiceStatesCallback, // Correctly reference the callback method here.
			sm.serviceStateGauge,
		)
		if err != nil {
			sm.logger.Error("Failed to register callback for tracking service states", zap.Error(err))
		}
	}
}

// recordServiceStatesCallback is the callback function that updates the gauge values for service states.
func (sm *StateManager) recordServiceStatesCallback(ctx context.Context, observer metric.Observer) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for service, state := range sm.serviceStates {
		observer.ObserveInt64(sm.serviceStateGauge, int64(state), metric.WithAttributes(
			attribute.String("service_type", string(service)),
			attribute.String("state", state.String()),
		))
	}
	return nil
}

// SetState updates the state of a service or component, logs the change, and emits a metric if applicable.
func (sm *StateManager) SetState(stateType StateType, state State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Retrieve the previous state for logging and comparison.
	previousState, exists := sm.serviceStates[stateType]
	if !exists {
		previousState = Uninitialized // Default to Uninitialized if the service does not exist.
	}

	// Update the state in the serviceStates map.
	sm.serviceStates[stateType] = state

	// Log the state change.
	sm.logger.Info("Node state changed",
		zap.String("state_type", string(stateType)),
		zap.String("previous_state", previousState.String()),
		zap.String("new_state", state.String()),
	)

	// Emit a state change metric if the observability instance is available.
	if sm.obs != nil {
		sm.obs.RecordServiceStateChange(context.Background(), stateType.String(), state.String())
	}

	// Notify any goroutines waiting on this state change if the service reached the `Started` state.
	if state == Started {
		if ch, exists := sm.waitChans[stateType]; exists {
			close(ch)
			delete(sm.waitChans, stateType) // Remove the channel once notification is complete.
		}
	}
}

// GetState retrieves the current state of a specified service or component.
func (sm *StateManager) GetState(stateType StateType) State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.serviceStates[stateType]
}

// WaitForState waits for a service or component to reach a desired state within a given timeout period.
// Returns an error if the timeout expires before the desired state is reached.
func (sm *StateManager) WaitForState(stateType StateType, desiredState State, timeout time.Duration) error {
	sm.mu.Lock()
	state, exists := sm.serviceStates[stateType]

	// If the service is not yet tracked, create a new wait channel.
	if !exists {
		sm.logger.Debug("Node component not found in state map. Waiting for the service to initialize.",
			zap.String("state_type", stateType.String()),
			zap.String("desired_state", desiredState.String()),
		)

		// Create a new channel to wait for the service to be added and updated.
		ch := make(chan struct{})
		sm.waitChans[stateType] = ch
		sm.mu.Unlock()

		// Wait for the service to be added to the state map or for the timeout to expire.
		select {
		case <-ch:
			// Once the service is added, recheck its state.
			sm.mu.RLock()
			state = sm.serviceStates[stateType]
			sm.mu.RUnlock()
		case <-time.After(timeout):
			sm.logger.Error("Node component did not appear in state map within the given timeout. It may not have been started.",
				zap.String("service_type", string(stateType)),
				zap.String("desired_state", desiredState.String()),
				zap.Duration("timeout", timeout),
			)
			return errors.Errorf("node component %s did not appear within %v", stateType, timeout)
		}
	} else {
		sm.mu.Unlock()
	}

	// If the service is already in the desired state, return immediately.
	if state == desiredState {
		sm.logger.Debug("Node component already in the desired state.",
			zap.String("state_type", string(stateType)),
			zap.String("state", desiredState.String()),
		)
		return nil
	}

	// Create a new wait channel if it does not already exist.
	sm.mu.Lock()
	ch, exists := sm.waitChans[stateType]
	if !exists {
		ch = make(chan struct{})
		sm.waitChans[stateType] = ch
	}
	sm.mu.Unlock()

	// Wait for the state to change or for the timeout to expire.
	sm.logger.Debug("Waiting for service to reach the desired state.",
		zap.String("service_type", string(stateType)),
		zap.String("desired_state", desiredState.String()),
		zap.Duration("timeout", timeout),
	)

	select {
	case <-ch:
		sm.logger.Debug("Service has reached the desired state.",
			zap.String("service_type", string(stateType)),
			zap.String("desired_state", desiredState.String()),
		)
		return nil // Desired state has been reached.
	case <-time.After(timeout):
		sm.logger.Error("Service did not reach the desired state within the given timeout.",
			zap.String("service_type", string(stateType)),
			zap.String("desired_state", desiredState.String()),
			zap.Duration("timeout", timeout),
		)
		return errors.Errorf("service %s did not reach state %s within %v", stateType, desiredState, timeout)
	}
}
