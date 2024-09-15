package benchmark

import (
	"crypto/rand"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/types"
)

// createWriteMessage generates a random write message
func createWriteMessage() messages.Message {
	var key [32]byte
	_, _ = rand.Read(key[:])
	return messages.Message{
		Handler: types.WriteHandlerType,
		Key:     key,
		Data:    []byte("benchmark test data"),
	}
}

// createReadMessage generates a read message for a given key
func createReadMessage(key [32]byte) messages.Message {
	return messages.Message{
		Handler: types.ReadHandlerType,
		Key:     key,
	}
}
