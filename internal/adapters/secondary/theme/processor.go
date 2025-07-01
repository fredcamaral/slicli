package theme

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// AssetProcessor processes theme assets
type AssetProcessor struct {
	minifyEnabled bool
}

// NewAssetProcessor creates a new asset processor
func NewAssetProcessor(minifyEnabled bool) *AssetProcessor {
	return &AssetProcessor{
		minifyEnabled: minifyEnabled,
	}
}

// Process processes content based on content type
func (p *AssetProcessor) Process(content []byte, contentType string, variables map[string]string) ([]byte, error) {
	switch contentType {
	case "text/css", "text/css; charset=utf-8":
		return p.ProcessCSS(content, variables)
	case "application/javascript", "text/javascript", "application/javascript; charset=utf-8":
		return p.ProcessJS(content, variables)
	case "text/html", "text/html; charset=utf-8":
		return p.ProcessHTML(content)
	default:
		// For other content types, return as-is
		return content, nil
	}
}

// ProcessCSS processes CSS with variable substitution
func (p *AssetProcessor) ProcessCSS(content []byte, variables map[string]string) ([]byte, error) {
	css := string(content)

	// Create a complete variable map including values from :root definitions
	allVariables := make(map[string]string)

	// First extract variables from :root definitions
	rootPattern := regexp.MustCompile(`--([a-zA-Z0-9-]+):\s*([^;]+);`)
	rootMatches := rootPattern.FindAllStringSubmatch(css, -1)
	for _, match := range rootMatches {
		if len(match) >= 3 {
			allVariables[match[1]] = strings.TrimSpace(match[2])
		}
	}

	// Override with provided variables
	for name, value := range variables {
		allVariables[name] = value
	}

	// Process var() calls recursively to handle nested patterns
	maxIterations := 10 // Prevent infinite loops
	for i := 0; i < maxIterations; i++ {
		changed := false

		// Handle patterns with fallbacks: var(--name, fallback)
		varWithFallbackPattern := regexp.MustCompile(`var\(--([a-zA-Z0-9-]+),\s*([^)]+)\)`)
		css = varWithFallbackPattern.ReplaceAllStringFunc(css, func(match string) string {
			parts := varWithFallbackPattern.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}
			varName := parts[1]
			fallback := strings.TrimSpace(parts[2])

			if value, ok := allVariables[varName]; ok {
				changed = true
				return value
			}
			// Use fallback
			changed = true
			return fallback
		})

		// Handle simple patterns: var(--name)
		varPattern := regexp.MustCompile(`var\(--([a-zA-Z0-9-]+)\)`)
		css = varPattern.ReplaceAllStringFunc(css, func(match string) string {
			varName := match[6 : len(match)-1] // Remove "var(--" and ")"
			if value, ok := allVariables[varName]; ok {
				changed = true
				return value
			}
			return match
		})

		// If no changes were made, we're done
		if !changed {
			break
		}
	}

	// Update :root CSS variables definitions with provided variables
	rootUpdatePattern := regexp.MustCompile(`(:root\s*\{[^}]*\})`)
	css = rootUpdatePattern.ReplaceAllStringFunc(css, func(match string) string {
		for name, value := range variables {
			// Replace --name: oldvalue; patterns with --name: newvalue;
			oldVar := fmt.Sprintf(`(--%s:\s*[^;]+;)`, regexp.QuoteMeta(name))
			newVar := fmt.Sprintf("--%s: %s;", name, value)
			varPattern := regexp.MustCompile(oldVar)
			match = varPattern.ReplaceAllString(match, newVar)
		}
		return match
	})

	// Process @import statements for theme inheritance
	// This is a simple implementation - in production you might want more sophisticated handling
	importPattern := regexp.MustCompile(`@import\s+["']([^"']+)["'];`)
	css = importPattern.ReplaceAllStringFunc(css, func(match string) string {
		// For now, just remove @import statements as they're handled at load time
		return "/* " + match + " */"
	})

	result := []byte(css)

	// Optionally minify
	if p.minifyEnabled {
		minified, err := p.MinifyCSS(result)
		if err != nil {
			// If minification fails, return processed but not minified
			return result, nil
		}
		result = minified
	}

	return result, nil
}

