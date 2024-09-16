package db

import (
	"context"
	"github.com/erigontech/mdbx-go/mdbx"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"os"
)

// Db represents a wrapper around an MDBX database environment. It manages
// the MDBX environment, database instance (DBI), and provides methods to interact
// with the database, such as setting, getting, deleting key-value pairs, and closing
// or destroying the database.
type Db struct {
	// ctx is the context used to manage the lifecycle of the MDBX database.
	ctx context.Context

	// opts holds the MDBX node configuration, including the file path, size limits,
	// growth step, and file permissions.
	opts config.MdbxNode

	// env is the MDBX environment handle, which is used to manage transactions and operations
	// on the database.
	env *mdbx.Env

	// dbi is the MDBX database instance handle used for interacting with the database.
	dbi mdbx.DBI
}

// NewDb creates a new MDBX database environment based on the provided configuration.
// It sets the database geometry (min size, max size, growth step), maximum readers,
// and file permissions. The function returns a Provider interface to allow for interaction
// with the database.
//
// Example usage:
//
//	db, err := NewDb(ctx, config.MdbxNode{...})
//	if err != nil {
//	    log.Fatalf("Failed to create MDBX database: %v", err)
//	}
//
// Parameters:
//
//	ctx (context.Context): The context for managing the lifecycle of the database.
//	opts (config.MdbxNode): The configuration options for the MDBX database.
//
// Returns:
//
//	Provider: A new MDBX database provider for interacting with the database.
//	error: Returns an error if the environment or database creation fails.
func NewDb(ctx context.Context, opts config.MdbxNode) (Provider, error) {
	env, err := mdbx.NewEnv()
	if err != nil {
		return nil, err
	}

	// Set database geometry (size limits and growth step)
	maxSize := int(opts.MaxSize * 1024 * 1024 * 1024) // Convert from GB to bytes * MaxSize
	minSize := int(opts.MinSize * 1024 * 1024 * 1024) // Convert from MB to bytes * MinSize
	growthStep := int(opts.GrowthStep)                // Growth step in bytes

	err = env.SetGeometry(minSize, -1, maxSize, -1, -1, growthStep)
	if err != nil {
		return nil, err
	}

	// Set the maximum number of readers
	if soErr := env.SetOption(mdbx.OptMaxReaders, uint64(opts.MaxReaders)); soErr != nil {
		return nil, soErr
	}

	// Open the environment with the specified file permissions

	if eoErr := env.Open(opts.Path, mdbx.Create, os.FileMode(opts.FilePermissions)); eoErr != nil {
		return nil, eoErr
	}

	// Open the database within the environment
	var dbi mdbx.DBI
	err = env.Update(func(txn *mdbx.Txn) error {
		dbi, err = txn.OpenRoot(mdbx.Create)
		return err
	})
	if err != nil {
		env.Close()
		return nil, err
	}

	return &Db{ctx: ctx, opts: opts, env: env, dbi: dbi}, nil
}

// Destroy removes the MDBX database files and cleans up the environment. This method
// first closes the database environment and then deletes the database files.
//
// Example usage:
//
//	err := db.Destroy()
//	if err != nil {
//	    log.Fatalf("Failed to destroy MDBX database: %v", err)
//	}
//
// Returns:
//
//	error: Returns an error if the database files cannot be removed or the environment fails to close.
func (db *Db) Destroy() error {
	// Close the environment before deleting the database files
	err := db.Close()
	if err != nil {
		return err
	}

	// Remove the database files
	err = os.RemoveAll(db.opts.Path)
	if err != nil {
		return errors.Wrap(err, "failed to remove database files")
	}
	return nil
}

// GetEnv returns the MDBX environment associated with this database instance.
//
// Example usage:
//
//	env := db.GetEnv()
//
// Returns:
//
//	*mdbx.Env: The MDBX environment handle.
func (db *Db) GetEnv() *mdbx.Env {
	return db.env
}

// GetDBI returns the MDBX database instance handle.
//
// Example usage:
//
//	dbi := db.GetDBI()
//
// Returns:
//
//	mdbx.DBI: The MDBX database instance handle.
func (db *Db) GetDBI() mdbx.DBI {
	return db.dbi
}

// Set stores a key-value pair in the MDBX database. This method starts a transaction
// to insert or update the value associated with the given key.
//
// Example usage:
//
//	err := db.Set([]byte("key"), []byte("value"))
//	if err != nil {
//	    log.Fatalf("Failed to set key-value pair: %v", err)
//	}
//
// Parameters:
//
//	key ([]byte): The key to store in the database.
//	value ([]byte): The value to associate with the key.
//
// Returns:
//
//	error: Returns an error if the key-value pair cannot be stored.
func (db *Db) Set(key, value []byte) error {
	return db.env.Update(func(txn *mdbx.Txn) error {
		cursor, err := txn.OpenCursor(db.GetDBI())
		if err != nil {
			return errors.Wrap(err, "failed to open cursor")
		}
		defer cursor.Close()

		return cursor.Put(key, value, 0)
	})
}

// Get retrieves the value associated with the given key from the MDBX database.
//
// Example usage:
//
//	value, err := db.Get([]byte("key"))
//	if err != nil {
//	    log.Fatalf("Failed to get value: %v", err)
//	}
//
// Parameters:
//
//	key ([]byte): The key to retrieve the value for.
//
// Returns:
//
//	[]byte: The value associated with the key.
//	error: Returns an error if the key is not found or the retrieval fails.
func (db *Db) Get(key []byte) ([]byte, error) {
	var value []byte
	err := db.env.View(func(txn *mdbx.Txn) error {
		var err error
		value, err = txn.Get(db.dbi, key)
		return err
	})
	return value, err
}

// Exists checks if a key exists in the MDBX database.
//
// Example usage:
//
//	exists, err := db.Exists([]byte("key"))
//	if err != nil {
//	    log.Fatalf("Failed to check key existence: %v", err)
//	}
//
// Parameters:
//
//	key ([]byte): The key to check for existence.
//
// Returns:
//
//	bool: True if the key exists, false otherwise.
//	error: Returns an error if the existence check fails.
func (db *Db) Exists(key []byte) (bool, error) {
	err := db.env.View(func(txn *mdbx.Txn) error {
		_, err := txn.Get(db.dbi, key)
		return err
	})

	if err == nil {
		return true, nil
	} else if errors.Is(err, mdbx.ErrNotFound) {
		return false, nil
	}

	return false, err
}

// Delete removes a key-value pair from the MDBX database.
//
// Example usage:
//
//	err := db.Delete([]byte("key"))
//	if err != nil {
//	    log.Fatalf("Failed to delete key-value pair: %v", err)
//	}
//
// Parameters:
//
//	key ([]byte): The key to remove from the database.
//
// Returns:
//
//	error: Returns an error if the key cannot be deleted.
func (db *Db) Delete(key []byte) error {
	return db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Del(db.dbi, key, nil)
	})
}

// Close closes the MDBX environment and releases any resources held by the database.
//
// Example usage:
//
//	err := db.Close()
//	if err != nil {
//	    log.Fatalf("Failed to close MDBX database: %v", err)
//	}
//
// Returns:
//
//	error: Returns an error if the environment cannot be closed.
func (db *Db) Close() error {
	db.env.Close()
	return nil
}
