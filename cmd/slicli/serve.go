package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/browser"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/config"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/sanitizer"
	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// External CDN URLs for third-party libraries.
// These are centralized here to make updates easier and to track dependencies.
const (
	// MermaidCDN is the CDN URL for the Mermaid.js diagram library
	MermaidCDN = "https://cdn.jsdelivr.net/npm/mermaid@10.6.1/dist/mermaid.min.js"
	// PrismCoreCDN is the CDN URL for the PrismJS core syntax highlighting library
	PrismCoreCDN = "https://unpkg.com/prismjs@1/components/prism-core.min.js"
	// PrismAutoloaderCDN is the CDN URL for the PrismJS language autoloader
	PrismAutoloaderCDN = "https://unpkg.com/prismjs@1/plugins/autoloader/prism-autoloader.min.js"
	// PrismThemeCDN is the CDN URL for the default PrismJS theme stylesheet
	PrismThemeCDN = "https://unpkg.com/prismjs@1/themes/prism.css"
)

// SlideData represents a single slide with its content and metadata
type SlideData struct {
	Index        int    `json:"index"`
	HTMLContent  string `json:"htmlContent"`
	SpeakerNotes string `json:"speakerNotes"`
	SlideClass   string `json:"slideClass"`
}

// PresentationData holds all slide data for presenter mode
type PresentationData struct {
	Slides     []SlideData `json:"slides"`
	TotalCount int         `json:"totalCount"`
	FilePath   string      `json:"filePath"`
	ThemeName  string      `json:"themeName"`
}

// WebSocketMessage represents a message sent over WebSocket for slide synchronization
type WebSocketMessage struct {
	Type       string `json:"type"`       // "slide_change", "presenter_ready", "audience_ready"
	SlideIndex int    `json:"slideIndex"` // Current slide index (1-based)
	Source     string `json:"source"`     // "presenter" or "audience"
}

// SlideHub manages WebSocket connections for slide synchronization
type SlideHub struct {
	clients      map[*websocket.Conn]string // conn -> source type
	broadcast    chan WebSocketMessage
	register     chan *websocket.Conn
	unregister   chan *websocket.Conn
	currentSlide int
	mu           sync.RWMutex
}

// NewSlideHub creates a new SlideHub for managing WebSocket connections
func NewSlideHub() *SlideHub {
	return &SlideHub{
		clients:      make(map[*websocket.Conn]string),
		broadcast:    make(chan WebSocketMessage, 256),
		register:     make(chan *websocket.Conn),
		unregister:   make(chan *websocket.Conn),
		currentSlide: 1,
	}
}

// Run starts the hub's main loop for managing connections and broadcasts
func (h *SlideHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = "unknown"
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.Lock()
			if msg.Type == "slide_change" {
				h.currentSlide = msg.SlideIndex
			}
			for conn := range h.clients {
				if err := conn.WriteJSON(msg); err != nil {
					conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.Unlock()
		}
	}
}

// SetClientType sets the type of a connected client (presenter or audience)
func (h *SlideHub) SetClientType(conn *websocket.Conn, clientType string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = clientType
}

// GetCurrentSlide returns the current slide index
func (h *SlideHub) GetCurrentSlide() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.currentSlide
}

// speakerNotesPattern matches HTML comments containing speaker notes
// Format: <!-- Notes: ... -->
var speakerNotesPattern = regexp.MustCompile(`(?s)<!--\s*Notes:\s*(.*?)\s*-->`)

// ErrPathTraversal is returned when a path contains traversal sequences
var ErrPathTraversal = errors.New("path traversal detected")

// ErrInvalidAssetName is returned when an asset name contains invalid characters
var ErrInvalidAssetName = errors.New("invalid asset name")

// validAssetNamePattern defines allowed characters in asset names
// Only alphanumeric, dashes, underscores, and dots are allowed
var validAssetNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// ValidateAssetName validates that an asset name is safe for file serving.
// It checks for:
// - Only alphanumeric characters, dashes, underscores, and dots
// - No path separators or ".." sequences
// - No null bytes or other control characters
func ValidateAssetName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidAssetName)
	}

	// Check for path traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("%w: contains '..'", ErrPathTraversal)
	}

	// Check for path separators
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("%w: contains path separator", ErrInvalidAssetName)
	}

	// Check for null bytes
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("%w: contains null byte", ErrInvalidAssetName)
	}

	// Validate against pattern
	if !validAssetNamePattern.MatchString(name) {
		return fmt.Errorf("%w: contains invalid characters", ErrInvalidAssetName)
	}

	return nil
}

// SafeFilePath constructs a safe file path and verifies it stays within the base directory.
// Returns the absolute path if safe, or an error if the path would escape the base directory.
func SafeFilePath(baseDir, requestedPath string) (string, error) {
	// Get absolute path of base directory
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base directory: %w", err)
	}

	// Construct the full path
	fullPath := filepath.Join(absBase, requestedPath)

	// Get absolute path of the result
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	// Clean both paths
	absBase = filepath.Clean(absBase)
	absPath = filepath.Clean(absPath)

	// Verify the resolved path is within the base directory
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("%w: path escapes base directory", ErrPathTraversal)
	}

	return absPath, nil
}

var (
	// Serve command flags
	port          int
	host          string
	noBrowser     bool
	themeName     string
	watchFiles    bool
	presenterPort int

	// Theme override flags
	overridePrimary     string
	overrideSecondary   string
	overrideBackground  string
	overrideText        string
	overrideFontHeading string
	overrideFontBody    string
	overrideFontSize    string
)

// ThemeOverrides holds CLI-specified theme customizations
type ThemeOverrides struct {
	Primary     string // Primary color (hex, e.g., #ff0000)
	Secondary   string // Secondary color (hex)
	Background  string // Background color (hex)
	Text        string // Text color (hex)
	FontHeading string // Heading font family
	FontBody    string // Body font family
	FontSize    string // Base font size (e.g., "18px")
}

// HasOverrides returns true if any override is set
func (t ThemeOverrides) HasOverrides() bool {
	return t.Primary != "" || t.Secondary != "" || t.Background != "" ||
		t.Text != "" || t.FontHeading != "" || t.FontBody != "" || t.FontSize != ""
}

// ToCSS generates CSS custom properties for the overrides.
// Only includes properties that were actually specified.
func (t ThemeOverrides) ToCSS() string {
	if !t.HasOverrides() {
		return ""
	}

	var cssVars []string
	if t.Primary != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-primary: %s;", t.Primary))
	}
	if t.Secondary != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-secondary: %s;", t.Secondary))
	}
	if t.Background != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-background: %s;", t.Background))
	}
	if t.Text != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-text: %s;", t.Text))
	}
	if t.FontHeading != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-font-heading: %s;", t.FontHeading))
	}
	if t.FontBody != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-font-body: %s;", t.FontBody))
	}
	if t.FontSize != "" {
		cssVars = append(cssVars, fmt.Sprintf("--override-font-size: %s;", t.FontSize))
	}

	return fmt.Sprintf(":root {\n    %s\n}", strings.Join(cssVars, "\n    "))
}

// collectThemeOverrides gathers theme overrides from CLI flags
func collectThemeOverrides() ThemeOverrides {
	return ThemeOverrides{
		Primary:     overridePrimary,
		Secondary:   overrideSecondary,
		Background:  overrideBackground,
		Text:        overrideText,
		FontHeading: overrideFontHeading,
		FontBody:    overrideFontBody,
		FontSize:    overrideFontSize,
	}
}

// Logger provides structured logging for the serve command
type Logger struct {
	verbose bool
	level   entities.LogLevel
}

// shouldLog checks if the message should be logged based on level
func (l *Logger) shouldLog(msgLevel entities.LogLevel) bool {
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

// Info logs informational messages
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelInfo) && l.verbose {
		log.Printf("[INFO] "+msg, args...)
	}
}

// Warn logs warning messages
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelWarn) {
		log.Printf("[WARN] "+msg, args...)
	}
}

// Error logs error messages (always shown if error level)
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelError) {
		log.Printf("[ERROR] "+msg, args...)
	}
}

// Success logs success messages
func (l *Logger) Success(msg string, args ...interface{}) {
	if l.shouldLog(entities.LogLevelInfo) && l.verbose {
		log.Printf("[SUCCESS] "+msg, args...)
	}
}

// newLogger creates a new logger instance (kept for potential future use)
// func newLogger(verbose bool) *Logger {
// 	return &Logger{
// 		verbose: verbose,
// 		level:   entities.LogLevelInfo, // Default level
// 	}
// }

// newLoggerWithLevel creates a new logger instance with specific level
func newLoggerWithLevel(verbose bool, level entities.LogLevel) *Logger {
	return &Logger{
		verbose: verbose,
		level:   level,
	}
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve [file]",
	Short: "Serve a presentation from a markdown file",
	Long: `Start a local HTTP server to display your markdown presentation.
The server includes live reload functionality and will automatically
update when the markdown file changes.

Example:
  slicli serve presentation.md
  slicli serve slides.md --port 8080 --no-browser`,
	Args: cobra.ExactArgs(1),
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Add command flags - defaults will be overridden by config loading
	serveCmd.Flags().IntVarP(&port, "port", "p", 0, "Port to serve on (overrides config)")
	serveCmd.Flags().StringVar(&host, "host", "", "Host to bind to (overrides config)")
	serveCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Don't open browser automatically (overrides config)")
	serveCmd.Flags().StringVarP(&themeName, "theme", "t", "", "Theme to use (overrides config)")
	serveCmd.Flags().BoolVarP(&watchFiles, "watch", "w", false, "Watch files for changes (overrides config)")
	serveCmd.Flags().IntVar(&presenterPort, "presenter-port", 0, "Port for presenter view (enables presenter mode)")

	// Theme override flags - allow customizing theme colors and fonts via CLI
	serveCmd.Flags().StringVar(&overridePrimary, "override-primary", "", "Override theme primary color (hex, e.g., #ff0000)")
	serveCmd.Flags().StringVar(&overrideSecondary, "override-secondary", "", "Override theme secondary color (hex, e.g., #00ff00)")
	serveCmd.Flags().StringVar(&overrideBackground, "override-background", "", "Override theme background color (hex, e.g., #ffffff)")
	serveCmd.Flags().StringVar(&overrideText, "override-text", "", "Override theme text color (hex, e.g., #333333)")
	serveCmd.Flags().StringVar(&overrideFontHeading, "override-font-heading", "", "Override heading font family (e.g., 'Arial, sans-serif')")
	serveCmd.Flags().StringVar(&overrideFontBody, "override-font-body", "", "Override body font family (e.g., 'Georgia, serif')")
	serveCmd.Flags().StringVar(&overrideFontSize, "override-font-size", "", "Override base font size (e.g., '18px')")
}

// validateServeArgs validates serve command arguments without starting server
func validateServeArgs(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
	}
	return nil
}

