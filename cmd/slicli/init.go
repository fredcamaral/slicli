package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Template types for presentation scaffolding
type templateType string

const (
	templateBasic     templateType = "basic"
	templateTechnical templateType = "technical"
	templatePitch     templateType = "pitch"
	templateAcademic  templateType = "academic"
)

// validTemplates contains all supported template types
var validTemplates = []templateType{
	templateBasic,
	templateTechnical,
	templatePitch,
	templateAcademic,
}

// Init command flags
var (
	initTemplate string
	initTheme    string
	initTitle    string
	initAuthor   string
	initForce    bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new presentation project",
	Long: `Initialize a new presentation project with scaffolded files.

Creates a presentation directory with:
  - presentation.md: Main presentation file with template content
  - slicli.toml: Local configuration file
  - assets/: Directory for images and resources

Available templates:
  - basic: Simple presentation structure (default)
  - technical: Developer-focused with code blocks and diagrams
  - pitch: Startup/business pitch deck structure
  - academic: Research paper style with citations

Example:
  slicli init                              # Initialize in current directory
  slicli init my-presentation              # Create new directory
  slicli init --template pitch --theme startup-pitch
  slicli init --title "Q4 Review" --author "John Doe"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Add command flags
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "basic",
		"Presentation template (basic, technical, pitch, academic)")
	initCmd.Flags().StringVar(&initTheme, "theme", "default",
		"Theme to use for presentation")
	initCmd.Flags().StringVar(&initTitle, "title", "Untitled Presentation",
		"Presentation title")
	initCmd.Flags().StringVar(&initAuthor, "author", "",
		"Author name")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false,
		"Overwrite existing files")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine target directory
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	// Validate template
	if !isValidTemplate(initTemplate) {
		return fmt.Errorf("invalid template '%s': must be one of %s",
			initTemplate, strings.Join(templateNames(), ", "))
	}

	// Get absolute path for display
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(targetDir)
	if err == nil && dirInfo.IsDir() {
		// Directory exists - check for existing files
		if !initForce {
			hasConflicts, conflictFiles := checkExistingFiles(targetDir)
			if hasConflicts {
				return fmt.Errorf("directory '%s' already contains presentation files: %s\n"+
					"Use --force to overwrite existing files",
					absPath, strings.Join(conflictFiles, ", "))
			}
		}
	} else if os.IsNotExist(err) {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("creating directory '%s': %w", targetDir, err)
		}
		fmt.Printf("Created directory: %s\n", absPath)
	} else if err != nil {
		return fmt.Errorf("checking directory '%s': %w", targetDir, err)
	}

	// Get author from flag or environment
	author := initAuthor
	if author == "" {
		author = os.Getenv("USER")
		if author == "" {
			author = os.Getenv("USERNAME")
		}
		if author == "" {
			author = "Author"
		}
	}

	// Generate current date
	currentDate := time.Now().Format("January 2, 2006")

	// Create scaffold context
	ctx := scaffoldContext{
		Title:    initTitle,
		Author:   author,
		Date:     currentDate,
		Theme:    initTheme,
		Template: templateType(initTemplate),
	}

	// Create presentation.md
	presentationPath := filepath.Join(targetDir, "presentation.md")
	if err := writeFileIfAllowed(presentationPath, generatePresentationContent(ctx), initForce); err != nil {
		return fmt.Errorf("creating presentation.md: %w", err)
	}
	fmt.Printf("Created: %s\n", presentationPath)

	// Create slicli.toml
	configPath := filepath.Join(targetDir, "slicli.toml")
	if err := writeFileIfAllowed(configPath, generateConfigContent(ctx), initForce); err != nil {
		return fmt.Errorf("creating slicli.toml: %w", err)
	}
	fmt.Printf("Created: %s\n", configPath)

	// Create assets directory
	assetsDir := filepath.Join(targetDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("creating assets directory: %w", err)
	}
	fmt.Printf("Created: %s/\n", assetsDir)

	// Create .gitkeep in assets to ensure directory is tracked
	gitkeepPath := filepath.Join(assetsDir, ".gitkeep")
	if err := writeFileIfAllowed(gitkeepPath, "", initForce); err != nil {
		return fmt.Errorf("creating .gitkeep: %w", err)
	}

	// Print next steps
	printNextSteps(absPath, ctx)

	return nil
}

// scaffoldContext holds the context for generating scaffold files
type scaffoldContext struct {
	Title    string
	Author   string
	Date     string
	Theme    string
	Template templateType
}

// isValidTemplate checks if the template name is valid
func isValidTemplate(name string) bool {
	for _, t := range validTemplates {
		if string(t) == name {
			return true
		}
	}
	return false
}

// templateNames returns the list of template names as strings
func templateNames() []string {
	names := make([]string, len(validTemplates))
	for i, t := range validTemplates {
		names[i] = string(t)
	}
	return names
}

// checkExistingFiles checks for files that would be overwritten
func checkExistingFiles(dir string) (bool, []string) {
	filesToCheck := []string{"presentation.md", "slicli.toml"}
	var conflicts []string

	for _, file := range filesToCheck {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			conflicts = append(conflicts, file)
		}
	}

	return len(conflicts) > 0, conflicts
}

// writeFileIfAllowed writes a file, checking for overwrites
func writeFileIfAllowed(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file already exists: %s", path)
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// generatePresentationContent generates the presentation markdown based on template
func generatePresentationContent(ctx scaffoldContext) string {
	switch ctx.Template {
	case templateTechnical:
		return generateTechnicalTemplate(ctx)
	case templatePitch:
		return generatePitchTemplate(ctx)
	case templateAcademic:
		return generateAcademicTemplate(ctx)
	default:
		return generateBasicTemplate(ctx)
	}
}

// generateBasicTemplate generates a basic presentation template
func generateBasicTemplate(ctx scaffoldContext) string {
	return fmt.Sprintf(`---
title: %s
author: %s
date: %s
theme: %s
---

# %s

**%s**

%s

---

# Agenda

- Introduction
- Main Points
- Conclusion

---

# Introduction

Your content here...

---

# Main Points

- Point 1
- Point 2
- Point 3

---

# Conclusion

Summary and key takeaways

---

# Questions?

Thank you for your attention!
`, ctx.Title, ctx.Author, ctx.Date, ctx.Theme,
		ctx.Title, ctx.Author, ctx.Date)
}

// generateTechnicalTemplate generates a technical/developer presentation template
func generateTechnicalTemplate(ctx scaffoldContext) string {
	return fmt.Sprintf(`---
title: %s
author: %s
date: %s
theme: %s
---

# %s

**%s**

%s

---

# Overview

- Problem Statement
- Architecture
- Implementation
- Demo
- Q&A

---

# Problem Statement

Describe the technical challenge...

---

# Architecture

`+"```mermaid"+`
graph TD
    A[Client] --> B[API Gateway]
    B --> C[Service A]
    B --> D[Service B]
    C --> E[Database]
    D --> E
`+"```"+`

---

# Implementation

## Key Components

- Component A: Description
- Component B: Description
- Component C: Description

---

# Code Example

`+"```go"+`
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
`+"```"+`

---

# Performance Metrics

| Metric | Before | After |
|--------|--------|-------|
| Latency | 500ms | 50ms |
| Throughput | 100 req/s | 1000 req/s |
| Memory | 2GB | 500MB |

---

# Demo

Live demonstration...

---

# Sequence Diagram

`+"```mermaid"+`
sequenceDiagram
    participant U as User
    participant A as API
    participant D as Database
    U->>A: Request
    A->>D: Query
    D-->>A: Result
    A-->>U: Response
`+"```"+`

---

# Lessons Learned

- Lesson 1
- Lesson 2
- Lesson 3

---

# Questions?

Thank you!
`, ctx.Title, ctx.Author, ctx.Date, ctx.Theme,
		ctx.Title, ctx.Author, ctx.Date)
}

// generatePitchTemplate generates a startup/pitch deck template
func generatePitchTemplate(ctx scaffoldContext) string {
	return fmt.Sprintf(`---
title: %s
author: %s
date: %s
theme: %s
---

# %s

**%s**

%s

---

# The Problem

What problem are you solving?

- Pain point 1
- Pain point 2
- Pain point 3

---

# Market Opportunity

## Total Addressable Market

- Market size: $X billion
- Growth rate: X%% annually
- Target segment: Description

---

# Our Solution

How do you solve the problem?

**Key Benefits:**
- Benefit 1
- Benefit 2
- Benefit 3

---

# Product Demo

Screenshots or live demo...

---

# Business Model

## Revenue Streams

- Stream 1: Description
- Stream 2: Description
- Stream 3: Description

---

# Traction

## Key Metrics

| Metric | Value |
|--------|-------|
| Users | X,XXX |
| Revenue | $XXX |
| Growth | XX%% MoM |

---

# Competition

## Competitive Landscape

| Feature | Us | Competitor A | Competitor B |
|---------|----|--------------|--------------|
| Feature 1 | Yes | No | Partial |
| Feature 2 | Yes | Yes | No |
| Feature 3 | Yes | No | No |

---

# Go-to-Market Strategy

1. Phase 1: Initial launch
2. Phase 2: Growth
3. Phase 3: Scale

---

# Team

## Leadership

- **CEO**: Background
- **CTO**: Background
- **COO**: Background

---

# Financial Projections

## 3-Year Outlook

| Year | Revenue | Users |
|------|---------|-------|
| Y1 | $X | X,XXX |
| Y2 | $XX | XX,XXX |
| Y3 | $XXX | XXX,XXX |

---

# The Ask

## Investment Round

- Raising: $X million
- Use of funds:
  - Product development: XX%%
  - Marketing: XX%%
  - Operations: XX%%

---

# Thank You

**Contact:**
- Email: email@example.com
- Website: www.example.com
`, ctx.Title, ctx.Author, ctx.Date, ctx.Theme,
		ctx.Title, ctx.Author, ctx.Date)
}

// generateAcademicTemplate generates an academic/research presentation template
func generateAcademicTemplate(ctx scaffoldContext) string {
	return fmt.Sprintf(`---
title: %s
author: %s
date: %s
theme: %s
---

# %s

**%s**

%s

---

# Abstract

Brief summary of your research...

---

# Outline

1. Introduction & Background
2. Research Question
3. Methodology
4. Results
5. Discussion
6. Conclusion
7. References

---

# Introduction

## Background

Context and motivation for this research...

---

# Literature Review

## Previous Work

- Author et al. (2023): Key finding
- Author et al. (2022): Key finding
- Author et al. (2021): Key finding

---

# Research Question

## Hypothesis

**H1**: Primary hypothesis statement

**H2**: Secondary hypothesis statement

---

# Methodology

## Research Design

- Study type: Description
- Sample size: N = XXX
- Data collection: Methods used
- Analysis: Statistical methods

---

# Data Collection

## Procedure

`+"```mermaid"+`
flowchart LR
    A[Recruitment] --> B[Consent]
    B --> C[Data Collection]
    C --> D[Analysis]
    D --> E[Results]
`+"```"+`

---

# Results

## Key Findings

| Variable | Mean | SD | p-value |
|----------|------|-----|---------|
| Measure A | X.XX | X.XX | < 0.05 |
| Measure B | X.XX | X.XX | < 0.01 |
| Measure C | X.XX | X.XX | n.s. |

---

# Results Visualization

## Figure 1: Main Effect

Description of findings visualized...

---

# Discussion

## Interpretation

- Finding 1: Interpretation
- Finding 2: Interpretation
- Finding 3: Interpretation

---

# Limitations

## Study Constraints

- Limitation 1
- Limitation 2
- Limitation 3

---

# Future Work

## Directions

1. Extension 1
2. Extension 2
3. Extension 3

---

# Conclusion

## Summary

Key takeaways from this research...

---

# References

1. Author, A. (2023). Title. *Journal*, Vol(Issue), pp-pp.
2. Author, B. (2022). Title. *Journal*, Vol(Issue), pp-pp.
3. Author, C. (2021). Title. *Journal*, Vol(Issue), pp-pp.

---

# Questions?

**Contact:**
%s

Thank you for your attention!
`, ctx.Title, ctx.Author, ctx.Date, ctx.Theme,
		ctx.Title, ctx.Author, ctx.Date, ctx.Author)
}

// generateConfigContent generates the slicli.toml configuration file
func generateConfigContent(ctx scaffoldContext) string {
	return fmt.Sprintf(`# slicli presentation configuration
# Generated on %s

[theme]
name = "%s"
# custom_path = ""  # Uncomment to use a custom theme path

[browser]
auto_open = true
# browser = "default"  # Specify browser: chrome, firefox, safari, etc.

[server]
host = "localhost"
port = 1000
# read_timeout = 30
# write_timeout = 30

[watcher]
interval_ms = 200
debounce_ms = 500

[plugins]
enabled = true
# directory = ""  # Custom plugins directory
# whitelist = []  # Only load specific plugins
# blacklist = []  # Exclude specific plugins

[metadata]
author = "%s"
# email = ""
# company = ""
# default_tags = []

[logging]
level = "info"
verbose = false
`, ctx.Date, ctx.Theme, ctx.Author)
}

// printNextSteps prints helpful information after initialization
func printNextSteps(absPath string, ctx scaffoldContext) {
	fmt.Println()
	fmt.Println("Presentation initialized successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println()
	fmt.Printf("  1. Edit your presentation:\n")
	fmt.Printf("     %s/presentation.md\n", absPath)
	fmt.Println()
	fmt.Printf("  2. Start the presentation server:\n")
	fmt.Printf("     cd %s && slicli serve presentation.md\n", absPath)
	fmt.Println()
	fmt.Printf("  3. Add images to the assets folder:\n")
	fmt.Printf("     %s/assets/\n", absPath)
	fmt.Println()
	fmt.Printf("  4. Customize theme in slicli.toml:\n")
	fmt.Printf("     Current theme: %s\n", ctx.Theme)
	fmt.Println()
	fmt.Println("Available themes:")
	fmt.Println("  default, dark, minimal, developer-dark, corporate-pro,")
	fmt.Println("  startup-pitch, academic-research, executive-pro, and more")
	fmt.Println()
	fmt.Println("Run 'slicli serve --help' for more options.")
}
