# Business Process Workflows

End-to-end business workflows for presentation development, plugin integration, and content delivery in slicli.

## Presentation Development Workflow

### Complete Development Lifecycle

```mermaid
sequenceDiagram
    participant Developer as Developer
    participant FileSystem as File System
    participant slicli as slicli CLI
    participant FileWatcher as File Watcher
    participant Browser as Browser
    participant WebSocket as WebSocket
    
    Developer->>FileSystem: Create presentation.md
    Developer->>slicli: slicli serve --theme corporate
    
    slicli->>slicli: Initialize theme (corporate)
    slicli->>slicli: Load plugins (code-exec, syntax-highlight, mermaid)
    slicli->>slicli: Start HTTP server (localhost:8080)
    slicli->>FileWatcher: Watch presentation files
    
    FileWatcher->>FileSystem: Monitor file changes (1s polling)
    
    slicli->>Browser: Open http://localhost:8080
    Browser->>slicli: GET / (initial load)
    slicli-->>Browser: Rendered presentation
    
    loop Development Iteration
        Developer->>FileSystem: Edit presentation.md
        FileWatcher->>FileSystem: Detect file change (SHA256)
        FileWatcher->>WebSocket: Broadcast change event
        WebSocket->>Browser: Live reload notification
        Browser->>slicli: Reload presentation
        slicli-->>Browser: Updated presentation
        
        Developer->>Developer: Review changes in browser
        
        alt Satisfied with Changes
            Developer->>FileSystem: Save final version
        else Need Further Changes
            Developer->>FileSystem: Continue editing
        end
    end
    
    Developer->>slicli: Export to multiple formats
    slicli-->>Developer: PDF, HTML, Images ready
    
    Note over Developer,WebSocket: Live development workflow with instant feedback
```

### Theme Selection & Customization

```mermaid
sequenceDiagram
    participant Developer as Developer
    participant slicli as slicli CLI
    participant ThemeLoader as Theme Loader
    participant ThemeCache as Theme Cache
    participant FileSystem as File System
    
    Developer->>slicli: slicli list-themes
    slicli->>ThemeLoader: DiscoverThemes()
    ThemeLoader->>FileSystem: Scan themes/ directory
    FileSystem-->>ThemeLoader: 13 built-in themes found
    ThemeLoader-->>slicli: Theme list with categories
    slicli-->>Developer: Available themes displayed
    
    Note over Developer: Themes: Corporate, Educational, Technical, Creative, etc.
    
    Developer->>slicli: slicli serve --theme executive-pro
    slicli->>ThemeCache: Get(executive-pro)
    
    alt Theme Not Cached
        ThemeCache->>ThemeLoader: LoadTheme(executive-pro)
        ThemeLoader->>FileSystem: Read theme.toml + CSS
        ThemeLoader->>ThemeLoader: Process theme configuration
        ThemeLoader->>ThemeLoader: Load theme assets
        ThemeLoader-->>ThemeCache: Theme engine ready
        ThemeCache->>ThemeCache: Cache theme (95% hit rate)
    end
    
    ThemeCache-->>slicli: Executive Pro theme loaded
    slicli-->>Developer: Presentation with corporate theme
    
    alt Developer Wants Customization
        Developer->>FileSystem: Create custom theme directory
        Developer->>FileSystem: Copy base theme files
        Developer->>FileSystem: Modify theme.toml and CSS
        Developer->>slicli: slicli serve --theme custom-corporate
        slicli->>ThemeLoader: Load custom theme
        ThemeLoader-->>slicli: Custom theme applied
    end
    
    Note over Developer,FileSystem: Theme system: 13 professional themes + custom theme support
```

## Plugin Development & Integration

### Plugin Development Workflow

