package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// ExportFormat represents different export formats
type ExportFormat string

const (
	FormatPDF        ExportFormat = "pdf"
	FormatHTML       ExportFormat = "html"
	FormatImages     ExportFormat = "images"
	FormatMarkdown   ExportFormat = "markdown"
	FormatPowerPoint ExportFormat = "pptx"
)

// ExportOptions contains configuration for export operations
type ExportOptions struct {
	Format          ExportFormat           `json:"format"`
	OutputPath      string                 `json:"output_path"`
	Theme           string                 `json:"theme,omitempty"`
	IncludeNotes    bool                   `json:"include_notes"`
	IncludeMetadata bool                   `json:"include_metadata"`
	Quality         string                 `json:"quality,omitempty"`     // low, medium, high
	PageSize        string                 `json:"page_size,omitempty"`   // A4, Letter, Custom
	Orientation     string                 `json:"orientation,omitempty"` // portrait, landscape
	Compression     bool                   `json:"compression"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ExportResult contains the results of an export operation
type ExportResult struct {
	Success     bool                   `json:"success"`
	Format      string                 `json:"format"`
	OutputPath  string                 `json:"output_path"`
	FileSize    int64                  `json:"file_size"`
	Duration    string                 `json:"duration"`
	PageCount   int                    `json:"page_count,omitempty"`
	Files       []string               `json:"files,omitempty"` // For multi-file exports
	Error       string                 `json:"error,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	GeneratedAt time.Time              `json:"generated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // Enhanced metadata including metrics
}

// ExportErrorType categorizes different types of export errors
type ExportErrorType string

const (
	ErrorTypeValidation    ExportErrorType = "validation"
	ErrorTypeRenderer      ExportErrorType = "renderer"
	ErrorTypeBrowser       ExportErrorType = "browser"
	ErrorTypeFilesystem    ExportErrorType = "filesystem"
	ErrorTypeTimeout       ExportErrorType = "timeout"
	ErrorTypeMemory        ExportErrorType = "memory"
	ErrorTypeNetwork       ExportErrorType = "network"
	ErrorTypeConfiguration ExportErrorType = "configuration"
)

// ExportError provides detailed error information with categorization
type ExportError struct {
	Type      ExportErrorType `json:"type"`
	Message   string          `json:"message"`
	Details   string          `json:"details,omitempty"`
	Code      string          `json:"code,omitempty"`
	Retryable bool            `json:"retryable"`
	Cause     error           `json:"-"`
}

func (e *ExportError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s error: %s - %s", e.Type, e.Message, e.Details)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

func (e *ExportError) Unwrap() error {
	return e.Cause
}

// FallbackInfo tracks fallback information when primary methods fail
type FallbackInfo struct {
	Reason       string    `json:"reason"`
	FallbackUsed string    `json:"fallback_used"`
	Timestamp    time.Time `json:"timestamp"`
}

// ExportMetrics tracks performance and reliability metrics
type ExportMetrics struct {
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	Duration         time.Duration  `json:"duration"`
	RetryCount       int            `json:"retry_count"`
	FallbacksUsed    []FallbackInfo `json:"fallbacks_used,omitempty"`
	MemoryUsage      int64          `json:"memory_usage,omitempty"`
	TempFilesCreated []string       `json:"temp_files_created,omitempty"`
	Warnings         []string       `json:"warnings,omitempty"`
}

// RetryConfig defines retry behavior for export operations
type RetryConfig struct {
	MaxRetries      int               `json:"max_retries"`
	InitialDelay    time.Duration     `json:"initial_delay"`
	MaxDelay        time.Duration     `json:"max_delay"`
	BackoffFactor   float64           `json:"backoff_factor"`
	RetryableErrors []ExportErrorType `json:"retryable_errors"`
}

// Service implements export functionality
type Service struct {
	renderers    map[ExportFormat]Renderer
	tmpDir       string
	retryConfig  RetryConfig
	metrics      map[string]*ExportMetrics     // Track metrics per export operation
	browsers     map[string]*BrowserAutomation // Track browser automation instances
	browserMutex sync.RWMutex                  // Protect concurrent access to browsers
}

// Renderer interface for different export formats
type Renderer interface {
	Render(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error)
	Supports(format ExportFormat) bool
	GetMimeType() string
}

// NewService creates a new export service
func NewService(tmpDir string) (*Service, error) {
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}

	// Ensure tmp directory exists
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		return nil, &ExportError{
			Type:      ErrorTypeFilesystem,
			Message:   "failed to create temporary directory",
			Details:   tmpDir,
			Retryable: false,
			Cause:     err,
		}
	}

	// Default retry configuration
	retryConfig := RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []ExportErrorType{
			ErrorTypeNetwork,
			ErrorTypeTimeout,
			ErrorTypeBrowser,
			ErrorTypeMemory,
		},
	}

	service := &Service{
		renderers:    make(map[ExportFormat]Renderer),
		tmpDir:       tmpDir,
		retryConfig:  retryConfig,
		metrics:      make(map[string]*ExportMetrics),
		browsers:     make(map[string]*BrowserAutomation),
		browserMutex: sync.RWMutex{},
	}

	// Register default renderers
	service.RegisterRenderer(FormatHTML, NewHTMLRenderer())
	service.RegisterRenderer(FormatPDF, NewPDFRenderer())
	service.RegisterRenderer(FormatImages, NewImageRenderer())
	service.RegisterRenderer(FormatMarkdown, NewMarkdownRenderer())

	return service, nil
}

