package capn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unpackdev/fdb/pkg/db"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
)

func TestCapnProtoHandler(t *testing.T) {
	// Setup
	ctx := context.Background()
	log := logger.NewNoOpLogger()
	mockDb := db.NewMockDB()
	telemetry := observability.NewNoopObserver()
	handler := NewHandler(mockDb, log, telemetry)

	t.Run("Get_NonExistentKey", func(t *testing.T) {
		// Create a get request for a non-existent key
		key := []byte("non-existent-key")
		reqData, err := CreateGetRequest(key)
		require.NoError(t, err)

		// Process the request
		respData, err := handler.Handle(ctx, reqData)
		require.NoError(t, err)

		// Parse the response
		resp, err := ParseResponse(respData)
		require.NoError(t, err)

		// Verify response
		value, exists, err := ParseGetResponse(resp)
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("Set_And_Get", func(t *testing.T) {
		// Set a key-value pair
		key := []byte("test-key")
		value := []byte("test-value")

		reqData, err := CreateSetRequest(key, value)
		require.NoError(t, err)

		respData, err := handler.Handle(ctx, reqData)
		require.NoError(t, err)

		resp, err := ParseResponse(respData)
		require.NoError(t, err)

		success, err := ParseSetResponse(resp)
		require.NoError(t, err)
		assert.True(t, success)

		// Now get the key and verify
		reqData, err = CreateGetRequest(key)
		require.NoError(t, err)

		respData, err = handler.Handle(ctx, reqData)
		require.NoError(t, err)

		resp, err = ParseResponse(respData)
		require.NoError(t, err)

		retrievedValue, exists, err := ParseGetResponse(resp)
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, value, retrievedValue)
	})

	t.Run("Exists", func(t *testing.T) {
		// Set a key first
		key := []byte("exists-test-key")
		value := []byte("exists-test-value")

		reqData, err := CreateSetRequest(key, value)
		require.NoError(t, err)

		_, err = handler.Handle(ctx, reqData)
		require.NoError(t, err)

		// Check if key exists
		reqData, err = CreateExistsRequest(key)
		require.NoError(t, err)

		respData, err := handler.Handle(ctx, reqData)
		require.NoError(t, err)

		resp, err := ParseResponse(respData)
		require.NoError(t, err)

		exists, err := ParseExistsResponse(resp)
		require.NoError(t, err)
		assert.True(t, exists)

		// Check if non-existent key exists
		reqData, err = CreateExistsRequest([]byte("non-existent-key"))
		require.NoError(t, err)

		respData, err = handler.Handle(ctx, reqData)
		require.NoError(t, err)

		resp, err = ParseResponse(respData)
		require.NoError(t, err)

		exists, err = ParseExistsResponse(resp)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		// Set a key first
		key := []byte("delete-test-key")
		value := []byte("delete-test-value")

		reqData, err := CreateSetRequest(key, value)
		require.NoError(t, err)

		_, err = handler.Handle(ctx, reqData)
		require.NoError(t, err)

		// Delete the key
		reqData, err = CreateDeleteRequest(key)
		require.NoError(t, err)

		respData, err := handler.Handle(ctx, reqData)
		require.NoError(t, err)

		resp, err := ParseResponse(respData)
		require.NoError(t, err)

		success, err := ParseDeleteResponse(resp)
		require.NoError(t, err)
		assert.True(t, success)

		// Verify key is deleted
		reqData, err = CreateExistsRequest(key)
		require.NoError(t, err)

		respData, err = handler.Handle(ctx, reqData)
		require.NoError(t, err)

		resp, err = ParseResponse(respData)
		require.NoError(t, err)

		exists, err := ParseExistsResponse(resp)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}
