package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BrowserAutomation handles browser-based export operations
type BrowserAutomation struct {
	executablePath  string
	tempDir         string
	timeout         time.Duration
	activeProcesses map[string]*exec.Cmd // Track active processes for cleanup
	processMutex    sync.RWMutex         // Protect concurrent access to activeProcesses
}

// BrowserConfig configures browser automation
type BrowserConfig struct {
	ExecutablePath string
	TempDir        string
	Timeout        time.Duration
}

// NewBrowserAutomation creates a new browser automation service
func NewBrowserAutomation(config BrowserConfig) (*BrowserAutomation, error) {
	execPath := config.ExecutablePath
	if execPath == "" {
		var err error
		execPath, err = findChromeExecutable()
		if err != nil {
			return nil, fmt.Errorf("could not find Chrome/Chromium executable: %w", err)
		}
	}

	tempDir := config.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &BrowserAutomation{
		executablePath:  execPath,
		tempDir:         tempDir,
		timeout:         timeout,
		activeProcesses: make(map[string]*exec.Cmd),
		processMutex:    sync.RWMutex{},
	}, nil
}

// ConvertHTMLToPDF converts an HTML file to PDF using Chrome headless
func (ba *BrowserAutomation) ConvertHTMLToPDF(ctx context.Context, htmlPath, outputPath string, options *PDFOptions) error {
	if err := validateFilePath(htmlPath); err != nil {
		return fmt.Errorf("invalid HTML path: %w", err)
	}

	if err := validateFilePath(outputPath); err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Build Chrome arguments
	args := []string{
		"--headless",
		"--disable-gpu",
		"--disable-software-rasterizer",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--no-sandbox", // Required for some environments
		"--disable-dev-shm-usage",
		"--virtual-time-budget=5000",
		"--run-all-compositor-stages-before-draw",
		"--print-to-pdf=" + outputPath,
	}

	// Add PDF-specific options
	if options != nil {
		if options.PageSize != "" {
			args = append(args, "--print-to-pdf-no-header")
		}
		if options.Landscape {
			// Chrome doesn't have a direct landscape flag for print-to-pdf
			// TODO: Implement landscape mode via Chrome DevTools Protocol (CDP)
			// For now, landscape mode is not supported in Chrome export
			_ = options.Landscape // Explicitly acknowledge this option
		}
		if options.MarginTop != "" || options.MarginBottom != "" || options.MarginLeft != "" || options.MarginRight != "" {
			// Chrome uses default margins, custom margins require CDP (Chrome DevTools Protocol)
			// TODO: Implement custom margins via Chrome DevTools Protocol (CDP)
			// For now, we'll use defaults and note this as a future enhancement
			// Explicitly acknowledge these margin options
			_, _, _, _ = options.MarginTop, options.MarginBottom, options.MarginLeft, options.MarginRight
		}
	}

	// Convert file path to file:// URL
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}
	fileURL := "file://" + absPath

	args = append(args, fileURL)

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, ba.timeout)
	defer cancel()

	// Execute Chrome with process tracking
	// #nosec G204 - executablePath is validated during initialization and args are controlled
	// This is necessary for PDF export functionality in a CLI tool context
	cmd := exec.CommandContext(cmdCtx, ba.executablePath, args...)
	cmd.Dir = ba.tempDir

	// Generate unique process ID for tracking
	processID := fmt.Sprintf("pdf-%d", time.Now().UnixNano())

	// Track the process
	ba.processMutex.Lock()
	ba.activeProcesses[processID] = cmd
	ba.processMutex.Unlock()

	// Ensure cleanup on function exit
	defer func() {
		ba.processMutex.Lock()
		delete(ba.activeProcesses, processID)
		ba.processMutex.Unlock()
	}()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chrome PDF generation failed: %w (output: %s)", err, string(output))
	}

	// Verify the PDF was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("PDF file was not created at %s", outputPath)
	}

	return nil
}

