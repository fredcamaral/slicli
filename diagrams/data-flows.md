# Data Processing & Persistence Flows

Data processing sequences for markdown transformation, plugin execution, theme loading, and caching in slicli.

## Markdown Processing Pipeline

### Complete Markdown to HTML Transformation

```mermaid
sequenceDiagram
    participant User as Markdown File
    participant FileSystem as File System
    participant PresentationSvc as PresentationService
    participant Parser as Goldmark Parser
    participant PluginSvc as PluginService
    participant ThemeSvc as ThemeService
    participant Sanitizer as HTML Sanitizer
    
    User->>FileSystem: presentation.md
    FileSystem->>PresentationSvc: LoadPresentation(path)
    PresentationSvc->>FileSystem: Read markdown content
    FileSystem-->>PresentationSvc: Raw markdown text
    
    PresentationSvc->>Parser: Parse markdown structure
    Parser->>Parser: Extract slides (---separators)
    Parser->>Parser: Identify plugin hooks
    Parser->>Parser: Parse frontmatter metadata
    Parser-->>PresentationSvc: Structured presentation
    
    PresentationSvc->>PluginSvc: ProcessPluginHooks(slides)
    
    loop For each slide with plugin content
        PluginSvc->>PluginSvc: Identify plugin patterns
        Note over PluginSvc: ```language, %%%mermaid, etc.
        PluginSvc->>PluginSvc: Execute plugin (sandboxed)
        PluginSvc->>PluginSvc: Replace plugin markers with output
    end
    
    PluginSvc-->>PresentationSvc: Slides with plugin content processed
    
    PresentationSvc->>ThemeSvc: ApplyTheme(presentation, themeName)
    ThemeSvc-->>PresentationSvc: Theme engine loaded
    
    PresentationSvc->>Parser: Convert to HTML (with theme)
    Parser-->>PresentationSvc: Raw HTML output
    
    PresentationSvc->>Sanitizer: SanitizeHTML(html)
    Sanitizer-->>PresentationSvc: Clean, safe HTML
    
    PresentationSvc-->>User: Final presentation HTML
    
    Note over User,Sanitizer: Complete transformation: Markdown → Plugins → Theme → HTML
```

## Plugin Execution & Caching Flow

### Plugin Processing with Multi-Level Caching

```mermaid
sequenceDiagram
    participant PluginSvc as PluginService
    participant PluginCache as Plugin Cache<br/>(LRU+TTL+Size)
    participant PluginLoader as PluginLoader
    participant CodeExecPlugin as Code Exec Plugin
    participant SyntaxPlugin as Syntax Highlight Plugin
    participant MermaidPlugin as Mermaid Plugin
    participant Sandbox as Execution Sandbox
    
    PluginSvc->>PluginCache: Get(contentHash, pluginName)
    
    alt Cache Hit (85% probability)
        PluginCache->>PluginCache: Update access time (LRU)
        PluginCache->>PluginCache: Increment hit counter
        PluginCache-->>PluginSvc: Cached plugin output (1-5ms)
    else Cache Miss (15% probability)
        PluginCache-->>PluginSvc: Cache miss
        
        PluginSvc->>PluginLoader: LoadPlugin(pluginName)
        PluginLoader->>PluginLoader: Load shared object (.so file)
        PluginLoader-->>PluginSvc: Plugin interface
        
        Note over PluginSvc,Sandbox: Sequential execution (performance bottleneck)
        
        alt Code Execution Plugin
            PluginSvc->>CodeExecPlugin: Execute(codeBlock, language)
            CodeExecPlugin->>Sandbox: Sandboxed execution
            Sandbox-->>CodeExecPlugin: Execution result
            CodeExecPlugin-->>PluginSvc: Formatted output (200-500ms)
        else Syntax Highlighting
            PluginSvc->>SyntaxPlugin: Highlight(code, language)
            SyntaxPlugin->>SyntaxPlugin: Apply Chroma highlighting
            SyntaxPlugin-->>PluginSvc: Highlighted HTML (50-150ms)
        else Mermaid Diagrams
            PluginSvc->>MermaidPlugin: Render(diagramCode)
            MermaidPlugin->>MermaidPlugin: Generate CDN-based HTML
            MermaidPlugin-->>PluginSvc: Diagram HTML (10-50ms)
        end
        
        PluginSvc->>PluginCache: Store(contentHash, output, size)
        
        alt Cache Size Check
            PluginCache->>PluginCache: Check total size vs limit (100MB)
            alt Size Limit Exceeded
                PluginCache->>PluginCache: Evict LRU entries (O(n) complexity)
                Note over PluginCache: Performance issue: Linear scan eviction
            end
        end
        
        PluginCache->>PluginCache: Set TTL expiration
        PluginCache-->>PluginSvc: Plugin output stored
    end
    
    Note over PluginSvc,Sandbox: Plugin cache: 85% hit rate, O(n) eviction needs optimization
