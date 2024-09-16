package benchmark

import "runtime"

// GetMemoryUsage captures the current memory usage in bytes.
func GetMemoryUsage() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.Alloc // Alloc gives the number of bytes currently allocated and in use
}