// RegisterRenderer registers a renderer for a specific format
func (s *Service) RegisterRenderer(format ExportFormat, renderer Renderer) {
	s.renderers[format] = renderer
}

// Export exports a presentation to the specified format with retry logic and comprehensive error handling
func (s *Service) Export(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error) {
	// Initialize metrics
	operationID := fmt.Sprintf("%s-%d", options.Format, time.Now().UnixNano())
	metrics := &ExportMetrics{
		StartTime:        time.Now(),
		TempFilesCreated: make([]string, 0),
		Warnings:         make([]string, 0),
		FallbacksUsed:    make([]FallbackInfo, 0),
	}
	s.metrics[operationID] = metrics

	// Validate options with detailed error categorization
	if err := s.validateOptionsDetailed(options); err != nil {
		metrics.EndTime = time.Now()
		metrics.Duration = time.Since(metrics.StartTime)
		return s.createErrorResult(err, metrics), err
	}

	// Get renderer for format
	renderer, exists := s.renderers[options.Format]
	if !exists {
		err := &ExportError{
			Type:      ErrorTypeConfiguration,
			Message:   "unsupported export format",
			Details:   string(options.Format),
			Code:      "UNSUPPORTED_FORMAT",
			Retryable: false,
		}
		metrics.EndTime = time.Now()
		metrics.Duration = time.Since(metrics.StartTime)
		return s.createErrorResult(err, metrics), err
	}

	// Ensure output directory exists
	if err := s.ensureOutputDirectory(options.OutputPath); err != nil {
		metrics.EndTime = time.Now()
		metrics.Duration = time.Since(metrics.StartTime)
		return s.createErrorResult(err, metrics), err
	}

	// Perform export with retry logic
	result, err := s.executeWithRetry(ctx, renderer, presentation, options, metrics)
	if err != nil {
		metrics.EndTime = time.Now()
		metrics.Duration = time.Since(metrics.StartTime)
		return s.createErrorResult(err, metrics), err
	}

	// Update metrics and result
	metrics.EndTime = time.Now()
	metrics.Duration = time.Since(metrics.StartTime)
	result.Duration = metrics.Duration.String()
	result.GeneratedAt = metrics.EndTime

	// Add metrics to result
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["export_metrics"] = metrics

	// Cleanup metrics after successful export
	delete(s.metrics, operationID)

	return result, nil
}

// GetSupportedFormats returns a list of supported export formats
func (s *Service) GetSupportedFormats() []ExportFormat {
	formats := make([]ExportFormat, 0, len(s.renderers))
	for format := range s.renderers {
		formats = append(formats, format)
	}
	return formats
}

// GetTempDir returns the temporary directory path
func (s *Service) GetTempDir() string {
	return s.tmpDir
}

// CreateTempFile creates a temporary file with the given extension
func (s *Service) CreateTempFile(prefix, extension string) (*os.File, error) {
	return os.CreateTemp(s.tmpDir, fmt.Sprintf("%s-*%s", prefix, extension))
}

