//go:build !darwin && !windows
// +build !darwin,!windows

package plugin

// setResourceLimits is a no-op stub for platforms other than darwin and windows.
// On Linux, memory limits are handled via cgroups in the executeLinuxWithCgroups method.
func (ml *MemoryLimiter) setResourceLimits(_ int64) error {
	// Linux uses cgroups for memory limiting, which is handled separately
	// This stub exists for build compatibility
	return nil
}