// MinifyCSS minifies CSS content
func (p *AssetProcessor) MinifyCSS(content []byte) ([]byte, error) {
	css := string(content)

	// Simple CSS minification
	// In production, you might want to use a proper CSS minifier library

	// Remove comments
	commentPattern := regexp.MustCompile(`/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`)
	css = commentPattern.ReplaceAllString(css, "")

	// Remove unnecessary whitespace
	css = regexp.MustCompile(`\s+`).ReplaceAllString(css, " ")

	// Remove whitespace around specific characters
	css = regexp.MustCompile(`\s*([{}:;,])\s*`).ReplaceAllString(css, "$1")

	// Remove trailing semicolon before closing brace
	css = strings.ReplaceAll(css, ";}", "}")

	// Remove leading/trailing whitespace
	css = strings.TrimSpace(css)

	return []byte(css), nil
}

// ProcessJS processes JavaScript files
func (p *AssetProcessor) ProcessJS(content []byte, variables map[string]string) ([]byte, error) {
	js := string(content)

	// Replace template variables {{variable-name}} with actual values
	templatePattern := regexp.MustCompile(`\{\{([a-zA-Z0-9-]+)\}\}`)
	js = templatePattern.ReplaceAllStringFunc(js, func(match string) string {
		// Extract variable name from {{name}}
		varName := match[2 : len(match)-2] // Remove "{{" and "}}"

		if value, ok := variables[varName]; ok {
			return value
		}
		// Keep original if variable not found
		return match
	})

	// Simple processing - in production you might want to use a JS minifier
	// For now, just remove single-line comments and trim whitespace

	if p.minifyEnabled {
		// Remove single-line comments (but not URLs with //)
		commentPattern := regexp.MustCompile(`(?m)^\s*//.*$`)
		js = commentPattern.ReplaceAllString(js, "")

		// Remove excessive newlines
		js = regexp.MustCompile(`\n{3,}`).ReplaceAllString(js, "\n\n")

		// Trim whitespace
		js = strings.TrimSpace(js)
	}

	return []byte(js), nil
}

// ProcessHTML processes HTML templates (for future use)
func (p *AssetProcessor) ProcessHTML(content []byte) ([]byte, error) {
	// For now, just return as-is
	// In the future, this could handle HTML minification or other processing
	return content, nil
}

// InlineAssets processes assets for inlining in HTML
func (p *AssetProcessor) InlineAssets(content []byte, assetType string) string {
	processed := content

	switch assetType {
	case "css":
		// Wrap in style tags for inlining
		return fmt.Sprintf("<style>%s</style>", string(processed))
	case "js":
		// Wrap in script tags for inlining
		return fmt.Sprintf("<script>%s</script>", string(processed))
	default:
		return string(processed)
	}
}

// CriticalRule represents a rule for identifying critical CSS
type CriticalRule struct {
	Pattern     *regexp.Regexp
	Description string
	Priority    int // Lower numbers = higher priority
}

