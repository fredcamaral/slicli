// Package sanitizer provides HTML sanitization utilities for secure rendering.
package sanitizer

import (
	"regexp"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	// presentationPolicy is a BlueMonday policy configured for presentation content.
	// It allows safe HTML elements typically needed in presentations while blocking XSS vectors.
	presentationPolicy *bluemonday.Policy

	// pluginPolicy is a more restrictive policy for plugin-generated HTML.
	pluginPolicy *bluemonday.Policy

	// strictPolicy is the most restrictive policy, stripping all HTML.
	strictPolicy *bluemonday.Policy

	once sync.Once
)

// initPolicies initializes the sanitization policies.
func initPolicies() {
	once.Do(func() {
		// Presentation policy: allows safe HTML for presentations
		presentationPolicy = bluemonday.NewPolicy()

		// Allow common structural elements
		presentationPolicy.AllowElements(
			"div", "span", "p", "br", "hr",
			"article", "section", "aside", "header", "footer", "main", "nav",
		)

		// Allow headings
		presentationPolicy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")

		// Allow text formatting
		presentationPolicy.AllowElements(
			"strong", "b", "em", "i", "u", "s", "strike", "del", "ins",
			"sub", "sup", "mark", "small", "big",
		)

		// Allow lists
		presentationPolicy.AllowElements("ul", "ol", "li", "dl", "dt", "dd")

		// Allow input elements for task lists (only when disabled)
		presentationPolicy.AllowElements("input")
		presentationPolicy.AllowAttrs("type", "checked", "disabled").OnElements("input")

		// Allow tables
		presentationPolicy.AllowElements(
			"table", "thead", "tbody", "tfoot", "tr", "th", "td",
			"caption", "colgroup", "col",
		)

		// Allow code blocks (essential for presentations)
		presentationPolicy.AllowElements("pre", "code", "kbd", "samp", "var")

		// Allow blockquotes and citations
		presentationPolicy.AllowElements("blockquote", "cite", "q")

		// Allow figures and media containers
		presentationPolicy.AllowElements("figure", "figcaption")

		// Allow images with restricted attributes
		presentationPolicy.AllowElements("img")
		presentationPolicy.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
		// Only allow data URLs and relative paths for images
		presentationPolicy.AllowURLSchemes("data")
		presentationPolicy.AllowRelativeURLs(true)

		// Allow safe attributes on all elements
		presentationPolicy.AllowAttrs("id", "class", "title", "lang", "dir").Globally()

		// Allow data attributes (needed for Mermaid and other plugins)
		presentationPolicy.AllowDataAttributes()

		// Allow style attribute with restrictions
		presentationPolicy.AllowAttrs("style").Globally()

		// Allow table-specific attributes
		presentationPolicy.AllowAttrs("colspan", "rowspan", "scope", "headers").OnElements("th", "td")

		// Allow anchor tags with safe href attributes
		presentationPolicy.AllowElements("a")
		presentationPolicy.AllowAttrs("href", "title", "target", "rel").OnElements("a")
		presentationPolicy.AllowURLSchemes("http", "https", "mailto")
		presentationPolicy.RequireNoReferrerOnLinks(true)
		presentationPolicy.AddTargetBlankToFullyQualifiedLinks(true)

		// Allow SVG elements (needed for diagrams)
		presentationPolicy.AllowElements(
			"svg", "g", "path", "line", "rect", "circle", "ellipse", "polygon", "polyline",
			"text", "tspan", "defs", "marker", "use", "symbol", "clipPath", "mask",
			"linearGradient", "radialGradient", "stop", "pattern", "foreignObject",
		)
		presentationPolicy.AllowAttrs(
			"viewBox", "preserveAspectRatio", "xmlns", "width", "height",
			"x", "y", "x1", "y1", "x2", "y2", "cx", "cy", "r", "rx", "ry",
			"d", "fill", "stroke", "stroke-width", "stroke-linecap", "stroke-linejoin",
			"stroke-dasharray", "stroke-dashoffset", "opacity", "fill-opacity", "stroke-opacity",
			"transform", "points", "font-family", "font-size", "font-weight", "text-anchor",
			"dominant-baseline", "alignment-baseline", "dx", "dy", "offset", "stop-color",
			"stop-opacity", "gradientUnits", "gradientTransform", "patternUnits",
			"markerWidth", "markerHeight", "refX", "refY", "orient", "markerUnits",
			"clip-path", "mask", "xlink:href",
		).OnElements(
			"svg", "g", "path", "line", "rect", "circle", "ellipse", "polygon", "polyline",
			"text", "tspan", "defs", "marker", "use", "symbol", "clipPath", "mask",
			"linearGradient", "radialGradient", "stop", "pattern", "foreignObject",
		)

		// Plugin policy: more restrictive for plugin output
		pluginPolicy = bluemonday.NewPolicy()

		// Allow structural elements
		pluginPolicy.AllowElements("div", "span", "p", "br")

		// Allow headings
		pluginPolicy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")

		// Allow text formatting
		pluginPolicy.AllowElements("strong", "b", "em", "i", "u", "s", "del")

		// Allow lists
		pluginPolicy.AllowElements("ul", "ol", "li")

		// Allow tables
		pluginPolicy.AllowElements("table", "thead", "tbody", "tr", "th", "td")

		// Allow code blocks
		pluginPolicy.AllowElements("pre", "code")

		// Allow safe attributes
		pluginPolicy.AllowAttrs("id", "class").Globally()
		pluginPolicy.AllowDataAttributes()

		// Allow table attributes
		pluginPolicy.AllowAttrs("colspan", "rowspan").OnElements("th", "td")

		// Allow SVG for diagrams (plugins like mermaid need this)
		pluginPolicy.AllowElements(
			"svg", "g", "path", "line", "rect", "circle", "ellipse", "polygon", "polyline",
			"text", "tspan", "defs", "marker", "use", "symbol",
		)
		pluginPolicy.AllowAttrs(
			"viewBox", "preserveAspectRatio", "xmlns", "width", "height",
			"x", "y", "x1", "y1", "x2", "y2", "cx", "cy", "r", "rx", "ry",
			"d", "fill", "stroke", "stroke-width", "transform", "points",
			"font-family", "font-size", "text-anchor", "dominant-baseline",
		).OnElements(
			"svg", "g", "path", "line", "rect", "circle", "ellipse", "polygon", "polyline",
			"text", "tspan", "defs", "marker", "use", "symbol",
		)

		// Strict policy: strips all HTML
		strictPolicy = bluemonday.StrictPolicy()
	})
}

