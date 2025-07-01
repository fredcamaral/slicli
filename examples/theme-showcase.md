---
title: SliCLI Theme Showcase
theme: default
---

# SliCLI Theme Showcase

A demonstration of the built-in themes and theming capabilities

---

## Typography Demo

### Heading Hierarchy

# Heading 1
## Heading 2  
### Heading 3
#### Heading 4
##### Heading 5
###### Heading 6

Regular paragraph text demonstrates the base font size and line height. The theme system provides consistent typography across all elements.

---

## Text Elements

**Bold text** and *italic text* can be combined for ***emphasis***.

> "Design is not just what it looks like and feels like. Design is how it works."
> — Steve Jobs

Links are styled consistently: [Visit SliCLI](https://github.com/slicli/slicli)

---

## Code Examples

Inline code: `const theme = 'default'`

```javascript
// Code block with syntax highlighting
function createPresentation(markdown, options = {}) {
    const { theme = 'default', plugins = [] } = options;
    
    return {
        slides: parseMarkdown(markdown),
        theme: loadTheme(theme),
        plugins: loadPlugins(plugins)
    };
}
```

---

## Lists and Structure

### Unordered Lists
- First level item
  - Second level item
  - Another second level
    - Third level item
- Back to first level

### Ordered Lists
1. Step one
2. Step two
   1. Sub-step 2.1
   2. Sub-step 2.2
3. Step three

---

## Tables

| Feature | Default | Minimal | Dark |
|---------|---------|---------|------|
| Typography | Modern sans-serif | Classic serif | Clean sans-serif |
| Colors | Blue accent | Black & white | Vibrant blue |
| Animations | Smooth | Minimal | Enhanced |
| Best for | Business | Academic | Technical |

---

## Components Demo

### Buttons

<button class="button">Primary Button</button>
<button class="button secondary">Secondary</button>
<button class="button outline">Outline</button>

### Cards

<div class="card">
<div class="card-header">Feature Card</div>
<div class="card-body">
Cards provide a clean way to group related content with visual hierarchy.
</div>
</div>

---

## Two Column Layout

<div class="columns">
<div>

### Left Column
- Responsive grid system
- Automatic stacking on mobile
- Flexible column widths
- Gap control

</div>
<div>

### Right Column
Perfect for:
- Comparisons
- Before/after
- Image + text
- Code + explanation

</div>
</div>

---

## Fragment Animations

<div class="fragment">First, this appears...</div>
<div class="fragment">Then this...</div>
<div class="fragment fade-up">And finally, this fades up!</div>

---

# Section Divider
## This slide uses the `section` class

---

## Images and Media

![Placeholder Image](https://via.placeholder.com/600x400/2563eb/ffffff?text=Theme+Demo)

Images are automatically styled with appropriate spacing and shadows.

---

## Responsive Design

### Mobile Optimized
- Touch-friendly navigation
- Readable typography
- Stacking layouts
- Optimized spacing

### Print Ready
- Clean print styles
- Page breaks
- Hidden navigation
- Black & white friendly

---

## Theme Variables

```css
:root {
    --primary-color: #2563eb;
    --background-color: #ffffff;
    --text-color: #1e293b;
    --font-family-body: 'Inter', sans-serif;
    --slide-padding: 4rem;
    --transition-speed: 300ms;
}
```

---

## Creating Custom Themes

1. **Copy a base theme**
   ```bash
   cp -r themes/default themes/my-theme
   ```

2. **Modify theme.toml**
   ```toml
   name = "my-theme"
   display_name = "My Custom Theme"
   ```

3. **Override CSS variables**
   ```css
   :root {
       --primary-color: #dc2626;
   }
   ```

---

## Try Different Themes

```bash
# Default theme (current)
slicli serve theme-showcase.md

# Minimal theme
slicli serve theme-showcase.md --theme minimal

# Dark theme
slicli serve theme-showcase.md --theme dark

# Custom theme
slicli serve theme-showcase.md --theme custom
```

---

# Thank You!

Explore more at [github.com/slicli/slicli](https://github.com/slicli/slicli)