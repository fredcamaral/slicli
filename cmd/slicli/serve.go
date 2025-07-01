package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

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
	htmlContent, err := loadPresentationContent(presentationPath)
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
func loadPresentationContent(presentationPath string) (string, error) {
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
	return processMarkdownToSlides(string(markdownContent), presentationPath), nil
}

// createHTTPServer creates and configures the HTTP server with handlers
func createHTTPServer(config *entities.Config, htmlContent string) *http.Server {
	mux := http.NewServeMux()

	// Serve the presentation
	mux.HandleFunc("/", createPresentationHandler(htmlContent))

	// Serve static assets
	mux.HandleFunc("/assets/", createAssetsHandler())

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
		switch r.URL.Path {
		case "/assets/style.css":
			w.Header().Set("Content-Type", "text/css")
			css := getDefaultCSS()
			if _, err := w.Write([]byte(css)); err != nil {
				log.Printf("[ERROR] Failed to write CSS: %v", err)
			}
		case "/assets/script.js":
			w.Header().Set("Content-Type", "text/javascript")
			js := getDefaultJS()
			if _, err := w.Write([]byte(js)); err != nil {
				log.Printf("[ERROR] Failed to write JS: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
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
func processMarkdownToSlides(markdown, filePath string) string {
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

		// Wrap in slide div
		slideHTML := fmt.Sprintf(`
		<div class="slide" id="slide-%d">
			<div class="slide-content">
				%s
			</div>
		</div>`, i+1, htmlContent)

		htmlSlides = append(htmlSlides, slideHTML)
	}

	// Generate complete HTML page
	return generatePresentationHTML(strings.Join(htmlSlides, "\n"), filePath)
}

// basicMarkdownToHTML provides basic markdown to HTML conversion
func basicMarkdownToHTML(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var html strings.Builder

	processor := &markdownProcessor{
		html:         &html,
		inCodeBlock:  false,
		codeLanguage: "",
	}

	for _, line := range lines {
		processor.processLine(strings.TrimSpace(line))
	}

	return html.String()
}

// markdownProcessor handles the stateful markdown to HTML conversion
type markdownProcessor struct {
	html         *strings.Builder
	inCodeBlock  bool
	codeLanguage string
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
		p.html.WriteString(`<div class="mermaid">`)
	} else {
		_, _ = fmt.Fprintf(p.html, `<pre class="code-block %s"><code>`, p.codeLanguage)
	}
}

// writeCodeBlockClosing writes the closing tags for code blocks
func (p *markdownProcessor) writeCodeBlockClosing() {
	if p.codeLanguage == "mermaid" {
		p.html.WriteString("</div>\n")
	} else {
		p.html.WriteString("</code></pre>\n")
	}
}

// handleCodeBlockContent handles content inside code blocks
func (p *markdownProcessor) handleCodeBlockContent(line string) {
	p.html.WriteString(line + "\n")
}

// handleRegularContent handles regular markdown content (headers, paragraphs, lists)
func (p *markdownProcessor) handleRegularContent(line string) {
	if line == "" {
		return
	}

	// Handle headers (h3, h2, h1 in priority order)
	if strings.HasPrefix(line, "### ") {
		p.html.WriteString("<h3>" + strings.TrimPrefix(line, "### ") + "</h3>\n")
	} else if strings.HasPrefix(line, "## ") {
		p.html.WriteString("<h2>" + strings.TrimPrefix(line, "## ") + "</h2>\n")
	} else if strings.HasPrefix(line, "# ") {
		p.html.WriteString("<h1>" + strings.TrimPrefix(line, "# ") + "</h1>\n")
	} else if strings.HasPrefix(line, "- ") {
		// Handle list items
		p.html.WriteString("<li>" + strings.TrimPrefix(line, "- ") + "</li>\n")
	} else {
		// Regular paragraph
		p.html.WriteString("<p>" + line + "</p>\n")
	}
}

// generatePresentationHTML creates the complete HTML page with plugin assets
func generatePresentationHTML(slidesHTML, filePath string) string {
	// TODO: In a real implementation, we would get the plugin renderer instance
	// to access stored assets and include them in the HTML head section
	// For now, we include default assets and common plugin dependencies

	pluginAssets := `
    <!-- Common plugin assets -->
    <script src="https://unpkg.com/mermaid@10/dist/mermaid.min.js"></script>
    <script src="https://unpkg.com/prismjs@1/components/prism-core.min.js"></script>
    <script src="https://unpkg.com/prismjs@1/plugins/autoloader/prism-autoloader.min.js"></script>
    <link rel="stylesheet" href="https://unpkg.com/prismjs@1/themes/prism.css">
    
    <!-- Plugin-specific assets would be injected here -->
    <!-- %PLUGIN_ASSETS% -->`

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SLICLI Presentation</title>
    <link rel="stylesheet" href="/assets/style.css">%s
</head>
<body>
    <div class="presentation-container">
        <div class="slides-wrapper">
            %s
        </div>
        <div class="navigation">
            <button onclick="previousSlide()">←</button>
            <span class="slide-counter">
                <span id="current-slide">1</span> / <span id="total-slides">1</span>
            </span>
            <button onclick="nextSlide()">→</button>
        </div>
        <div class="presentation-info">
            <strong>File:</strong> %s
        </div>
    </div>
    <script src="/assets/script.js"></script>
    <script>
        // Initialize Mermaid with specific configuration
        mermaid.initialize({
            startOnLoad: true,
            theme: 'default',
            securityLevel: 'loose',
            fontFamily: 'monospace',
            flowchart: {
                htmlLabels: true,
                curve: 'linear'
            },
            er: {
                useMaxWidth: false
            }
        });
        
        // Initialize Prism.js for syntax highlighting
        if (window.Prism) {
            Prism.highlightAll();
        }
        
        // Force re-render of plugin content after page load
        document.addEventListener('DOMContentLoaded', function() {
            setTimeout(function() {
                // Re-initialize Mermaid diagrams
                if (window.mermaid) {
                    mermaid.init(undefined, document.querySelectorAll('.mermaid'));
                }
                
                // Re-highlight code blocks
                if (window.Prism) {
                    Prism.highlightAll();
                }
            }, 500);
        });
    </script>
</body>
</html>`, pluginAssets, slidesHTML, filePath)
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