```mermaid
sequenceDiagram
    participant PluginDev as Plugin Developer
    participant FileSystem as File System
    participant GoCompiler as Go Compiler
    participant slicli as slicli CLI
    participant PluginLoader as Plugin Loader
    participant Sandbox as Plugin Sandbox
    
    PluginDev->>FileSystem: Create plugin directory
    PluginDev->>FileSystem: Write main.go (plugin implementation)
    
    Note over PluginDev: Plugin must implement PluginInterface
    
    PluginDev->>FileSystem: Create Makefile
    PluginDev->>GoCompiler: make build (buildmode=plugin)
    GoCompiler->>FileSystem: Compile to .so shared object
    FileSystem-->>GoCompiler: plugin.so created
    
    PluginDev->>slicli: Test plugin with presentation
    slicli->>PluginLoader: LoadPlugin(plugin.so)
    PluginLoader->>PluginLoader: Validate plugin interface
    PluginLoader->>PluginLoader: Load shared object
    PluginLoader-->>slicli: Plugin loaded
    
    slicli->>Sandbox: ExecutePlugin(testInput)
    Sandbox->>Sandbox: Set memory limit (100MB)
    Sandbox->>Sandbox: Set timeout (30s)
    Sandbox->>Sandbox: Execute plugin code
    Sandbox-->>slicli: Plugin output
    
    alt Plugin Test Success
        slicli-->>PluginDev: Plugin working correctly
        PluginDev->>FileSystem: Package plugin for distribution
    else Plugin Test Failure
        slicli-->>PluginDev: Error details (sandbox violations, etc.)
        PluginDev->>FileSystem: Fix plugin code
        PluginDev->>GoCompiler: Rebuild plugin
    end
    
    Note over PluginDev,Sandbox: Plugin development: Sandboxed testing with resource limits
```

### Plugin Marketplace Integration

```mermaid
sequenceDiagram
    participant User as User
    participant slicli as slicli CLI
    participant MarketplaceClient as Marketplace Client
    participant GitHubAPI as GitHub API
    participant PluginInstaller as Plugin Installer
    participant FileSystem as File System
    
    User->>slicli: slicli marketplace search "charts"
    slicli->>MarketplaceClient: SearchPlugins("charts")
    MarketplaceClient->>GitHubAPI: Query plugin repositories
    GitHubAPI-->>MarketplaceClient: Plugin metadata
    MarketplaceClient-->>slicli: Search results
    slicli-->>User: Available chart plugins
    
    User->>slicli: slicli marketplace install chart-generator
    slicli->>MarketplaceClient: GetPluginInfo(chart-generator)
    MarketplaceClient->>GitHubAPI: Get plugin details
    GitHubAPI-->>MarketplaceClient: Plugin info + download URL
    
    slicli->>PluginInstaller: InstallPlugin(pluginInfo)
    PluginInstaller->>PluginInstaller: Validate plugin signature
    PluginInstaller->>GitHubAPI: Download plugin binary
    GitHubAPI-->>PluginInstaller: Plugin .so file
    
    PluginInstaller->>FileSystem: Install to plugins/ directory
    PluginInstaller->>FileSystem: Create plugin configuration
    PluginInstaller-->>slicli: Plugin installed
    
    slicli->>slicli: Register new plugin
    slicli-->>User: chart-generator plugin ready
    
    Note over User,FileSystem: Open source marketplace: Community plugins with MIT licensing
```

## Export & Delivery Workflows

### Multi-Format Export Process

```mermaid
sequenceDiagram
    participant User as User
    participant slicli as slicli CLI
    participant ExportSvc as Export Service
    participant BrowserAuto as Browser Automation
    participant Chrome as Chrome/Chromium
    participant FileSystem as File System
    
    User->>slicli: slicli export --formats pdf,html,images
    slicli->>ExportSvc: Export(presentation, multipleFormats)
    
    ExportSvc->>ExportSvc: Create export batch ID
    ExportSvc->>ExportSvc: Initialize retry config
    
    par PDF Export
        ExportSvc->>BrowserAuto: LaunchBrowser(pdf-export)
        BrowserAuto->>Chrome: Start headless Chrome
        ExportSvc->>BrowserAuto: LoadPresentation()
        BrowserAuto->>Chrome: Navigate + render
        ExportSvc->>BrowserAuto: PrintToPDF(A4, high-quality)
        BrowserAuto->>Chrome: Generate PDF
        Chrome-->>BrowserAuto: PDF data
        BrowserAuto->>FileSystem: Save presentation.pdf
    and HTML Export
        ExportSvc->>BrowserAuto: LaunchBrowser(html-export)
        BrowserAuto->>Chrome: Start headless Chrome
        ExportSvc->>BrowserAuto: LoadPresentation()
        ExportSvc->>BrowserAuto: ExtractHTML()
        BrowserAuto->>Chrome: Get full HTML + assets
        Chrome-->>BrowserAuto: Complete HTML bundle
        BrowserAuto->>FileSystem: Save presentation.html + assets/
    and Images Export
        ExportSvc->>BrowserAuto: LaunchBrowser(image-export)
        BrowserAuto->>Chrome: Start headless Chrome
        loop For each slide (N slides)
            ExportSvc->>BrowserAuto: NavigateToSlide(index)
            ExportSvc->>BrowserAuto: Screenshot(1920x1080, PNG)
            BrowserAuto->>Chrome: Capture slide
            Chrome-->>BrowserAuto: Image data
            BrowserAuto->>FileSystem: Save slide-{index}.png
        end
    end
    
    ExportSvc->>ExportSvc: Generate export report
    ExportSvc->>ExportSvc: Schedule cleanup (24h)
    ExportSvc-->>User: Export complete (PDF: 2.3MB, HTML: 1.8MB, Images: 15MB)
    
    Note over User,FileSystem: Parallel export: PDF, HTML, Images generated simultaneously
```

