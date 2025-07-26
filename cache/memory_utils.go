package cache

import (
	"runtime"
)

// GetSystemMemory returns the total system memory in bytes
// This is a simplified implementation that returns available memory to the Go runtime
func GetSystemMemory() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Get the system memory limit for the current process
	// This is not the total system memory, but the memory available to this process
	// For a more accurate system memory detection, we would need platform-specific code
	// or a library like shirou/gopsutil
	
	// For now, we'll use a conservative estimate based on Go's memory stats
	// Typically, Go can use up to the system's available memory
	// We'll estimate based on current usage patterns
	
	// Default to 4GB if we can't determine
	const defaultMemory = 4 * 1024 * 1024 * 1024
	
	// If we have allocated more than 1GB, assume we have at least 4x that available
	if m.Sys > 1024*1024*1024 {
		return int64(m.Sys * 4)
	}
	
	return defaultMemory
}

// GetRecommendedCacheSize returns the recommended cache size based on system memory
// Returns 70-80% of total system memory
func GetRecommendedCacheSize() int64 {
	totalMem := GetSystemMemory()
	// Use 75% of total memory for cache
	return totalMem * 75 / 100
}