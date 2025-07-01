package main

import "strings"

// Language aliases and common extensions
var languageAliases = map[string]string{
	// Common aliases
	"js":         "javascript",
	"ts":         "typescript",
	"py":         "python",
	"rb":         "ruby",
	"yml":        "yaml",
	"sh":         "bash",
	"shell":      "bash",
	"dockerfile": "docker",
	"makefile":   "make",
	"c++":        "cpp",
	"c#":         "csharp",
	"f#":         "fsharp",
	"objc":       "objective-c",
	"md":         "markdown",

	// Web technologies
	"htm":  "html",
	"vue":  "vue",
	"jsx":  "javascript",
	"tsx":  "typescript",
	"scss": "sass",

	// Config files
	"conf":   "ini",
	"cfg":    "ini",
	"env":    "bash",
	"dotenv": "bash",
	"jsonc":  "json",
	"json5":  "json",

	// Database
	"postgres": "postgresql",
	"mssql":    "sql",
	"mysql":    "sql",
	"sqlite":   "sql",
}

// Common file patterns for language detection
var languagePatterns = map[string][]string{
	"go": {
		"package main",
		"func main()",
		"import (",
		"type .* struct",
		"interface{}",
	},
	"python": {
		"def ",
		"import ",
		"from .* import",
		"class .*:",
		"if __name__",
		"print(",
	},
	"javascript": {
		"function ",
		"const ",
		"let ",
		"var ",
		"console.log",
		"require(",
		"module.exports",
		"export ",
		"import ",
	},
	"typescript": {
		"interface ",
		"type ",
		": string",
		": number",
		": boolean",
		"enum ",
		"namespace ",
		"declare ",
	},
	"java": {
		"public class",
		"private ",
		"protected ",
		"static void main",
		"System.out.println",
		"import java.",
		"package ",
	},
	"rust": {
		"fn main()",
		"let mut ",
		"impl ",
		"trait ",
		"struct ",
		"enum ",
		"use std::",
		"println!",
	},
	"ruby": {
		"def ",
		"end",
		"class ",
		"module ",
		"require ",
		"puts ",
		"attr_",
	},
	"shell": {
		"#!/bin/bash",
		"#!/bin/sh",
		"echo ",
		"export ",
		"if [ ",
		"for .* in",
		"while ",
		"case ",
	},
}

// resolveLanguage resolves language aliases to their canonical names
func resolveLanguage(lang string) string {
	if canonical, ok := languageAliases[strings.ToLower(lang)]; ok {
		return canonical
	}
	return lang
}

// guessLanguage attempts to guess the language from content patterns
func guessLanguage(content string) string {
	content = strings.ToLower(content)

	// Count pattern matches for each language
	scores := make(map[string]int)

	for lang, patterns := range languagePatterns {
		for _, pattern := range patterns {
			if strings.Contains(content, strings.ToLower(pattern)) {
				scores[lang]++
			}
		}
	}

	// Find language with highest score
	var bestLang string
	maxScore := 0

	for lang, score := range scores {
		if score > maxScore {
			maxScore = score
			bestLang = lang
		}
	}

	if bestLang != "" {
		return bestLang
	}

	return "text"
}

// supportedLanguages returns a list of all supported languages
func supportedLanguages() []string {
	// This is a subset of commonly used languages
	// Chroma supports many more
	return []string{
		"bash", "c", "cpp", "csharp", "css", "diff", "docker",
		"go", "html", "java", "javascript", "json", "kotlin",
		"lua", "makefile", "markdown", "objective-c", "perl",
		"php", "python", "r", "ruby", "rust", "scala", "shell",
		"sql", "swift", "toml", "typescript", "xml", "yaml",
	}
}

// supportedStyles returns a list of available syntax highlighting styles
func supportedStyles() []string {
	return []string{
		"github", "monokai", "dracula", "solarized-dark", "solarized-light",
		"vs", "xcode", "autumn", "borland", "bw", "colorful", "emacs",
		"friendly", "fruity", "manni", "murphy", "native", "paraiso-dark",
		"paraiso-light", "pastie", "perldoc", "pygments", "rainbow_dash",
		"rrt", "tango", "trac", "vim", "zenburn",
	}
}
