package main

import (
	"bytes"
	"context"
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
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/browser"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/config"
	"github.com/fredcamaral/slicli/internal/domain/entities"
)

var (
	// Serve command flags
	port       int
	host       string
	noBrowser  bool
	themeName  string
	watchFiles bool
)

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

	// Get verbose flag and create logger with logging configuration
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Override verbose setting from config if flag wasn't explicitly set
	if !cmd.Flags().Changed("verbose") {
		verbose = finalConfig.Logging.Verbose
	}

	logger := newLoggerWithLevel(verbose, finalConfig.Logging.GetLevel())
	printStartupInfo(logger, presentationPath, finalConfig)

	// Load presentation content
	htmlContent, err := loadPresentationContent(presentationPath, finalConfig)
	if err != nil {
		return err
	}

	// Create HTTP server
	server := createHTTPServer(finalConfig, htmlContent)

	// Start server and handle lifecycle
	return startAndManageServer(server, finalConfig, logger)
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
func loadPresentationContent(presentationPath string, config *entities.Config) (string, error) {
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
	return processMarkdownToSlides(string(markdownContent), presentationPath, config), nil
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

	// Create HTTP server using configuration values
	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
		Handler:      mux,
		ReadTimeout:  config.Server.GetReadTimeout(),
		WriteTimeout: config.Server.GetWriteTimeout(),
		IdleTimeout:  60 * time.Second,
	}
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
			// Try to serve from web/assets directory
			assetPath := filepath.Join("web", strings.TrimPrefix(cleanPath, "/"))
			
			// Check if file exists and is not a directory
			fileInfo, err := os.Stat(assetPath)
			if err != nil || fileInfo.IsDir() {
				http.NotFound(w, r)
				return
			}
			
			// Set appropriate content type
			setContentType(w, cleanPath)
			
			// Serve the file
			http.ServeFile(w, r, assetPath)
		}
	}
}

// createThemeAssetsHandler creates the handler for serving theme assets
func createThemeAssetsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Security: Clean and validate the path
		cleanPath := filepath.Clean(r.URL.Path)
		
		// Prevent path traversal attacks
		if strings.Contains(cleanPath, "..") {
			http.NotFound(w, r)
			return
		}
		
		// Remove /themes/ prefix to get the actual theme path
		themePath := strings.TrimPrefix(cleanPath, "/themes/")
		
		// Try multiple possible theme locations
		possiblePaths := []string{
			filepath.Join("themes", themePath),                    // Current directory
			filepath.Join("..", "..", "themes", themePath),       // Two levels up (when in subdirectory)
			filepath.Join(os.Getenv("HOME"), ".slicli", "themes", themePath), // User home
		}
		
		var fullPath string
		var fileInfo os.FileInfo
		var err error
		
		// Find the first existing path
		for _, path := range possiblePaths {
			fileInfo, err = os.Stat(path)
			if err == nil && !fileInfo.IsDir() {
				fullPath = path
				break
			}
		}
		
		// If no valid path found, return 404
		if fullPath == "" {
			http.NotFound(w, r)
			return
		}
		
		// Set appropriate content type
		setContentType(w, cleanPath)
		
		// Serve the file
		http.ServeFile(w, r, fullPath)
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

// applyCliFlags applies CLI flag overrides to the configuration
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
}

// processMarkdownToSlides converts markdown content to HTML slides
func processMarkdownToSlides(markdown, filePath string, config *entities.Config) string {
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
	return generatePresentationHTML(strings.Join(htmlSlides, "\n"), filePath, config)
}

