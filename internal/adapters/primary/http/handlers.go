package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/export"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/microcosm-cc/bluemonday"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string    `json:"error"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// SlidesResponse represents the slides API response
type SlidesResponse struct {
	Title  string          `json:"title"`
	Author string          `json:"author,omitempty"`
	Date   string          `json:"date,omitempty"`
	Theme  string          `json:"theme"`
	Slides []SlideResponse `json:"slides"`
}

// SlideResponse represents a single slide in the API response
type SlideResponse struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	HTML  string `json:"html"`
	Notes string `json:"notes,omitempty"`
}

// ConfigResponse represents the configuration API response
type ConfigResponse struct {
	Version         string   `json:"version"`
	Theme           string   `json:"theme"`
	WebSocketURL    string   `json:"websocket_url"`
	LiveReload      bool     `json:"live_reload"`
	SupportedThemes []string `json:"supported_themes"`
}

// handlePresentation serves the main presentation HTML
func (s *Server) handlePresentation(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	// Get the current presentation
	presentation := s.GetPresentation()
	if presentation == nil {
		// No presentation loaded, show default
		presentation = &entities.Presentation{
			Title: "No Presentation Loaded",
			Theme: "default",
			Slides: []entities.Slide{
				{Index: 0, Title: "No presentation loaded", HTML: "<h1>No presentation loaded</h1><p>Please specify a presentation file.</p>"},
			},
		}
	}

	// Render the presentation
	html, err := s.renderer.RenderPresentation(ctx, presentation)
	if err != nil {
		s.handleError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(html); err != nil {
		s.logger.Error("Failed to write presentation response: %v", err)
	}
}

// handleSlides returns the slides data as JSON
func (s *Server) handleSlides(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the current presentation
	presentation := s.GetPresentation()
	if presentation == nil {
		// No presentation loaded
		presentation = &entities.Presentation{
			Title: "No Presentation Loaded",
			Theme: "default",
			Slides: []entities.Slide{
				{Index: 0, Title: "No presentation loaded", HTML: "<h1>No presentation loaded</h1>"},
			},
		}
	}

	response := s.presentationToResponse(presentation)
	s.writeJSON(w, response)
}

// handleConfig returns the server configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := ConfigResponse{
		Version:         "1.0.0",
		Theme:           "default",
		WebSocketURL:    "/ws",
		LiveReload:      true,
		SupportedThemes: []string{"default", "dark", "light"},
	}

	s.writeJSON(w, config)
}

// handleError handles error responses with sanitized messages
func (s *Server) handleError(w http.ResponseWriter, err error, status int) {
	// Sanitize error message to prevent information disclosure
	var message string
	switch status {
	case http.StatusBadRequest:
		message = "Invalid request"
	case http.StatusNotFound:
		message = "Resource not found"
	case http.StatusMethodNotAllowed:
		message = "Method not allowed"
	case http.StatusTooManyRequests:
		message = "Too many requests"
	case http.StatusInternalServerError:
		message = "Internal server error"
	default:
		message = "An error occurred"
	}

	// Log the actual error for debugging (server-side only)
	s.logger.Error("HTTP error (status %d): %v", status, err)

	response := ErrorResponse{
		Error:   http.StatusText(status),
		Message: message, // Use sanitized message instead of err.Error()
		Time:    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		s.logger.Error("Failed to encode error response: %v", encodeErr)
	}
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON response: %v", err)
		s.handleError(w, err, http.StatusInternalServerError)
	}
}

// createHTMLSanitizer creates a restrictive HTML sanitizer for slide content
func createHTMLSanitizer() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	// Allow basic text formatting
	p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowElements("p", "br", "hr")
	p.AllowElements("strong", "b", "em", "i", "u", "s", "mark")
	p.AllowElements("ul", "ol", "li")
	p.AllowElements("blockquote", "pre", "code")
	p.AllowElements("a").AllowAttrs("href").OnElements("a")
	p.AllowElements("img").AllowAttrs("src", "alt", "title").OnElements("img")
	p.AllowElements("table", "thead", "tbody", "tr", "th", "td")
	p.AllowElements("div", "span").AllowAttrs("class").OnElements("div", "span")

	// Allow safe attributes
	p.AllowAttrs("class", "id").OnElements("h1", "h2", "h3", "h4", "h5", "h6", "p", "div", "span")

	return p
}

var htmlSanitizer = createHTMLSanitizer()

// presentationToResponse converts a presentation to API response with sanitized HTML
func (s *Server) presentationToResponse(p *entities.Presentation) SlidesResponse {
	slides := make([]SlideResponse, len(p.Slides))
	for i, slide := range p.Slides {
		slides[i] = SlideResponse{
			Index: slide.Index,
			Title: htmlSanitizer.Sanitize(slide.Title), // Sanitize title
			HTML:  htmlSanitizer.Sanitize(slide.HTML),  // Sanitize HTML content
			Notes: htmlSanitizer.Sanitize(slide.Notes), // Sanitize notes
		}
	}

	dateStr := ""
	if !p.Date.IsZero() {
		dateStr = p.Date.Format("2006-01-02")
	}

	return SlidesResponse{
		Title:  htmlSanitizer.Sanitize(p.Title),  // Sanitize title
		Author: htmlSanitizer.Sanitize(p.Author), // Sanitize author
		Date:   dateStr,
		Theme:  p.Theme, // Theme is controlled server-side, safe
		Slides: slides,
	}
}

// handlePresenterView serves the presenter mode interface
func (s *Server) handlePresenterView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get the current presentation
	presentation := s.GetPresentation()
	if presentation == nil {
		// No presentation loaded, show default
		presentation = &entities.Presentation{
			Title: "No Presentation Loaded",
			Theme: "default",
			Slides: []entities.Slide{
				{Index: 0, Title: "No presentation loaded", HTML: "<h1>No presentation loaded</h1><p>Please specify a presentation file.</p>"},
			},
		}
	}

	// Check if renderer supports presenter mode
	type PresenterRenderer interface {
		RenderPresenter(ctx context.Context, p *entities.Presentation) ([]byte, error)
	}

	presenterRenderer, ok := s.renderer.(PresenterRenderer)
	if !ok {
		// Fallback to basic presenter view
		http.Error(w, "Presenter mode not supported by renderer", http.StatusServiceUnavailable)
		return
	}

	// Render the presenter interface
	html, err := presenterRenderer.RenderPresenter(ctx, presentation)
	if err != nil {
		s.handleError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(html); err != nil {
		s.logger.Error("Failed to write presenter response: %v", err)
	}
}

// handlePresenterState returns the current presenter state
func (s *Server) handlePresenterState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have a sync service
	s.mu.RLock()
	syncService := s.syncService
	s.mu.RUnlock()

	if syncService == nil {
		http.Error(w, "Presenter mode not available", http.StatusServiceUnavailable)
		return
	}

	state := syncService.GetState()
	s.writeJSON(w, state)
}

// handlePresenterNotes handles speaker notes operations
func (s *Server) handlePresenterNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetPresenterNotes(w, r)
	case http.MethodPost:
		s.handleSetPresenterNotes(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetPresenterNotes retrieves notes for a specific slide
func (s *Server) handleGetPresenterNotes(w http.ResponseWriter, r *http.Request) {
	slideID := r.URL.Query().Get("slideId")
	if slideID == "" {
		http.Error(w, "slideId parameter required", http.StatusBadRequest)
		return
	}

	// For now, return empty notes - this will be integrated with NotesService later
	notes := map[string]interface{}{
		"slideId": slideID,
		"content": "",
		"html":    "",
	}

	s.writeJSON(w, notes)
}

// handleSetPresenterNotes sets notes for a specific slide
func (s *Server) handleSetPresenterNotes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SlideID string `json:"slideId"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SlideID == "" {
		http.Error(w, "slideId is required", http.StatusBadRequest)
		return
	}

	// For now, just return success - this will be integrated with NotesService later
	response := map[string]interface{}{
		"slideId": req.SlideID,
		"content": req.Content,
		"status":  "saved",
	}

	s.writeJSON(w, response)
}