// CleanupTempFiles removes temporary files older than the specified duration
func (s *Service) CleanupTempFiles(maxAge time.Duration) error {
	return filepath.Walk(s.tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if file is a temporary export file and is old enough
		if strings.Contains(info.Name(), "slicli-export-") && time.Since(info.ModTime()) > maxAge {
			return os.Remove(path)
		}

		return nil
	})
}

// validateOptions validates export options (legacy method for backward compatibility)
func (s *Service) validateOptions(options *ExportOptions) error {
	return s.validateOptionsDetailed(options)
}

// validateOptionsDetailed provides enhanced validation with detailed error categorization
func (s *Service) validateOptionsDetailed(options *ExportOptions) error {
	if options == nil {
		return &ExportError{
			Type:      ErrorTypeValidation,
			Message:   "export options cannot be nil",
			Code:      "NULL_OPTIONS",
			Retryable: false,
		}
	}

	if options.Format == "" {
		return &ExportError{
			Type:      ErrorTypeValidation,
			Message:   "export format is required",
			Code:      "MISSING_FORMAT",
			Retryable: false,
		}
	}

	if options.OutputPath == "" {
		return &ExportError{
			Type:      ErrorTypeValidation,
			Message:   "output path is required",
			Code:      "MISSING_OUTPUT_PATH",
			Retryable: false,
		}
	}

	// Validate quality setting
	if options.Quality != "" {
		validQualities := map[string]bool{"low": true, "medium": true, "high": true}
		if !validQualities[options.Quality] {
			return &ExportError{
				Type:      ErrorTypeValidation,
				Message:   "invalid quality setting",
				Details:   options.Quality + " (must be low, medium, or high)",
				Code:      "INVALID_QUALITY",
				Retryable: false,
			}
		}
	}

	// Validate page size
	if options.PageSize != "" {
		validSizes := map[string]bool{"A4": true, "Letter": true, "Custom": true}
		if !validSizes[options.PageSize] {
			return &ExportError{
				Type:      ErrorTypeValidation,
				Message:   "invalid page size",
				Details:   options.PageSize + " (must be A4, Letter, or Custom)",
				Code:      "INVALID_PAGE_SIZE",
				Retryable: false,
			}
		}
	}

	// Validate orientation
	if options.Orientation != "" {
		validOrientations := map[string]bool{"portrait": true, "landscape": true}
		if !validOrientations[options.Orientation] {
			return &ExportError{
				Type:      ErrorTypeValidation,
				Message:   "invalid orientation",
				Details:   options.Orientation + " (must be portrait or landscape)",
				Code:      "INVALID_ORIENTATION",
				Retryable: false,
			}
		}
	}

	return nil
}

// ensureOutputDirectory ensures the output directory exists
func (s *Service) ensureOutputDirectory(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return &ExportError{
			Type:      ErrorTypeFilesystem,
			Message:   "failed to create output directory",
			Details:   dir,
			Code:      "MKDIR_FAILED",
			Retryable: false,
			Cause:     err,
		}
	}
	return nil
}

// executeWithRetry executes the export with retry logic
func (s *Service) executeWithRetry(ctx context.Context, renderer Renderer, presentation *entities.Presentation, options *ExportOptions, metrics *ExportMetrics) (*ExportResult, error) {
	var lastErr error

	for attempt := 0; attempt <= s.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := s.calculateBackoffDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, &ExportError{
					Type:      ErrorTypeTimeout,
					Message:   "export cancelled during retry",
					Code:      "CANCELLED",
					Retryable: false,
					Cause:     ctx.Err(),
				}
			}
			metrics.RetryCount = attempt
		}

		// Attempt the export
		result, err := renderer.Render(ctx, presentation, options)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Categorize the error
		exportErr := s.categorizeError(err)

		// Check if error is retryable
		if !s.isRetryableError(exportErr) {
			break
		}

		// Log retry attempt
		metrics.Warnings = append(metrics.Warnings,
			fmt.Sprintf("Attempt %d failed: %s (retrying)", attempt+1, exportErr.Message))
	}

	return nil, s.categorizeError(lastErr)
}

