# SliCLI Themes

This directory contains the built-in themes for SliCLI presentations. All themes are professionally designed, fully responsive, and optimized for various presentation contexts.

## Available Themes (15 Total)

### Executive Themes
- **Executive Pro** (`executive-pro`) - Premium executive theme for C-suite and board presentations with sophisticated typography
- **Corporate Pro** (`corporate-pro`) - Professional corporate theme with business-focused layouts

### Developer & Technical Themes  
- **Developer Dark** (`developer-dark`) - Dark theme optimized for developers with syntax highlighting and terminal aesthetics
- **TechConf Pro** (`techconf-pro`) - Modern technical conference theme perfect for developer presentations

### Academic & Educational Themes
- **Academic Research** (`academic-research`) - Clean academic theme for scholarly presentations and research papers
- **Education Plus** (`education-plus`) - Friendly educational theme perfect for teaching materials and courses
- **Scientific Pro** (`scientific-pro`) - Technical theme designed for research presentations and scientific content

### Business & Industry Themes
- **Startup Pitch** (`startup-pitch`) - Modern bold theme designed for investor presentations and pitches
- **Finance Pro** (`finance-pro`) - Data-focused theme optimized for financial presentations and charts
- **Healthcare Pro** (`healthcare-pro`) - Professional accessible theme for medical presentations

### Creative & Design Themes
- **Creative Studio** (`creative-studio`) - Colorful creative theme for design presentations and portfolios
- **Modern Minimal** (`modern-minimal`) - Minimalist elegant theme for clean presentations

### Utility Themes
- **Default** (`default`) - Versatile general-purpose theme with clean design
- **Minimal** (`minimal`) - Distraction-free theme focused purely on content
- **Dark** (`dark`) - Modern dark theme optimized for low-light environments

## Theme Structure

```
theme-name/
├── theme.toml          # Theme configuration and metadata
├── assets/
│   ├── css/           # Stylesheets
│   │   ├── main.css   # Main theme styles
│   │   ├── variables.css    # CSS custom properties
│   │   ├── reset.css        # Browser reset
│   │   ├── typography.css   # Text styles
│   │   ├── layout.css       # Layout system
│   │   ├── components.css   # UI components
│   │   ├── responsive.css   # Media queries
│   │   └── print.css        # Print styles
│   ├── js/            # Optional JavaScript
│   └── fonts/         # Optional custom fonts
└── templates/         # Optional template overrides
```

## Using Themes

### In Configuration
```yaml
# slicli.yaml
theme: dark
```

### Command Line
```bash
# Use any theme by name
slicli serve presentation.md --theme executive-pro
slicli serve presentation.md --theme developer-dark
slicli serve presentation.md --theme minimal

# List all available themes
slicli themes list
```

### Per Presentation
```markdown
---
title: "My Presentation"
theme: executive-pro
author: "Your Name"
---

# Your presentation content here
```

## Creating Custom Themes

### Quick Start
1. Copy an existing theme:
   ```bash
   cp -r themes/default themes/my-theme
   ```

2. Update `theme.toml`:
   ```toml
   name = "my-theme"
   display_name = "My Theme"
   ```

3. Modify CSS variables and styles

### From Scratch
See the [theme documentation](../docs/themes.md) for detailed instructions.

## Theme Development

### CSS Variables
Themes use CSS custom properties for customization:

```css
:root {
    --primary-color: #2563eb;
    --background-color: #ffffff;
    --font-family-body: 'Inter', sans-serif;
    /* ... more variables ... */
}
```

### Required Styles
- `.slide` - Slide container
- `.slide.active` - Active slide
- `.navigation` - Navigation controls
- `.progress-bar` - Progress indicator
- Typography (h1-h6, p, lists, etc.)

### Best Practices
- Maintain WCAG 2.1 AA contrast ratios
- Include responsive breakpoints
- Provide print styles
- Support reduced motion preferences
- Test across browsers

## Contributing Themes

To contribute a new theme:

1. Create theme following the structure above
2. Test with various content types
3. Include documentation
4. Submit pull request

## License

All themes are MIT licensed unless otherwise specified in their theme.toml file.