// basicMarkdownToHTML provides complete markdown to HTML conversion using Goldmark
func basicMarkdownToHTML(markdown string) string {
	// Configure Goldmark with extensions for full markdown support
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,        // GitHub Flavored Markdown (tables, strikethrough, etc.)
			extension.Table,      // Tables support
			extension.Strikethrough, // ~~strikethrough~~ support
			extension.TaskList,   // - [ ] task list support
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Convert line breaks to <br>
			html.WithXHTML(),     // XHTML compliant output
			html.WithUnsafe(),    // Allow raw HTML (needed for Mermaid)
		),
	)

	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		log.Printf("[ERROR] Failed to convert markdown: %v", err)
		return markdown // Return original markdown on error
	}

	// Post-process for Mermaid diagrams
	html := buf.String()
	html = postProcessMermaidDiagrams(html)
	
	return html
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
	html         *strings.Builder
	inCodeBlock  bool
	codeLanguage string
	mermaidContent strings.Builder // Store original Mermaid content
	inTable      bool
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
func generatePresentationHTML(slidesHTML, filePath string, config *entities.Config) string {
	// TODO: In a real implementation, we would get the plugin renderer instance
	// to access stored assets and include them in the HTML head section
	// For now, we include default assets and common plugin dependencies

	pluginAssets := `
    <!-- Common plugin assets -->
    <script src="https://cdn.jsdelivr.net/npm/mermaid@10.6.1/dist/mermaid.min.js"></script>
    <script src="https://unpkg.com/prismjs@1/components/prism-core.min.js"></script>
    <script src="https://unpkg.com/prismjs@1/plugins/autoloader/prism-autoloader.min.js"></script>
    <link rel="stylesheet" href="https://unpkg.com/prismjs@1/themes/prism.css">`

	themeName := "default"
	if config != nil && config.Theme.Name != "" {
		themeName = config.Theme.Name
	}

	// Build the HTML template with placeholders
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SLICLI Presentation</title>
    <style>
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
    <div class="navigation">
        <button onclick="previousSlide()">←</button>
        <span class="slide-counter">
            <span id="current-slide">1</span> / <span id="total-slides">{SLIDE_COUNT}</span>
        </span>
        <button onclick="nextSlide()">→</button>
    </div>
    <div class="presentation-info">
        <strong>File:</strong> {FILE_PATH}
        <strong>Theme:</strong> {THEME_NAME}
    </div>
    <script>
        // Basic slide navigation
        let currentSlide = 1;
        const slides = document.querySelectorAll('.slide');
        const totalSlides = slides.length;
        
        function showSlide(n) {
            slides.forEach(slide => {
                slide.style.display = 'none';
                slide.style.setProperty('display', 'none', 'important');
            });
            currentSlide = n;
            if (currentSlide > totalSlides) currentSlide = 1;
            if (currentSlide < 1) currentSlide = totalSlides;
            const activeSlide = slides[currentSlide - 1];
            activeSlide.style.setProperty('display', 'flex', 'important'); // Override CSS with important
            document.getElementById('current-slide').textContent = currentSlide;
            document.getElementById('total-slides').textContent = totalSlides;
        }
        
        function nextSlide() {
            showSlide(currentSlide + 1);
        }
        
        function previousSlide() {
            showSlide(currentSlide - 1);
        }
        
        // Keyboard navigation
        document.addEventListener('keydown', (e) => {
            if (e.key === 'ArrowRight') nextSlide();
            if (e.key === 'ArrowLeft') previousSlide();
        });
        
        // Initialize first slide and hide others
        showSlide(1);
        
        // Ensure proper slide display on load
        document.addEventListener('DOMContentLoaded', function() {
            // Hide all slides except the first
            slides.forEach((slide, index) => {
                if (index === 0) {
                    slide.style.display = 'flex'; // Use flex as per theme CSS
                } else {
                    slide.style.display = 'none';
                }
            });
        });
        
        // Initialize Mermaid after slides are set up
        async function initializeMermaid() {
            if (typeof mermaid !== 'undefined') {
                mermaid.initialize({
                    startOnLoad: false,  // Don't auto-start
                    theme: 'dark',
                    securityLevel: 'loose'
                });
                
                // Manually render all visible mermaid diagrams
                try {
                    const mermaidElements = document.querySelectorAll('.mermaid');
                    console.log('Found', mermaidElements.length, 'mermaid elements');
                    
                    for (let i = 0; i < mermaidElements.length; i++) {
                        const element = mermaidElements[i];
                        
                        // Get the original markdown content from data attribute or textContent
                        let graphDefinition = element.getAttribute('data-original') || element.textContent || element.innerText || '';
                        
                        // If we got the processed content, skip this element (already rendered)
                        if (graphDefinition.includes('#mermaid-') || graphDefinition.includes('font-family')) {
                            console.log('Skipping diagram', i, '- already rendered or contains styling');
                            return;
                        }
                        
                        // Clean up the definition
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
	html := strings.ReplaceAll(htmlTemplate, "{THEME_NAME}", themeName)
	html = strings.ReplaceAll(html, "{PLUGIN_ASSETS}", pluginAssets)
	html = strings.ReplaceAll(html, "{SLIDES_HTML}", slidesHTML)
	html = strings.ReplaceAll(html, "{FILE_PATH}", filePath)
	html = strings.ReplaceAll(html, "{SLIDE_COUNT}", fmt.Sprintf("%d", slideCount))
	return html
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