// validateServeConfig validates configuration after it's loaded
func validateServeConfig(config *entities.Config) error {
	// Port validation
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", config.Server.Port)
	}

	// Host validation
	if strings.Contains(config.Server.Host, " ") || strings.Contains(config.Server.Host, "!") {
		return fmt.Errorf("invalid host: %s", config.Server.Host)
	}

	return nil
}

func runServe(cmd *cobra.Command, args []string) error {
	presentationPath := args[0]

	// Load and validate configuration
	finalConfig, err := loadAndValidateConfig(cmd, presentationPath)
	if err != nil {
		return err
	}

	// Collect theme overrides from CLI flags
	themeOverrides := collectThemeOverrides()

	// Get verbose flag and create logger with logging configuration
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Override verbose setting from config if flag wasn't explicitly set
	if !cmd.Flags().Changed("verbose") {
		verbose = finalConfig.Logging.Verbose
	}

	logger := newLoggerWithLevel(verbose, finalConfig.Logging.GetLevel())
	printStartupInfo(logger, presentationPath, finalConfig)

	// Read markdown content
	markdownContent, err := os.ReadFile(presentationPath) // #nosec G304 - path validated
	if err != nil {
		return fmt.Errorf("reading presentation file: %w", err)
	}

	// Check if presenter mode is enabled
	presenterModeEnabled := presenterPort > 0

	var hub *SlideHub
	var htmlContent string

	if presenterModeEnabled {
		// Process markdown with speaker notes extraction
		presentationData := processMarkdownToSlidesWithNotes(string(markdownContent), presentationPath, finalConfig)

		// Create SlideHub for WebSocket synchronization
		hub = NewSlideHub()
		go hub.Run()

		// Generate audience HTML with sync support
		slidesHTML := generateSlidesHTMLFromData(presentationData)
		htmlContent = generateAudienceHTMLWithSync(slidesHTML, presentationPath, finalConfig, finalConfig.Server.Port, themeOverrides)

		// Create and start presenter server
		presenterServer := createPresenterHTTPServer(finalConfig, presentationData, hub, finalConfig.Server.Port)

		// Start presenter server in background
		go func() {
			logger.Info("Presenter view available at: http://%s:%d", finalConfig.Server.Host, presenterPort)
			if err := presenterServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Presenter server error: %v", err)
			}
		}()
	} else {
		// Standard mode without presenter features
		htmlContent = processMarkdownToSlides(string(markdownContent), presentationPath, finalConfig, themeOverrides)
	}

	// Create main HTTP server
	server := createHTTPServerWithHub(finalConfig, htmlContent, hub)

	// Start server and handle lifecycle
	return startAndManageServer(server, finalConfig, logger)
}

// generateSlidesHTMLFromData generates HTML slides from PresentationData
func generateSlidesHTMLFromData(data PresentationData) string {
	var htmlSlides []string
	for _, slide := range data.Slides {
		slideHTML := fmt.Sprintf(`<div class="slide %s" id="slide-%d">%s</div>`,
			slide.SlideClass, slide.Index, slide.HTMLContent)
		htmlSlides = append(htmlSlides, slideHTML)
	}
	return strings.Join(htmlSlides, "\n")
}

// loadAndValidateConfig loads configuration and validates it
func loadAndValidateConfig(cmd *cobra.Command, presentationPath string) (*entities.Config, error) {
	// Load configuration with proper precedence: CLI flags > local config > global config > defaults
	finalConfig, err := loadAndMergeConfig(cmd, presentationPath)
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}

	// Validate configuration
	if err := finalConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Additional serve-specific validation
	if err := validateServeConfig(finalConfig); err != nil {
		return nil, err
	}

	return finalConfig, nil
}

// printStartupInfo prints startup information if verbose mode is enabled
func printStartupInfo(logger *Logger, presentationPath string, config *entities.Config) {
	logger.Info("Starting server for presentation: %s", presentationPath)
	logger.Info("Attempting to start server at: http://%s:%d", config.Server.Host, config.Server.Port)
	if config.Browser.AutoOpen {
		logger.Info("Browser will open automatically if server starts successfully")
	}
	if config.Theme.Name != "" {
		logger.Info("Using theme: %s", config.Theme.Name)
	}
}

// loadPresentationContent validates and loads the presentation file content
func loadPresentationContent(presentationPath string, config *entities.Config, overrides ThemeOverrides) (string, error) {
	// Validate and read the presentation file
	fileInfo, err := os.Stat(presentationPath)
	if err != nil {
		return "", fmt.Errorf("accessing presentation file: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", fmt.Errorf("presentation path is not a regular file: %s", presentationPath)
	}

	// Read the validated presentation file
	markdownContent, err := os.ReadFile(presentationPath) // #nosec G304 - path validated above
	if err != nil {
		return "", fmt.Errorf("reading presentation file: %w", err)
	}

	// Process markdown into HTML slides
	return processMarkdownToSlides(string(markdownContent), presentationPath, config, overrides), nil
}

// createHTTPServer creates and configures the HTTP server with handlers
func createHTTPServer(config *entities.Config, htmlContent string) *http.Server {
	mux := http.NewServeMux()

	// Serve the presentation
	mux.HandleFunc("/", createPresentationHandler(htmlContent))

	// Serve static assets
	mux.HandleFunc("/assets/", createAssetsHandler())

	// Serve theme assets
	mux.HandleFunc("/themes/", createThemeAssetsHandler())

	// Create HTTP server using configuration values with security middleware
	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
		Handler:      securityMiddleware(mux),
		ReadTimeout:  config.Server.GetReadTimeout(),
		WriteTimeout: config.Server.GetWriteTimeout(),
		IdleTimeout:  60 * time.Second,
	}
}

// createHTTPServerWithHub creates the HTTP server with optional WebSocket support for presenter mode
func createHTTPServerWithHub(config *entities.Config, htmlContent string, hub *SlideHub) *http.Server {
	mux := http.NewServeMux()

	// Serve the presentation
	mux.HandleFunc("/", createPresentationHandler(htmlContent))

	// Serve static assets
	mux.HandleFunc("/assets/", createAssetsHandler())

	// Serve theme assets
	mux.HandleFunc("/themes/", createThemeAssetsHandler())

	// Add WebSocket handler if presenter mode is enabled (hub is provided)
	if hub != nil {
		mux.HandleFunc("/ws", createWebSocketHandler(hub))
	}

	// Choose security middleware based on whether WebSocket support is needed
	var handler http.Handler
	if hub != nil {
		handler = securityMiddlewareWithWebSocket(mux)
	} else {
		handler = securityMiddleware(mux)
	}

	// Create HTTP server using configuration values with security middleware
	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
		Handler:      handler,
		ReadTimeout:  config.Server.GetReadTimeout(),
		WriteTimeout: config.Server.GetWriteTimeout(),
		IdleTimeout:  60 * time.Second,
	}
}

// securityMiddlewareWithWebSocket adds security headers with WebSocket CSP support
func securityMiddlewareWithWebSocket(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable XSS filter in browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Content Security Policy with WebSocket support
		// Allow self, inline styles/scripts, CDN resources, and WebSocket connections
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"img-src 'self' data: https:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'")

		// Referrer policy for privacy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy (formerly Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// securityMiddleware adds security headers to all HTTP responses
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable XSS filter in browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Content Security Policy - restrict resource loading
		// Allow self, inline styles/scripts (needed for presentation), and CDN resources for Mermaid/Prism
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"img-src 'self' data: https:; "+
				"font-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'")

		// Referrer policy for privacy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy (formerly Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// createPresentationHandler creates the handler for serving presentation content
func createPresentationHandler(htmlContent string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(htmlContent)); err != nil {
			// Use a simple log format for the serve command's basic server
			log.Printf("[ERROR] Failed to write response: %v", err)
		}
	}
}

// createAssetsHandler creates the handler for serving static assets
func createAssetsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Security: Clean and validate the path
		cleanPath := filepath.Clean(r.URL.Path)

		// Prevent path traversal attacks
		if strings.Contains(cleanPath, "..") {
			http.NotFound(w, r)
			return
		}

		// Check if it's a known asset path from web/assets
		switch cleanPath {
		case "/assets/style.css":
			// For compatibility, serve default CSS if file doesn't exist
			cssPath := filepath.Join("web", "assets", "css", "main.css")
			if _, err := os.Stat(cssPath); err == nil {
				http.ServeFile(w, r, cssPath)
			} else {
				w.Header().Set("Content-Type", "text/css")
				css := getDefaultCSS()
				if _, err := w.Write([]byte(css)); err != nil {
					log.Printf("[ERROR] Failed to write CSS: %v", err)
				}
			}
		case "/assets/script.js":
			// For compatibility, serve default JS if file doesn't exist
			jsPath := filepath.Join("web", "assets", "js", "slicli.js")
			if _, err := os.Stat(jsPath); err == nil {
				http.ServeFile(w, r, jsPath)
			} else {
				w.Header().Set("Content-Type", "text/javascript")
				js := getDefaultJS()
				if _, err := w.Write([]byte(js)); err != nil {
					log.Printf("[ERROR] Failed to write JS: %v", err)
				}
			}
		default:
			// Parse the path components and validate each one
			pathAfterAssets := strings.TrimPrefix(cleanPath, "/assets/")
			if pathAfterAssets == "" || pathAfterAssets == cleanPath {
				http.NotFound(w, r)
				return
			}

			// Validate each path component
			pathParts := strings.Split(pathAfterAssets, "/")
			for _, part := range pathParts {
				if part == "" {
					continue
				}
				if err := ValidateAssetName(part); err != nil {
					log.Printf("[WARN] Invalid asset path component '%s': %v", part, err)
					http.NotFound(w, r)
					return
				}
			}

			// Use SafeFilePath to construct and validate the path
			safePath, err := SafeFilePath("web/assets", pathAfterAssets)
			if err != nil {
				log.Printf("[WARN] Path validation failed for '%s': %v", pathAfterAssets, err)
				http.NotFound(w, r)
				return
			}

			// Check if file exists and is not a directory
			fileInfo, err := os.Stat(safePath)
			if err != nil || fileInfo.IsDir() {
				http.NotFound(w, r)
				return
			}

			// Set appropriate content type
			setContentType(w, cleanPath)

			// Serve the file
			http.ServeFile(w, r, safePath)
		}
	}
}

