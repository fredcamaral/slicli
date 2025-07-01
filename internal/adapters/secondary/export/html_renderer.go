package export

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// HTMLRenderer implements export to static HTML
type HTMLRenderer struct {
	template *template.Template
}

// NewHTMLRenderer creates a new HTML renderer
func NewHTMLRenderer() *HTMLRenderer {
	tmpl := template.New("export")
	tmpl = tmpl.Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) // #nosec G203 - intentional safe HTML template function
		},
	})

	// Parse the static HTML template
	_, err := tmpl.Parse(staticHTMLTemplate)
	if err != nil {
		// Fallback to basic template if parsing fails
		tmpl, _ = template.New("export").Parse(basicHTMLTemplate)
	}

	return &HTMLRenderer{
		template: tmpl,
	}
}

// Render exports the presentation to static HTML
func (r *HTMLRenderer) Render(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error) {
	// Prepare template data
	data := struct {
		Title        string
		Author       string
		Date         string
		Theme        string
		Slides       []entities.Slide
		IncludeNotes bool
		GeneratedAt  string
		SlideCount   int
		Metadata     map[string]interface{}
	}{
		Title:        presentation.Title,
		Author:       presentation.Author,
		Date:         presentation.Date.Format("2006-01-02"),
		Theme:        options.Theme,
		Slides:       presentation.Slides,
		IncludeNotes: options.IncludeNotes,
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
		SlideCount:   len(presentation.Slides),
		Metadata:     options.Metadata,
	}

	// Apply theme if specified
	if options.Theme == "" {
		data.Theme = presentation.Theme
	}

	// Create output file
	outputFile, err := os.Create(options.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	defer func() { _ = outputFile.Close() }()

	// Execute template
	if err := r.template.Execute(outputFile, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	// Get file size
	fileSize, _ := GetFileSize(options.OutputPath)

	return &ExportResult{
		Success:    true,
		Format:     string(FormatHTML),
		OutputPath: options.OutputPath,
		FileSize:   fileSize,
		PageCount:  len(presentation.Slides),
	}, nil
}

// Supports returns true if this renderer supports the given format
func (r *HTMLRenderer) Supports(format ExportFormat) bool {
	return format == FormatHTML
}

// GetMimeType returns the MIME type for HTML exports
func (r *HTMLRenderer) GetMimeType() string {
	return "text/html"
}

// Static HTML template for standalone presentations
const staticHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <meta name="author" content="{{.Author}}">
    <meta name="generator" content="slicli - CLI Presentation Generator">
    <meta name="export-date" content="{{.GeneratedAt}}">
    
    <style>
        /* Reset and base styles */
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            overflow-x: hidden;
        }
        
        /* Presentation container */
        .presentation {
            max-width: 1200px;
            margin: 0 auto;
            position: relative;
            height: 100vh;
            overflow: hidden;
            perspective: 1000px;
        }
        
        /* Slide styles */
        .slide {
            background: white;
            padding: 60px;
            margin: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            min-height: 600px;
            position: absolute;
            width: calc(100% - 40px);
            opacity: 0;
            transform: translateX(100%);
            transition: all 0.5s cubic-bezier(0.4, 0.0, 0.2, 1);
            z-index: 1;
        }
        
        .slide.active {
            opacity: 1;
            transform: translateX(0);
            z-index: 2;
        }
        
        .slide.prev {
            transform: translateX(-100%);
            opacity: 0;
        }
        
        .slide.next {
            transform: translateX(100%);
            opacity: 0;
        }
        
        /* Typography */
        .slide h1 {
            font-size: 3em;
            margin-bottom: 0.5em;
            color: #2c3e50;
        }
        
        .slide h2 {
            font-size: 2.5em;
            margin-bottom: 0.5em;
            color: #34495e;
        }
        
        .slide h3 {
            font-size: 2em;
            margin-bottom: 0.5em;
            color: #34495e;
        }
        
        .slide p {
            font-size: 1.2em;
            margin-bottom: 1em;
        }
        
        .slide ul, .slide ol {
            margin-left: 2em;
            margin-bottom: 1em;
        }
        
        .slide li {
            font-size: 1.2em;
            margin-bottom: 0.5em;
        }
        
        /* Code blocks */
        .slide pre {
            background: #f4f4f4;
            border: 1px solid #ddd;
            border-radius: 4px;
            padding: 1em;
            overflow-x: auto;
            margin-bottom: 1em;
        }
        
        .slide code {
            background: #f4f4f4;
            padding: 0.2em 0.4em;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
        }
        
        .slide pre code {
            background: none;
            padding: 0;
        }
        
        /* Tables */
        .slide table {
            border-collapse: collapse;
            width: 100%;
            margin-bottom: 1em;
        }
        
        .slide th, .slide td {
            border: 1px solid #ddd;
            padding: 0.5em;
            text-align: left;
        }
        
        .slide th {
            background: #f4f4f4;
            font-weight: bold;
        }
        
        /* Blockquotes */
        .slide blockquote {
            border-left: 4px solid #ddd;
            padding-left: 1em;
            color: #666;
            font-style: italic;
            margin-bottom: 1em;
        }
        
        /* Speaker notes */
        .speaker-notes {
            {{if not .IncludeNotes}}display: none;{{else}}
            margin-top: 2em;
            padding-top: 2em;
            border-top: 2px dashed #ddd;
            font-size: 0.9em;
            color: #666;
            font-style: italic;
            {{end}}
        }
        
        /* Navigation controls */
        .controls {
            position: fixed;
            bottom: 20px;
            right: 20px;
            display: flex;
            gap: 10px;
            z-index: 100;
        }
        
        .controls button {
            padding: 10px 20px;
            font-size: 16px;
            background: #3498db;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            transition: background 0.3s;
        }
        
        .controls button:hover {
            background: #2980b9;
        }
        
        .controls button:disabled {
            background: #95a5a6;
            cursor: not-allowed;
        }
        
        /* Slide counter */
        .slide-number {
            position: fixed;
            bottom: 20px;
            left: 20px;
            font-size: 14px;
            color: #666;
            z-index: 100;
        }
        
        /* Metadata */
        .metadata {
            position: fixed;
            top: 20px;
            right: 20px;
            font-size: 14px;
            color: #666;
            text-align: right;
            z-index: 100;
        }
        
        /* Progress bar */
        .progress-bar {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            height: 4px;
            background: rgba(0, 0, 0, 0.1);
            z-index: 1000;
        }
        
        .progress-bar-fill {
            height: 100%;
            background: linear-gradient(90deg, #3498db, #2ecc71);
            transition: width 0.3s ease;
            width: 0%;
        }
        
        /* Export info */
        .export-info {
            position: fixed;
            bottom: 20px;
            left: 50%;
            transform: translateX(-50%);
            background: rgba(0, 0, 0, 0.7);
            color: white;
            padding: 5px 10px;
            border-radius: 4px;
            font-size: 12px;
            z-index: 1000;
        }
        
        /* Print styles */
        @media print {
            .controls,
            .slide-number,
            .metadata,
            .progress-bar,
            .export-info {
                display: none;
            }
            
            .slide {
                page-break-after: always;
                display: block !important;
                position: relative !important;
                opacity: 1 !important;
                transform: none !important;
                box-shadow: none;
                margin: 0;
                width: 100% !important;
            }
            
            .speaker-notes {
                display: block !important;
            }
        }
        
        /* Responsive design */
        @media (max-width: 768px) {
            .slide {
                padding: 30px;
                margin: 10px;
                min-height: 400px;
            }
            
            .slide h1 { font-size: 2em; }
            .slide h2 { font-size: 1.7em; }
            .slide h3 { font-size: 1.4em; }
            .slide p, .slide li { font-size: 1em; }
        }
    </style>
</head>
<body>
    <div class="presentation" data-theme="{{.Theme}}">
        <!-- Progress bar -->
        <div class="progress-bar">
            <div class="progress-bar-fill"></div>
        </div>
        
        <!-- Metadata -->
        <div class="metadata">
            {{if .Author}}<div>{{.Author}}</div>{{end}}
            {{if .Date}}<div>{{.Date}}</div>{{end}}
        </div>
        
        <!-- Slides -->
        {{range $index, $slide := .Slides}}
        <div class="slide" data-index="{{$index}}">
            {{$slide.HTML | safeHTML}}
            {{if $.IncludeNotes}}{{if $slide.Notes}}
            <div class="speaker-notes">
                <strong>Speaker Notes:</strong><br>
                {{$slide.Notes}}
            </div>
            {{end}}{{end}}
        </div>
        {{end}}
        
        <!-- Controls -->
        <div class="controls">
            <button id="prev">Previous</button>
            <button id="next">Next</button>
            <button id="fullscreen">Fullscreen</button>
        </div>
        
        <!-- Slide counter -->
        <div class="slide-number">
            <span id="current-slide">1</span> / <span id="total-slides">{{.SlideCount}}</span>
        </div>
        
        <!-- Export info -->
        <div class="export-info">
            Exported from slicli on {{.GeneratedAt}}
        </div>
    </div>
    
    <script>
        // Standalone presentation JavaScript
        (function() {
            'use strict';
            
            let currentSlide = 0;
            const slides = document.querySelectorAll('.slide');
            const totalSlides = slides.length;
            
            function showSlide(n) {
                slides.forEach((slide, index) => {
                    slide.classList.remove('active', 'prev', 'next');
                    
                    if (index === currentSlide) {
                        slide.classList.add('active');
                    } else if (index < currentSlide) {
                        slide.classList.add('prev');
                    } else {
                        slide.classList.add('next');
                    }
                });
                
                updateSlideCounter();
                updateButtonStates();
                updateProgressBar();
            }
            
            function nextSlide() {
                if (currentSlide < totalSlides - 1) {
                    currentSlide++;
                    showSlide(currentSlide);
                }
            }
            
            function previousSlide() {
                if (currentSlide > 0) {
                    currentSlide--;
                    showSlide(currentSlide);
                }
            }
            
            function updateSlideCounter() {
                document.getElementById('current-slide').textContent = currentSlide + 1;
                document.getElementById('total-slides').textContent = totalSlides;
            }
            
            function updateButtonStates() {
                document.getElementById('prev').disabled = currentSlide === 0;
                document.getElementById('next').disabled = currentSlide === totalSlides - 1;
            }
            
            function updateProgressBar() {
                const fill = document.querySelector('.progress-bar-fill');
                const progress = totalSlides > 1 ? (currentSlide / (totalSlides - 1)) * 100 : 0;
                fill.style.width = progress + '%';
            }
            
            function toggleFullscreen() {
                if (!document.fullscreenElement) {
                    document.documentElement.requestFullscreen();
                } else {
                    if (document.exitFullscreen) {
                        document.exitFullscreen();
                    }
                }
            }
            
            // Event listeners
            document.getElementById('prev').addEventListener('click', previousSlide);
            document.getElementById('next').addEventListener('click', nextSlide);
            document.getElementById('fullscreen').addEventListener('click', toggleFullscreen);
            
            // Keyboard navigation
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
                    case 'Home':
                        e.preventDefault();
                        currentSlide = 0;
                        showSlide(currentSlide);
                        break;
                    case 'End':
                        e.preventDefault();
                        currentSlide = totalSlides - 1;
                        showSlide(currentSlide);
                        break;
                    case 'f':
                    case 'F':
                        toggleFullscreen();
                        break;
                }
            });
            
            // Initialize
            showSlide(0);
        })();
    </script>
</body>
</html>`

// Basic fallback template
const basicHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .slide { margin-bottom: 60px; padding: 20px; border: 1px solid #ccc; }
        h1, h2, h3 { color: #333; }
        .speaker-notes { background: #f9f9f9; padding: 10px; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>{{.Title}}</h1>
    {{if .Author}}<p><strong>Author:</strong> {{.Author}}</p>{{end}}
    {{if .Date}}<p><strong>Date:</strong> {{.Date}}</p>{{end}}
    
    {{range $index, $slide := .Slides}}
    <div class="slide">
        <h2>Slide {{add $index 1}}</h2>
        {{$slide.HTML | safeHTML}}
        {{if $.IncludeNotes}}{{if $slide.Notes}}
        <div class="speaker-notes">
            <strong>Speaker Notes:</strong><br>
            {{$slide.Notes}}
        </div>
        {{end}}{{end}}
    </div>
    {{end}}
    
    <footer>
        <p><em>Generated by slicli on {{.GeneratedAt}}</em></p>
    </footer>
</body>
</html>`
