//go:build windows
// +build windows

package plugin

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	// Job object functions
	procCreateJobObject           = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject   = kernel32.NewProc("SetInformationJobObjectW")
	procAssignProcessToJobObject  = kernel32.NewProc("AssignProcessToJobObject")
	procQueryInformationJobObject = kernel32.NewProc("QueryInformationJobObjectW")
	procTerminateJobObject        = kernel32.NewProc("TerminateJobObject")
	procCloseHandle               = kernel32.NewProc("CloseHandle")

	// Process functions
	procGetCurrentProcess   = kernel32.NewProc("GetCurrentProcess")
	procCreateProcess       = kernel32.NewProc("CreateProcessW")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procGetExitCodeProcess  = kernel32.NewProc("GetExitCodeProcess")
)

// Windows-specific constants
const (
	JobObjectBasicLimitInformation      = 2
	JobObjectBasicUIRestrictions        = 4
	JobObjectSecurityLimitInformation   = 5
	JobObjectEndOfJobTimeInformation    = 6
	JobObjectBasicAccountingInformation = 1
	JobObjectBasicProcessIdList         = 3

	JOB_OBJECT_LIMIT_WORKINGSET        = 0x00000001
	JOB_OBJECT_LIMIT_PROCESS_TIME      = 0x00000002
	JOB_OBJECT_LIMIT_JOB_TIME          = 0x00000004
	JOB_OBJECT_LIMIT_ACTIVE_PROCESS    = 0x00000008
	JOB_OBJECT_LIMIT_PROCESS_MEMORY    = 0x00000100
	JOB_OBJECT_LIMIT_JOB_MEMORY        = 0x00000200
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x00002000

	INFINITE      = 0xFFFFFFFF
	WAIT_OBJECT_0 = 0
	WAIT_TIMEOUT  = 258
)

// Windows job object structures
type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	IoInfo                IO_COUNTERS
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type PROCESS_INFORMATION struct {
	Process   syscall.Handle
	Thread    syscall.Handle
	ProcessId uint32
	ThreadId  uint32
}

type STARTUPINFO struct {
	Cb            uint32
	_             *uint16
	Desktop       *uint16
	Title         *uint16
	X             uint32
	Y             uint32
	XSize         uint32
	YSize         uint32
	XCountChars   uint32
	YCountChars   uint32
	FillAttribute uint32
	Flags         uint32
	ShowWindow    uint16
	_             uint16
	_             *byte
	StdInput      syscall.Handle
	StdOutput     syscall.Handle
	StdError      syscall.Handle
}

// WindowsJobObjectManager manages Windows job objects for memory limiting
type WindowsJobObjectManager struct {
	jobObjects map[string]syscall.Handle
	mu         sync.Mutex
}

// NewWindowsJobObjectManager creates a new Windows job object manager
func NewWindowsJobObjectManager() *WindowsJobObjectManager {
	return &WindowsJobObjectManager{
		jobObjects: make(map[string]syscall.Handle),
	}
}

// initializeWindowsJobObjects sets up Windows job objects (implementation for memory_limiter.go)
func (ml *MemoryLimiter) initializeWindowsJobObjects() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("Windows job objects only available on Windows")
	}

	// Windows job objects require no global initialization
	return nil
}

// executeWindowsWithJobObjects executes plugin with Windows job objects (implementation for memory_limiter.go)
func (ml *MemoryLimiter) executeWindowsWithJobObjects(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, memoryLimitBytes int64, timeout time.Duration) (pluginapi.PluginOutput, error) {
	if runtime.GOOS != "windows" {
		return pluginapi.PluginOutput{}, fmt.Errorf("Windows job objects only available on Windows")
	}

	// Create job object for this execution
	jobManager := NewWindowsJobObjectManager()
	jobName := fmt.Sprintf("slicli-plugin-%s-%d", p.Name(), time.Now().UnixNano())

	jobHandle, err := jobManager.CreateJobObject(jobName, memoryLimitBytes)
	if err != nil {
		return pluginapi.PluginOutput{}, fmt.Errorf("creating job object: %w", err)
	}
	defer jobManager.CloseJobObject(jobName)

	// Execute with job object constraints
	return ml.executeWithJobObject(ctx, p, input, jobHandle, timeout)
}

