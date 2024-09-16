// Package db provides an abstraction for key-value database operations through
// the Provider interface. It defines a common interface for database interactions,
// including setting, retrieving, checking the existence of, and deleting key-value
// pairs, as well as managing the lifecycle of the database (e.g., closing and destroying).
//
// This package allows for flexible use of different database backends, such as MDBX,
// while maintaining a consistent API for interacting with the database.
//
// Key components of this package include:
//
//   - **Provider interface**: Defines the essential methods required for any key-value
//     database implementation, including Set, Get, Exists, Delete, Close, and Destroy.
//
//   - **Db struct**: A concrete implementation of the Provider interface for managing
//     MDBX-based databases. It handles opening, closing, and interacting with the MDBX
//     environment.
//
// Example usage:
//
//	// Initialize a new database using MDBX
//	db, err := NewDb(ctx, config.MdbxNode{...})
//	if err != nil {
//	    log.Fatalf("Failed to create MDBX database: %v", err)
//	}
//
//	// Set a key-value pair
//	err = db.Set([]byte("key"), []byte("value"))
//	if err != nil {
//	    log.Fatalf("Failed to set key-value pair: %v", err)
//	}
//
//	// Get a value by key
//	value, err := db.Get([]byte("key"))
//	if err != nil {
//	    log.Fatalf("Failed to get value: %v", err)
//	}
//
//	// Close the database
//	err = db.Close()
//	if err != nil {
//	    log.Fatalf("Failed to close database: %v", err)
//	}
package db