// createThemeAssetsHandler creates the handler for serving theme assets
// It implements strict path validation to prevent directory traversal attacks.
func createThemeAssetsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Security: Clean the path
		cleanPath := filepath.Clean(r.URL.Path)

		// Prevent path traversal attacks
		if strings.Contains(cleanPath, "..") {
			http.NotFound(w, r)
			return
		}

		// Remove /themes/ prefix to get the actual theme path
		themePath := strings.TrimPrefix(cleanPath, "/themes/")
		if themePath == "" {
			http.NotFound(w, r)
			return
		}

		// Split the path into theme name and asset path
		parts := strings.SplitN(themePath, "/", 2)
		if len(parts) == 0 {
			http.NotFound(w, r)
			return
		}

		themeName := parts[0]
		var assetPath string
		if len(parts) > 1 {
			assetPath = parts[1]
		}

		// Validate theme name (only alphanumeric, dashes, underscores)
		if err := ValidateAssetName(themeName); err != nil {
			log.Printf("[WARN] Invalid theme name '%s': %v", themeName, err)
			http.NotFound(w, r)
			return
		}

		// Validate each component of the asset path
		if assetPath != "" {
			assetParts := strings.Split(assetPath, "/")
			for _, part := range assetParts {
				if part == "" {
					continue
				}
				if err := ValidateAssetName(part); err != nil {
					log.Printf("[WARN] Invalid asset path component '%s': %v", part, err)
					http.NotFound(w, r)
					return
				}
			}
		}

		// Possible theme base directories
		baseDirectories := []string{
			"themes", // Current directory
			filepath.Join(os.Getenv("HOME"), ".slicli", "themes"), // User home
		}

		var safePath string
		var found bool

		// Try each base directory with proper path validation
		for _, baseDir := range baseDirectories {
			// Construct the safe path and verify it's within the base directory
			candidatePath, err := SafeFilePath(filepath.Join(baseDir, themeName), assetPath)
			if err != nil {
				continue // Path validation failed, try next base
			}

			// Check if file exists and is not a directory
			fileInfo, err := os.Stat(candidatePath)
			if err != nil || fileInfo.IsDir() {
				continue // File doesn't exist or is directory, try next base
			}

			safePath = candidatePath
			found = true
			break
		}

		// If no valid path found, return 404
		if !found {
			http.NotFound(w, r)
			return
		}

		// Set appropriate content type
		setContentType(w, cleanPath)

		// Serve the file
		http.ServeFile(w, r, safePath)
	}
}

// setContentType sets the appropriate content type based on file extension
func setContentType(w http.ResponseWriter, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
}

// startAndManageServer starts the server and manages its lifecycle
func startAndManageServer(server *http.Server, config *entities.Config, logger *Logger) error {
	// Create channels for server status
	serverStarted := make(chan struct{})
	serverErr := make(chan error, 1)

	// Start server in a goroutine
	go startServerAsync(server, config, serverStarted, serverErr)

	// Wait for server to start and handle post-startup tasks
	if err := waitForServerStart(serverStarted, serverErr, config, logger); err != nil {
		return err
	}

	// Handle shutdown gracefully
	return handleServerShutdown(server, serverErr, config, logger)
}

// startServerAsync starts the server asynchronously with port validation
func startServerAsync(server *http.Server, config *entities.Config, serverStarted chan struct{}, serverErr chan error) {
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)

	// First, check if the port is already in use by attempting to listen on it
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		serverErr <- fmt.Errorf("port %d is already in use or cannot be bound: %w", config.Server.Port, err)
		return
	}

	// Close the test listener to avoid race condition
	if err := listener.Close(); err != nil {
		serverErr <- fmt.Errorf("failed to release port after testing: %w", err)
		return
	}

	// Create a new listener for the actual server to avoid race condition
	actualListener, err := net.Listen("tcp", addr)
	if err != nil {
		serverErr <- fmt.Errorf("failed to bind to port %d: %w", config.Server.Port, err)
		return
	}

	// Signal that we can proceed (port is bound and ready)
	close(serverStarted)

	// Now serve using the bound listener to eliminate race condition
	if err := server.Serve(actualListener); err != nil && err != http.ErrServerClosed {
		serverErr <- fmt.Errorf("server error: %w", err)
	}
}

// waitForServerStart waits for the server to start and handles post-startup tasks
func waitForServerStart(serverStarted chan struct{}, serverErr chan error, config *entities.Config, logger *Logger) error {
	select {
	case err := <-serverErr:
		return err
	case <-serverStarted:
		// Server has successfully started
		logger.Success("Server running at: http://%s:%d", config.Server.Host, config.Server.Port)

		// Open browser if configured
		if config.Browser.AutoOpen {
			openBrowserIfConfigured(config, logger)
		}
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("server failed to start within expected time")
	}
}

// openBrowserIfConfigured opens the browser if auto-open is enabled
func openBrowserIfConfigured(config *entities.Config, logger *Logger) {
	browserLauncher := browser.NewLauncher()
	url := fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)

	if err := browserLauncher.Launch(url, false); err != nil {
		logger.Warn("Failed to open browser: %v", err)
	}
}

// handleServerShutdown handles graceful server shutdown on signals
func handleServerShutdown(server *http.Server, serverErr chan error, config *entities.Config, logger *Logger) error {
	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-sigChan:
		logger.Info("\nShutting down server...")

		// Stop server gracefully using configured timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.Server.GetShutdownTimeout())
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Error during shutdown: %v", err)
			return fmt.Errorf("server shutdown: %w", err)
		}

		return nil
	}
}

// loadAndMergeConfig loads and merges configuration from multiple sources
func loadAndMergeConfig(cmd *cobra.Command, presentationPath string) (*entities.Config, error) {
	loader := config.NewTOMLLoader()
	ctx := context.Background()

	// Start with default configuration
	finalConfig := config.GetDefaultConfig()

	// Load global configuration if it exists
	globalConfig, err := loader.LoadGlobal(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}
	if globalConfig != nil {
		mergeConfigs(finalConfig, globalConfig)
	}

	// Load local configuration from presentation directory if it exists
	presentationDir := filepath.Dir(presentationPath)
	localConfig, err := loader.LoadLocal(ctx, presentationDir)
	if err != nil {
		return nil, fmt.Errorf("loading local config: %w", err)
	}
	if localConfig != nil {
		mergeConfigs(finalConfig, localConfig)
	}

	// Override with CLI flags if provided
	applyCliFlags(cmd, finalConfig)

	return finalConfig, nil
}

// mergeConfigs merges source config into target config (source takes precedence)
func mergeConfigs(target, source *entities.Config) {
	mergeServerConfig(target, source)
	mergeThemeConfig(target, source)
	mergeBrowserConfig(target, source)
	mergeWatcherConfig(target, source)
	mergePluginsConfig(target, source)
	mergeMetadataConfig(target, source)
}

// mergeServerConfig merges server configuration from source to target
func mergeServerConfig(target, source *entities.Config) {
	if source.Server.Host != "" {
		target.Server.Host = source.Server.Host
	}
	if source.Server.Port != 0 {
		target.Server.Port = source.Server.Port
	}
	if source.Server.ReadTimeout != 0 {
		target.Server.ReadTimeout = source.Server.ReadTimeout
	}
	if source.Server.WriteTimeout != 0 {
		target.Server.WriteTimeout = source.Server.WriteTimeout
	}
	if source.Server.ShutdownTimeout != 0 {
		target.Server.ShutdownTimeout = source.Server.ShutdownTimeout
	}
	if len(source.Server.CORSOrigins) > 0 {
		target.Server.CORSOrigins = source.Server.CORSOrigins
	}
}

// mergeThemeConfig merges theme configuration from source to target
func mergeThemeConfig(target, source *entities.Config) {
	if source.Theme.Name != "" {
		target.Theme.Name = source.Theme.Name
	}
	if source.Theme.CustomPath != "" {
		target.Theme.CustomPath = source.Theme.CustomPath
	}
}

// mergeBrowserConfig merges browser configuration from source to target
func mergeBrowserConfig(target, source *entities.Config) {
	target.Browser.AutoOpen = source.Browser.AutoOpen
	if source.Browser.Browser != "" {
		target.Browser.Browser = source.Browser.Browser
	}
}

// mergeWatcherConfig merges watcher configuration from source to target
func mergeWatcherConfig(target, source *entities.Config) {
	if source.Watcher.IntervalMs != 0 {
		target.Watcher.IntervalMs = source.Watcher.IntervalMs
	}
	if source.Watcher.DebounceMs != 0 {
		target.Watcher.DebounceMs = source.Watcher.DebounceMs
	}
	if source.Watcher.MaxRetries != 0 {
		target.Watcher.MaxRetries = source.Watcher.MaxRetries
	}
	if source.Watcher.RetryDelayMs != 0 {
		target.Watcher.RetryDelayMs = source.Watcher.RetryDelayMs
	}
}

// mergePluginsConfig merges plugins configuration from source to target
func mergePluginsConfig(target, source *entities.Config) {
	target.Plugins.Enabled = source.Plugins.Enabled
	if source.Plugins.Directory != "" {
		target.Plugins.Directory = source.Plugins.Directory
	}
	if len(source.Plugins.Whitelist) > 0 {
		target.Plugins.Whitelist = source.Plugins.Whitelist
	}
	if len(source.Plugins.Blacklist) > 0 {
		target.Plugins.Blacklist = source.Plugins.Blacklist
	}
}

// mergeMetadataConfig merges metadata configuration from source to target
func mergeMetadataConfig(target, source *entities.Config) {
	if source.Metadata.Author != "" {
		target.Metadata.Author = source.Metadata.Author
	}
	if source.Metadata.Email != "" {
		target.Metadata.Email = source.Metadata.Email
	}
	if source.Metadata.Company != "" {
		target.Metadata.Company = source.Metadata.Company
	}
	if len(source.Metadata.DefaultTags) > 0 {
		target.Metadata.DefaultTags = source.Metadata.DefaultTags
	}
	if len(source.Metadata.Custom) > 0 {
		if target.Metadata.Custom == nil {
			target.Metadata.Custom = make(map[string]string)
		}
		for k, v := range source.Metadata.Custom {
			target.Metadata.Custom[k] = v
		}
	}
}

// applyCliFlags applies CLI flag overrides to the configuration.
func applyCliFlags(cmd *cobra.Command, config *entities.Config) {
	// Apply CLI flag overrides (highest precedence)
	if cmd.Flags().Changed("port") {
		config.Server.Port = port
	}
	if cmd.Flags().Changed("host") {
		config.Server.Host = host
	}
	if cmd.Flags().Changed("no-browser") {
		config.Browser.AutoOpen = !noBrowser
	}
	if cmd.Flags().Changed("theme") {
		config.Theme.Name = themeName
	}
	// Note: watch flag is registered but not currently connected to config
	// The watch feature is handled separately if implemented
}

// processMarkdownToSlides converts markdown content to HTML slides
func processMarkdownToSlides(markdown, filePath string, config *entities.Config, overrides ThemeOverrides) string {
	// Split markdown by slide separator (---)
	slides := strings.Split(markdown, "\n---\n")

	var htmlSlides []string
	for i, slide := range slides {
		slideContent := strings.TrimSpace(slide)
		if slideContent == "" {
			continue
		}

		// Basic markdown to HTML conversion
		htmlContent := basicMarkdownToHTML(slideContent)

		// Determine slide type based on content
		slideClass := determineSlideClass(slideContent, i)

		// Wrap in slide div with proper classes
		slideHTML := fmt.Sprintf(`<div class="slide %s" id="slide-%d">%s</div>`, slideClass, i+1, htmlContent)

		htmlSlides = append(htmlSlides, slideHTML)
	}

	// Generate complete HTML page
	return generatePresentationHTML(strings.Join(htmlSlides, "\n"), filePath, config, overrides)
}