```

## Theme Loading & Processing

### Theme System with CSS Processing

```mermaid
sequenceDiagram
    participant ThemeSvc as ThemeService
    participant ThemeCache as Theme Cache<br/>(LRU+TTL)
    participant FileSystem as File System
    participant TOMLParser as TOML Parser
    participant CSSProcessor as CSS Processor
    participant AssetLoader as Asset Loader
    
    ThemeSvc->>ThemeCache: Get(themeName)
    
    alt Theme Cache Hit (95% probability)
        ThemeCache->>ThemeCache: Update last access time
        ThemeCache->>ThemeCache: Increment hit counter
        ThemeCache-->>ThemeSvc: Cached theme engine (1-2ms)
    else Theme Cache Miss (5% probability)
        ThemeCache-->>ThemeSvc: Cache miss
        
        ThemeSvc->>FileSystem: Read theme.toml
        FileSystem-->>ThemeSvc: Theme configuration
        
        ThemeSvc->>TOMLParser: Parse configuration
        TOMLParser->>TOMLParser: Extract theme metadata
        TOMLParser->>TOMLParser: Parse CSS references
        TOMLParser->>TOMLParser: Extract asset paths
        TOMLParser-->>ThemeSvc: Parsed theme config
        
        ThemeSvc->>CSSProcessor: LoadStylesheet(cssPath)
        CSSProcessor->>FileSystem: Read CSS files
        FileSystem-->>CSSProcessor: CSS content
        
        CSSProcessor->>CSSProcessor: Process CSS variables
        CSSProcessor->>CSSProcessor: Apply theme inheritance
        CSSProcessor->>CSSProcessor: Minify CSS (if configured)
        CSSProcessor-->>ThemeSvc: Processed CSS
        
        ThemeSvc->>AssetLoader: LoadAssets(assetPaths)
        AssetLoader->>FileSystem: Read theme assets
        FileSystem-->>AssetLoader: Asset data
        AssetLoader-->>ThemeSvc: Theme assets
        
        ThemeSvc->>ThemeSvc: Create theme engine
        ThemeSvc->>ThemeCache: Store(themeName, themeEngine)
        
        alt Count-Based Eviction
            ThemeCache->>ThemeCache: Check theme count vs limit
            alt Count Limit Exceeded
                ThemeCache->>ThemeCache: Evict least recently used
                Note over ThemeCache: Missing: Size-based eviction
            end
        end
        
        ThemeCache-->>ThemeSvc: Theme cached (50-200ms)
    end
    
    Note over ThemeSvc,AssetLoader: Theme cache: 95% hit rate, needs size-based eviction