### Presentation Distribution Workflow

```mermaid
sequenceDiagram
    participant Presenter as Presenter
    participant slicli as slicli CLI
    participant WebServer as Built-in Web Server
    participant Audience as Audience Browsers
    participant PresenterMode as Presenter Mode
    participant WebSocket as WebSocket Hub
    
    Presenter->>slicli: slicli serve --presenter-mode
    slicli->>WebServer: Start server (localhost:8080)
    slicli->>PresenterMode: Initialize presenter interface
    slicli->>WebSocket: Start WebSocket hub
    
    Presenter->>PresenterMode: Open http://localhost:8080/presenter
    PresenterMode-->>Presenter: Presenter controls loaded
    
    par Audience Connection
        Audience->>WebServer: Open http://localhost:8080
        WebServer-->>Audience: Presentation interface
        Audience->>WebSocket: Connect for live updates
        WebSocket-->>Audience: Connected to presenter
    and Presenter Control
        Presenter->>PresenterMode: View speaker notes
        PresenterMode-->>Presenter: Current slide notes
        Presenter->>PresenterMode: Navigate to next slide
        PresenterMode->>WebSocket: Broadcast navigation
        WebSocket->>Audience: Update to slide N+1
        Audience->>Audience: Slide changes automatically
    end
    
    loop During Presentation
        Presenter->>PresenterMode: Control navigation (next/prev/goto)
        PresenterMode->>WebSocket: Broadcast slide changes
        WebSocket->>Audience: Synchronized slide updates
        
        Presenter->>PresenterMode: Start/stop timer
        PresenterMode->>WebSocket: Broadcast timer state
        WebSocket->>Audience: Timer display updates
        
        alt Audience Questions
            Audience->>WebSocket: Send question (if enabled)
            WebSocket->>PresenterMode: Question notification
            PresenterMode-->>Presenter: New question alert
        end
    end
    
    Note over Presenter,Audience: Live presentation: Real-time synchronization between presenter and audience
```

## Content Creation Workflows

### Educational Content Creation

```mermaid
sequenceDiagram
    participant Educator as Educator
    participant slicli as slicli CLI
    participant PluginSvc as Plugin Service
    participant CodeExecPlugin as Code Execution
    participant MermaidPlugin as Mermaid Diagrams
    participant SyntaxPlugin as Syntax Highlighting
    
    Educator->>slicli: Create technical presentation
    Educator->>slicli: slicli serve --theme academic-research
    
    Note over Educator: Writing educational content with interactive elements
    
    Educator->>slicli: Add live code examples
    slicli->>PluginSvc: Process ```python code blocks
    PluginSvc->>CodeExecPlugin: Execute Python code
    CodeExecPlugin->>CodeExecPlugin: Sandboxed execution
    CodeExecPlugin-->>PluginSvc: Code output + execution time
    PluginSvc-->>slicli: Formatted code result
    
    Educator->>slicli: Add system diagrams
    slicli->>PluginSvc: Process %%%mermaid blocks
    PluginSvc->>MermaidPlugin: Render diagrams
    MermaidPlugin->>MermaidPlugin: Generate SVG via CDN
    MermaidPlugin-->>PluginSvc: Interactive diagram HTML
    PluginSvc-->>slicli: Embedded diagram
    
    Educator->>slicli: Add syntax-highlighted examples
    slicli->>PluginSvc: Process ```go code blocks
    PluginSvc->>SyntaxPlugin: Highlight Go code
    SyntaxPlugin->>SyntaxPlugin: Apply Chroma highlighting (200+ languages)
    SyntaxPlugin-->>PluginSvc: Highlighted HTML
    PluginSvc-->>slicli: Styled code display
    
    Educator->>slicli: Preview interactive presentation
    slicli-->>Educator: Presentation with executable code, diagrams, and highlighting
    
    Educator->>slicli: Export for distribution
    slicli-->>Educator: PDF (static), HTML (interactive), Images (slides)
    
    Note over Educator,SyntaxPlugin: Educational workflow: Interactive content with live code execution
```

### Corporate Presentation Workflow

