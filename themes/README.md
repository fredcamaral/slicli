# SliCLI Themes

This directory contains the built-in themes for SliCLI presentations.

## Available Themes

### Default
- **Path:** `themes/default/`
- **Description:** Professional theme with clean typography and modern design
- **Best for:** Business presentations, technical talks, educational content

### Minimal
- **Path:** `themes/minimal/`
- **Description:** Distraction-free theme focused on content
- **Best for:** Academic presentations, text-heavy content, minimalist aesthetic

### Dark
- **Path:** `themes/dark/`
- **Description:** Modern dark theme optimized for low-light environments
- **Best for:** Developer talks, evening presentations, code demonstrations

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
slicli serve presentation.md --theme minimal
```

### Per Presentation
```markdown
---
theme: dark
---
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