```

## File System Monitoring & Change Detection

### File Watching with Checksum Validation

```mermaid
sequenceDiagram
    participant FileWatcher as Polling File Watcher
    participant FileSystem as File System
    participant Checksum as SHA256 Calculator
    participant ChangeDetector as Change Detector
    participant EventBus as Change Event Bus
    participant WebSocket as WebSocket Hub
    participant Clients as Connected Clients
    
    loop Every 1 second (configurable)
        FileWatcher->>FileSystem: Stat watched files
        FileSystem-->>FileWatcher: File metadata (size, modtime)
        
        FileWatcher->>ChangeDetector: CompareMetadata(current, cached)
        
        alt Metadata Changed (size or modtime)
            ChangeDetector->>Checksum: CalculateChecksum(filePath)
            Checksum->>FileSystem: Read entire file
            FileSystem-->>Checksum: File content
            Checksum->>Checksum: Calculate SHA256 hash
            Checksum-->>ChangeDetector: File checksum
            
            ChangeDetector->>ChangeDetector: Compare with cached checksum
            
            alt Checksum Different (actual change)
                ChangeDetector->>ChangeDetector: Apply debounce logic
                ChangeDetector->>EventBus: FileChangeEvent
                EventBus->>WebSocket: Broadcast change
                WebSocket->>Clients: Reload notification
                ChangeDetector->>ChangeDetector: Update cached metadata
            else Checksum Same (false positive)
                ChangeDetector->>ChangeDetector: Update metadata, no event
            end
        else Metadata Unchanged
            Note over FileWatcher,Checksum: Skip checksum calculation (optimization opportunity)
        end
    end
    
    Note over FileWatcher,Clients: Current: SHA256 on every poll. Optimization: Skip if size/time unchanged
```

## Export Data Processing

### Multi-Format Export Pipeline

```mermaid
sequenceDiagram
    participant ExportSvc as ExportService
    participant PresentationSvc as PresentationService
    participant BrowserAuto as Browser Automation
    participant Chrome as Headless Chrome
    participant TempStorage as Temp File Manager
    participant FormatConverter as Format Converter
    
    ExportSvc->>PresentationSvc: GetFullPresentation()
    PresentationSvc-->>ExportSvc: Complete presentation HTML
    
    ExportSvc->>BrowserAuto: LaunchBrowser()
    BrowserAuto->>Chrome: Start headless instance
    Chrome-->>BrowserAuto: Browser ready
    
    ExportSvc->>BrowserAuto: LoadPresentation(html)
    BrowserAuto->>Chrome: Navigate to presentation
    Chrome->>Chrome: Render presentation
    Chrome-->>BrowserAuto: Page loaded
    
    alt PDF Export
        ExportSvc->>BrowserAuto: PrintToPDF(options)
        BrowserAuto->>Chrome: Print with A4/Letter settings
        Chrome-->>BrowserAuto: PDF data
        BrowserAuto->>TempStorage: WritePDF(data)
    else HTML Export
        ExportSvc->>BrowserAuto: ExtractHTML()
        BrowserAuto->>Chrome: Get full HTML + assets
        Chrome-->>BrowserAuto: Complete HTML
        BrowserAuto->>TempStorage: WriteHTML(html)
    else Images Export
        loop For each slide
            ExportSvc->>BrowserAuto: NavigateToSlide(index)
            BrowserAuto->>Chrome: Scroll to slide
            ExportSvc->>BrowserAuto: Screenshot(quality)
            BrowserAuto->>Chrome: Capture screenshot
            Chrome-->>BrowserAuto: Image data
            BrowserAuto->>TempStorage: WriteImage(data, index)
        end
    else PowerPoint Export
        ExportSvc->>FormatConverter: ConvertToPPTX(slides)
        FormatConverter->>FormatConverter: Generate PPTX structure
        FormatConverter->>TempStorage: WritePPTX(data)
    end
    
    TempStorage->>TempStorage: Calculate file size
    TempStorage->>TempStorage: Set cleanup timer (24h)
    TempStorage-->>ExportSvc: Export result with metadata
    
    Note over ExportSvc,TempStorage: Export formats: PDF, HTML, Images, PPTX with browser automation