// basicMarkdownToHTML provides complete markdown to HTML conversion using Goldmark
func basicMarkdownToHTML(markdown string) string {
	// Configure Goldmark with extensions for full markdown support
	// SECURITY: Do not use html.WithUnsafe() - it allows XSS attacks
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,           // GitHub Flavored Markdown (tables, strikethrough, etc.)
			extension.Table,         // Tables support
			extension.Strikethrough, // ~~strikethrough~~ support
			extension.TaskList,      // - [ ] task list support
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Convert line breaks to <br>
			html.WithXHTML(),     // XHTML compliant output
			// SECURITY: Removed html.WithUnsafe() to prevent XSS
		),
	)

	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		log.Printf("[ERROR] Failed to convert markdown: %v", err)
		return markdown // Return original markdown on error
	}

	// Post-process for Mermaid diagrams
	htmlOutput := buf.String()
	htmlOutput = postProcessMermaidDiagrams(htmlOutput)

	// SECURITY: Sanitize HTML output to prevent XSS
	htmlOutput = sanitizer.SanitizeHTML(htmlOutput)

	return htmlOutput
}

// determineSlideClass determines the appropriate CSS class for a slide based on its content
func determineSlideClass(slideContent string, slideIndex int) string {
	lines := strings.Split(strings.TrimSpace(slideContent), "\n")
	if len(lines) == 0 {
		return "dev-content"
	}

	firstLine := strings.TrimSpace(lines[0])

	// First slide is typically a title slide
	if slideIndex == 0 {
		return "dev-title"
	}

	// Check if slide starts with a single H1 and has minimal content (section slide)
	if strings.HasPrefix(firstLine, "# ") {
		// Count meaningful content lines (non-empty, non-separator)
		contentLines := 0
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line != "" && line != "---" {
				contentLines++
			}
		}

		// If H1 with minimal content, it's likely a section header
		if contentLines <= 2 {
			return "dev-section"
		}
	}

	// Check for specific patterns that indicate section slides
	if strings.Contains(strings.ToLower(firstLine), "questions") ||
		strings.Contains(strings.ToLower(firstLine), "thank you") ||
		strings.Contains(strings.ToLower(firstLine), "demo") ||
		strings.Contains(strings.ToLower(firstLine), "roadmap") {
		return "dev-section"
	}

	// Default to content slide
	return "dev-content"
}

// postProcessMermaidDiagrams converts Goldmark's code blocks to Mermaid divs
func postProcessMermaidDiagrams(html string) string {
	// Pattern to match Goldmark's code blocks with mermaid language (multiline with DOTALL)
	mermaidPattern := regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)

	return mermaidPattern.ReplaceAllStringFunc(html, func(match string) string {
		// Extract the Mermaid content from the code block
		submatch := mermaidPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		mermaidContent := submatch[1]

		// Decode HTML entities that Goldmark encoded
		mermaidContent = strings.ReplaceAll(mermaidContent, "&lt;", "<")
		mermaidContent = strings.ReplaceAll(mermaidContent, "&gt;", ">")
		mermaidContent = strings.ReplaceAll(mermaidContent, "&amp;", "&")
		mermaidContent = strings.ReplaceAll(mermaidContent, "&#34;", `"`)
		mermaidContent = strings.ReplaceAll(mermaidContent, "&#39;", `'`)

		// Clean up extra whitespace but preserve structure
		mermaidContent = strings.TrimSpace(mermaidContent)

		// Escape content for HTML attribute
		escapedContent := strings.ReplaceAll(mermaidContent, `"`, `&quot;`)
		escapedContent = strings.ReplaceAll(escapedContent, `'`, `&#39;`)
		escapedContent = strings.ReplaceAll(escapedContent, "\n", "&#10;")

		// Return Mermaid div with data-original attribute
		return fmt.Sprintf(`<div class="mermaid" data-original="%s">%s</div>`, escapedContent, mermaidContent)
	})
}

// markdownProcessor handles the stateful markdown to HTML conversion
type markdownProcessor struct {
	html            *strings.Builder
	inCodeBlock     bool
	codeLanguage    string
	mermaidContent  strings.Builder // Store original Mermaid content
	inTable         bool
	isFirstTableRow bool
}

// processLine processes a single line of markdown
func (p *markdownProcessor) processLine(line string) {
	// Handle code block boundaries
	if strings.HasPrefix(line, "```") {
		p.handleCodeBlockBoundary(line)
		return
	}

	// Handle content inside code blocks
	if p.inCodeBlock {
		p.handleCodeBlockContent(line)
		return
	}

	// Handle regular markdown content
	p.handleRegularContent(line)
}

// handleCodeBlockBoundary handles the start and end of code blocks
func (p *markdownProcessor) handleCodeBlockBoundary(line string) {
	if p.inCodeBlock {
		// End of code block
		p.writeCodeBlockClosing()
		p.inCodeBlock = false
	} else {
		// Start of code block
		p.codeLanguage = strings.TrimPrefix(line, "```")
		if p.codeLanguage == "" {
			p.codeLanguage = "text"
		}
		p.writeCodeBlockOpening()
		p.inCodeBlock = true
	}
}

// writeCodeBlockOpening writes the opening tags for code blocks
func (p *markdownProcessor) writeCodeBlockOpening() {
	if p.codeLanguage == "mermaid" {
		// For Mermaid, we'll store the content first, then create the div
		p.html.WriteString(`<div class="mermaid" data-original="">`)
	} else {
		_, _ = fmt.Fprintf(p.html, `<pre class="code-block %s"><code>`, p.codeLanguage)
	}
}

// writeCodeBlockClosing writes the closing tags for code blocks
func (p *markdownProcessor) writeCodeBlockClosing() {
	if p.codeLanguage == "mermaid" {
		// Get the collected Mermaid content and update the opening div
		mermaidContent := strings.TrimSpace(p.mermaidContent.String())

		// Escape quotes for HTML attribute
		escapedContent := strings.ReplaceAll(mermaidContent, `"`, `&quot;`)
		escapedContent = strings.ReplaceAll(escapedContent, `'`, `&#39;`)

		// Replace the empty data-original with the actual content
		currentHTML := p.html.String()
		updatedHTML := strings.Replace(currentHTML, `data-original=""`, `data-original="`+escapedContent+`"`, 1)

		// Reset and write the updated HTML
		p.html.Reset()
		p.html.WriteString(updatedHTML)

		// Reset Mermaid content for next diagram
		p.mermaidContent.Reset()

		p.html.WriteString("</div>\n")
	} else {
		p.html.WriteString("</code></pre>\n")
	}
}

// handleCodeBlockContent handles content inside code blocks
func (p *markdownProcessor) handleCodeBlockContent(line string) {
	if p.codeLanguage == "mermaid" {
		// For Mermaid, collect content for data attribute but also write as-is
		p.mermaidContent.WriteString(line + "\n")
		p.html.WriteString(line + "\n")
	} else {
		// For other code blocks, escape HTML entities
		escaped := strings.ReplaceAll(line, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		p.html.WriteString(escaped + "\n")
	}
}

// handleRegularContent handles regular markdown content (headers, paragraphs, lists)
func (p *markdownProcessor) handleRegularContent(line string) {
	if line == "" {
		return
	}

	// Handle headers (h3, h2, h1 in priority order)
	if strings.HasPrefix(line, "### ") {
		content := processInlineMarkdown(strings.TrimPrefix(line, "### "))
		p.html.WriteString("<h3>" + content + "</h3>\n")
	} else if strings.HasPrefix(line, "## ") {
		content := processInlineMarkdown(strings.TrimPrefix(line, "## "))
		p.html.WriteString("<h2>" + content + "</h2>\n")
	} else if strings.HasPrefix(line, "# ") {
		content := processInlineMarkdown(strings.TrimPrefix(line, "# "))
		p.html.WriteString("<h1>" + content + "</h1>\n")
	} else if strings.HasPrefix(line, "- ") {
		// Handle list items
		content := processInlineMarkdown(strings.TrimPrefix(line, "- "))
		p.html.WriteString("<li>" + content + "</li>\n")
	} else if strings.Contains(line, "|") && strings.Count(line, "|") >= 2 {
		// Handle table rows
		p.handleTableRow(line)
	} else {
		// End table if we were in one
		if p.inTable {
			p.html.WriteString("</table>\n")
			p.inTable = false
		}
		// Regular paragraph
		content := processInlineMarkdown(line)
		p.html.WriteString("<p>" + content + "</p>\n")
	}
}

// handleTableRow handles markdown table rows
func (p *markdownProcessor) handleTableRow(line string) {
	// Skip separator rows (rows with only |, -, and spaces)
	if regexp.MustCompile(`^[\|\-\s]*$`).MatchString(line) {
		return
	}

	// Start table if not already in one
	if !p.inTable {
		p.html.WriteString("<table>\n")
		p.inTable = true
		p.isFirstTableRow = true
	}

	// Split by | and clean up cells
	cells := strings.Split(line, "|")

	// Remove empty cells at start and end
	if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}

	// Start row
	p.html.WriteString("<tr>")

	// Process each cell
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		content := processInlineMarkdown(cell)

		if p.isFirstTableRow {
			p.html.WriteString("<th>" + content + "</th>")
		} else {
			p.html.WriteString("<td>" + content + "</td>")
		}
	}

	// End row
	p.html.WriteString("</tr>\n")

	// No longer first row
	p.isFirstTableRow = false
}

// processInlineMarkdown handles inline markdown formatting
func processInlineMarkdown(text string) string {
	// Process inline code first to avoid conflicts
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "<code>$1</code>")

	// Process bold text (**text**)
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "<strong>$1</strong>")

	// Process italic text (*text*) - but not list markers
	// Use negative lookbehind/lookahead would be ideal, but Go doesn't support it
	// So we'll use a more specific pattern
	text = regexp.MustCompile(`(\s|^)\*([^*\s][^*]*[^*\s])\*(\s|$)`).ReplaceAllString(text, "$1<em>$2</em>$3")

	// Process strikethrough (~~text~~)
	text = regexp.MustCompile(`~~([^~]+)~~`).ReplaceAllString(text, "<s>$1</s>")

	return text
}

// processInlinePattern applies a regex pattern to convert inline markdown
func processInlinePattern(text, pattern, replacement string) string {
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(text, replacement)
}