// calculateBackoffDelay calculates the delay for exponential backoff
func (s *Service) calculateBackoffDelay(attempt int) time.Duration {
	delay := float64(s.retryConfig.InitialDelay) * math.Pow(s.retryConfig.BackoffFactor, float64(attempt-1))
	if delay > float64(s.retryConfig.MaxDelay) {
		delay = float64(s.retryConfig.MaxDelay)
	}
	return time.Duration(delay)
}

// isRetryableError checks if an error is retryable
func (s *Service) isRetryableError(err error) bool {
	var exportErr *ExportError
	if errors.As(err, &exportErr) {
		for _, retryableType := range s.retryConfig.RetryableErrors {
			if exportErr.Type == retryableType {
				return exportErr.Retryable
			}
		}
	}
	return false
}

// categorizeError categorizes an error into an ExportError
func (s *Service) categorizeError(err error) *ExportError {
	if err == nil {
		return nil
	}

	// If already an ExportError, return as-is
	var exportErr *ExportError
	if errors.As(err, &exportErr) {
		return exportErr
	}

	errMsg := err.Error()

	// Categorize based on error message patterns
	switch {
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline"):
		return &ExportError{
			Type:      ErrorTypeTimeout,
			Message:   "operation timed out",
			Details:   errMsg,
			Code:      "TIMEOUT",
			Retryable: true,
			Cause:     err,
		}
	case strings.Contains(errMsg, "chrome") || strings.Contains(errMsg, "browser") || strings.Contains(errMsg, "headless"):
		return &ExportError{
			Type:      ErrorTypeBrowser,
			Message:   "browser automation failed",
			Details:   errMsg,
			Code:      "BROWSER_ERROR",
			Retryable: true,
			Cause:     err,
		}
	case strings.Contains(errMsg, "memory") || strings.Contains(errMsg, "out of memory"):
		return &ExportError{
			Type:      ErrorTypeMemory,
			Message:   "insufficient memory",
			Details:   errMsg,
			Code:      "OUT_OF_MEMORY",
			Retryable: true,
			Cause:     err,
		}
	case strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "access"):
		return &ExportError{
			Type:      ErrorTypeFilesystem,
			Message:   "file access denied",
			Details:   errMsg,
			Code:      "ACCESS_DENIED",
			Retryable: false,
			Cause:     err,
		}
	case strings.Contains(errMsg, "network") || strings.Contains(errMsg, "connection"):
		return &ExportError{
			Type:      ErrorTypeNetwork,
			Message:   "network error",
			Details:   errMsg,
			Code:      "NETWORK_ERROR",
			Retryable: true,
			Cause:     err,
		}
	default:
		return &ExportError{
			Type:      ErrorTypeRenderer,
			Message:   "renderer error",
			Details:   errMsg,
			Code:      "RENDERER_ERROR",
			Retryable: false,
			Cause:     err,
		}
	}
}

// createErrorResult creates an error result with metrics
func (s *Service) createErrorResult(err error, metrics *ExportMetrics) *ExportResult {
	var exportErr *ExportError
	if !errors.As(err, &exportErr) {
		exportErr = s.categorizeError(err)
	}

	result := &ExportResult{
		Success:     false,
		Error:       exportErr.Error(),
		Duration:    metrics.Duration.String(),
		GeneratedAt: metrics.EndTime,
		Warnings:    metrics.Warnings,
		Metadata:    make(map[string]interface{}),
	}

	// Add error details to metadata
	result.Metadata["error_type"] = exportErr.Type
	result.Metadata["error_code"] = exportErr.Code
	result.Metadata["retryable"] = exportErr.Retryable
	result.Metadata["retry_count"] = metrics.RetryCount
	result.Metadata["export_metrics"] = metrics

	return result
}

// SetRetryConfig updates the retry configuration
func (s *Service) SetRetryConfig(config RetryConfig) {
	s.retryConfig = config
}

// GetRetryConfig returns the current retry configuration
func (s *Service) GetRetryConfig() RetryConfig {
	return s.retryConfig
}

// GetActiveExports returns information about currently running exports
func (s *Service) GetActiveExports() map[string]*ExportMetrics {
	activeExports := make(map[string]*ExportMetrics)
	for id, metrics := range s.metrics {
		if metrics.EndTime.IsZero() {
			activeExports[id] = metrics
		}
	}
	return activeExports
}