// CreateJobObject creates a new job object with memory limits
func (wjm *WindowsJobObjectManager) CreateJobObject(name string, memoryLimitBytes int64) (syscall.Handle, error) {
	wjm.mu.Lock()
	defer wjm.mu.Unlock()

	// Create job object
	jobNamePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, fmt.Errorf("converting job name to UTF16: %w", err)
	}

	ret, _, err := procCreateJobObject.Call(
		0, // lpJobAttributes (NULL for default security)
		uintptr(unsafe.Pointer(jobNamePtr)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("CreateJobObject failed: %w", err)
	}

	jobHandle := syscall.Handle(ret)

	// Set memory limits
	if err := wjm.setJobMemoryLimit(jobHandle, memoryLimitBytes); err != nil {
		// Close the job object if we can't set limits
		procCloseHandle.Call(uintptr(jobHandle))
		return 0, fmt.Errorf("setting memory limit: %w", err)
	}

	// Store the handle
	wjm.jobObjects[name] = jobHandle

	return jobHandle, nil
}

// setJobMemoryLimit sets memory limits on a job object
func (wjm *WindowsJobObjectManager) setJobMemoryLimit(jobHandle syscall.Handle, memoryLimitBytes int64) error {
	// Prepare extended limit information
	extendedInfo := JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: JOB_OBJECT_LIMIT_PROCESS_MEMORY | JOB_OBJECT_LIMIT_JOB_MEMORY | JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
		ProcessMemoryLimit: uintptr(memoryLimitBytes),
		JobMemoryLimit:     uintptr(memoryLimitBytes),
	}

	// Set the job object information
	ret, _, err := procSetInformationJobObject.Call(
		uintptr(jobHandle),
		JobObjectBasicLimitInformation,
		uintptr(unsafe.Pointer(&extendedInfo)),
		unsafe.Sizeof(extendedInfo),
	)

	if ret == 0 {
		return fmt.Errorf("SetInformationJobObject failed: %w", err)
	}

	return nil
}

// AssignCurrentProcessToJobObject assigns the current process to a job object
func (wjm *WindowsJobObjectManager) AssignCurrentProcessToJobObject(jobHandle syscall.Handle) error {
	// Get current process handle
	currentProcess, _, _ := procGetCurrentProcess.Call()

	// Assign current process to job object
	ret, _, err := procAssignProcessToJobObject.Call(
		uintptr(jobHandle),
		currentProcess,
	)

	if ret == 0 {
		return fmt.Errorf("AssignProcessToJobObject failed: %w", err)
	}

	return nil
}

