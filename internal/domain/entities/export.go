package entities

import "time"

// ExportFormat represents different export formats
type ExportFormat string

// Export format constants
const (
	ExportFormatPDF        ExportFormat = "pdf"
	ExportFormatHTML       ExportFormat = "html"
	ExportFormatImages     ExportFormat = "images"
	ExportFormatMarkdown   ExportFormat = "markdown"
	ExportFormatPowerPoint ExportFormat = "pptx"
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
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