// generatePresentationHTML creates the complete HTML page with plugin assets
func generatePresentationHTML(slidesHTML, filePath string, config *entities.Config, overrides ThemeOverrides) string {
	// TODO: In a real implementation, we would get the plugin renderer instance
	// to access stored assets and include them in the HTML head section
	// For now, we include default assets and common plugin dependencies

	pluginAssets := fmt.Sprintf(`
    <!-- Common plugin assets -->
    <script src="%s"></script>
    <script src="%s"></script>
    <script src="%s"></script>
    <link rel="stylesheet" href="%s">`, MermaidCDN, PrismCoreCDN, PrismAutoloaderCDN, PrismThemeCDN)

	themeNameLocal := "default"
	if config != nil && config.Theme.Name != "" {
		themeNameLocal = config.Theme.Name
	}

	// Generate theme override CSS if any overrides are set
	overrideCSS := ""
	if overrides.HasOverrides() {
		overrideCSS = fmt.Sprintf(`
        /* Theme overrides from CLI flags */
        %s`, overrides.ToCSS())
	}

	// Build the HTML template with placeholders
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SLICLI Presentation</title>
    <style>{OVERRIDE_CSS}
        /* Minimal base reset - let theme handle everything else */
        html, body {
            margin: 0;
            padding: 0;
            width: 100%;
            height: 100%;
            overflow: hidden;
        }

        /* Initial slide setup - hide all slides by default */
        .slide {
            display: none !important;
        }

        /* Show only the first slide initially */
        .slide:first-child {
            display: flex !important;
        }

        /* Progress bar */
        .progress-bar {
            position: fixed;
            bottom: 0;
            left: 0;
            height: 4px;
            background: linear-gradient(90deg, #667eea, #764ba2);
            transition: width 0.3s ease;
            z-index: 1000;
        }

        /* Help modal styles */
        .modal-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.7);
            z-index: 2000;
            justify-content: center;
            align-items: center;
            opacity: 0;
            transition: opacity 0.3s ease;
        }

        .modal-overlay.visible {
            display: flex;
            opacity: 1;
        }

        .modal-content {
            background: #1a1a2e;
            border-radius: 12px;
            padding: 2rem;
            max-width: 500px;
            width: 90%;
            max-height: 80vh;
            overflow-y: auto;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
            transform: translateY(-20px);
            transition: transform 0.3s ease;
        }

        .modal-overlay.visible .modal-content {
            transform: translateY(0);
        }

        .modal-content h2 {
            margin: 0 0 1.5rem 0;
            color: #fff;
            font-size: 1.5rem;
            border-bottom: 2px solid #667eea;
            padding-bottom: 0.5rem;
        }

        .modal-content table {
            width: 100%;
            border-collapse: collapse;
        }

        .modal-content td {
            padding: 0.5rem 0;
            color: #e0e0e0;
            border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }

        .modal-content td:first-child {
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
            color: #667eea;
            font-weight: bold;
            width: 100px;
        }

        .modal-close-btn {
            display: block;
            width: 100%;
            margin-top: 1.5rem;
            padding: 0.75rem;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 1rem;
            transition: background 0.2s;
        }

        .modal-close-btn:hover {
            background: #5a67d8;
        }

        /* Blank screen overlay */
        .blank-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: #000;
            z-index: 1500;
            cursor: pointer;
        }

        .blank-overlay.visible {
            display: block;
        }

        /* Go to slide input */
        .goto-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.7);
            z-index: 2000;
            justify-content: center;
            align-items: center;
        }

        .goto-overlay.visible {
            display: flex;
        }

        .goto-input-container {
            background: #1a1a2e;
            padding: 1.5rem;
            border-radius: 12px;
            text-align: center;
        }

        .goto-input-container label {
            display: block;
            color: #fff;
            margin-bottom: 0.75rem;
            font-size: 1rem;
        }

        .goto-input-container input {
            width: 80px;
            padding: 0.5rem;
            font-size: 1.5rem;
            text-align: center;
            border: 2px solid #667eea;
            border-radius: 6px;
            background: #16213e;
            color: #fff;
            outline: none;
        }

        .goto-input-container input:focus {
            border-color: #764ba2;
        }

        /* Fullscreen adjustments */
        .fullscreen .navigation {
            opacity: 0;
            transition: opacity 0.3s ease;
        }

        .fullscreen .navigation:hover {
            opacity: 1;
        }

        .fullscreen .presentation-info {
            display: none;
        }

        /* Help button in navigation */
        .help-btn {
            background: transparent;
            color: #667eea;
            border: 2px solid #667eea;
            padding: 0.3rem 0.6rem;
            border-radius: 4px;
            cursor: pointer;
            font-size: 1rem;
            font-weight: bold;
            transition: all 0.2s;
            margin-left: 1rem;
        }

        .help-btn:hover {
            background: #667eea;
            color: white;
        }

        /* Fullscreen button */
        .fullscreen-btn {
            background: transparent;
            color: #667eea;
            border: 2px solid #667eea;
            padding: 0.3rem 0.6rem;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.9rem;
            transition: all 0.2s;
        }

        .fullscreen-btn:hover {
            background: #667eea;
            color: white;
        }
    </style>
    <!-- Main CSS is optional, theme should override -->
    <!-- <link rel="stylesheet" href="/assets/css/main.css"> -->
    <!-- Theme CSS -->
    <link rel="stylesheet" href="/themes/{THEME_NAME}/style.css">
    {PLUGIN_ASSETS}
</head>
<body class="theme-{THEME_NAME} presentation">
    <div class="slides-container">
        {SLIDES_HTML}
    </div>

    <!-- Progress bar -->
    <div class="progress-bar" id="progress-bar"></div>

    <!-- Blank screen overlay -->
    <div class="blank-overlay" id="blank-overlay" onclick="toggleBlankScreen()"></div>

    <!-- Help modal -->
    <div class="modal-overlay" id="help-modal">
        <div class="modal-content">
            <h2>Keyboard Shortcuts</h2>
            <table>
                <tr><td>Left/Right</td><td>Previous/Next slide</td></tr>
                <tr><td>Home/End</td><td>First/Last slide</td></tr>
                <tr><td>1-9, 0</td><td>Jump to slide 1-10</td></tr>
                <tr><td>G</td><td>Go to specific slide</td></tr>
                <tr><td>F</td><td>Toggle fullscreen</td></tr>
                <tr><td>B or .</td><td>Blank screen (pause)</td></tr>
                <tr><td>? or H</td><td>Show this help</td></tr>
                <tr><td>Esc</td><td>Exit fullscreen/Close modal</td></tr>
            </table>
            <button class="modal-close-btn" onclick="hideHelp()">Close</button>
        </div>
    </div>

    <!-- Go to slide input -->
    <div class="goto-overlay" id="goto-overlay">
        <div class="goto-input-container">
            <label for="goto-input">Go to slide:</label>
            <input type="number" id="goto-input" min="1" max="{SLIDE_COUNT}" autofocus>
        </div>
    </div>

    <div class="navigation">
        <button onclick="previousSlide()" title="Previous slide (Left arrow)">&#8592;</button>
        <span class="slide-counter">
            <span id="current-slide">1</span> / <span id="total-slides">{SLIDE_COUNT}</span>
        </span>
        <button onclick="nextSlide()" title="Next slide (Right arrow)">&#8594;</button>
        <button class="fullscreen-btn" onclick="toggleFullscreen()" title="Fullscreen (F)">&#x26F6;</button>
        <button class="help-btn" onclick="showHelp()" title="Help (?)">?</button>
    </div>
    <div class="presentation-info">
        <strong>File:</strong> {FILE_PATH}
        <strong>Theme:</strong> {THEME_NAME}
    </div>
    <script>
        // State variables
        let currentSlide = 1;
        const slides = document.querySelectorAll('.slide');
        const totalSlides = slides.length;
        let isBlankScreen = false;
        let isHelpVisible = false;
        let isGotoVisible = false;

        // Update progress bar
        function updateProgressBar() {
            const progress = (currentSlide / totalSlides) * 100;
            document.getElementById('progress-bar').style.width = progress + '%';
        }

        function showSlide(n) {
            slides.forEach(slide => {
                slide.style.display = 'none';
                slide.style.setProperty('display', 'none', 'important');
            });
            currentSlide = n;
            if (currentSlide > totalSlides) currentSlide = 1;
            if (currentSlide < 1) currentSlide = totalSlides;
            const activeSlide = slides[currentSlide - 1];
            activeSlide.style.setProperty('display', 'flex', 'important');
            document.getElementById('current-slide').textContent = currentSlide;
            document.getElementById('total-slides').textContent = totalSlides;
            updateProgressBar();
        }

        function nextSlide() {
            showSlide(currentSlide + 1);
        }

        function previousSlide() {
            showSlide(currentSlide - 1);
        }

        // Help modal functions
        function showHelp() {
            const modal = document.getElementById('help-modal');
            modal.classList.add('visible');
            isHelpVisible = true;
        }

        function hideHelp() {
            const modal = document.getElementById('help-modal');
            modal.classList.remove('visible');
            isHelpVisible = false;
        }

        // Fullscreen functions
        function toggleFullscreen() {
            if (!document.fullscreenElement) {
                document.documentElement.requestFullscreen().then(() => {
                    document.body.classList.add('fullscreen');
                }).catch(err => {
                    console.error('Fullscreen error:', err);
                });
            } else {
                document.exitFullscreen().then(() => {
                    document.body.classList.remove('fullscreen');
                }).catch(err => {
                    console.error('Exit fullscreen error:', err);
                });
            }
        }

        // Listen for fullscreen changes (e.g., when user presses Esc in fullscreen)
        document.addEventListener('fullscreenchange', () => {
            if (!document.fullscreenElement) {
                document.body.classList.remove('fullscreen');
            }
        });

        // Blank screen functions
        function toggleBlankScreen() {
            const overlay = document.getElementById('blank-overlay');
            isBlankScreen = !isBlankScreen;
            if (isBlankScreen) {
                overlay.classList.add('visible');
            } else {
                overlay.classList.remove('visible');
            }
        }

        // Go to slide functions
        function showGotoInput() {
            const overlay = document.getElementById('goto-overlay');
            const input = document.getElementById('goto-input');
            overlay.classList.add('visible');
            input.value = '';
            input.focus();
            isGotoVisible = true;
        }

        function hideGotoInput() {
            const overlay = document.getElementById('goto-overlay');
            overlay.classList.remove('visible');
            isGotoVisible = false;
        }

        function handleGotoInput(e) {
            if (e.key === 'Enter') {
                const input = document.getElementById('goto-input');
                const slideNum = parseInt(input.value, 10);
                if (slideNum >= 1 && slideNum <= totalSlides) {
                    showSlide(slideNum);
                }
                hideGotoInput();
            } else if (e.key === 'Escape') {
                hideGotoInput();
            }
        }

        // Set up goto input listener
        document.getElementById('goto-input').addEventListener('keydown', handleGotoInput);

        // Click outside goto overlay to close
        document.getElementById('goto-overlay').addEventListener('click', function(e) {
            if (e.target === this) {
                hideGotoInput();
            }
        });

        // Click outside help modal to close
        document.getElementById('help-modal').addEventListener('click', function(e) {
            if (e.target === this) {
                hideHelp();
            }
        });

        // Keyboard navigation
        document.addEventListener('keydown', (e) => {
            // If goto input is visible, let it handle its own keys
            if (isGotoVisible) {
                return;
            }

            // Handle Escape key
            if (e.key === 'Escape') {
                if (isHelpVisible) {
                    hideHelp();
                    return;
                }
                if (isBlankScreen) {
                    toggleBlankScreen();
                    return;
                }
                // Fullscreen exit is handled by the browser
                return;
            }

            // If help modal is visible, only respond to Escape (handled above)
            if (isHelpVisible) {
                return;
            }

            // If blank screen is visible, only respond to B, ., or Escape
            if (isBlankScreen) {
                if (e.key === 'b' || e.key === 'B' || e.key === '.') {
                    toggleBlankScreen();
                }
                return;
            }

            // Navigation keys
            switch (e.key) {
                case 'ArrowRight':
                case 'PageDown':
                case ' ':
                    e.preventDefault();
                    nextSlide();
                    break;
                case 'ArrowLeft':
                case 'PageUp':
                    e.preventDefault();
                    previousSlide();
                    break;
                case 'Home':
                    e.preventDefault();
                    showSlide(1);
                    break;
                case 'End':
                    e.preventDefault();
                    showSlide(totalSlides);
                    break;
                case 'f':
                case 'F':
                    e.preventDefault();
                    toggleFullscreen();
                    break;
                case '?':
                case 'h':
                case 'H':
                    e.preventDefault();
                    showHelp();
                    break;
                case 'b':
                case 'B':
                case '.':
                    e.preventDefault();
                    toggleBlankScreen();
                    break;
                case 'g':
                case 'G':
                    e.preventDefault();
                    showGotoInput();
                    break;
                case '1':
                case '2':
                case '3':
                case '4':
                case '5':
                case '6':
                case '7':
                case '8':
                case '9':
                    e.preventDefault();
                    const slideNum = parseInt(e.key, 10);
                    if (slideNum <= totalSlides) {
                        showSlide(slideNum);
                    }
                    break;
                case '0':
                    e.preventDefault();
                    if (totalSlides >= 10) {
                        showSlide(10);
                    }
                    break;
            }
        });

        // Initialize first slide and hide others
        showSlide(1);

        // Ensure proper slide display on load
        document.addEventListener('DOMContentLoaded', function() {
            // Hide all slides except the first
            slides.forEach((slide, index) => {
                if (index === 0) {
                    slide.style.display = 'flex';
                } else {
                    slide.style.display = 'none';
                }
            });
            updateProgressBar();
        });

        // Initialize Mermaid after slides are set up
        async function initializeMermaid() {
            if (typeof mermaid !== 'undefined') {
                mermaid.initialize({
                    startOnLoad: false,
                    theme: 'dark',
                    securityLevel: 'strict'
                });

                try {
                    const mermaidElements = document.querySelectorAll('.mermaid');
                    console.log('Found', mermaidElements.length, 'mermaid elements');

                    for (let i = 0; i < mermaidElements.length; i++) {
                        const element = mermaidElements[i];
                        let graphDefinition = element.getAttribute('data-original') || element.textContent || element.innerText || '';

                        if (graphDefinition.includes('#mermaid-') || graphDefinition.includes('font-family')) {
                            console.log('Skipping diagram', i, '- already rendered or contains styling');
                            return;
                        }

                        graphDefinition = graphDefinition.trim();
                        graphDefinition = graphDefinition.replace(/&gt;/g, '>').replace(/&lt;/g, '<').replace(/&amp;/g, '&');

                        const id = 'mermaid-' + i;
                        console.log('Rendering diagram', i, 'content:', JSON.stringify(graphDefinition));

                        try {
                            const { svg } = await mermaid.render(id, graphDefinition);
                            element.innerHTML = svg;
                            element.classList.add('mermaid-rendered');
                            console.log('Successfully rendered diagram', i);
                        } catch (renderError) {
                            console.error('Mermaid render error for diagram', i, ':', renderError);
                            element.innerHTML = '<div style="color: red; padding: 10px; border: 1px solid red;">Error: ' + renderError.message + '</div>';
                        }
                    }
                } catch (e) {
                    console.error('Mermaid initialization error:', e);
                }
            }
        }

        // Wait for DOM and scripts to be ready
        document.addEventListener('DOMContentLoaded', async function() {
            // Initialize Prism.js for syntax highlighting
            if (window.Prism) {
                Prism.highlightAll();
            }

            // Initialize Mermaid after a short delay to ensure everything is loaded
            setTimeout(async () => {
                await initializeMermaid();
            }, 100);
        });
    </script>
