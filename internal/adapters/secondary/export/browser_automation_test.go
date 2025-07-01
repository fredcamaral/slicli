package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBrowserAutomation(t *testing.T) {
	tests := []struct {
		name           string
		config         BrowserConfig
		expectError    bool
		expectedFields map[string]interface{}
	}{
		{
			name: "default configuration",
			config: BrowserConfig{
				ExecutablePath: "/fake/chrome/path",
				TempDir:        "",
				Timeout:        0,
			},
			expectError: false,
			expectedFields: map[string]interface{}{
				"timeout": 30 * time.Second,
			},
		},
		{
			name: "custom configuration",
			config: BrowserConfig{
				ExecutablePath: "/custom/chrome",
				TempDir:        "/custom/temp",
				Timeout:        60 * time.Second,
			},
			expectError: false,
			expectedFields: map[string]interface{}{
				"executablePath": "/custom/chrome",
				"tempDir":        "/custom/temp",
				"timeout":        60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ba, err := NewBrowserAutomation(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, ba)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ba)
				assert.NotNil(t, ba.activeProcesses)

				// Check expected fields
				for field, expected := range tt.expectedFields {
					switch field {
					case "executablePath":
						assert.Equal(t, expected, ba.executablePath)
					case "tempDir":
						assert.Equal(t, expected, ba.tempDir)
					case "timeout":
						assert.Equal(t, expected, ba.timeout)
					}
				}
			}
		})
	}
}

func TestBrowserAutomation_ProcessTracking(t *testing.T) {
	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        t.TempDir(),
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	// Initially no processes
	assert.Equal(t, 0, ba.GetActiveProcessCount())

	// Simulate process tracking (without actually running Chrome)
	ba.processMutex.Lock()
	ba.activeProcesses["test-1"] = nil // Simulate tracked process
	ba.activeProcesses["test-2"] = nil // Simulate another tracked process
	ba.processMutex.Unlock()

	// Should show tracked processes
	assert.Equal(t, 2, ba.GetActiveProcessCount())

	// Test resource usage
	usage := ba.GetResourceUsage()
	assert.Equal(t, 2, usage["active_processes"])
	assert.Contains(t, usage, "temp_directory")
	assert.Contains(t, usage, "executable_path")
	assert.Contains(t, usage, "timeout")
}

func TestBrowserAutomation_Cleanup(t *testing.T) {
	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        t.TempDir(),
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	// Add some fake processes
	ba.processMutex.Lock()
	ba.activeProcesses["test-1"] = nil
	ba.activeProcesses["test-2"] = nil
	ba.processMutex.Unlock()

	// Test cleanup
	err = ba.Cleanup()
	assert.NoError(t, err)
	assert.Equal(t, 0, ba.GetActiveProcessCount())
}

func TestBrowserAutomation_KillActiveProcesses(t *testing.T) {
	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        t.TempDir(),
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	// Add some fake processes
	ba.processMutex.Lock()
	ba.activeProcesses["test-1"] = nil
	ba.activeProcesses["test-2"] = nil
	ba.processMutex.Unlock()

	// Test kill processes
	err = ba.KillActiveProcesses()
	assert.NoError(t, err)
	assert.Equal(t, 0, ba.GetActiveProcessCount())
}

func TestBrowserAutomation_GetResourceUsage(t *testing.T) {
	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        "/tmp/test",
		Timeout:        45 * time.Second,
	})
	require.NoError(t, err)

	// Add different types of processes
	ba.processMutex.Lock()
	ba.activeProcesses["pdf-123"] = nil
	ba.activeProcesses["image-456"] = nil
	ba.activeProcesses["pdf-789"] = nil
	ba.processMutex.Unlock()

	usage := ba.GetResourceUsage()

	assert.Equal(t, 3, usage["active_processes"])
	assert.Equal(t, 2, usage["pdf_processes"])
	assert.Equal(t, 1, usage["image_processes"])
	assert.Equal(t, "/tmp/test", usage["temp_directory"])
	assert.Equal(t, "/fake/chrome", usage["executable_path"])
	assert.Equal(t, "45s", usage["timeout"])
}

func TestBrowserAutomation_CleanupTempFiles(t *testing.T) {
	tempDir := t.TempDir()

	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        tempDir,
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	// Create some fake Chrome temp files
	testFiles := []string{
		"chrome_test_file",
		"Crashpad_test",
		"regular_file.txt",
	}

	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Test cleanup temp files
	err = ba.cleanupTempFiles()
	// We expect no error even if patterns don't match real files
	assert.NoError(t, err)

	// Verify regular file still exists (it shouldn't match Chrome patterns)
	regularFile := filepath.Join(tempDir, "regular_file.txt")
	_, err = os.Stat(regularFile)
	assert.NoError(t, err, "Regular file should not be deleted")
}

