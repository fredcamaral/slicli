package parser

import (
	"testing"
)

func BenchmarkGoldmarkParser_Parse(b *testing.B) {
	parser := NewGoldmarkParser()
	ctx := b.Context()

	// Typical presentation content
	content := []byte(`---
title: Benchmark Presentation
author: Test Author
theme: default
---

# Introduction

Welcome to this benchmark presentation with **bold** and *italic* text.

Note: This is a speaker note

---

## Main Content

Here's a list:
- Item 1
- Item 2
- Item 3

And a code block:
` + "```go\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}\n```" + `

---

### Complex Slide

| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |
| Cell 3   | Cell 4   |

> This is a blockquote with some content

1. Ordered item 1
2. Ordered item 2

---

## Conclusion

Thank you for watching!`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(ctx, content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPresentationParserAdapter_Parse(b *testing.B) {
	markdownParser := NewGoldmarkParser()
	adapter := NewPresentationParserAdapter(markdownParser)

	// Same content as above
	content := []byte(`---
title: Benchmark Presentation
author: Test Author
theme: default
---

# Introduction

Welcome to this benchmark presentation with **bold** and *italic* text.

Note: This is a speaker note

---

## Main Content

Here's a list:
- Item 1
- Item 2
- Item 3

And a code block:
` + "```go\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}\n```" + `

---

### Complex Slide

| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |
| Cell 3   | Cell 4   |

> This is a blockquote with some content

1. Ordered item 1
2. Ordered item 2

---

## Conclusion

Thank you for watching!`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.Parse(content)
		if err != nil {
			b.Fatal(err)
		}
	}
}