// GetCriticalCSS extracts critical CSS for above-the-fold content
// This implementation identifies CSS rules that are essential for initial page rendering
func (p *AssetProcessor) GetCriticalCSS(fullCSS []byte) ([]byte, error) {
	css := string(fullCSS)
	var critical bytes.Buffer
	processedRules := make(map[string]bool) // Prevent duplicates

	// Define critical CSS patterns in priority order
	criticalRules := []CriticalRule{
		// 1. Base elements (highest priority)
		{regexp.MustCompile(`(?s)html\s*{[^}]+}`), "HTML base styles", 1},
		{regexp.MustCompile(`(?s)body\s*{[^}]+}`), "Body base styles", 1},
		{regexp.MustCompile(`(?s)\*\s*{[^}]+}`), "Universal reset", 1},
		{regexp.MustCompile(`(?s)\*,\s*\*::before,\s*\*::after\s*{[^}]+}`), "Universal box-sizing", 1},

		// 2. Layout containers
		{regexp.MustCompile(`(?s)\.container\s*{[^}]+}`), "Main container", 2},
		{regexp.MustCompile(`(?s)\.slide\s*{[^}]+}`), "Slide container", 2},
		{regexp.MustCompile(`(?s)\.presentation\s*{[^}]+}`), "Presentation wrapper", 2},
		{regexp.MustCompile(`(?s)\.slide-content\s*{[^}]+}`), "Slide content area", 2},

		// 3. Typography (critical for content visibility)
		{regexp.MustCompile(`(?s)h[1-6]\s*{[^}]+}`), "Heading styles", 3},
		{regexp.MustCompile(`(?s)p\s*{[^}]+}`), "Paragraph styles", 3},
		{regexp.MustCompile(`(?s)a\s*{[^}]+}`), "Link styles", 3},
		{regexp.MustCompile(`(?s)strong\s*{[^}]+}`), "Bold text", 3},
		{regexp.MustCompile(`(?s)em\s*{[^}]+}`), "Italic text", 3},

		// 4. Lists and text formatting
		{regexp.MustCompile(`(?s)ul\s*{[^}]+}`), "Unordered lists", 4},
		{regexp.MustCompile(`(?s)ol\s*{[^}]+}`), "Ordered lists", 4},
		{regexp.MustCompile(`(?s)li\s*{[^}]+}`), "List items", 4},
		{regexp.MustCompile(`(?s)blockquote\s*{[^}]+}`), "Block quotes", 4},

		// 5. Code blocks (common in presentations)
		{regexp.MustCompile(`(?s)code\s*{[^}]+}`), "Inline code", 5},
		{regexp.MustCompile(`(?s)pre\s*{[^}]+}`), "Code blocks", 5},
		{regexp.MustCompile(`(?s)\.highlight\s*{[^}]+}`), "Syntax highlighting", 5},

		// 6. Navigation and controls
		{regexp.MustCompile(`(?s)\.nav\s*{[^}]+}`), "Navigation", 6},
		{regexp.MustCompile(`(?s)\.controls\s*{[^}]+}`), "Presentation controls", 6},
		{regexp.MustCompile(`(?s)\.progress\s*{[^}]+}`), "Progress indicator", 6},

		// 7. Visibility and display utilities
		{regexp.MustCompile(`(?s)\.hidden\s*{[^}]+}`), "Hidden elements", 7},
		{regexp.MustCompile(`(?s)\.visible\s*{[^}]+}`), "Visible elements", 7},
		{regexp.MustCompile(`(?s)\.show\s*{[^}]+}`), "Show utility", 7},
		{regexp.MustCompile(`(?s)\.hide\s*{[^}]+}`), "Hide utility", 7},

		// 8. CSS Custom Properties (CSS Variables)
		{regexp.MustCompile(`(?s):root\s*{[^}]+}`), "CSS custom properties", 8},

		// 9. Media queries for critical responsive behavior
		{regexp.MustCompile(`(?s)@media\s*\([^{]*max-width:\s*768px[^{]*\)\s*{[^{}]*(?:{[^{}]*}[^{}]*)*}`), "Mobile breakpoint", 9},
		{regexp.MustCompile(`(?s)@media\s*\([^{]*min-width:\s*769px[^{]*\)\s*{[^{}]*(?:{[^{}]*}[^{}]*)*}`), "Desktop breakpoint", 9},

		// 10. Font face declarations
		{regexp.MustCompile(`(?s)@font-face\s*{[^}]+}`), "Font face declarations", 10},
	}

	// Extract critical CSS rules
	for _, rule := range criticalRules {
		matches := rule.Pattern.FindAllString(css, -1)
		for _, match := range matches {
			// Check for duplicates
			if !processedRules[match] {
				processedRules[match] = true
				critical.WriteString("/* " + rule.Description + " */\n")
				critical.WriteString(match)
				critical.WriteString("\n\n")
			}
		}
	}

	// Extract any remaining critical utility classes
	utilityPatterns := []*regexp.Regexp{
		// Flexbox utilities
		regexp.MustCompile(`(?s)\.flex\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.flex-col\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.flex-row\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.justify-center\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.items-center\s*{[^}]+}`),

		// Grid utilities
		regexp.MustCompile(`(?s)\.grid\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.grid-cols-\d+\s*{[^}]+}`),

		// Spacing utilities (margin/padding)
		regexp.MustCompile(`(?s)\.[mp][trblxy]?-\d+\s*{[^}]+}`),

		// Text utilities
		regexp.MustCompile(`(?s)\.text-(left|center|right)\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.text-(xs|sm|base|lg|xl|2xl|3xl|4xl|5xl|6xl)\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.font-(thin|light|normal|medium|semibold|bold|extrabold|black)\s*{[^}]+}`),

		// Background and color utilities
		regexp.MustCompile(`(?s)\.bg-\w+\s*{[^}]+}`),
		regexp.MustCompile(`(?s)\.text-\w+\s*{[^}]+}`),
	}

	// Process utility patterns
	for _, pattern := range utilityPatterns {
		matches := pattern.FindAllString(css, -1)
		for _, match := range matches {
			if !processedRules[match] {
				processedRules[match] = true
				critical.WriteString("/* Utility class */\n")
				critical.WriteString(match)
				critical.WriteString("\n\n")
			}
		}
	}

	// If no critical CSS was found, include basic fallback rules
	if critical.Len() == 0 {
		critical.WriteString("/* Fallback critical CSS */\n")
		critical.WriteString("html,body{margin:0;padding:0;font-family:sans-serif;}\n")
		critical.WriteString(".slide{display:block;width:100%;height:100vh;}\n")
		critical.WriteString("h1,h2,h3,h4,h5,h6{margin:0 0 1rem 0;}\n")
		critical.WriteString("p{margin:0 0 1rem 0;}\n")
	}

	// Optionally minify the critical CSS
	if p.minifyEnabled {
		return p.MinifyCSS(critical.Bytes())
	}

	return critical.Bytes(), nil
}

