package benchmark

import (
	"crypto/rand"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/types"
	"math"
	"time"
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

// calculateStdDev calculates the standard deviation of the given latencies.
func calculateStdDev(latencies []time.Duration) float64 {
	if len(latencies) == 0 {
		return 0.0
	}

	// Calculate the mean latency in seconds
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	mean := total.Seconds() / float64(len(latencies))

	// Calculate variance
	var variance float64
	for _, latency := range latencies {
		diff := latency.Seconds() - mean
		variance += diff * diff
	}
	variance /= float64(len(latencies))

	// Return standard deviation (square root of variance)
	return math.Sqrt(variance)
}
