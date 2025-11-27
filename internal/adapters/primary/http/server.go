package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/cors"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/optimization"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

const (
	// DefaultShutdownTimeout is the maximum time to wait for active connections
	// to complete during graceful shutdown. 5 seconds allows most in-flight requests
	// to complete while preventing indefinite hangs during shutdown.
	DefaultShutdownTimeout = 5 * time.Second

	// serverReadTimeout is the maximum duration for reading the entire request.
	// 15 seconds accommodates slow networks and large presentation uploads
	// while preventing resource exhaustion from idle connections.
	serverReadTimeout = 15 * time.Second

	// serverWriteTimeout is the maximum duration before timing out writes of the response.
	// 15 seconds allows for large presentation rendering and file responses.
	serverWriteTimeout = 15 * time.Second

	// serverIdleTimeout is the maximum time to wait for the next request when
	// keep-alives are enabled. 60 seconds allows for efficient connection reuse
	// during active presentation viewing.
	serverIdleTimeout = 60 * time.Second

	// corsMaxAge is the maximum time (in seconds) that preflight request results
	// can be cached by the browser. 5 minutes (300 seconds) reduces preflight
	// overhead while allowing reasonable flexibility for CORS policy updates.
	corsMaxAge = 300
)

// HTTPLogger provides structured logging for the HTTP server
type HTTPLogger struct {
	component string
	verbose   bool
	level     entities.LogLevel
}

// NewHTTPLogger creates a new HTTP logger instance
func NewHTTPLogger(component string, verbose bool) *HTTPLogger {
	return &HTTPLogger{
		component: component,
		verbose:   verbose,
		level:     entities.LogLevelInfo, // Default level
	}
}

// NewHTTPLoggerWithLevel creates a new HTTP logger instance with specific level
func NewHTTPLoggerWithLevel(component string, verbose bool, level entities.LogLevel) *HTTPLogger {
	return &HTTPLogger{
		component: component,
		verbose:   verbose,
		level:     level,
	}
}

// shouldLog checks if the message should be logged based on level
func (l *HTTPLogger) shouldLog(msgLevel entities.LogLevel) bool {
	levelMap := map[entities.LogLevel]int{
		entities.LogLevelDebug: 0,
		entities.LogLevelInfo:  1,
		entities.LogLevelWarn:  2,
		entities.LogLevelError: 3,
	}

	currentLevel := levelMap[l.level]
	messageLevel := levelMap[msgLevel]

	return messageLevel >= currentLevel
}

// Debug logs debug messages (only if debug level is enabled)
func (l *HTTPLogger) Debug(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelDebug) {
		log.Printf("[DEBUG] [%s] "+msg, append([]interface{}{l.component}, args...)...)
	}
}

// Info logs informational messages (only if info level or higher is enabled)
func (l *HTTPLogger) Info(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelInfo) {
		log.Printf("[INFO] [%s] "+msg, append([]interface{}{l.component}, args...)...)
	}
}

// Warn logs warning messages (only if warn level or higher is enabled)
func (l *HTTPLogger) Warn(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelWarn) {
		log.Printf("[WARN] [%s] "+msg, append([]interface{}{l.component}, args...)...)
	}
}

// Error logs error messages (always logged)
func (l *HTTPLogger) Error(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelError) {
		log.Printf("[ERROR] [%s] "+msg, append([]interface{}{l.component}, args...)...)
	}
}

// Success logs success messages (only if info level or higher is enabled)
func (l *HTTPLogger) Success(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelInfo) {
		log.Printf("[SUCCESS] [%s] "+msg, append([]interface{}{l.component}, args...)...)
	}
}

// SetLevel updates the logging level
func (l *HTTPLogger) SetLevel(level entities.LogLevel) {
	l.level = level
}

// Server implements the HTTPServer interface
type Server struct {
	server          *http.Server
	connMgr         *ConnectionManager
	rateLimiter     *rateLimiter
	presenter       ports.PresentationService
	renderer        ports.Renderer
	presentation    *entities.Presentation // Store current presentation
	syncService     ports.PresentationSync
	exportService   ports.ExportService
	optimizationSvc *optimization.OptimizationService
	config          *entities.ServerConfig // Store server configuration
	logger          *HTTPLogger            // Structured logger
	mu              sync.RWMutex
	running         bool
}

// NewServer creates a new HTTP server
// config must not be nil - use config.GetDefaultConfig().Server if needed
// Returns an error if config is nil
func NewServer(presenter ports.PresentationService, renderer ports.Renderer, config *entities.ServerConfig) (*Server, error) {
	if config == nil {
		return nil, errors.New("server config cannot be nil")
	}
	return &Server{
		presenter:   presenter,
		renderer:    renderer,
		connMgr:     NewConnectionManager(),
		rateLimiter: newRateLimiter(),
		config:      config,
		logger:      NewHTTPLogger("server", false), // Default logger, can be overridden
	}, nil
}