// MinifyJS minifies JavaScript content
func (p *AssetProcessor) MinifyJS(content []byte) ([]byte, error) {
	if !p.minifyEnabled {
		return content, nil
	}

	js := string(content)

	// Simple minification: remove comments and extra whitespace
	// Remove single line comments
	singleLineCommentPattern := regexp.MustCompile(`//[^\n]*`)
	js = singleLineCommentPattern.ReplaceAllString(js, "")

	// Remove multi-line comments
	multiLineCommentPattern := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	js = multiLineCommentPattern.ReplaceAllString(js, "")

	// Remove extra whitespace
	js = regexp.MustCompile(`\s+`).ReplaceAllString(js, " ")

	// Remove whitespace around operators
	js = regexp.MustCompile(`\s*([=+\-*/(){},;:])\s*`).ReplaceAllString(js, "$1")

	// Trim
	js = strings.TrimSpace(js)

	return []byte(js), nil
}

// GetCriticalCSSForContent extracts critical CSS based on actual HTML content
// This analyzes the content to identify which CSS rules are actually used
func (p *AssetProcessor) GetCriticalCSSForContent(fullCSS []byte, htmlContent string) ([]byte, error) {
	// Extract all CSS selectors from the full CSS
	cssSelectors := p.extractCSSSelectors(string(fullCSS))

	// Find which selectors are actually used in the HTML content
	usedSelectors := p.findUsedSelectors(cssSelectors, htmlContent)

	// Extract the CSS rules for used selectors
	criticalCSS := p.extractRulesForSelectors(string(fullCSS), usedSelectors)

	// Always include base critical rules regardless of content analysis
	baseCSS, err := p.GetCriticalCSS(fullCSS)
	if err != nil {
		return nil, err
	}

	// Combine base critical CSS with content-specific critical CSS
	var combined bytes.Buffer
	combined.WriteString("/* Base critical CSS */\n")
	combined.Write(baseCSS)
	combined.WriteString("\n/* Content-specific critical CSS */\n")
	combined.WriteString(criticalCSS)

	if p.minifyEnabled {
		return p.MinifyCSS(combined.Bytes())
	}

	return combined.Bytes(), nil
}

// extractCSSSelectors extracts all CSS selectors from CSS content
func (p *AssetProcessor) extractCSSSelectors(css string) []string {
	var selectors []string

	// Pattern to match CSS selectors (simplified)
	// This matches selectors before opening braces
	selectorPattern := regexp.MustCompile(`([^{}]+)\s*{`)
	matches := selectorPattern.FindAllStringSubmatch(css, -1)

	for _, match := range matches {
		if len(match) > 1 {
			// Clean up the selector
			selector := strings.TrimSpace(match[1])

			// Skip @-rules and comments
			if strings.HasPrefix(selector, "@") || strings.HasPrefix(selector, "/*") {
				continue
			}

			// Split multiple selectors separated by commas
			parts := strings.Split(selector, ",")
			for _, part := range parts {
				cleanSelector := strings.TrimSpace(part)
				if cleanSelector != "" {
					selectors = append(selectors, cleanSelector)
				}
			}
		}
	}

	return selectors
}

// findUsedSelectors determines which selectors are used in the HTML content
func (p *AssetProcessor) findUsedSelectors(selectors []string, htmlContent string) []string {
	var usedSelectors []string

	for _, selector := range selectors {
		// Check if selector is used in HTML
		if p.isSelectorUsed(selector, htmlContent) {
			usedSelectors = append(usedSelectors, selector)
		}
	}

	return usedSelectors
}

