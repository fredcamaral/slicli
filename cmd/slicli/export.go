package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/config"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/export"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/parser"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/infrastructure/logging"
)

var (
	// Export command flags
	exportFormat       string
	exportOutput       string
	exportTheme        string
	exportQuality      string
	exportPageSize     string
	exportOrientation  string
	exportIncludeNotes bool
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export a presentation to various formats",
	Long: `Export a markdown presentation to PDF, HTML, images, or markdown.

The export command transforms your markdown slides into the desired output format.
Supported formats: pdf, html, images, markdown.

Example:
  slicli export presentation.md --format pdf --output slides.pdf
  slicli export slides.md --format images --output ./images/
  slicli export deck.md --format html --output index.html
  slicli export notes.md --format markdown --include-notes`,
	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)

	// Add command flags with defaults
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "pdf", "Export format (pdf, html, images, markdown)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output path (default: auto-generated from input filename)")
	exportCmd.Flags().StringVarP(&exportTheme, "theme", "t", "", "Theme to use for export")
	exportCmd.Flags().StringVarP(&exportQuality, "quality", "q", "high", "Export quality (low, medium, high)")
	exportCmd.Flags().StringVar(&exportPageSize, "page-size", "A4", "Page size for PDF export (A4, Letter)")
	exportCmd.Flags().StringVar(&exportOrientation, "orientation", "landscape", "Page orientation (portrait, landscape)")
	exportCmd.Flags().BoolVar(&exportIncludeNotes, "include-notes", false, "Include speaker notes in export")
}

func runExport(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	// Get verbose flag for logging
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Load configuration
	finalConfig, err := loadExportConfig(cmd, inputPath)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Create logger
	logger := logging.NewLogger("export",
		logging.WithVerbose(verbose),
		logging.WithLevel(finalConfig.Logging.GetLevel()),
	)

	// Validate input file
	if err := validateInputFile(inputPath); err != nil {
		return err
	}

	logger.Info("Exporting presentation: %s", inputPath)

	// Read markdown content
	markdownContent, err := os.ReadFile(inputPath) // #nosec G304 - validated above
	if err != nil {
		return fmt.Errorf("reading presentation file: %w", err)
	}

	// Parse markdown into presentation entity
	presentation, err := parsePresentation(markdownContent, inputPath, finalConfig)
	if err != nil {
		return fmt.Errorf("parsing presentation: %w", err)
	}

	logger.Info("Parsed %d slides from presentation", presentation.SlideCount())

	// Determine output path
	outputPath := determineOutputPath(inputPath, exportFormat, exportOutput)

	// Create export options
	options := &entities.ExportOptions{
		Format:       entities.ExportFormat(exportFormat),
		OutputPath:   outputPath,
		Theme:        getExportTheme(finalConfig),
		Quality:      exportQuality,
		PageSize:     exportPageSize,
		Orientation:  exportOrientation,
		IncludeNotes: exportIncludeNotes,
	}

	logger.Info("Export format: %s", exportFormat)
	logger.Info("Output path: %s", outputPath)
	if options.Theme != "" {
		logger.Info("Using theme: %s", options.Theme)
	}

	// Create export service
	exportService, err := export.NewService("")
	if err != nil {
		return fmt.Errorf("creating export service: %w", err)
	}
	defer func() {
		if cleanupErr := exportService.Cleanup(); cleanupErr != nil {
			logger.Warn("Failed to cleanup export service: %v", cleanupErr)
		}
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	// Show progress message
	fmt.Printf("Exporting to %s...\n", exportFormat)

	// Perform export
	result, err := exportService.Export(ctx, presentation, options)
	if err != nil {
		return handleExportError(err)
	}

	// Display result
	displayExportResult(result, logger)

	return nil
}

// validateInputFile validates that the input file exists and is a regular file
func validateInputFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("presentation file not found: %s", path)
		}
		return fmt.Errorf("accessing presentation file: %w", err)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("presentation path is not a regular file: %s", path)
	}

	return nil
}