// GetJobMemoryUsage returns current memory usage for a job object
func (wjm *WindowsJobObjectManager) GetJobMemoryUsage(jobHandle syscall.Handle) (int64, error) {
	var extendedInfo JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var returnLength uint32

	ret, _, err := procQueryInformationJobObject.Call(
		uintptr(jobHandle),
		JobObjectBasicLimitInformation,
		uintptr(unsafe.Pointer(&extendedInfo)),
		unsafe.Sizeof(extendedInfo),
		uintptr(unsafe.Pointer(&returnLength)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("QueryInformationJobObject failed: %w", err)
	}

	return int64(extendedInfo.PeakJobMemoryUsed), nil
}

// CloseJobObject closes and cleans up a job object
func (wjm *WindowsJobObjectManager) CloseJobObject(name string) error {
	wjm.mu.Lock()
	defer wjm.mu.Unlock()

	handle, exists := wjm.jobObjects[name]
	if !exists {
		return fmt.Errorf("job object %s not found", name)
	}

	// Terminate all processes in the job object
	ret, _, _ := procTerminateJobObject.Call(
		uintptr(handle),
		1, // Exit code
	)

	// Close the handle
	ret, _, err := procCloseHandle.Call(uintptr(handle))
	if ret == 0 {
		return fmt.Errorf("CloseHandle failed: %w", err)
	}

	delete(wjm.jobObjects, name)
	return nil
}

// executeWithJobObject executes the plugin within a job object with monitoring
func (ml *MemoryLimiter) executeWithJobObject(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, jobHandle syscall.Handle, timeout time.Duration) (pluginapi.PluginOutput, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Result channel
	type result struct {
		output pluginapi.PluginOutput
		err    error
	}
	resultChan := make(chan result, 1)

	// Start monitoring goroutine
	monitorDone := make(chan struct{})
	go ml.monitorWindowsJobMemory(execCtx, jobHandle, p.Name(), monitorDone)

	// Execute plugin in goroutine with job object constraints
	go func() {
		defer close(monitorDone)

		// Assign current goroutine's process to job object
		jobManager := NewWindowsJobObjectManager()
		if err := jobManager.AssignCurrentProcessToJobObject(jobHandle); err != nil {
			resultChan <- result{err: fmt.Errorf("assigning process to job object: %w", err)}
			return
		}

		// Execute the plugin
		output, err := p.Execute(execCtx, input)
		resultChan <- result{output: output, err: err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		return res.output, res.err
	case <-execCtx.Done():
		return pluginapi.PluginOutput{}, fmt.Errorf("execution timeout or cancelled")
	}
}

// GetJobObjectMemoryUsage returns memory usage for all active job objects
func (wjm *WindowsJobObjectManager) GetJobObjectMemoryUsage() map[string]int64 {
	wjm.mu.Lock()
	defer wjm.mu.Unlock()

	usage := make(map[string]int64)
	for name, handle := range wjm.jobObjects {
		if memUsage, err := wjm.GetJobMemoryUsage(handle); err == nil {
			usage[name] = memUsage
		}
	}

	return usage
}

// Cleanup closes all job objects
func (wjm *WindowsJobObjectManager) Cleanup() error {
	wjm.mu.Lock()
	defer wjm.mu.Unlock()

	var errors []string
	for name := range wjm.jobObjects {
		if err := wjm.CloseJobObject(name); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", fmt.Sprintf("%v", errors))
	}

	return nil
}

// monitorWindowsJobMemory monitors memory usage for Windows job objects with enforcement
func (ml *MemoryLimiter) monitorWindowsJobMemory(ctx context.Context, jobHandle syscall.Handle, pluginName string, done <-chan struct{}) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Get job memory limit for comparison
	var memoryLimit int64
	var extendedInfo JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var returnLength uint32

	ret, _, _ := procQueryInformationJobObject.Call(
		uintptr(jobHandle),
		JobObjectBasicLimitInformation,
		uintptr(unsafe.Pointer(&extendedInfo)),
		unsafe.Sizeof(extendedInfo),
		uintptr(unsafe.Pointer(&returnLength)),
	)

	if ret != 0 {
		memoryLimit = int64(extendedInfo.JobMemoryLimit)
	}

	var lastWarningTime time.Time
	warningThreshold := float64(0.8)  // Warn when usage exceeds 80% of limit
	criticalThreshold := float64(0.9) // Critical when usage exceeds 90% of limit

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			// Query current memory usage
			ret, _, _ := procQueryInformationJobObject.Call(
				uintptr(jobHandle),
				JobObjectBasicLimitInformation,
				uintptr(unsafe.Pointer(&extendedInfo)),
				unsafe.Sizeof(extendedInfo),
				uintptr(unsafe.Pointer(&returnLength)),
			)

			if ret != 0 {
				usage := int64(extendedInfo.PeakJobMemoryUsed)

				// Calculate usage percentage
				if memoryLimit > 0 && usage > 0 {
					usagePercent := float64(usage) / float64(memoryLimit)
					now := time.Now()

					// Log warnings for high memory usage (throttled to avoid spam)
					if usagePercent > criticalThreshold && now.Sub(lastWarningTime) > time.Second {
						log.Printf("[CRITICAL] Plugin %s memory usage: %.1f%% (%d MB / %d MB) in Windows job object",
							pluginName, usagePercent*100, usage/(1024*1024), memoryLimit/(1024*1024))
						lastWarningTime = now
					} else if usagePercent > warningThreshold && now.Sub(lastWarningTime) > 5*time.Second {
						log.Printf("[WARNING] Plugin %s memory usage: %.1f%% (%d MB / %d MB) in Windows job object",
							pluginName, usagePercent*100, usage/(1024*1024), memoryLimit/(1024*1024))
						lastWarningTime = now
					}
				}
			}
		}
	}
}
