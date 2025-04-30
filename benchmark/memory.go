package benchmark

import "runtime"

// getMemoryUsage returns the amount of memory allocated to the process in bytes
func getMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}