func TestValidateBrowserSetup(t *testing.T) {
	tests := []struct {
		name            string
		config          BrowserConfig
		expectError     bool
		expectAvailable bool
	}{
		{
			name: "missing executable",
			config: BrowserConfig{
				ExecutablePath: "/nonexistent/chrome",
			},
			expectError:     true,
			expectAvailable: false,
		},
		{
			name: "auto-detect executable (will likely fail in test env)",
			config: BrowserConfig{
				ExecutablePath: "",
			},
			expectError:     true,
			expectAvailable: false,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBrowserSetup(ctx, tt.config)

			assert.Equal(t, tt.expectAvailable, result.Available)

			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			}
		})
	}
}

func TestService_BrowserManagement(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	// Create mock browser automation
	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        t.TempDir(),
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	// Test registration
	service.RegisterBrowserAutomation("test-browser", ba)

	stats := service.GetExportStatistics()
	assert.Equal(t, 1, stats["active_browsers"])
	assert.Equal(t, 0, stats["total_browser_processes"])

	// Add some processes to the browser
	ba.processMutex.Lock()
	ba.activeProcesses["test-process"] = nil
	ba.processMutex.Unlock()

	stats = service.GetExportStatistics()
	assert.Equal(t, 1, stats["total_browser_processes"])

	// Test unregistration
	service.UnregisterBrowserAutomation("test-browser")

	stats = service.GetExportStatistics()
	assert.Equal(t, 0, stats["active_browsers"])
	assert.Equal(t, 0, stats["total_browser_processes"])
}

func TestService_BrowserResourceUsage(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	// Create mock browser automation instances
	ba1, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome1",
		TempDir:        t.TempDir(),
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	ba2, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome2",
		TempDir:        t.TempDir(),
		Timeout:        45 * time.Second,
	})
	require.NoError(t, err)

	service.RegisterBrowserAutomation("browser-1", ba1)
	service.RegisterBrowserAutomation("browser-2", ba2)

	// Get resource usage
	usage := service.GetBrowserResourceUsage()

	assert.Len(t, usage, 2)
	assert.Contains(t, usage, "browser-1")
	assert.Contains(t, usage, "browser-2")

	// Check individual browser usage
	browser1Usage := usage["browser-1"].(map[string]interface{})
	assert.Equal(t, "/fake/chrome1", browser1Usage["executable_path"])
	assert.Equal(t, "30s", browser1Usage["timeout"])

	browser2Usage := usage["browser-2"].(map[string]interface{})
	assert.Equal(t, "/fake/chrome2", browser2Usage["executable_path"])
	assert.Equal(t, "45s", browser2Usage["timeout"])
}

func TestService_ComprehensiveCleanup(t *testing.T) {
	tempDir := t.TempDir()
	service, err := NewService(tempDir)
	require.NoError(t, err)

	// Register multiple browser instances
	for i := 0; i < 3; i++ {
		ba, err := NewBrowserAutomation(BrowserConfig{
			ExecutablePath: "/fake/chrome",
			TempDir:        tempDir,
			Timeout:        30 * time.Second,
		})
		require.NoError(t, err)

		// Add some fake processes
		ba.processMutex.Lock()
		ba.activeProcesses["test-process"] = nil
		ba.processMutex.Unlock()

		service.RegisterBrowserAutomation(fmt.Sprintf("%s-%d", t.Name(), i), ba)
	}

	// Verify browsers are registered
	stats := service.GetExportStatistics()
	assert.Equal(t, 3, stats["active_browsers"])

	// Test comprehensive cleanup
	err = service.Cleanup()
	assert.NoError(t, err)

	// Verify cleanup
	stats = service.GetExportStatistics()
	assert.Equal(t, 0, stats["active_browsers"])
	assert.Equal(t, 0, stats["total_browser_processes"])
}

func TestService_KillAllBrowserProcesses(t *testing.T) {
	service, err := NewService("")
	require.NoError(t, err)

	// Register browser with processes
	ba, err := NewBrowserAutomation(BrowserConfig{
		ExecutablePath: "/fake/chrome",
		TempDir:        t.TempDir(),
		Timeout:        30 * time.Second,
	})
	require.NoError(t, err)

	// Add fake processes
	ba.processMutex.Lock()
	ba.activeProcesses["pdf-process"] = nil
	ba.activeProcesses["image-process"] = nil
	ba.processMutex.Unlock()

	service.RegisterBrowserAutomation("test-browser", ba)

	// Verify processes exist
	assert.Equal(t, 2, ba.GetActiveProcessCount())

	// Kill all processes
	err = service.KillAllBrowserProcesses()
	assert.NoError(t, err)

	// Verify processes are gone
	assert.Equal(t, 0, ba.GetActiveProcessCount())
}