</body>
</html>`

	// Count total slides
	slideCount := len(strings.Split(slidesHTML, `<div class="slide"`)) - 1

	// Replace placeholders
	html := strings.ReplaceAll(htmlTemplate, "{THEME_NAME}", themeNameLocal)
	html = strings.ReplaceAll(html, "{PLUGIN_ASSETS}", pluginAssets)
	html = strings.ReplaceAll(html, "{SLIDES_HTML}", slidesHTML)
	html = strings.ReplaceAll(html, "{FILE_PATH}", filePath)
	html = strings.ReplaceAll(html, "{SLIDE_COUNT}", fmt.Sprintf("%d", slideCount))
	html = strings.ReplaceAll(html, "{OVERRIDE_CSS}", overrideCSS)
	return html
}

// extractSpeakerNotes extracts speaker notes from HTML comments in slide content
// Notes are expected in format: <!-- Notes: ... -->
func extractSpeakerNotes(slideContent string) string {
	matches := speakerNotesPattern.FindStringSubmatch(slideContent)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// removeSpeakerNotes removes speaker notes comments from slide content
func removeSpeakerNotes(slideContent string) string {
	return speakerNotesPattern.ReplaceAllString(slideContent, "")
}

// processMarkdownToSlidesWithNotes converts markdown content to SlideData with speaker notes
func processMarkdownToSlidesWithNotes(markdown, filePath string, cfg *entities.Config) PresentationData {
	// Split markdown by slide separator (---)
	rawSlides := strings.Split(markdown, "\n---\n")

	themeNameLocal := "default"
	if cfg != nil && cfg.Theme.Name != "" {
		themeNameLocal = cfg.Theme.Name
	}

	var slides []SlideData
	slideIndex := 1

	for i, rawSlide := range rawSlides {
		slideContent := strings.TrimSpace(rawSlide)
		if slideContent == "" {
			continue
		}

		// Extract speaker notes before processing
		speakerNotes := extractSpeakerNotes(slideContent)

		// Remove speaker notes from content before converting to HTML
		cleanContent := removeSpeakerNotes(slideContent)

		// Convert markdown to HTML
		htmlContent := basicMarkdownToHTML(cleanContent)

		// Determine slide class
		slideClass := determineSlideClass(cleanContent, i)

		slides = append(slides, SlideData{
			Index:        slideIndex,
			HTMLContent:  htmlContent,
			SpeakerNotes: speakerNotes,
			SlideClass:   slideClass,
		})
		slideIndex++
	}

	return PresentationData{
		Slides:     slides,
		TotalCount: len(slides),
		FilePath:   filePath,
		ThemeName:  themeNameLocal,
	}
}

// WebSocket upgrader with security settings
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from localhost only for security
		origin := r.Header.Get("Origin")
		return strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1")
	},
}

// createWebSocketHandler creates a WebSocket handler for slide synchronization
func createWebSocketHandler(hub *SlideHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ERROR] WebSocket upgrade failed: %v", err)
			return
		}

		hub.register <- conn

		// Send current slide state to newly connected client
		initialMsg := WebSocketMessage{
			Type:       "sync",
			SlideIndex: hub.GetCurrentSlide(),
			Source:     "server",
		}
		if err := conn.WriteJSON(initialMsg); err != nil {
			log.Printf("[ERROR] Failed to send initial state: %v", err)
		}

		// Handle incoming messages
		go func() {
			defer func() {
				hub.unregister <- conn
			}()

			for {
				var msg WebSocketMessage
				if err := conn.ReadJSON(&msg); err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						log.Printf("[ERROR] WebSocket read error: %v", err)
					}
					break
				}

				// Update client type if announced
				if msg.Type == "presenter_ready" || msg.Type == "audience_ready" {
					hub.SetClientType(conn, msg.Source)
				}

				// Broadcast slide changes to all clients
				if msg.Type == "slide_change" {
					hub.broadcast <- msg
				}
			}
		}()
	}
}

// createPresenterHandler creates the handler for serving the presenter view
func createPresenterHandler(presentationData PresentationData, mainServerPort int) http.HandlerFunc {
	presenterHTML := generatePresenterHTML(presentationData, mainServerPort)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(presenterHTML)); err != nil {
			log.Printf("[ERROR] Failed to write presenter response: %v", err)
		}
	}
}

// createSlidesDataHandler creates the handler for serving slide data as JSON
func createSlidesDataHandler(presentationData PresentationData) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(presentationData); err != nil {
			log.Printf("[ERROR] Failed to encode slides data: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// generatePresenterHTML creates the HTML for the presenter view
func generatePresenterHTML(data PresentationData, mainServerPort int) string {
	// Serialize slides data for JavaScript
	slidesJSON, err := json.Marshal(data.Slides)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal slides data: %v", err)
		slidesJSON = []byte("[]")
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Presenter View - SLICLI</title>
    <style>
        * { box-sizing: border-box; }

        body {
            margin: 0;
            padding: 0;
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a2e;
            color: #eee;
            height: 100vh;
            overflow: hidden;
        }

        .presenter-container {
            display: grid;
            grid-template-rows: auto 1fr auto auto;
            height: 100vh;
            gap: 10px;
            padding: 10px;
        }

        .header-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            background: #16213e;
            padding: 10px 20px;
            border-radius: 8px;
        }

        .timer {
            font-size: 1.8rem;
            font-weight: bold;
            font-family: 'Monaco', 'Menlo', monospace;
            color: #4ecca3;
        }

        .slide-counter {
            font-size: 1.4rem;
            color: #7f8c8d;
        }

        .current-time {
            font-size: 1.2rem;
            color: #7f8c8d;
        }

        .slides-area {
            display: grid;
            grid-template-columns: 2fr 1fr;
            gap: 15px;
            min-height: 0;
        }

        .current-slide-container, .next-slide-container {
            background: #16213e;
            border-radius: 8px;
            padding: 15px;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }

        .slide-label {
            font-size: 0.9rem;
            color: #7f8c8d;
            margin-bottom: 10px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .slide-preview {
            flex: 1;
            background: white;
            border-radius: 6px;
            overflow: auto;
            padding: 20px;
            color: #333;
        }

        .slide-preview.next { opacity: 0.85; }
        .slide-preview h1 { font-size: 1.5rem; margin: 0 0 0.5rem 0; }
        .slide-preview h2 { font-size: 1.2rem; margin: 0 0 0.5rem 0; }
        .slide-preview h3 { font-size: 1rem; margin: 0 0 0.5rem 0; }
        .slide-preview p { font-size: 0.9rem; margin: 0.5rem 0; }
        .slide-preview li { font-size: 0.9rem; margin: 0.25rem 0; }
        .slide-preview code { font-size: 0.8rem; }
        .slide-preview pre { font-size: 0.75rem; overflow-x: auto; }

        .notes-area {
            background: #16213e;
            border-radius: 8px;
            padding: 15px;
            overflow: auto;
            max-height: 200px;
        }

        .notes-area h3 {
            margin: 0 0 10px 0;
            color: #4ecca3;
            font-size: 1rem;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .notes-content {
            font-size: 1rem;
            line-height: 1.6;
            color: #ddd;
            white-space: pre-wrap;
        }

        .notes-content:empty::after {
            content: "No speaker notes for this slide.";
            color: #666;
            font-style: italic;
        }

        .controls-area {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 20px;
            padding: 15px;
            background: #16213e;
            border-radius: 8px;
        }

        .control-btn {
            padding: 12px 30px;
            font-size: 1.1rem;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
            font-weight: 500;
        }

        .control-btn.nav {
            background: #4ecca3;
            color: #1a1a2e;
        }

        .control-btn.nav:hover {
            background: #45b393;
            transform: translateY(-2px);
        }

        .control-btn.nav:disabled {
            background: #3a3a5a;
            color: #666;
            cursor: not-allowed;
            transform: none;
        }

        .control-btn.end {
            background: #e74c3c;
            color: white;
        }

        .control-btn.end:hover { background: #c0392b; }

        .control-btn.timer-control {
            background: #3498db;
            color: white;
        }

        .control-btn.timer-control:hover { background: #2980b9; }

        .connection-status {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 0.85rem;
        }

        .status-dot {
            width: 10px;
            height: 10px;
            border-radius: 50%%;
            background: #e74c3c;
        }

        .status-dot.connected { background: #4ecca3; }

        .shortcuts-hint {
            position: fixed;
            bottom: 10px;
            left: 10px;
            font-size: 0.75rem;
            color: #666;
        }

        .shortcuts-hint kbd {
            background: #2a2a4a;
            padding: 2px 6px;
            border-radius: 3px;
            margin: 0 2px;
        }
    </style>
</head>
<body>
    <div class="presenter-container">
        <div class="header-bar">
            <div class="timer" id="timer">00:00:00</div>
            <div class="slide-counter">
                Slide <span id="current-slide-num">1</span> / <span id="total-slides">%d</span>
            </div>
            <div class="connection-status">
                <span class="status-dot" id="status-dot"></span>
                <span id="connection-text">Connecting...</span>
            </div>
            <div class="current-time" id="current-time"></div>
        </div>

        <div class="slides-area">
            <div class="current-slide-container">
                <div class="slide-label">Current Slide</div>
                <div class="slide-preview current" id="current-slide-content"></div>
            </div>
            <div class="next-slide-container">
                <div class="slide-label">Next Slide</div>
                <div class="slide-preview next" id="next-slide-content"></div>
            </div>
        </div>

        <div class="notes-area">
            <h3>Speaker Notes</h3>
            <div class="notes-content" id="speaker-notes"></div>
        </div>

        <div class="controls-area">
            <button class="control-btn timer-control" id="timer-btn" onclick="toggleTimer()">Start Timer</button>
            <button class="control-btn nav" id="prev-btn" onclick="previousSlide()">Previous</button>
            <button class="control-btn nav" id="next-btn" onclick="nextSlide()">Next</button>
            <button class="control-btn end" onclick="endPresentation()">End Presentation</button>
        </div>
    </div>

    <div class="shortcuts-hint">
        <kbd>Left</kbd> Previous | <kbd>Right</kbd> Next | <kbd>Space</kbd> Next | <kbd>T</kbd> Timer | <kbd>R</kbd> Reset
    </div>

    <script>
        const slides = %s;
        const totalSlides = slides.length;
        let currentSlide = 1;
        let timerRunning = false;
        let timerStartTime = null;
        let timerElapsed = 0;
        let timerInterval = null;
        let ws = null;
        const mainServerPort = %d;
        const wsUrl = 'ws://localhost:' + mainServerPort + '/ws';

        document.addEventListener('DOMContentLoaded', function() {
            updateSlideDisplay();
            updateCurrentTime();
            setInterval(updateCurrentTime, 1000);
            connectWebSocket();
        });

        function connectWebSocket() {
            ws = new WebSocket(wsUrl);

            ws.onopen = function() {
                document.getElementById('status-dot').classList.add('connected');
                document.getElementById('connection-text').textContent = 'Connected';
                ws.send(JSON.stringify({
                    type: 'presenter_ready',
                    slideIndex: currentSlide,
                    source: 'presenter'
                }));
            };

            ws.onmessage = function(event) {
                const msg = JSON.parse(event.data);
                if (msg.type === 'slide_change' && msg.source !== 'presenter') {
                    currentSlide = msg.slideIndex;
                    updateSlideDisplay();
                } else if (msg.type === 'sync') {
                    currentSlide = msg.slideIndex;
                    updateSlideDisplay();
                }
            };

            ws.onclose = function() {
                document.getElementById('status-dot').classList.remove('connected');
                document.getElementById('connection-text').textContent = 'Disconnected';
                setTimeout(connectWebSocket, 3000);
            };

            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }

        function broadcastSlideChange() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({
                    type: 'slide_change',
                    slideIndex: currentSlide,
                    source: 'presenter'
                }));
            }
        }

        function updateSlideDisplay() {
            const currentSlideData = slides[currentSlide - 1];
            const nextSlideData = slides[currentSlide] || null;

            document.getElementById('current-slide-content').innerHTML = currentSlideData ? currentSlideData.htmlContent : '';

            if (nextSlideData) {
                document.getElementById('next-slide-content').innerHTML = nextSlideData.htmlContent;
            } else {
                document.getElementById('next-slide-content').innerHTML = '<div style="color: #666; text-align: center; padding: 50px;">End of presentation</div>';
            }

            document.getElementById('speaker-notes').textContent = currentSlideData ? currentSlideData.speakerNotes : '';
            document.getElementById('current-slide-num').textContent = currentSlide;
            document.getElementById('total-slides').textContent = totalSlides;
            document.getElementById('prev-btn').disabled = currentSlide <= 1;
            document.getElementById('next-btn').disabled = currentSlide >= totalSlides;
        }

        function nextSlide() {
            if (currentSlide < totalSlides) {
                currentSlide++;
                updateSlideDisplay();
                broadcastSlideChange();
            }
        }

        function previousSlide() {
            if (currentSlide > 1) {
                currentSlide--;
                updateSlideDisplay();
                broadcastSlideChange();
            }
        }

        function goToSlide(n) {
            if (n >= 1 && n <= totalSlides) {
                currentSlide = n;
                updateSlideDisplay();
                broadcastSlideChange();
            }
        }

        function toggleTimer() {
            if (timerRunning) {
                pauseTimer();
            } else {
                startTimer();
            }
        }

        function startTimer() {
            timerRunning = true;
            timerStartTime = Date.now() - timerElapsed;
            document.getElementById('timer-btn').textContent = 'Pause Timer';
            timerInterval = setInterval(function() {
                timerElapsed = Date.now() - timerStartTime;
                updateTimerDisplay();
            }, 100);
        }

        function pauseTimer() {
            timerRunning = false;
            document.getElementById('timer-btn').textContent = 'Resume Timer';
            if (timerInterval) clearInterval(timerInterval);
        }

        function resetTimer() {
            timerRunning = false;
            timerElapsed = 0;
            timerStartTime = null;
            document.getElementById('timer-btn').textContent = 'Start Timer';
            if (timerInterval) clearInterval(timerInterval);
            updateTimerDisplay();
        }

        function updateTimerDisplay() {
            const totalSeconds = Math.floor(timerElapsed / 1000);
            const hours = Math.floor(totalSeconds / 3600);
            const minutes = Math.floor((totalSeconds %% 3600) / 60);
            const seconds = totalSeconds %% 60;
            document.getElementById('timer').textContent =
                String(hours).padStart(2, '0') + ':' +
                String(minutes).padStart(2, '0') + ':' +
                String(seconds).padStart(2, '0');
        }

        function updateCurrentTime() {
            const now = new Date();
            document.getElementById('current-time').textContent =
                now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        }

        function endPresentation() {
            if (confirm('End the presentation?')) {
                window.close();
            }
        }

        document.addEventListener('keydown', function(e) {
            switch(e.key) {
                case 'ArrowRight':
                case ' ':
                    e.preventDefault();
                    nextSlide();
                    break;
                case 'ArrowLeft':
                    e.preventDefault();
                    previousSlide();
                    break;
                case 't':
                case 'T':
                    toggleTimer();
                    break;
                case 'r':
                case 'R':
                    resetTimer();
                    break;
                case 'Home':
                    goToSlide(1);
                    break;
                case 'End':
                    goToSlide(totalSlides);
                    break;
            }
        });
    </script>
</body>
</html>`, data.TotalCount, string(slidesJSON), mainServerPort)
}