// handlePresenterNavigate handles navigation commands from the presenter
func (s *Server) handlePresenterNavigate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"`
		Slide  int    `json:"slide,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if we have a sync service
	s.mu.RLock()
	syncService := s.syncService
	s.mu.RUnlock()

	if syncService == nil {
		http.Error(w, "Presenter mode not available", http.StatusServiceUnavailable)
		return
	}

	// Create navigation event data
	eventData := map[string]interface{}{
		"action": req.Action,
	}

	if req.Slide >= 0 {
		eventData["slide"] = float64(req.Slide)
	}

	// Create and broadcast sync event
	syncEvent := entities.NewSyncEvent("navigation", eventData)
	if err := syncService.Broadcast(syncEvent); err != nil {
		s.handleError(w, err, http.StatusInternalServerError)
		return
	}

	// Return updated state
	state := syncService.GetState()
	s.writeJSON(w, state)
}

// handlePresenterTimer handles timer control commands from the presenter
func (s *Server) handlePresenterTimer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate action
	validActions := map[string]bool{
		"pause":  true,
		"resume": true,
		"reset":  true,
	}

	if !validActions[req.Action] {
		http.Error(w, "Invalid timer action", http.StatusBadRequest)
		return
	}

	// Check if we have a sync service
	s.mu.RLock()
	syncService := s.syncService
	s.mu.RUnlock()

	if syncService == nil {
		http.Error(w, "Presenter mode not available", http.StatusServiceUnavailable)
		return
	}

	// Create and broadcast timer event
	syncEvent := entities.NewSyncEvent("timer", map[string]interface{}{
		"action": req.Action,
	})

	if err := syncService.Broadcast(syncEvent); err != nil {
		s.handleError(w, err, http.StatusInternalServerError)
		return
	}

	// Return updated state
	state := syncService.GetState()
	s.writeJSON(w, state)
}

// handleExport handles presentation export requests
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Format          string                 `json:"format"`
		Theme           string                 `json:"theme,omitempty"`
		IncludeNotes    bool                   `json:"include_notes"`
		IncludeMetadata bool                   `json:"include_metadata"`
		Quality         string                 `json:"quality,omitempty"`
		PageSize        string                 `json:"page_size,omitempty"`
		Orientation     string                 `json:"orientation,omitempty"`
		Compression     bool                   `json:"compression"`
		Metadata        map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get the current presentation
	presentation := s.GetPresentation()
	if presentation == nil {
		http.Error(w, "No presentation loaded", http.StatusBadRequest)
		return
	}

	// Check if we have an export service
	s.mu.RLock()
	exportService := s.exportService
	s.mu.RUnlock()

	if exportService == nil {
		http.Error(w, "Export service not available", http.StatusServiceUnavailable)
		return
	}

	// Generate output path based on presentation title and format
	filename := fmt.Sprintf("%s.%s", presentation.Title, req.Format)
	if req.Format == "images" {
		filename = presentation.Title // Directory for images
	}
	outputPath := filepath.Join(exportService.GetTempDir(), filename)

	// Prepare export options
	options := &export.ExportOptions{
		Format:          export.ExportFormat(req.Format),
		OutputPath:      outputPath,
		Theme:           req.Theme,
		IncludeNotes:    req.IncludeNotes,
		IncludeMetadata: req.IncludeMetadata,
		Quality:         req.Quality,
		PageSize:        req.PageSize,
		Orientation:     req.Orientation,
		Compression:     req.Compression,
		Metadata:        req.Metadata,
	}

	// Perform export
	result, err := exportService.Export(r.Context(), presentation, options)
	if err != nil {
		s.handleError(w, err, http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, result)
}

// handleExportDownload handles downloading exported files
func (s *Server) handleExportDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get filename from query parameters
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "File parameter required", http.StatusBadRequest)
		return
	}

	// Check if we have an export service
	s.mu.RLock()
	exportService := s.exportService
	s.mu.RUnlock()

	if exportService == nil {
		http.Error(w, "Export service not available", http.StatusServiceUnavailable)
		return
	}

	// Construct file path (security: only allow files in temp directory)
	filePath := filepath.Join(exportService.GetTempDir(), filepath.Base(filename))

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Determine MIME type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	var mimeType string
	switch ext {
	case ".html":
		mimeType = "text/html"
	case ".pdf":
		mimeType = "application/pdf"
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".md":
		mimeType = "text/markdown"
	default:
		mimeType = "application/octet-stream"
	}

	// Set headers for download
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))

	// Serve file
	http.ServeFile(w, r, filePath)
}

// handleExportFormats returns available export formats
func (s *Server) handleExportFormats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have an export service
	s.mu.RLock()
	exportService := s.exportService
	s.mu.RUnlock()

	if exportService == nil {
		http.Error(w, "Export service not available", http.StatusServiceUnavailable)
		return
	}

	formats := exportService.GetSupportedFormats()
	response := map[string]interface{}{
		"formats": formats,
		"options": map[string]interface{}{
			"qualities":    []string{"low", "medium", "high"},
			"page_sizes":   []string{"A4", "Letter", "Custom"},
			"orientations": []string{"portrait", "landscape"},
		},
	}

	s.writeJSON(w, response)
}

// handlePerformanceHealth returns performance health status
func (s *Server) handlePerformanceHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have an optimization service
	s.mu.RLock()
	optimizationSvc := s.optimizationSvc
	s.mu.RUnlock()

	if optimizationSvc == nil {
		http.Error(w, "Performance monitoring not available", http.StatusServiceUnavailable)
		return
	}

	// Get health status
	monitor := optimizationSvc.GetPerformanceMonitor()
	healthStatus := monitor.GetHealthStatus()

	s.writeJSON(w, healthStatus)
}

// handlePerformanceMetrics returns detailed performance metrics
func (s *Server) handlePerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have an optimization service
	s.mu.RLock()
	optimizationSvc := s.optimizationSvc
	s.mu.RUnlock()

	if optimizationSvc == nil {
		http.Error(w, "Performance monitoring not available", http.StatusServiceUnavailable)
		return
	}

	// Get comprehensive metrics
	monitor := optimizationSvc.GetPerformanceMonitor()
	metrics := monitor.GetMetrics()
	memoryStats := monitor.GetMemoryStats()
	optimizationStats := optimizationSvc.GetOptimizationStats()

	// Create a JSON-safe metrics structure
	metricsResponse := map[string]interface{}{
		"app_start_time":        metrics.AppStartTime,
		"last_operation_time":   metrics.LastOperationTime,
		"plugin_load_duration":  metrics.PluginLoadDuration,
		"render_duration":       metrics.RenderDuration,
		"server_start_duration": metrics.ServerStartDuration,
		"memory_usage":          metrics.MemoryUsage,
		"goroutine_count":       metrics.GoroutineCount,
		"heap_size":             metrics.HeapSize,
		"stack_size":            metrics.StackSize,
		"gc_count":              metrics.GCCount,
		"slide_render_count":    metrics.SlideRenderCount,
		"plugin_executions":     metrics.PluginExecutions,
		"http_requests":         metrics.HTTPRequests,
		"websocket_connections": metrics.WebSocketConnections,
		"average_render_time":   metrics.AverageRenderTime,
		"plugin_cache_hit_rate": metrics.PluginCacheHitRate,
		"memory_growth_rate":    metrics.MemoryGrowthRate,
	}

	response := map[string]interface{}{
		"metrics":      metricsResponse,
		"memory":       memoryStats,
		"optimization": optimizationStats,
		"timestamp":    time.Now(),
	}

	s.writeJSON(w, response)
}

// handlePerformanceOptimize triggers immediate optimization
func (s *Server) handlePerformanceOptimize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have an optimization service
	s.mu.RLock()
	optimizationSvc := s.optimizationSvc
	s.mu.RUnlock()

	if optimizationSvc == nil {
		http.Error(w, "Performance optimization not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request for optimization type
	var req struct {
		Type string `json:"type,omitempty"` // "gc", "memory", "cache", "all"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to full optimization if no body
		req.Type = "all"
	}

	// Get current mode from query parameters for context
	mode := r.URL.Query().Get("mode") // "presentation" or "development"

	// Apply mode-specific optimizations
	switch mode {
	case "presentation":
		optimizationSvc.TuneForPresentation()
	case "development":
		optimizationSvc.TuneForDevelopment()
	}

	// Trigger optimization based on type
	switch req.Type {
	case "gc":
		monitor := optimizationSvc.GetPerformanceMonitor()
		monitor.TriggerGC()
	case "cache":
		executor := optimizationSvc.GetConcurrentExecutor()
		executor.ClearExpiredCache()
	case "all", "":
		optimizationSvc.ForceOptimization()
	default:
		http.Error(w, "Invalid optimization type", http.StatusBadRequest)
		return
	}

	// Return updated metrics
	monitor := optimizationSvc.GetPerformanceMonitor()
	healthStatus := monitor.GetHealthStatus()

	response := map[string]interface{}{
		"status":    "optimization_triggered",
		"type":      req.Type,
		"mode":      mode,
		"health":    healthStatus,
		"timestamp": time.Now(),
	}

	s.writeJSON(w, response)
}
