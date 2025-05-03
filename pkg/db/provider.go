package db

// Provider defines the interface for a key-value database provider. This interface
// abstracts the basic operations for interacting with a database, allowing for
// different implementations (e.g., MDBX, BoltDB) to conform to the same interface
// and be used interchangeably in the application.
//
// The Provider interface includes the following methods for key-value operations:
// - Set: Store a key-value pair in the database.
// - Get: Retrieve the value associated with a key.
// - Exists: Check if a key exists in the database.
// - Delete: Remove a key-value pair from the database.
// - Close: Close the database connection.
// - Destroy: Permanently remove the database files and environment.
type Provider interface {

	// Set stores a key-value pair in the database. If the key already exists,
	// the value is updated.
	//
	// Example usage:
	//   err := provider.Set([]byte("key"), []byte("value"))
	//   if err != nil {
	//       log.Fatalf("Failed to set key-value pair: %v", err)
	//   }
	//
	// Parameters:
	//   key ([]byte): The key to store in the database.
	//   value ([]byte): The value associated with the key.
	//
	// Returns:
	//   error: Returns an error if the key-value pair cannot be stored.
	Set(key, value []byte) error

	// Get retrieves the value associated with the given key from the database.
	//
	// Example usage:
	//   value, err := provider.Get([]byte("key"))
	//   if err != nil {
	//       log.Fatalf("Failed to retrieve value: %v", err)
	//   }
	//
	// Parameters:
	//   key ([]byte): The key to retrieve the value for.
	//
	// Returns:
	//   []byte: The value associated with the key.
	//   error: Returns an error if the key is not found or the retrieval fails.
	Get(key []byte) ([]byte, error)

	// Exists checks if the specified key exists in the database.
	//
	// Example usage:
	//   exists, err := provider.Exists([]byte("key"))
	//   if err != nil {
	//       log.Fatalf("Failed to check key existence: %v", err)
	//   }
	//
	// Parameters:
	//   key ([]byte): The key to check for existence.
	//
	// Returns:
	//   bool: True if the key exists, false otherwise.
	//   error: Returns an error if the existence check fails.
	Exists(key []byte) (bool, error)

	// Delete removes a key-value pair from the database.
	//
	// Example usage:
	//   err := provider.Delete([]byte("key"))
	//   if err != nil {
	//       log.Fatalf("Failed to delete key-value pair: %v", err)
	//   }
	//
	// Parameters:
	//   key ([]byte): The key to remove from the database.
	//
	// Returns:
	//   error: Returns an error if the key cannot be deleted.
	Delete(key []byte) error

	// Close gracefully closes the database, releasing any resources held by the
	// database environment.
	//
	// Example usage:
	//   err := provider.Close()
	//   if err != nil {
	//       log.Fatalf("Failed to close database: %v", err)
	//   }
	//
	// Returns:
	//   error: Returns an error if the database cannot be closed properly.
	Close() error

	// Destroy permanently removes the database files and cleans up the environment.
	//
	// Example usage:
	//   err := provider.Destroy()
	//   if err != nil {
	//       log.Fatalf("Failed to destroy database: %v", err)
	//   }
	//
	// Returns:
	//   error: Returns an error if the database files cannot be removed or if there
	//   is an issue cleaning up the environment.
	Destroy() error
}