// NewServerWithLogging creates a new HTTP server with logging configuration
// Returns an error if config is nil
func NewServerWithLogging(presenter ports.PresentationService, renderer ports.Renderer, config *entities.ServerConfig, loggingConfig *entities.LoggingConfig) (*Server, error) {
	if config == nil {
		return nil, errors.New("server config cannot be nil")
	}

	level := entities.LogLevelInfo
	verbose := false

	if loggingConfig != nil {
		level = loggingConfig.GetLevel()
		verbose = loggingConfig.Verbose
	}

	return &Server{
		presenter:   presenter,
		renderer:    renderer,
		connMgr:     NewConnectionManager(),
		rateLimiter: newRateLimiter(),
		config:      config,
		logger:      NewHTTPLoggerWithLevel("server", verbose, level),
	}, nil
}

// SetLogger sets the HTTP logger with verbose configuration
func (s *Server) SetLogger(verbose bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = NewHTTPLogger("server", verbose)
}

// SetSyncService sets the presentation sync service
func (s *Server) SetSyncService(syncService ports.PresentationSync) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncService = syncService
}

// SetExportService sets the export service
func (s *Server) SetExportService(exportService ports.ExportService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exportService = exportService
}

// SetOptimizationService sets the optimization service
func (s *Server) SetOptimizationService(optimizationSvc *optimization.OptimizationService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.optimizationSvc = optimizationSvc
}

// SetPresentation sets the current presentation
func (s *Server) SetPresentation(p *entities.Presentation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presentation = p
}

// GetPresentation returns the current presentation
func (s *Server) GetPresentation() *entities.Presentation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.presentation
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context, port int, host string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("server already running")
	}

	// Start connection manager
	go s.connMgr.Run(ctx)

	// Start optimization service if available
	if s.optimizationSvc != nil {
		if err := s.optimizationSvc.Start(ctx); err != nil {
			s.logger.Warn("Failed to start optimization service: %v", err)
		}
	}

	router := s.setupRoutes()

	// Add CORS middleware with configurable origins from config
	corsOrigins := s.config.GetCORSOrigins()

	c := cors.New(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Accept"},
		AllowCredentials: false,
		MaxAge:           corsMaxAge,
	})
	handler := c.Handler(router)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      handler,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}
	s.running = true
	s.mu.Unlock()

	// Start server in goroutine
	go func() {
		s.logger.Info("HTTP server starting on %s:%d", host, port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return errors.New("server not running")
	}

	// Close all WebSocket connections
	s.connMgr.CloseAll()

	// Stop rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}

	// Stop optimization service if available
	if s.optimizationSvc != nil {
		s.optimizationSvc.Stop()
	}

	// Shutdown HTTP server with graceful timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, DefaultShutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	s.running = false
	return nil
}

// NotifyClients sends an update event to all connected clients
func (s *Server) NotifyClients(event ports.UpdateEvent) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return errors.New("server not running")
	}

	s.connMgr.Broadcast(event)
	return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// API endpoints
	mux.HandleFunc("/api/slides", s.handleSlides)
	mux.HandleFunc("/api/config", s.handleConfig)

	// Presenter API endpoints
	mux.HandleFunc("/api/presenter/state", s.handlePresenterState)
	mux.HandleFunc("/api/presenter/notes", s.handlePresenterNotes)
	mux.HandleFunc("/api/presenter/navigate", s.handlePresenterNavigate)
	mux.HandleFunc("/api/presenter/timer", s.handlePresenterTimer)

	// Export API endpoints
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/export/formats", s.handleExportFormats)
	mux.HandleFunc("/api/export/download", s.handleExportDownload)

	// Performance monitoring endpoints
	mux.HandleFunc("/api/performance/health", s.handlePerformanceHealth)
	mux.HandleFunc("/api/performance/metrics", s.handlePerformanceMetrics)
	mux.HandleFunc("/api/performance/optimize", s.handlePerformanceOptimize)

	// Presentation endpoints
	mux.HandleFunc("/presenter", s.handlePresenterView)
	mux.HandleFunc("/", s.handlePresentation)

	// Static files with path validation
	mux.Handle("/assets/", http.StripPrefix("/assets/", s.secureFileServer("web/assets")))

	// Apply middleware in order: security -> rate limiting -> logging -> recovery
	handler := securityHeadersMiddleware(mux)
	handler = createRateLimitMiddleware(handler, s.rateLimiter)
	handler = createLoggingMiddleware(handler, s.logger)
	handler = createRecoveryMiddleware(handler, s.logger)

	return handler
}

// secureFileServer creates a secure file server that prevents path traversal
func (s *Server) secureFileServer(root string) http.Handler {
	fs := http.FileServer(http.Dir(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path to prevent traversal
		cleanPath := filepath.Clean(r.URL.Path)

		// Ensure the path doesn't contain .. or other suspicious patterns
		if strings.Contains(cleanPath, "..") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if the requested file exists and is within the root directory
		fullPath := filepath.Join(root, cleanPath)
		absRoot, err := filepath.Abs(root)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Ensure the absolute path is within the root directory
		if !strings.HasPrefix(absPath, absRoot) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if file exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// Set security headers for static files
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=3600")

		// Serve the file
		fs.ServeHTTP(w, r)
	})
}
