//go:build windows
// +build windows

package plugin

import (
	"context"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

func TestWindowsJobObjectManager_Creation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	assert.NotNil(t, manager)
	assert.Empty(t, manager.jobObjects)
}

func TestWindowsJobObjectManager_CreateJobObject(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	jobName := "test-job-object"
	memoryLimit := int64(100 * 1024 * 1024) // 100MB

	handle, err := manager.CreateJobObject(jobName, memoryLimit)
	assert.NoError(t, err)
	assert.NotEqual(t, syscall.InvalidHandle, handle)

	// Verify job object is stored
	assert.Contains(t, manager.jobObjects, jobName)
	assert.Equal(t, handle, manager.jobObjects[jobName])
}

func TestWindowsJobObjectManager_CreateJobObjectInvalidName(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	// Test with invalid UTF-16 sequence
	invalidName := string([]byte{0xFF, 0xFE, 0xFF, 0xFE})
	memoryLimit := int64(100 * 1024 * 1024)

	_, err := manager.CreateJobObject(invalidName, memoryLimit)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "converting job name to UTF16")
}

func TestWindowsJobObjectManager_SetMemoryLimit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	// Create a job object first
	jobName := "memory-limit-test"
	memoryLimit := int64(50 * 1024 * 1024) // 50MB

	handle, err := manager.CreateJobObject(jobName, memoryLimit)
	require.NoError(t, err)

	// Test setting memory limit explicitly
	err = manager.setJobMemoryLimit(handle, memoryLimit)
	assert.NoError(t, err)
}

func TestWindowsJobObjectManager_AssignCurrentProcess(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	jobName := "process-assignment-test"
	memoryLimit := int64(100 * 1024 * 1024)

	handle, err := manager.CreateJobObject(jobName, memoryLimit)
	require.NoError(t, err)

	// Test assigning current process
	err = manager.AssignCurrentProcessToJobObject(handle)
	assert.NoError(t, err)
}

func TestWindowsJobObjectManager_GetMemoryUsage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	jobName := "memory-usage-test"
	memoryLimit := int64(100 * 1024 * 1024)

	handle, err := manager.CreateJobObject(jobName, memoryLimit)
	require.NoError(t, err)

	// Get memory usage (should be 0 or small amount initially)
	usage, err := manager.GetJobMemoryUsage(handle)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, usage, int64(0))
}

func TestWindowsJobObjectManager_GetJobObjectMemoryUsage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	// Create multiple job objects
	jobNames := []string{"job1", "job2", "job3"}
	memoryLimit := int64(50 * 1024 * 1024)

	for _, name := range jobNames {
		_, err := manager.CreateJobObject(name, memoryLimit)
		require.NoError(t, err)
	}

	// Get usage for all job objects
	usage := manager.GetJobObjectMemoryUsage()
	assert.Len(t, usage, len(jobNames))

	for _, name := range jobNames {
		assert.Contains(t, usage, name)
		assert.GreaterOrEqual(t, usage[name], int64(0))
	}
}

func TestWindowsJobObjectManager_CloseJobObject(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()

	jobName := "close-test"
	memoryLimit := int64(100 * 1024 * 1024)

	handle, err := manager.CreateJobObject(jobName, memoryLimit)
	require.NoError(t, err)

	// Verify job object exists
	assert.Contains(t, manager.jobObjects, jobName)

	// Close the job object
	err = manager.CloseJobObject(jobName)
	assert.NoError(t, err)

	// Verify job object is removed
	assert.NotContains(t, manager.jobObjects, jobName)
}

func TestWindowsJobObjectManager_CloseNonExistentJobObject(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()

	err := manager.CloseJobObject("non-existent-job")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job object non-existent-job not found")
}

func TestWindowsJobObjectManager_Cleanup(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	manager := NewWindowsJobObjectManager()

	// Create multiple job objects
	jobNames := []string{"cleanup1", "cleanup2", "cleanup3"}
	memoryLimit := int64(50 * 1024 * 1024)

	for _, name := range jobNames {
		_, err := manager.CreateJobObject(name, memoryLimit)
		require.NoError(t, err)
	}

	// Verify all job objects exist
	assert.Len(t, manager.jobObjects, len(jobNames))

	// Cleanup all job objects
	err := manager.Cleanup()
	assert.NoError(t, err)

	// Verify all job objects are removed
	assert.Empty(t, manager.jobObjects)
}

func TestMemoryLimiter_WindowsInitialization(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	limiter, err := NewMemoryLimiter()
	assert.NoError(t, err)
	assert.NotNil(t, limiter)

	err = limiter.Cleanup()
	assert.NoError(t, err)
}

func TestMemoryLimiter_WindowsJobObjectExecution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	limiter, err := NewMemoryLimiter()
	require.NoError(t, err)
	defer limiter.Cleanup()

	plugin := &MockMemoryTestPlugin{
		name:          "windows-test-plugin",
		memoryToUse:   1024 * 1024, // 1MB
		sleepDuration: 100 * time.Millisecond,
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "test content for Windows",
	}

	memoryLimit := int64(50 * 1024 * 1024) // 50MB
	timeout := 5 * time.Second

	output, err := limiter.ExecuteWithMemoryLimit(ctx, plugin, input, memoryLimit, timeout)
	assert.NoError(t, err)
	assert.Equal(t, "<p>Memory test completed</p>", output.HTML)
}

