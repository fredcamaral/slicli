package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuessLanguage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "Go code",
			content: `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`,
			expected: "go",
		},
		{
			name: "Python code",
			content: `def hello(name):
    print(f"Hello, {name}")
    
if __name__ == "__main__":
    hello("World")`,
			expected: "python",
		},
		{
			name: "JavaScript code",
			content: `function greet(name) {
    console.log("Hello, " + name);
}

const message = "World";
greet(message);`,
			expected: "javascript",
		},
		{
			name: "TypeScript code",
			content: `interface Person {
    name: string;
    age: number;
}

function greet(person: Person): void {
    console.log(person.name);
}`,
			expected: "typescript",
		},
		{
			name: "Shell script",
			content: `#!/bin/bash
echo "Starting script"
export PATH=$PATH:/usr/local/bin
if [ -f config.txt ]; then
    source config.txt
fi`,
			expected: "shell",
		},
		{
			name: "Unknown content",
			content: `This is just some random text
without any specific programming language patterns`,
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guessLanguage(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSupportedLanguages(t *testing.T) {
	langs := supportedLanguages()

	// Should have a reasonable number of languages
	assert.Greater(t, len(langs), 20)

	// Check for some common languages
	expectedLangs := []string{"go", "python", "javascript", "java", "rust", "ruby"}
	for _, expected := range expectedLangs {
		assert.Contains(t, langs, expected)
	}
}

func TestSupportedStyles(t *testing.T) {
	styles := supportedStyles()

	// Should have multiple styles
	assert.Greater(t, len(styles), 10)

	// Check for some common styles
	expectedStyles := []string{"github", "monokai", "dracula", "solarized-dark"}
	for _, expected := range expectedStyles {
		assert.Contains(t, styles, expected)
	}
}
