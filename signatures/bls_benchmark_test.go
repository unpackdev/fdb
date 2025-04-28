// bls_benchmark_test.go
package signatures

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Constants for the benchmarks
const (
	preGeneratedKeys     = 1000 // Number of keys to pre-generate for concurrent benchmarks
	preGeneratedMessages = 1000 // Number of messages to pre-generate
)

// generateSampleData generates a sample message of given size.
func generateSampleData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

// BenchmarkBLSKeyPair_GenerateKey benchmarks the key pair generation.
func BenchmarkBLSKeyPair_GenerateKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		kp := &BLSKeyPair{}
		err := kp.GenerateKey()
		if err != nil {
			b.Fatalf("Failed to generate key pair: %v", err)
		}
	}
}

// BenchmarkBLSSigner_Sign benchmarks the signing operation.
func BenchmarkBLSSigner_Sign(b *testing.B) {
	// Initialize a BLSSigner
	signer, err := NewBLSSigner()
	require.NoError(b, err, "Failed to initialize BLSSigner")

	// Pre-generate a sample message
	message := generateSampleData(256) // 256 bytes message

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := signer.Sign(message)
		if err != nil {
			b.Fatalf("Failed to sign data: %v", err)
		}
	}
}

// BenchmarkBLSSigner_Verify benchmarks the verification operation.
func BenchmarkBLSSigner_Verify(b *testing.B) {
	// Initialize a BLSSigner
	signer, err := NewBLSSigner()
	require.NoError(b, err, "Failed to initialize BLSSigner")

	// Pre-generate a sample message and its signature
	message := generateSampleData(256) // 256 bytes message
	signature, err := signer.Sign(message)
	require.NoError(b, err, "Failed to sign data")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		valid, err := signer.Verify(message, signature)
		if err != nil {
			b.Fatalf("Failed to verify signature: %v", err)
		}
		if !valid {
			b.Fatalf("Signature verification failed")
		}
	}
}

// BenchmarkAggregateSignatures benchmarks the signature aggregation process.
func BenchmarkAggregateSignatures(b *testing.B) {
	// Number of signatures to aggregate
	numSignatures := 100

	// Initialize multiple BLSSigners
	signers := make([]*BLSSigner, numSignatures)
	for i := 0; i < numSignatures; i++ {
		signer, err := NewBLSSigner()
		require.NoError(b, err, "Failed to initialize BLSSigner")
		signers[i] = signer
	}

	// Pre-generate sample messages and their signatures
	messages := make([][]byte, numSignatures)
	signatures := make([][]byte, numSignatures)
	for i := 0; i < numSignatures; i++ {
		messages[i] = generateSampleData(256) // 256 bytes message
		signature, err := signers[i].Sign(messages[i])
		require.NoError(b, err, "Failed to sign data")
		signatures[i] = signature
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Aggregate all signatures
		aggSig, err := AggregateSignatures(signatures)
		if err != nil {
			b.Fatalf("Failed to aggregate signatures: %v", err)
		}

		// Optionally, verify the aggregated signature
		// Note: Proper verification would require aggregated public keys
		// and is beyond the scope of this benchmark.
		_ = aggSig
	}
}

// BenchmarkBLSSigner_ConcurrentSign benchmarks signing operations under concurrent conditions.
func BenchmarkBLSSigner_ConcurrentSign(b *testing.B) {
	// Number of concurrent goroutines
	numGoroutines := 100

	// Initialize a pool of BLSSigners
	signers := make([]*BLSSigner, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		signer, err := NewBLSSigner()
		require.NoError(b, err, "Failed to initialize BLSSigner")
		signers[i] = signer
	}

	// Pre-generate a sample message
	message := generateSampleData(256) // 256 bytes message

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine will perform signing operations
	for i := 0; i < numGoroutines; i++ {
		go func(signer *BLSSigner) {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				_, err := signer.Sign(message)
				if err != nil {
					b.Fatalf("Failed to sign data: %v", err)
				}
			}
		}(signers[i])
	}

	wg.Wait()
}

// BenchmarkBLSSigner_ConcurrentVerify benchmarks verification operations under concurrent conditions.
func BenchmarkBLSSigner_ConcurrentVerify(b *testing.B) {
	// Number of concurrent goroutines
	numGoroutines := 100

	// Initialize a pool of BLSSigners
	signers := make([]*BLSSigner, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		signer, err := NewBLSSigner()
		require.NoError(b, err, "Failed to initialize BLSSigner")
		signers[i] = signer
	}

	// Pre-generate sample messages and their signatures
	messages := make([][]byte, numGoroutines)
	signatures := make([][]byte, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		messages[i] = generateSampleData(256) // 256 bytes message
		signature, err := signers[i].Sign(messages[i])
		require.NoError(b, err, "Failed to sign data")
		signatures[i] = signature
	}

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine will perform verification operations
	for i := 0; i < numGoroutines; i++ {
		go func(signer *BLSSigner, message, signature []byte) {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				valid, err := signer.Verify(message, signature)
				if err != nil {
					b.Fatalf("Failed to verify signature: %v", err)
				}
				if !valid {
					b.Fatalf("Signature verification failed")
				}
			}
		}(signers[i], messages[i], signatures[i])
	}

	wg.Wait()
}