```mermaid
sequenceDiagram
    participant Executive as Executive
    participant Designer as Designer
    participant slicli as slicli CLI
    participant ThemeSvc as Theme Service
    participant ExportSvc as Export Service
    participant DistributionSvc as Distribution Service
    
    Executive->>Designer: Request executive presentation
    Designer->>slicli: slicli serve --theme executive-pro
    slicli->>ThemeSvc: Load corporate theme
    ThemeSvc-->>slicli: C-suite appropriate styling
    
    Designer->>slicli: Create markdown presentation
    Designer->>slicli: Add corporate branding elements
    Designer->>slicli: Include data visualizations
    Designer->>slicli: Preview presentation
    slicli-->>Designer: Corporate-styled presentation
    
    Designer->>Executive: Share preview link
    Executive->>slicli: Review presentation
    Executive->>Designer: Request modifications
    
    loop Revision Cycle
        Designer->>slicli: Make requested changes
        slicli->>slicli: Live reload with changes
        Executive->>slicli: Review updated version
        
        alt Executive Approved
            break Revision complete
        else More Changes Needed
            Executive->>Designer: Additional feedback
        end
    end
    
    Executive->>slicli: Approve for distribution
    Designer->>ExportSvc: Export presentation (multiple formats)
    
    par
        ExportSvc->>ExportSvc: Generate PDF (for printing)
    and
        ExportSvc->>ExportSvc: Generate HTML (for web)
    and
        ExportSvc->>ExportSvc: Generate Images (for social media)
    end
    
    ExportSvc-->>DistributionSvc: Export package ready
    DistributionSvc->>DistributionSvc: Prepare distribution channels
    DistributionSvc-->>Executive: Presentation ready for delivery
    
    Note over Executive,DistributionSvc: Corporate workflow: Professional themes + multi-format distribution
```

## Performance Optimization Workflows

### Cache Warming & Optimization

```mermaid
sequenceDiagram
    participant Admin as System Admin
    participant slicli as slicli CLI
    participant CacheManager as Cache Manager
    participant PluginCache as Plugin Cache
    participant ThemeCache as Theme Cache
    participant PerformanceMonitor as Performance Monitor
    
    Admin->>slicli: slicli performance analyze
    slicli->>PerformanceMonitor: AnalyzeSystemPerformance()
    
    PerformanceMonitor->>PluginCache: GetCacheStats()
    PluginCache-->>PerformanceMonitor: {hitRate: 0.85, size: 90MB, evictions: 150}
    
    PerformanceMonitor->>ThemeCache: GetCacheStats()
    ThemeCache-->>PerformanceMonitor: {hitRate: 0.95, count: 12, evictions: 5}
    
    PerformanceMonitor-->>slicli: Performance report
    slicli-->>Admin: Optimization recommendations
    
    Note over Admin: Recommendations: Cache warming, eviction optimization
    
    Admin->>slicli: slicli performance optimize --warm-cache
    slicli->>CacheManager: WarmCaches()
    
    par
        CacheManager->>ThemeCache: Preload common themes
        ThemeCache->>ThemeCache: Load default, corporate, technical themes
    and
        CacheManager->>PluginCache: Preprocess common patterns
        PluginCache->>PluginCache: Cache frequent plugin combinations
    end
    
    CacheManager-->>slicli: Cache warming complete
    
    Admin->>slicli: slicli performance optimize --eviction-strategy
    slicli->>CacheManager: OptimizeEviction()
    
    Note over CacheManager: Future: Replace O(n) with heap-based eviction
    CacheManager->>PluginCache: Implement efficient eviction
    CacheManager-->>slicli: Eviction optimized
    
    slicli-->>Admin: Performance optimization complete
    
    Note over Admin,PerformanceMonitor: Performance tuning: Cache optimization for better response times
```

## Key Business Value Metrics

**Development Velocity**:
- Live reload: ~1 second from file save to browser update
- Plugin processing: 85% cache hit rate reduces execution time
- Theme switching: 95% cache hit rate enables instant theme changes

**Content Quality**:
- Interactive code execution with sandboxed safety
- Professional themes for different business contexts
- Multi-format export for diverse distribution needs

**Collaboration Efficiency**:
- Real-time presenter mode for live presentations
- WebSocket synchronization between presenter and audience
- Version control friendly markdown source format

**Operational Excellence**:
- Browser automation for consistent export quality
- Comprehensive error handling with retry logic
- Resource monitoring and automatic cleanup

slicli enables efficient presentation development workflows while maintaining professional quality output for business, educational, and technical content.