// parsePresentation parses markdown content into a Presentation entity
func parsePresentation(content []byte, filePath string, cfg *entities.Config) (*entities.Presentation, error) {
	// Create markdown parser
	markdownParser := parser.NewGoldmarkParser()

	// Create presentation parser adapter
	presentationParser := parser.NewPresentationParserAdapter(markdownParser)

	// Parse content into presentation
	presentation, err := presentationParser.Parse(content)
	if err != nil {
		return nil, err
	}

	// Apply theme from config if not set in presentation
	if presentation.Theme == "" && cfg != nil && cfg.Theme.Name != "" {
		presentation.Theme = cfg.Theme.Name
	}

	// Apply defaults
	presentation.ApplyDefaults()

	return presentation, nil
}

// determineOutputPath generates the output path based on input and format
func determineOutputPath(inputPath, format, explicitOutput string) string {
	// Use explicit output if provided
	if explicitOutput != "" {
		return explicitOutput
	}

	// Generate output path from input filename
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	dir := filepath.Dir(inputPath)

	switch entities.ExportFormat(format) {
	case entities.ExportFormatPDF:
		return filepath.Join(dir, baseName+".pdf")
	case entities.ExportFormatHTML:
		return filepath.Join(dir, baseName+".html")
	case entities.ExportFormatImages:
		return filepath.Join(dir, baseName+"_images")
	case entities.ExportFormatMarkdown:
		return filepath.Join(dir, baseName+"_exported.md")
	default:
		return filepath.Join(dir, baseName+"_export")
	}
}

// getExportTheme returns the theme to use for export
func getExportTheme(cfg *entities.Config) string {
	// Command line flag takes precedence
	if exportTheme != "" {
		return exportTheme
	}

	// Fall back to config
	if cfg != nil && cfg.Theme.Name != "" {
		return cfg.Theme.Name
	}

	return "default"
}

// loadExportConfig loads configuration for the export command
func loadExportConfig(cmd *cobra.Command, presentationPath string) (*entities.Config, error) {
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

	// Apply CLI flags
	applyExportCliFlags(cmd, finalConfig)

	return finalConfig, nil
}

// applyExportCliFlags applies export-specific CLI flag overrides
func applyExportCliFlags(cmd *cobra.Command, cfg *entities.Config) {
	if cmd.Flags().Changed("theme") {
		cfg.Theme.Name = exportTheme
	}
}

// handleExportError formats export errors for user-friendly display
func handleExportError(err error) error {
	// Check if it's an export-specific error
	if exportErr, ok := err.(*export.ExportError); ok {
		switch exportErr.Type {
		case export.ErrorTypeBrowser:
			return fmt.Errorf("browser automation failed: %s\n\nHint: PDF and image exports require a browser (Chrome/Chromium). Make sure it's installed", exportErr.Message)
		case export.ErrorTypeFilesystem:
			return fmt.Errorf("file system error: %s\n\nHint: Check that you have write permissions for the output directory", exportErr.Message)
		case export.ErrorTypeTimeout:
			return fmt.Errorf("export timed out: %s\n\nHint: Try reducing the number of slides or using a lower quality setting", exportErr.Message)
		case export.ErrorTypeValidation:
			return fmt.Errorf("validation error: %s", exportErr.Message)
		default:
			return fmt.Errorf("export failed: %s", exportErr.Error())
		}
	}

	return fmt.Errorf("export failed: %w", err)
}

// displayExportResult displays the export result to the user
func displayExportResult(result *entities.ExportResult, logger *logging.Logger) {
	if !result.Success {
		fmt.Printf("Export failed: %s\n", result.Error)
		for _, warning := range result.Warnings {
			fmt.Printf("Warning: %s\n", warning)
		}
		return
	}

	fmt.Println("Export completed successfully!")
	fmt.Printf("Output: %s\n", result.OutputPath)
	fmt.Printf("Format: %s\n", result.Format)

	if result.FileSize > 0 {
		fmt.Printf("Size: %s\n", formatFileSize(result.FileSize))
	}

	if result.PageCount > 0 {
		fmt.Printf("Pages: %d\n", result.PageCount)
	}

	if len(result.Files) > 0 {
		fmt.Printf("Files generated: %d\n", len(result.Files))
		logger.Info("Generated files:")
		for _, file := range result.Files {
			logger.Info("  - %s", file)
		}
	}

	fmt.Printf("Duration: %s\n", result.Duration)

	// Show warnings if any
	for _, warning := range result.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