// generateAudienceHTMLWithSync creates the HTML for the audience view with WebSocket sync
func generateAudienceHTMLWithSync(slidesHTML, filePath string, cfg *entities.Config, wsPort int, overrides ThemeOverrides) string {
	themeNameLocal := "default"
	if cfg != nil && cfg.Theme.Name != "" {
		themeNameLocal = cfg.Theme.Name
	}

	pluginAssets := fmt.Sprintf(`
    <script src="%s"></script>
    <script src="%s"></script>
    <script src="%s"></script>
    <link rel="stylesheet" href="%s">`, MermaidCDN, PrismCoreCDN, PrismAutoloaderCDN, PrismThemeCDN)

	// Generate theme override CSS if any overrides are set
	overrideCSS := ""
	if overrides.HasOverrides() {
		overrideCSS = fmt.Sprintf(`
        /* Theme overrides from CLI flags */
        %s`, overrides.ToCSS())
	}

	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SLICLI Presentation</title>
    <style>{OVERRIDE_CSS}
        html, body {
            margin: 0;
            padding: 0;
            width: 100%%;
            height: 100%%;
            overflow: hidden;
        }
        .slide {
            display: none !important;
        }
        .slide:first-child {
            display: flex !important;
        }
        .sync-indicator {
            position: fixed;
            top: 10px;
            left: 10px;
            padding: 5px 10px;
            background: rgba(0,0,0,0.7);
            color: white;
            border-radius: 4px;
            font-size: 12px;
            z-index: 1000;
            display: flex;
            align-items: center;
            gap: 6px;
        }
        .sync-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%%;
            background: #e74c3c;
        }
        .sync-dot.connected {
            background: #4ecca3;
        }
    </style>
    <link rel="stylesheet" href="/themes/{THEME_NAME}/style.css">
    {PLUGIN_ASSETS}