// isSelectorUsed checks if a CSS selector matches elements in HTML content
func (p *AssetProcessor) isSelectorUsed(selector, htmlContent string) bool {
	// Simplified selector matching - in production you might want a proper CSS selector engine

	// Handle class selectors (.classname)
	if strings.HasPrefix(selector, ".") {
		className := strings.TrimPrefix(selector, ".")
		// Remove pseudo-selectors and combinators for basic matching
		className = strings.Split(className, ":")[0]
		className = strings.Split(className, " ")[0]
		classPattern := regexp.MustCompile(`class=["'][^"']*\b` + regexp.QuoteMeta(className) + `\b[^"']*["']`)
		return classPattern.MatchString(htmlContent)
	}

	// Handle ID selectors (#idname)
	if strings.HasPrefix(selector, "#") {
		idName := strings.TrimPrefix(selector, "#")
		idName = strings.Split(idName, ":")[0]
		idName = strings.Split(idName, " ")[0]
		idPattern := regexp.MustCompile(`id=["']` + regexp.QuoteMeta(idName) + `["']`)
		return idPattern.MatchString(htmlContent)
	}

	// Handle element selectors (tagname)
	elementName := strings.Split(selector, ":")[0]
	elementName = strings.Split(elementName, " ")[0]
	elementName = strings.Split(elementName, ".")[0]
	elementName = strings.Split(elementName, "#")[0]

	// Check for common HTML elements
	if elementName != "" {
		elementPattern := regexp.MustCompile(`<` + regexp.QuoteMeta(elementName) + `\b`)
		return elementPattern.MatchString(htmlContent)
	}

	return false
}

// extractRulesForSelectors extracts CSS rules for specific selectors
func (p *AssetProcessor) extractRulesForSelectors(css string, selectors []string) string {
	var result bytes.Buffer
	processedRules := make(map[string]bool)

	for _, selector := range selectors {
		// Find the complete CSS rule for this selector
		rule := p.findCSSRule(css, selector)
		if rule != "" && !processedRules[rule] {
			processedRules[rule] = true
			result.WriteString(rule)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// findCSSRule finds the complete CSS rule containing a specific selector
func (p *AssetProcessor) findCSSRule(css, targetSelector string) string {
	// Pattern to match complete CSS rules
	rulePattern := regexp.MustCompile(`([^{}]+)\s*{([^{}]+)}`)
	matches := rulePattern.FindAllStringSubmatch(css, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			selectorPart := match[1]
			rulePart := match[2]

			// Check if target selector is in this rule's selectors
			selectors := strings.Split(selectorPart, ",")
			for _, sel := range selectors {
				if strings.TrimSpace(sel) == targetSelector {
					return fmt.Sprintf("%s{%s}", strings.TrimSpace(selectorPart), strings.TrimSpace(rulePart))
				}
			}
		}
	}

	return ""
}

// EstimateCriticalCSSSize estimates the size reduction from critical CSS extraction
func (p *AssetProcessor) EstimateCriticalCSSSize(fullCSS []byte) (int64, int64, float64) {
	fullSize := int64(len(fullCSS))

	criticalCSS, err := p.GetCriticalCSS(fullCSS)
	if err != nil {
		return fullSize, 0, 0
	}

	criticalSize := int64(len(criticalCSS))
	reduction := float64(fullSize-criticalSize) / float64(fullSize) * 100

	return fullSize, criticalSize, reduction
}

// GetCriticalCSSRules returns a list of critical CSS rules with their priorities
func (p *AssetProcessor) GetCriticalCSSRules() []CriticalRule {
	// Return the same rules used in GetCriticalCSS for inspection/debugging
	rules := []CriticalRule{
		{regexp.MustCompile(`(?s)html\s*{[^}]+}`), "HTML base styles", 1},
		{regexp.MustCompile(`(?s)body\s*{[^}]+}`), "Body base styles", 1},
		{regexp.MustCompile(`(?s)\*\s*{[^}]+}`), "Universal reset", 1},
		{regexp.MustCompile(`(?s)\.slide\s*{[^}]+}`), "Slide container", 2},
		{regexp.MustCompile(`(?s)h[1-6]\s*{[^}]+}`), "Heading styles", 3},
		{regexp.MustCompile(`(?s)p\s*{[^}]+}`), "Paragraph styles", 3},
		// Add more as needed
	}

	// Sort by priority
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	return rules
}

// Ensure AssetProcessor implements ports.AssetProcessor
var _ ports.AssetProcessor = (*AssetProcessor)(nil)
