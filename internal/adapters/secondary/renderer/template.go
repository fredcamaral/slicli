package renderer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// TemplateRenderer implements the Renderer interface using Go templates
type TemplateRenderer struct {
	templates *template.Template
}

// NewTemplateRenderer creates a new template-based renderer
func NewTemplateRenderer() (*TemplateRenderer, error) {
	// Define default templates
	tmpl := template.New("presentation")

	// Add template functions
	tmpl = tmpl.Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) // #nosec G203 - intentional safe HTML template function
		},
	})

	// Parse default templates
	_, err := tmpl.Parse(defaultPresentationTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing presentation template: %w", err)
	}

	_, err = tmpl.New("slide").Parse(defaultSlideTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing slide template: %w", err)
	}

	_, err = tmpl.New("presenter").Parse(defaultPresenterTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing presenter template: %w", err)
	}

	return &TemplateRenderer{
		templates: tmpl,
	}, nil
}

// RenderPresentation renders a complete presentation to HTML
func (r *TemplateRenderer) RenderPresentation(ctx context.Context, p *entities.Presentation) ([]byte, error) {
	data := struct {
		Title    string
		Author   string
		Date     string
		Theme    string
		Slides   []entities.Slide
		Metadata map[string]interface{}
	}{
		Title:    p.Title,
		Author:   p.Author,
		Date:     p.Date.Format("2006-01-02"),
		Theme:    p.Theme,
		Slides:   p.Slides,
		Metadata: p.Metadata,
	}

	var buf bytes.Buffer
	if err := r.templates.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing presentation template: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderSlide renders a single slide to HTML
func (r *TemplateRenderer) RenderSlide(ctx context.Context, s *entities.Slide) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, "slide", s); err != nil {
		return nil, fmt.Errorf("executing slide template: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderPresenter renders the presenter mode view
func (r *TemplateRenderer) RenderPresenter(ctx context.Context, p *entities.Presentation) ([]byte, error) {
	data := struct {
		Title       string
		Author      string
		Date        string
		Theme       string
		TotalSlides int
		Metadata    map[string]interface{}
	}{
		Title:       p.Title,
		Author:      p.Author,
		Date:        p.Date.Format("2006-01-02"),
		Theme:       p.Theme,
		TotalSlides: len(p.Slides),
		Metadata:    p.Metadata,
	}

	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, "presenter", data); err != nil {
		return nil, fmt.Errorf("executing presenter template: %w", err)
	}

	return buf.Bytes(), nil
}

// Default templates
const defaultPresentationTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    
    <!-- External CSS with responsive design -->
    <link rel="stylesheet" href="/assets/css/main.css">
    
    <style>
        /* Template-specific overrides - main.css handles most styling */
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .slide h1 { font-size: 2.5em; color: #2c3e50; }
        .slide h2 { font-size: 2em; color: #34495e; }
        .slide h3 { font-size: 1.5em; color: #34495e; }
        .slide pre { background: #f4f4f4; padding: 1em; border-radius: 4px; }
        .slide code { background: #f4f4f4; padding: 0.2em 0.4em; border-radius: 3px; }
        .slide blockquote { border-left: 4px solid #ddd; padding-left: 1em; color: #666; }
        .slide table { border-collapse: collapse; width: 100%; }
        .slide table th, .slide table td { border: 1px solid #ddd; padding: 0.5em; }
    </style>
</head>
<body>
    <div class="presentation" data-theme="{{.Theme}}" data-transition="slide">
        <div class="metadata">
            {{if .Author}}<div>{{.Author}}</div>{{end}}
            {{if .Date}}<div>{{.Date}}</div>{{end}}
        </div>
        
        {{range $index, $slide := .Slides}}
        <div class="slide" data-index="{{$index}}">
            {{$slide.HTML | safeHTML}}
            {{if $slide.Notes}}
            <div class="speaker-notes" style="display: none;">
                {{$slide.Notes}}
            </div>
            {{end}}
        </div>
        {{end}}
        
        <div class="controls">
            <button id="prev" onclick="previousSlide()">Previous</button>
            <button id="next" onclick="nextSlide()">Next</button>
        </div>
        
        <div class="slide-number">
            <span id="current-slide">1</span> / <span id="total-slides">{{len .Slides}}</span>
        </div>
        
        <!-- Mobile Navigation -->
        <div class="mobile-nav" style="display: none;">
            <div class="mobile-nav-content">
                <button id="mobile-prev" aria-label="Previous slide">‹</button>
                <div class="mobile-slide-counter">
                    <span id="mobile-current-slide">1</span> / <span id="mobile-total-slides">{{len .Slides}}</span>
                </div>
                <button id="mobile-next" aria-label="Next slide">›</button>
            </div>
        </div>
        
        <!-- Swipe Indicators -->
        <div class="swipe-indicator left" aria-hidden="true">‹</div>
        <div class="swipe-indicator right" aria-hidden="true">›</div>
    </div>
    
    <!-- External JavaScript with enhanced touch support -->
    <script src="/assets/js/slicli.js"></script>
</body>
</html>`

const defaultSlideTemplate = `{{.HTML | safeHTML}}
{{if .Notes}}
<div class="speaker-notes" style="display: none;">
    {{.Notes}}
</div>
{{end}}`

const defaultPresenterTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Presenter Mode - {{.Title}}</title>
    
    <!-- Presenter CSS -->
    <link rel="stylesheet" href="/assets/css/presenter.css">
    
    <!-- Base styles for content -->
    <style>
        /* Ensure presenter mode takes full screen */
        body, html {
            margin: 0;
            padding: 0;
            width: 100%;
            height: 100%;
            overflow: hidden;
        }
        
        /* Hide default content when presenter mode is active */
        .presentation-content {
            display: none;
        }
    </style>
</head>
<body>
    <!-- The presenter interface will be dynamically created by presenter.js -->
    
    <!-- Hidden data for JavaScript -->
    <script type="application/json" id="presenter-data">
    {
        "title": "{{.Title}}",
        "author": "{{.Author}}",
        "totalSlides": {{.TotalSlides}},
        "currentSlide": 0,
        "websocketUrl": "/ws?mode=presenter"
    }
    </script>
    
    <!-- Presenter JavaScript -->
    <script src="/assets/js/presenter.js"></script>
</body>
</html>`