```

## Cache Performance & Memory Management

### Cache Eviction Strategies

```mermaid
sequenceDiagram
    participant Cache as Cache Manager
    participant PluginCache as Plugin Cache
    participant ThemeCache as Theme Cache
    participant MemoryMonitor as Memory Monitor
    participant EvictionEngine as Eviction Engine
    
    Cache->>MemoryMonitor: CheckMemoryUsage()
    MemoryMonitor-->>Cache: Memory status
    
    alt Memory Pressure Detected
        Cache->>PluginCache: GetCacheStats()
        PluginCache-->>Cache: {size: 90MB, entries: 1000, hitRate: 0.85}
        
        Cache->>EvictionEngine: EvictLRUEntries(pluginCache, targetSize)
        
        Note over EvictionEngine: Current: O(n) linear scan (performance issue)
        EvictionEngine->>EvictionEngine: Scan all entries for LRU
        EvictionEngine->>EvictionEngine: Sort by access time
        EvictionEngine->>PluginCache: RemoveEntries(lruKeys)
        
        Cache->>ThemeCache: GetCacheStats()
        ThemeCache-->>Cache: {count: 15, hitRate: 0.95}
        
        alt Theme Count Limit Exceeded
            Cache->>EvictionEngine: EvictLRUThemes(targetCount)
            EvictionEngine->>EvictionEngine: Find least recently used theme
            EvictionEngine->>ThemeCache: RemoveTheme(lruTheme)
        end
    end
    
    Cache->>Cache: Update cache metrics
    Cache->>Cache: Schedule next memory check
    
    Note over Cache,EvictionEngine: Optimization needed: Replace O(n) with heap-based eviction
```

## Database/Storage Architecture

### File-Based Storage with Caching

```mermaid
sequenceDiagram
    participant Application as Application Layer
    participant CacheLayer as Multi-Level Cache
    participant FileStorage as File Storage Layer
    participant FileSystem as Operating System
    
    Application->>CacheLayer: Request data
    
    CacheLayer->>CacheLayer: Check L1 Cache (Plugin Cache)
    alt L1 Hit
        CacheLayer-->>Application: Return cached data (1-5ms)
    else L1 Miss
        CacheLayer->>CacheLayer: Check L2 Cache (Theme Cache)
        alt L2 Hit
            CacheLayer-->>Application: Return cached data (5-10ms)
        else L2 Miss
            CacheLayer->>FileStorage: Load from disk
            FileStorage->>FileSystem: Read file
            FileSystem-->>FileStorage: File data
            FileStorage->>FileStorage: Process/parse data
            FileStorage-->>CacheLayer: Processed data
            CacheLayer->>CacheLayer: Store in appropriate cache
            CacheLayer-->>Application: Return data (50-200ms)
        end
    end
    
    Note over Application,FileSystem: Storage: File-based with sophisticated caching (no traditional database)
```

## Key Performance Characteristics

**Plugin Cache Performance**:
- Hit Rate: 85% (excellent)
- Eviction: O(n) complexity (optimization needed)
- Memory Limit: 100MB default
- TTL: Configurable expiration

**Theme Cache Performance**:
- Hit Rate: 95% (excellent)
- Eviction: Count-based only (needs size limits)
- Memory Usage: Uncontrolled (needs monitoring)
- TTL: Configurable expiration

**File System Monitoring**:
- Polling Frequency: 1-second intervals
- Change Detection: SHA256 checksums
- Optimization Opportunity: Skip checksum if size/modtime unchanged (95% I/O reduction)

**Processing Bottlenecks**:
- Sequential Plugin Execution: 650ms current vs 300ms potential with concurrency
- File Checksum Calculation: 100% of polls vs 5% needed
- Cache Eviction: Linear scan vs heap-based optimization

**Memory Usage**:
- Base Application: ~25MB
- Plugin Cache: Up to 100MB
- Theme Cache: ~20MB estimated
- Export Processing: Variable (10-500MB)

All data flows maintain clean architecture boundaries with domain logic isolated from infrastructure concerns.