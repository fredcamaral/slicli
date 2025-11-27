//go:build darwin
// +build darwin

package plugin

import (
	"fmt"
	"syscall"
)

// setResourceLimits attempts to set memory resource limits on macOS using syscall.Setrlimit.
//
// IMPORTANT: On macOS, RLIMIT_AS (address space limit) has limited effectiveness:
// - It may not prevent all memory allocations
// - Some system libraries may bypass this limit
// - The limit applies to virtual memory, not physical memory
//
// This is provided as a best-effort mechanism. For strict memory isolation,
// use containerized execution (Docker).
func (ml *MemoryLimiter) setResourceLimits(memoryBytes int64) error {
	if memoryBytes <= 0 {
		return fmt.Errorf("memory limit must be positive, got %d", memoryBytes)
	}

	var rLimit syscall.Rlimit

	// Get current limits first
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &rLimit); err != nil {
		return fmt.Errorf("getting current resource limits: %w", err)
	}

	// Set new limits (both soft and hard)
	// Note: We can only lower the hard limit, not raise it without privileges
	newLimit := uint64(memoryBytes)

	// If requested limit is higher than current hard limit, use current hard limit
	if newLimit > rLimit.Max && rLimit.Max > 0 {
		newLimit = rLimit.Max
	}

	rLimit.Cur = newLimit
	// Only set hard limit if we have permission (running as root)
	// Otherwise, just set the soft limit
	if rLimit.Max == 0 || newLimit < rLimit.Max {
		rLimit.Max = newLimit
	}

	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &rLimit); err != nil {
		return fmt.Errorf("setting resource limits: %w", err)
	}

	return nil
}
