//go:build !windows
// +build !windows

package plugin

import (
	"context"
	"fmt"
	"time"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// initializeWindowsJobObjects is a no-op on non-Windows platforms
func (ml *MemoryLimiter) initializeWindowsJobObjects() error {
	// Windows job objects are not available on non-Windows platforms
	return nil
}

// executeWindowsWithJobObjects is not available on non-Windows platforms
func (ml *MemoryLimiter) executeWindowsWithJobObjects(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, memoryLimitBytes int64, timeout time.Duration) (pluginapi.PluginOutput, error) {
	return pluginapi.PluginOutput{}, fmt.Errorf("windows job objects only available on Windows")
}
