package capn

import (
	"capnproto.org/go/capnp/v3"
	"context"
	"fmt"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/protocols/capn/schema"
	"go.opentelemetry.io/otel/trace"
)

const (
	// HandlerType for Cap'n Proto messages
	HandlerType = "capnp"
)

// Handler handles Cap'n Proto formatted messages
type Handler struct {
	db        db.Provider
	logger    logger.Logger
	telemetry observability.Observer
}

// NewHandler creates a new Cap'n Proto message handler
func NewHandler(dbProvider db.Provider, logger logger.Logger, telemetry observability.Observer) *Handler {
	return &Handler{
		db:        dbProvider,
		logger:    logger,
		telemetry: telemetry,
	}
}

// Type returns the handler type
func (h *Handler) Type() string {
	return HandlerType
}

// Handle processes an FDB request
func (h *Handler) Handle(ctx context.Context, message []byte) ([]byte, error) {
	var span trace.Span
	if h.telemetry != nil {
		ctx, span = h.telemetry.StartSpan(ctx, "capnp.Handle")
		defer span.End()
	}

	// Parse message
	msg, err := capnp.Unmarshal(message)
	if err != nil {
		h.logger.Error("failed to unmarshal message", "error", err)
		return CreateErrorResponse(fmt.Sprintf("failed to unmarshal message: %v", err))
	}

	request, err := schema.ReadRootRequest(msg)
	if err != nil {
		h.logger.Error("failed to read request from message", "error", err)
		return CreateErrorResponse(fmt.Sprintf("failed to read request: %v", err))
	}

	switch request.Which() {
	case schema.Request_Which_get:
		return h.handleGetRequest(ctx, request)
	case schema.Request_Which_set:
		return h.handleSetRequest(ctx, request)
	case schema.Request_Which_delete:
		return h.handleDeleteRequest(ctx, request)
	case schema.Request_Which_exists:
		return h.handleExistsRequest(ctx, request)
	default:
		h.logger.Error("unknown request type", "type", request.Which().String())
		return CreateErrorResponse(fmt.Sprintf("unknown request type: %s", request.Which().String()))
	}
}

func (h *Handler) handleGetRequest(ctx context.Context, request schema.Request) ([]byte, error) {
	get := request.Get()
	if !get.HasKey() {
		return CreateErrorResponse("missing key in get request")
	}

	key, err := get.Key()
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
	}

	value, err := h.db.Get(key)
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to get value: %v", err))
	}

	exists := value != nil
	return CreateGetResponse(value, exists)
}

func (h *Handler) handleSetRequest(ctx context.Context, request schema.Request) ([]byte, error) {
	set := request.Set()
	if !set.HasKey() {
		return CreateErrorResponse("missing key in set request")
	}
	if !set.HasValue() {
		return CreateErrorResponse("missing value in set request")
	}

	key, err := set.Key()
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
	}

	value, err := set.Value()
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to get value: %v", err))
	}

	if err := h.db.Set(key, value); err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to set value: %v", err))
	}

	return CreateSetResponse(true)
}

func (h *Handler) handleDeleteRequest(ctx context.Context, request schema.Request) ([]byte, error) {
	del := request.Delete()
	if !del.HasKey() {
		return CreateErrorResponse("missing key in delete request")
	}

	key, err := del.Key()
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
	}

	if err := h.db.Delete(key); err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to delete value: %v", err))
	}

	return CreateDeleteResponse(true)
}

func (h *Handler) handleExistsRequest(ctx context.Context, request schema.Request) ([]byte, error) {
	exists := request.Exists()
	if !exists.HasKey() {
		return CreateErrorResponse("missing key in exists request")
	}

	key, err := exists.Key()
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
	}

	doesExist, err := h.db.Exists(key)
	if err != nil {
		return CreateErrorResponse(fmt.Sprintf("failed to check if value exists: %v", err))
	}

	return CreateExistsResponse(doesExist)
}
