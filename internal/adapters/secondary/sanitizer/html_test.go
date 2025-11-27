package sanitizer

import (
	"strings"
	"testing"
)

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSafe bool   // true if output should not contain dangerous content
		contains string // optional: check that output contains this
	}{
		{
			name:     "removes script tags",
			input:    `<div>Hello<script>alert('xss')</script>World</div>`,
			wantSafe: true,
		},
		{
			name:     "removes onclick handlers",
			input:    `<div onclick="alert('xss')">Click me</div>`,
			wantSafe: true,
		},
		{
			name:     "removes onerror handlers",
			input:    `<img src="x" onerror="alert('xss')">`,
			wantSafe: true,
		},
		{
			name:     "removes javascript URLs",
			input:    `<a href="javascript:alert('xss')">Click</a>`,
			wantSafe: true,
		},
		{
			name:     "allows safe div",
			input:    `<div class="slide">Content</div>`,
			wantSafe: true,
			contains: `<div class="slide">Content</div>`,
		},
		{
			name:     "allows headings",
			input:    `<h1>Title</h1><h2>Subtitle</h2>`,
			wantSafe: true,
			contains: `<h1>Title</h1>`,
		},
		{
			name:     "allows code blocks",
			input:    `<pre><code class="language-go">fmt.Println("hello")</code></pre>`,
			wantSafe: true,
			contains: `<code class="language-go">`,
		},
		{
			name:     "allows tables",
			input:    `<table><tr><th>Header</th></tr><tr><td>Cell</td></tr></table>`,
			wantSafe: true,
			contains: `<table>`,
		},
		{
			name:     "allows data attributes",
			input:    `<div data-slide="1" data-theme="dark">Content</div>`,
			wantSafe: true,
			contains: `data-slide="1"`,
		},
		{
			name:     "removes iframe",
			input:    `<iframe src="https://evil.com"></iframe>`,
			wantSafe: true,
		},
		{
			name:     "removes object tags",
			input:    `<object data="evil.swf"></object>`,
			wantSafe: true,
		},
		{
			name:     "removes embed tags",
			input:    `<embed src="evil.swf">`,
			wantSafe: true,
		},
		{
			name:     "removes base tags",
			input:    `<base href="https://evil.com">`,
			wantSafe: true,
		},
		{
			name:     "removes form tags",
			input:    `<form action="https://evil.com"><input type="submit"></form>`,
			wantSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHTML(tt.input)

			if tt.wantSafe {
				// Check for dangerous content
				dangerousPatterns := []string{
					"<script",
					"onclick",
					"onerror",
					"onload",
					"onmouseover",
					"javascript:",
					"<iframe",
					"<object",
					"<embed",
					"<base",
					"<form",
				}
				for _, pattern := range dangerousPatterns {
					if strings.Contains(strings.ToLower(result), pattern) {
						t.Errorf("SanitizeHTML() result contains dangerous pattern %q: %s", pattern, result)
					}
				}
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("SanitizeHTML() result does not contain expected %q, got: %s", tt.contains, result)
			}
		})
	}
}

func TestSanitizePluginOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSafe bool
		contains string
	}{
		{
			name:     "removes script tags",
			input:    `<div>Hello<script>alert('xss')</script>World</div>`,
			wantSafe: true,
		},
		{
			name:     "allows safe div",
			input:    `<div class="mermaid">graph TD; A-->B;</div>`,
			wantSafe: true,
			contains: `<div class="mermaid">`,
		},
		{
			name:     "allows SVG for diagrams",
			input:    `<svg viewBox="0 0 100 100"><rect x="10" y="10" width="80" height="80"/></svg>`,
			wantSafe: true,
			contains: `<svg`,
		},
		{
			name:     "removes style attribute",
			input:    `<div style="background:url(javascript:alert('xss'))">Test</div>`,
			wantSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePluginOutput(tt.input)

			if tt.wantSafe {
				dangerousPatterns := []string{
					"<script",
					"onclick",
					"onerror",
					"javascript:",
				}
				for _, pattern := range dangerousPatterns {
					if strings.Contains(strings.ToLower(result), pattern) {
						t.Errorf("SanitizePluginOutput() result contains dangerous pattern %q: %s", pattern, result)
					}
				}
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("SanitizePluginOutput() result does not contain expected %q, got: %s", tt.contains, result)
			}
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips all HTML",
			input:    `<div><strong>Hello</strong> <em>World</em></div>`,
			expected: "Hello World",
		},
		{
			name:     "handles nested tags",
			input:    `<div><p><span>Nested</span></p></div>`,
			expected: "Nested",
		},
		{
			name:     "removes scripts completely",
			input:    `Hello<script>alert('xss')</script>World`,
			expected: "HelloWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripHTML(tt.input)
			if result != tt.expected {
				t.Errorf("StripHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizeAttribute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escapes less than",
			input:    `<script>`,
			expected: `&lt;script&gt;`,
		},
		{
			name:     "escapes quotes",
			input:    `"onload="alert('xss')`,
			expected: `&quot;onload=&quot;alert(&#39;xss&#39;)`,
		},
		{
			name:     "escapes ampersand",
			input:    `foo&bar`,
			expected: `foo&amp;bar`,
		},
		{
			name:     "handles safe values",
			input:    `my-class-name`,
			expected: `my-class-name`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAttribute(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeAttribute() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "allows http",
			input:    `https://example.com/page`,
			expected: `https://example.com/page`,
		},
		{
			name:     "allows mailto",
			input:    `mailto:user@example.com`,
			expected: `mailto:user@example.com`,
		},
		{
			name:     "allows relative URLs",
			input:    `/path/to/page`,
			expected: `/path/to/page`,
		},
		{
			name:     "blocks javascript URLs",
			input:    `javascript:alert('xss')`,
			expected: ``,
		},
		{
			name:     "blocks data URLs",
			input:    `data:text/html,<script>alert('xss')</script>`,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// BenchmarkSanitizeHTML benchmarks the HTML sanitization performance
func BenchmarkSanitizeHTML(b *testing.B) {
	html := `<div class="slide">
		<h1>Title</h1>
		<p>Some <strong>bold</strong> and <em>italic</em> text.</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
		</ul>
		<pre><code class="language-go">fmt.Println("hello")</code></pre>
	</div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeHTML(html)
	}
}
