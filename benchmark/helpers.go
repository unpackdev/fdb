package benchmark

import (
	"crypto/rand"
	"github.com/unpackdev/fdb"
)

// createWriteMessage generates a random write message
func createWriteMessage() fdb.Message {
	var key [32]byte
	_, _ = rand.Read(key[:])
	return fdb.Message{
		Handler: fdb.WriteHandlerType,
		Key:     key,
		Data:    []byte("benchmark test data"),
	}
}

// createReadMessage generates a read message for a given key
func createReadMessage(key [32]byte) fdb.Message {
	return fdb.Message{
		Handler: fdb.ReadHandlerType,
		Key:     key,
	}
}
