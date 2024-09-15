package errors

import "errors"

var (
	// ErrNotFound is returned when a key is not found in the database
	ErrNotFound = errors.New("key not found")
)