// GetExportStatistics returns comprehensive export statistics
func (s *Service) GetExportStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	// Get active exports count
	activeExports := s.GetActiveExports()
	stats["active_exports"] = len(activeExports)

	// Get retry configuration
	stats["retry_config"] = s.retryConfig

	// Get supported formats
	stats["supported_formats"] = s.GetSupportedFormats()

	// Get temp directory info
	stats["temp_directory"] = s.tmpDir

	// Get browser automation statistics
	s.browserMutex.RLock()
	stats["active_browsers"] = len(s.browsers)
	totalActiveProcesses := 0
	for _, browser := range s.browsers {
		totalActiveProcesses += browser.GetActiveProcessCount()
	}
	stats["total_browser_processes"] = totalActiveProcesses
	s.browserMutex.RUnlock()

	return stats
}

// RegisterBrowserAutomation registers a browser automation instance for tracking
func (s *Service) RegisterBrowserAutomation(id string, browser *BrowserAutomation) {
	s.browserMutex.Lock()
	defer s.browserMutex.Unlock()
	s.browsers[id] = browser
}

// UnregisterBrowserAutomation removes a browser automation instance from tracking
func (s *Service) UnregisterBrowserAutomation(id string) {
	s.browserMutex.Lock()
	defer s.browserMutex.Unlock()
	if browser, exists := s.browsers[id]; exists {
		// Cleanup the browser automation instance
		_ = browser.Cleanup()
		delete(s.browsers, id)
	}
}

// CleanupAllBrowsers cleans up all registered browser automation instances
func (s *Service) CleanupAllBrowsers() error {
	s.browserMutex.Lock()
	defer s.browserMutex.Unlock()

	var errs []error
	for id, browser := range s.browsers {
		if err := browser.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup browser %s: %w", id, err))
		}
		delete(s.browsers, id)
	}

	if len(errs) > 0 {
		return fmt.Errorf("browser cleanup errors: %v", errs)
	}
	return nil
}

// KillAllBrowserProcesses forcefully terminates all browser processes
func (s *Service) KillAllBrowserProcesses() error {
	s.browserMutex.RLock()
	defer s.browserMutex.RUnlock()

	var errs []error
	for id, browser := range s.browsers {
		if err := browser.KillActiveProcesses(); err != nil {
			errs = append(errs, fmt.Errorf("failed to kill processes for browser %s: %w", id, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("kill process errors: %v", errs)
	}
	return nil
}

// Cleanup performs comprehensive cleanup of all resources
func (s *Service) Cleanup() error {
	var errs []error

	// Cleanup all browser automation instances
	if err := s.CleanupAllBrowsers(); err != nil {
		errs = append(errs, fmt.Errorf("browser cleanup failed: %w", err))
	}

	// Cleanup temporary files
	if err := s.CleanupTempFiles(24 * time.Hour); err != nil {
		errs = append(errs, fmt.Errorf("temp file cleanup failed: %w", err))
	}

	// Clear active export metrics (they should be completed by now)
	s.metrics = make(map[string]*ExportMetrics)

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// GetBrowserResourceUsage returns resource usage for all browser instances
func (s *Service) GetBrowserResourceUsage() map[string]interface{} {
	s.browserMutex.RLock()
	defer s.browserMutex.RUnlock()

	usage := make(map[string]interface{})
	for id, browser := range s.browsers {
		usage[id] = browser.GetResourceUsage()
	}
	return usage
}

// GetFileSize returns the size of a file in bytes
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// CopyFile copies a file from src to dst with path validation
func CopyFile(src, dst string) error {
	// Validate file paths to prevent directory traversal
	if err := validateFilePath(src); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if err := validateFilePath(dst); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	sourceFile, err := os.Open(filepath.Clean(src)) // #nosec G304 - path validated above
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(filepath.Clean(dst)) // #nosec G304 - path validated above
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// validateFilePath validates a file path to prevent directory traversal attacks
func validateFilePath(path string) error {
	if path == "" {
		return errors.New("empty path")
	}

	// Check for directory traversal patterns before cleaning
	if strings.Contains(path, "..") {
		return errors.New("path contains directory traversal")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Ensure the path is absolute or relative within current working directory
	if filepath.IsAbs(cleanPath) {
		return nil // Absolute paths are allowed for export operations
	}

	// For relative paths, ensure they don't escape the working directory
	if strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return errors.New("invalid path format")
	}

	return nil
}
