package fdb

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name      string
	key       []byte
	value     []byte
	expectErr bool
}

func setupTestManager(t *testing.T) *Manager {
	ctx := context.Background()

	// Create a temporary directory for the test database
	path := "./testdb"
	err := os.Mkdir(path, 0755)
	assert.NoError(t, err)

	// Create options for the database
	opts := MdbxNodes{
		{Path: path, Name: "test"},
	}

	// Initialize Manager
	manager, err := NewManager(ctx, opts)
	assert.NoError(t, err)

	// Teardown function to clean up after the test
	return manager
}

func TestManagerDbOperations(t *testing.T) {
	manager := setupTestManager(t)
	// Get database from manager
	db, err := manager.GetDb("test")
	assert.NoError(t, err)

	defer db.Destroy()

	// Define test cases
	tests := []testCase{
		{name: "Set and Get Key", key: []byte("key1"), value: []byte("value1"), expectErr: false},
		{name: "Check Exists Key", key: []byte("key2"), value: []byte("value2"), expectErr: false},
		{name: "Delete Key", key: []byte("key3"), value: []byte("value3"), expectErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Set operation
			err := db.Set(tt.key, tt.value)
			assert.NoError(t, err)

			// Test Get operation
			value, err := db.Get(tt.key)
			assert.NoError(t, err)
			assert.Equal(t, tt.value, value)

			// Test Exists operation
			exists, err := db.Exists(tt.key)
			assert.NoError(t, err)
			assert.True(t, exists)

			// Test Delete operation
			err = db.Delete(tt.key)
			assert.NoError(t, err)

			// Test Get after Delete (should return ErrNotFound)
			_, err = db.Get(tt.key)
			assert.Error(t, err)
			assert.Equal(t, ErrNotFound, err)

			// Test Exists after Delete (should return false)
			exists, err = db.Exists(tt.key)
			assert.NoError(t, err)
			assert.False(t, exists)
		})
	}

	// Test the Destroy method to ensure database is deleted properly
	err = db.Destroy()
	assert.NoError(t, err)
}