</head>
<body class="theme-{THEME_NAME} presentation">
    <div class="sync-indicator">
        <span class="sync-dot" id="sync-dot"></span>
        <span id="sync-status">Connecting...</span>
    </div>
    <div class="slides-container">
        {SLIDES_HTML}
    </div>
    <div class="navigation">
        <button onclick="previousSlide()">Previous</button>
        <span class="slide-counter">
            <span id="current-slide">1</span> / <span id="total-slides">{SLIDE_COUNT}</span>
        </span>
        <button onclick="nextSlide()">Next</button>
    </div>
    <div class="presentation-info">
        <strong>File:</strong> {FILE_PATH}
        <strong>Theme:</strong> {THEME_NAME}
    </div>
    <script>
        let currentSlide = 1;
        const slides = document.querySelectorAll('.slide');
        const totalSlides = slides.length;
        let ws = null;
        const wsPort = {WS_PORT};

        function showSlide(n) {
            slides.forEach(slide => {
                slide.style.setProperty('display', 'none', 'important');
            });
            currentSlide = n;
            if (currentSlide > totalSlides) currentSlide = 1;
            if (currentSlide < 1) currentSlide = totalSlides;
            const activeSlide = slides[currentSlide - 1];
            activeSlide.style.setProperty('display', 'flex', 'important');
            document.getElementById('current-slide').textContent = currentSlide;
            document.getElementById('total-slides').textContent = totalSlides;
        }

        function nextSlide() {
            showSlide(currentSlide + 1);
            broadcastSlideChange();
        }

        function previousSlide() {
            showSlide(currentSlide - 1);
            broadcastSlideChange();
        }

        function broadcastSlideChange() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({
                    type: 'slide_change',
                    slideIndex: currentSlide,
                    source: 'audience'
                }));
            }
        }

        function connectWebSocket() {
            if (wsPort === 0) return;
            ws = new WebSocket('ws://localhost:' + wsPort + '/ws');

            ws.onopen = function() {
                document.getElementById('sync-dot').classList.add('connected');
                document.getElementById('sync-status').textContent = 'Synced';
                ws.send(JSON.stringify({
                    type: 'audience_ready',
                    slideIndex: currentSlide,
                    source: 'audience'
                }));
            };

            ws.onmessage = function(event) {
                const msg = JSON.parse(event.data);
                if (msg.type === 'slide_change' && msg.source !== 'audience') {
                    currentSlide = msg.slideIndex;
                    showSlide(currentSlide);
                } else if (msg.type === 'sync') {
                    currentSlide = msg.slideIndex;
                    showSlide(currentSlide);
                }
            };

            ws.onclose = function() {
                document.getElementById('sync-dot').classList.remove('connected');
                document.getElementById('sync-status').textContent = 'Disconnected';
                setTimeout(connectWebSocket, 3000);
            };
        }

        document.addEventListener('keydown', (e) => {
            if (e.key === 'ArrowRight') nextSlide();
            if (e.key === 'ArrowLeft') previousSlide();
        });

        showSlide(1);
        connectWebSocket();

        document.addEventListener('DOMContentLoaded', async function() {
            if (window.Prism) Prism.highlightAll();

            if (typeof mermaid !== 'undefined') {
                mermaid.initialize({
                    startOnLoad: false,
                    theme: 'dark',
                    securityLevel: 'strict'
                });

                const mermaidElements = document.querySelectorAll('.mermaid');
                for (let i = 0; i < mermaidElements.length; i++) {
                    const element = mermaidElements[i];
                    let graphDefinition = element.getAttribute('data-original') || element.textContent || '';
                    if (graphDefinition.includes('#mermaid-')) continue;

                    graphDefinition = graphDefinition.trim()
                        .replace(/&gt;/g, '>')
                        .replace(/&lt;/g, '<')
                        .replace(/&amp;/g, '&');

                    try {
                        const { svg } = await mermaid.render('mermaid-' + i, graphDefinition);
                        element.innerHTML = svg;
                    } catch (e) {
                        console.error('Mermaid error:', e);
                    }
                }
            }
        });
    </script>
</body>
</html>`

	slideCount := len(strings.Split(slidesHTML, `<div class="slide"`)) - 1

	html := strings.ReplaceAll(htmlTemplate, "{THEME_NAME}", themeNameLocal)
	html = strings.ReplaceAll(html, "{PLUGIN_ASSETS}", pluginAssets)
	html = strings.ReplaceAll(html, "{SLIDES_HTML}", slidesHTML)
	html = strings.ReplaceAll(html, "{FILE_PATH}", filePath)
	html = strings.ReplaceAll(html, "{SLIDE_COUNT}", fmt.Sprintf("%d", slideCount))
	html = strings.ReplaceAll(html, "{WS_PORT}", fmt.Sprintf("%d", wsPort))
	html = strings.ReplaceAll(html, "{OVERRIDE_CSS}", overrideCSS)
	return html
}

// createPresenterHTTPServer creates and configures the presenter HTTP server
func createPresenterHTTPServer(cfg *entities.Config, presentationData PresentationData, hub *SlideHub, mainServerPort int) *http.Server {
	mux := http.NewServeMux()

	// Presenter view
	mux.HandleFunc("/", createPresenterHandler(presentationData, mainServerPort))
	mux.HandleFunc("/presenter", createPresenterHandler(presentationData, mainServerPort))

	// Slides data API
	mux.HandleFunc("/api/slides", createSlidesDataHandler(presentationData))

	// WebSocket for synchronization
	mux.HandleFunc("/ws", createWebSocketHandler(hub))

	// Serve theme assets (same as main server)
	mux.HandleFunc("/themes/", createThemeAssetsHandler())

	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, presenterPort),
		Handler:      securityMiddlewareForPresenter(mux),
		ReadTimeout:  cfg.Server.GetReadTimeout(),
		WriteTimeout: cfg.Server.GetWriteTimeout(),
		IdleTimeout:  60 * time.Second,
	}
}

// securityMiddlewareForPresenter adds security headers for presenter mode
// with WebSocket support
func securityMiddlewareForPresenter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// Allow WebSocket connections
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"img-src 'self' data: https:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' ws://localhost:* wss://localhost:*; "+
				"frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// getDefaultCSS returns basic CSS for presentations
func getDefaultCSS() string {
	return `
body {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: #333;
    overflow: hidden;
}

.presentation-container {
    width: 100vw;
    height: 100vh;
    display: flex;
    flex-direction: column;
}

.slides-wrapper {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2rem;
}

.slide {
    display: none;
    width: 100%;
    max-width: 1000px;
    background: white;
    border-radius: 12px;
    box-shadow: 0 20px 40px rgba(0,0,0,0.1);
    animation: slideIn 0.3s ease-out;
}

.slide.active {
    display: block;
}

.slide-content {
    padding: 3rem;
    line-height: 1.6;
}

.slide-content h1 {
    color: #2d3748;
    margin-bottom: 1.5rem;
    border-bottom: 3px solid #667eea;
    padding-bottom: 0.5rem;
}

.slide-content h2 {
    color: #4a5568;
    margin-bottom: 1rem;
    margin-top: 2rem;
}

.slide-content h3 {
    color: #718096;
    margin-bottom: 0.75rem;
    margin-top: 1.5rem;
}

.slide-content li {
    margin-bottom: 0.5rem;
    list-style: none;
    position: relative;
    padding-left: 1.5rem;
}

.slide-content li:before {
    content: "•";
    color: #667eea;
    font-weight: bold;
    position: absolute;
    left: 0;
}

.code-block {
    background: #f7fafc;
    border: 1px solid #e2e8f0;
    border-radius: 8px;
    padding: 1rem;
    margin: 1rem 0;
    overflow-x: auto;
    font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
    font-size: 0.9rem;
    line-height: 1.4;
}

.navigation {
    background: rgba(255,255,255,0.9);
    padding: 1rem;
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 2rem;
    border-top: 1px solid rgba(0,0,0,0.1);
}

.navigation button {
    background: #667eea;
    color: white;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    cursor: pointer;
    font-size: 1.2rem;
    transition: background 0.2s;
}

.navigation button:hover {
    background: #5a67d8;
}

.navigation button:disabled {
    background: #a0aec0;
    cursor: not-allowed;
}

.slide-counter {
    font-weight: 500;
    color: #4a5568;
}

.presentation-info {
    position: fixed;
    top: 1rem;
    right: 1rem;
    background: rgba(255,255,255,0.9);
    padding: 0.5rem 1rem;
    border-radius: 6px;
    font-size: 0.8rem;
    color: #718096;
}

@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateY(20px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

/* Mermaid diagram styling */
.mermaid {
    text-align: center;
    margin: 2rem 0;
}
`
}

// getServerURL constructs the server URL from host and port
func getServerURL() string {
	return fmt.Sprintf("http://%s:%d", host, port)
}

// getDefaultJS returns basic JavaScript for presentations
func getDefaultJS() string {
	return `
let currentSlide = 1;
let totalSlides = 1;

document.addEventListener('DOMContentLoaded', function() {
    const slides = document.querySelectorAll('.slide');
    totalSlides = slides.length;
    
    // Update total slides counter
    document.getElementById('total-slides').textContent = totalSlides;
    
    // Show first slide
    showSlide(1);
    
    // Keyboard navigation
    document.addEventListener('keydown', function(e) {
        switch(e.key) {
            case 'ArrowLeft':
                previousSlide();
                break;
            case 'ArrowRight':
                nextSlide();
                break;
            case 'Home':
                showSlide(1);
                break;
            case 'End':
                showSlide(totalSlides);
                break;
        }
    });
});

function showSlide(n) {
    const slides = document.querySelectorAll('.slide');
    
    if (n > totalSlides) n = totalSlides;
    if (n < 1) n = 1;
    
    currentSlide = n;
    
    // Hide all slides
    slides.forEach(slide => slide.classList.remove('active'));
    
    // Show current slide
    const currentSlideElement = document.getElementById('slide-' + n);
    if (currentSlideElement) {
        currentSlideElement.classList.add('active');
    }
    
    // Update counter
    document.getElementById('current-slide').textContent = currentSlide;
    
    // Update navigation buttons
    const prevBtn = document.querySelector('.navigation button:first-child');
    const nextBtn = document.querySelector('.navigation button:last-child');
    
    prevBtn.disabled = currentSlide === 1;
    nextBtn.disabled = currentSlide === totalSlides;
}

function nextSlide() {
    showSlide(currentSlide + 1);
}

function previousSlide() {
    showSlide(currentSlide - 1);
}
`
}