func TestMemoryLimiter_WindowsJobObjectTimeout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	limiter, err := NewMemoryLimiter()
	require.NoError(t, err)
	defer limiter.Cleanup()

	plugin := &MockMemoryTestPlugin{
		name:          "windows-timeout-plugin",
		sleepDuration: 2 * time.Second, // Longer than timeout
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "timeout test",
	}

	memoryLimit := int64(50 * 1024 * 1024)
	timeout := 500 * time.Millisecond // Short timeout

	_, err = limiter.ExecuteWithMemoryLimit(ctx, plugin, input, memoryLimit, timeout)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestMemoryLimiter_WindowsJobObjectCancellation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	limiter, err := NewMemoryLimiter()
	require.NoError(t, err)
	defer limiter.Cleanup()

	plugin := &MockMemoryTestPlugin{
		name:          "windows-cancel-plugin",
		sleepDuration: 2 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	input := pluginapi.PluginInput{
		Content: "cancellation test",
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	memoryLimit := int64(50 * 1024 * 1024)
	timeout := 5 * time.Second

	_, err = limiter.ExecuteWithMemoryLimit(ctx, plugin, input, memoryLimit, timeout)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestMemoryLimiter_WindowsJobObjectInvalidHandle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	limiter, err := NewMemoryLimiter()
	require.NoError(t, err)
	defer limiter.Cleanup()

	plugin := &MockMemoryTestPlugin{
		name: "invalid-handle-test",
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "test content",
	}

	// Test execution with invalid job handle
	invalidHandle := syscall.Handle(0)
	_, err = limiter.executeWithJobObject(ctx, plugin, input, invalidHandle, 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assigning process to job object")
}

// Benchmark tests for Windows job objects
func BenchmarkWindowsJobObjectManager_CreateAndClose(b *testing.B) {
	if runtime.GOOS != "windows" {
		b.Skip("Windows-specific benchmark")
	}

	manager := NewWindowsJobObjectManager()
	memoryLimit := int64(100 * 1024 * 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobName := "benchmark-job"
		handle, err := manager.CreateJobObject(jobName, memoryLimit)
		if err != nil {
			b.Fatalf("Failed to create job object: %v", err)
		}

		err = manager.CloseJobObject(jobName)
		if err != nil {
			b.Fatalf("Failed to close job object: %v", err)
		}

		// Cleanup handle to avoid leaks
		_ = handle
	}
}

func BenchmarkWindowsJobObjectManager_MemoryUsageQuery(b *testing.B) {
	if runtime.GOOS != "windows" {
		b.Skip("Windows-specific benchmark")
	}

	manager := NewWindowsJobObjectManager()
	defer manager.Cleanup()

	jobName := "benchmark-usage-job"
	memoryLimit := int64(100 * 1024 * 1024)

	handle, err := manager.CreateJobObject(jobName, memoryLimit)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GetJobMemoryUsage(handle)
		if err != nil {
			b.Fatalf("Failed to get memory usage: %v", err)
		}
	}
}

// Test Windows-specific syscall wrappers
func TestWindowsSyscallWrappers(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	// Test that all required DLL functions are available
	testFunctions := []struct {
		name string
		proc *syscall.LazyProc
	}{
		{"CreateJobObjectW", procCreateJobObject},
		{"SetInformationJobObjectW", procSetInformationJobObject},
		{"AssignProcessToJobObject", procAssignProcessToJobObject},
		{"QueryInformationJobObjectW", procQueryInformationJobObject},
		{"TerminateJobObject", procTerminateJobObject},
		{"CloseHandle", procCloseHandle},
		{"GetCurrentProcess", procGetCurrentProcess},
	}

	for _, tf := range testFunctions {
		t.Run(tf.name, func(t *testing.T) {
			err := tf.proc.Find()
			assert.NoError(t, err, "Function %s should be available", tf.name)
		})
	}
}

func TestWindowsStructureSizes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	// Test that structure sizes are reasonable (basic sanity check)
	basicLimitSize := unsafe.Sizeof(JOBOBJECT_BASIC_LIMIT_INFORMATION{})
	extendedLimitSize := unsafe.Sizeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION{})
	ioCountersSize := unsafe.Sizeof(IO_COUNTERS{})

	assert.Greater(t, basicLimitSize, uintptr(0))
	assert.Greater(t, extendedLimitSize, basicLimitSize) // Extended should be larger
	assert.Greater(t, ioCountersSize, uintptr(0))

	t.Logf("JOBOBJECT_BASIC_LIMIT_INFORMATION size: %d bytes", basicLimitSize)
	t.Logf("JOBOBJECT_EXTENDED_LIMIT_INFORMATION size: %d bytes", extendedLimitSize)
	t.Logf("IO_COUNTERS size: %d bytes", ioCountersSize)
}