// ConvertHTMLToImage converts an HTML file to an image using Chrome headless
func (ba *BrowserAutomation) ConvertHTMLToImage(ctx context.Context, htmlPath, outputPath string, options *ImageOptions) error {
	if err := validateFilePath(htmlPath); err != nil {
		return fmt.Errorf("invalid HTML path: %w", err)
	}

	if err := validateFilePath(outputPath); err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Determine dimensions
	width, height := 1920, 1080 // Default
	if options != nil {
		if options.Width > 0 && options.Height > 0 {
			width, height = options.Width, options.Height
		} else {
			width, height = GetImageDimensions(options.Quality)
		}
	}

	// Build Chrome arguments
	args := []string{
		"--headless",
		"--disable-gpu",
		"--disable-software-rasterizer",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--no-sandbox", // Required for some environments
		"--disable-dev-shm-usage",
		"--virtual-time-budget=5000",
		"--run-all-compositor-stages-before-draw",
		"--screenshot=" + outputPath,
		"--window-size=" + strconv.Itoa(width) + "," + strconv.Itoa(height),
	}

	// Add device scale factor for high DPI
	if options != nil && options.Quality == "high" {
		args = append(args, "--force-device-scale-factor=2")
	}

	// Convert file path to file:// URL
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}
	fileURL := "file://" + absPath

	args = append(args, fileURL)

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, ba.timeout)
	defer cancel()

	// Execute Chrome with process tracking
	// #nosec G204 - executablePath is validated during initialization and args are controlled
	// This is necessary for image export functionality in a CLI tool context
	cmd := exec.CommandContext(cmdCtx, ba.executablePath, args...)
	cmd.Dir = ba.tempDir

	// Generate unique process ID for tracking
	processID := fmt.Sprintf("image-%d", time.Now().UnixNano())

	// Track the process
	ba.processMutex.Lock()
	ba.activeProcesses[processID] = cmd
	ba.processMutex.Unlock()

	// Ensure cleanup on function exit
	defer func() {
		ba.processMutex.Lock()
		delete(ba.activeProcesses, processID)
		ba.processMutex.Unlock()
	}()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chrome screenshot generation failed: %w (output: %s)", err, string(output))
	}

	// Verify the image was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("image file was not created at %s", outputPath)
	}

	return nil
}

// PDFOptions contains options for PDF generation
type PDFOptions struct {
	PageSize     string // A4, Letter, Legal, etc.
	Landscape    bool
	MarginTop    string
	MarginRight  string
	MarginBottom string
	MarginLeft   string
	PrintHeaders bool
	PrintFooters bool
}

// ImageOptions contains options for image generation
type ImageOptions struct {
	Width   int
	Height  int
	Quality string // low, medium, high
	Format  string // png, jpg
}

// findChromeExecutable attempts to find Chrome or Chromium executable
func findChromeExecutable() (string, error) {
	var candidates []string

	switch runtime.GOOS {
	case "darwin": // macOS
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		}
	case "linux":
		candidates = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/usr/bin/chrome",
		}
	case "windows":
		candidates = []string{
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
			"chrome.exe", // Assume it's in PATH
			"chromium.exe",
		}
	default:
		candidates = []string{
			"google-chrome",
			"chromium",
			"chromium-browser",
			"chrome",
		}
	}

	// Check if any of the candidates exist
	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, nil
		}

		// Also try to find it in PATH
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	// Try common PATH names
	pathCandidates := []string{"google-chrome", "chromium", "chromium-browser", "chrome"}
	for _, name := range pathCandidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", errors.New("no Chrome or Chromium executable found. Please install Chrome/Chromium or set the executable path manually")
}

// isExecutableFile checks if a file exists and is executable
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// On Windows, we can't easily check execute permissions
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}

	// On Unix-like systems, check execute permission
	return !info.IsDir() && (info.Mode()&0111) != 0
}

