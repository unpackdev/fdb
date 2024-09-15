package fdb

import (
	"context"
	"github.com/erigontech/mdbx-go/mdbx"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/pkg/config"
	"os"
)

type Db struct {
	ctx  context.Context
	opts config.MdbxNode
	env  *mdbx.Env
	dbi  mdbx.DBI
}

func NewDb(ctx context.Context, opts config.MdbxNode) (Provider, error) {
	env, err := mdbx.NewEnv()
	if err != nil {
		return nil, err
	}

	// Set geometry (mapsize, etc.) with an upper limit of 10TB
	err = env.SetGeometry(-1, -1, 1024*1024*1024*1024, -1, -1, 4096)
	if err != nil {
		return nil, err
	}

	if err := env.SetOption(mdbx.OptMaxReaders, 4096); err != nil {
		return nil, err
	}

	// Open the environment
	err = env.Open(opts.Path, 0, 0664)

	if err != nil {
		return nil, err
	}

	var dbi mdbx.DBI
	err = env.Update(func(txn *mdbx.Txn) (err error) {
		dbi, err = txn.OpenRoot(mdbx.Create)
		return err
	})

	if err != nil {
		env.Close()
		return nil, err
	}

	return &Db{ctx: ctx, opts: opts, env: env, dbi: dbi}, nil
}

// Destroy removes the database files and cleans up the environment
func (db *Db) Destroy() error {
	// Close the environment before deleting the database
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

func (db *Db) GetEnv() *mdbx.Env {
	return db.env
}

func (db *Db) GetDBI() mdbx.DBI {
	return db.dbi
}

// Set stores a key-value pair in the database
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

// Get retrieves a value by key from the database
func (db *Db) Get(key []byte) ([]byte, error) {
	var value []byte
	err := db.env.View(func(txn *mdbx.Txn) error {
		var err error
		value, err = txn.Get(db.dbi, key)
		return err
	})
	return value, err
}

// Exists checks if a key exists in the database
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

// Delete removes a key-value pair from the database
func (db *Db) Delete(key []byte) error {
	return db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Del(db.dbi, key, nil)
	})
}

// Close closes the database environment
func (db *Db) Close() error {
	db.env.Close()
	return nil
}