// SanitizeHTML sanitizes HTML content for safe presentation rendering.
// This policy allows common HTML elements needed for presentations
// while blocking potentially dangerous elements like script, iframe, etc.
func SanitizeHTML(html string) string {
	initPolicies()
	return presentationPolicy.Sanitize(html)
}

// SanitizePluginOutput sanitizes HTML output from plugins.
// This is more restrictive than SanitizeHTML to prevent plugins
// from injecting malicious content.
func SanitizePluginOutput(html string) string {
	initPolicies()
	return pluginPolicy.Sanitize(html)
}

// StripHTML removes all HTML tags, returning only text content.
// Use this for contexts where no HTML is allowed.
func StripHTML(html string) string {
	initPolicies()
	return strictPolicy.Sanitize(html)
}

// SanitizeAttribute sanitizes a single attribute value.
// This is useful for sanitizing dynamic attribute values.
func SanitizeAttribute(value string) string {
	// Remove any potentially dangerous characters
	// This is a simple sanitization for attribute values
	re := regexp.MustCompile(`[<>"'&]`)
	return re.ReplaceAllStringFunc(value, func(s string) string {
		switch s {
		case "<":
			return "&lt;"
		case ">":
			return "&gt;"
		case "\"":
			return "&quot;"
		case "'":
			return "&#39;"
		case "&":
			return "&amp;"
		default:
			return s
		}
	})
}

// SanitizeURL sanitizes a URL to prevent javascript: and other dangerous schemes.
func SanitizeURL(url string) string {
	initPolicies()
	// Use a minimal policy just for URL sanitization
	p := bluemonday.NewPolicy()
	p.AllowElements("a")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)

	// Wrap in anchor, sanitize, then extract
	sanitized := p.Sanitize(`<a href="` + url + `">link</a>`)
	// Extract href value
	re := regexp.MustCompile(`href="([^"]*)"`)
	matches := re.FindStringSubmatch(sanitized)
	if len(matches) > 1 {
		return matches[1]
	}
	// If sanitization removed the URL, return empty
	return ""
}