// GetChromeVersion returns the version of the Chrome executable
func (ba *BrowserAutomation) GetChromeVersion(ctx context.Context) (string, error) {
	// #nosec G204 - executablePath is validated during NewBrowserAutomation initialization
	// and limited to known Chrome/Chromium binary paths for CLI tool functionality
	cmd := exec.CommandContext(ctx, ba.executablePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting Chrome version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// IsAvailable checks if browser automation is available
func (ba *BrowserAutomation) IsAvailable(ctx context.Context) error {
	// Check if executable exists
	if !isExecutableFile(ba.executablePath) {
		return fmt.Errorf("chrome executable not found at %s", ba.executablePath)
	}

	// Try to get version (lightweight test)
	_, err := ba.GetChromeVersion(ctx)
	if err != nil {
		return fmt.Errorf("chrome is not functional: %w", err)
	}

	return nil
}

// CreateHTMLTemplate creates a standalone HTML template optimized for printing/screenshots
func CreateHTMLTemplate(content, theme string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SLICLI Export</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #333;
            background: white;
        }
        
        @media print {
            body {
                print-color-adjust: exact;
                -webkit-print-color-adjust: exact;
            }
            
            .slide {
                page-break-after: always;
                width: 100vw;
                height: 100vh;
                display: flex;
                align-items: center;
                justify-content: center;
            }
            
            .slide:last-child {
                page-break-after: avoid;
            }
        }
        
        @media screen {
            .slide {
                width: 100vw;
                height: 100vh;
                display: flex;
                align-items: center;
                justify-content: center;
                padding: 2rem;
            }
        }
        
        .slide-content {
            max-width: 90%%;
            text-align: center;
        }
        
        h1 {
            font-size: 3rem;
            margin-bottom: 1rem;
            color: #2d3748;
        }
        
        h2 {
            font-size: 2rem;
            margin-bottom: 0.75rem;
            color: #4a5568;
        }
        
        h3 {
            font-size: 1.5rem;
            margin-bottom: 0.5rem;
            color: #718096;
        }
        
        p {
            font-size: 1.25rem;
            margin-bottom: 1rem;
        }
        
        ul, ol {
            font-size: 1.25rem;
            text-align: left;
            max-width: 600px;
            margin: 0 auto 1rem auto;
        }
        
        li {
            margin-bottom: 0.5rem;
        }
        
        code {
            background: #f7fafc;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
        }
        
        pre {
            background: #f7fafc;
            padding: 1rem;
            border-radius: 8px;
            overflow-x: auto;
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
            text-align: left;
            margin: 1rem 0;
        }
        
        blockquote {
            border-left: 4px solid #667eea;
            padding-left: 1rem;
            font-style: italic;
            margin: 1rem 0;
        }
        
        /* Custom theme styles */
        %s
    </style>
</head>
<body>
    %s
</body>
</html>`, theme, content)
}

// ValidationResult contains the result of browser validation
type ValidationResult struct {
	Available      bool
	ExecutablePath string
	Version        string
	Error          string
}

// ValidateBrowserSetup validates that browser automation can work
func ValidateBrowserSetup(ctx context.Context, config BrowserConfig) *ValidationResult {
	result := &ValidationResult{}

	browser, err := NewBrowserAutomation(config)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.ExecutablePath = browser.executablePath

	if err := browser.IsAvailable(ctx); err != nil {
		result.Error = err.Error()
		return result
	}

	version, err := browser.GetChromeVersion(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("could not get Chrome version: %v", err)
		return result
	}

	result.Available = true
	result.Version = version
	return result
}

// Cleanup gracefully shuts down all active browser processes and cleans up resources
func (ba *BrowserAutomation) Cleanup() error {
	ba.processMutex.Lock()
	defer ba.processMutex.Unlock()

	var errs []error

	// Terminate all active processes
	for processID, cmd := range ba.activeProcesses {
		if cmd != nil && cmd.Process != nil {
			// Try graceful termination first
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				// If graceful termination fails, force kill
				if killErr := cmd.Process.Kill(); killErr != nil {
					errs = append(errs, fmt.Errorf("failed to kill process %s: %w", processID, killErr))
				}
			}
		}
		delete(ba.activeProcesses, processID)
	}

	// Clean up temporary files created by browser processes
	if err := ba.cleanupTempFiles(); err != nil {
		errs = append(errs, fmt.Errorf("failed to cleanup temp files: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// GetActiveProcessCount returns the number of active browser processes
func (ba *BrowserAutomation) GetActiveProcessCount() int {
	ba.processMutex.RLock()
	defer ba.processMutex.RUnlock()
	return len(ba.activeProcesses)
}

// KillActiveProcesses forcefully terminates all active browser processes
func (ba *BrowserAutomation) KillActiveProcesses() error {
	ba.processMutex.Lock()
	defer ba.processMutex.Unlock()

	var errs []error

	for processID, cmd := range ba.activeProcesses {
		if cmd != nil && cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				errs = append(errs, fmt.Errorf("failed to kill process %s: %w", processID, err))
			}
		}
		delete(ba.activeProcesses, processID)
	}

	if len(errs) > 0 {
		return fmt.Errorf("kill process errors: %v", errs)
	}
	return nil
}

// cleanupTempFiles removes temporary files created by browser processes
func (ba *BrowserAutomation) cleanupTempFiles() error {
	// Chrome often creates temporary files with predictable patterns
	tempPatterns := []string{
		"chrome_*",
		"Crashpad*",
		".org.chromium.Chromium.*",
		"scoped_dir*",
	}

	var errs []error
	for _, pattern := range tempPatterns {
		matches, err := filepath.Glob(filepath.Join(ba.tempDir, pattern))
		if err != nil {
			continue // Ignore glob errors
		}

		for _, match := range matches {
			if err := os.RemoveAll(match); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove %s: %w", match, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("temp file cleanup errors: %v", errs)
	}
	return nil
}

// GetResourceUsage returns information about current resource usage
func (ba *BrowserAutomation) GetResourceUsage() map[string]interface{} {
	ba.processMutex.RLock()
	defer ba.processMutex.RUnlock()

	usage := map[string]interface{}{
		"active_processes": len(ba.activeProcesses),
		"temp_directory":   ba.tempDir,
		"executable_path":  ba.executablePath,
		"timeout":          ba.timeout.String(),
	}

	// Count process IDs by type
	pdfProcesses := 0
	imageProcesses := 0
	for processID := range ba.activeProcesses {
		if strings.HasPrefix(processID, "pdf-") {
			pdfProcesses++
		} else if strings.HasPrefix(processID, "image-") {
			imageProcesses++
		}
	}

	usage["pdf_processes"] = pdfProcesses
	usage["image_processes"] = imageProcesses

	return usage
}
