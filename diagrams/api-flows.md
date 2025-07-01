# API Flow Diagrams

Request/response sequences for slicli's REST API endpoints and WebSocket communication.

## Core Presentation API Flow

### GET / (Main Presentation Interface)

```mermaid
sequenceDiagram
    participant Client as Browser Client
    participant HTTPServer as HTTP Server
    participant PresentationSvc as PresentationService
    participant PluginSvc as PluginService
    participant ThemeSvc as ThemeService
    participant PluginCache as Plugin Cache
    participant ThemeCache as Theme Cache
    
    Client->>HTTPServer: GET /
    HTTPServer->>HTTPServer: Path validation (exact "/" only)
    HTTPServer->>PresentationSvc: LoadPresentation(ctx, path)
    
    PresentationSvc->>ThemeSvc: LoadTheme(ctx, themeName)
    ThemeSvc->>ThemeCache: Get(themeName)
    
    alt Theme Cache Hit (95% probability)
        ThemeCache-->>ThemeSvc: Return cached theme
    else Theme Cache Miss (5% probability)  
        ThemeSvc->>ThemeSvc: Load TOML + CSS processing
        ThemeSvc->>ThemeCache: Store theme (LRU + TTL)
        ThemeSvc-->>PresentationSvc: Theme loaded (50-200ms)
    end
    
    PresentationSvc->>PluginSvc: ProcessPlugins(ctx, markdown)
    PluginSvc->>PluginCache: Check cached results
    
    alt Plugin Cache Hit (85% probability)
        PluginCache-->>PluginSvc: Return cached output
    else Plugin Cache Miss (15% probability)
        Note over PluginSvc: Sequential plugin execution (performance bottleneck)
        PluginSvc->>PluginSvc: Execute plugins (200-650ms)
        PluginSvc->>PluginCache: Store results (LRU + TTL + Size)
    end
    
    PluginSvc-->>PresentationSvc: Processed content
    PresentationSvc->>PresentationSvc: HTML sanitization (BlueMonday)
    PresentationSvc-->>HTTPServer: Rendered presentation
    HTTPServer-->>Client: 200 text/html
    
    Note over Client,ThemeCache: Complete presentation load: 85ms + plugin time
```

### GET /api/slides (JSON Data Endpoint)

```mermaid
sequenceDiagram
    participant Client as JS Client
    participant HTTPServer as HTTP Server
    participant PresentationSvc as PresentationService
    participant PluginSvc as PluginService
    participant PluginCache as Plugin Cache
    
    Client->>HTTPServer: GET /api/slides
    HTTPServer->>HTTPServer: Method validation (GET only)
    HTTPServer->>PresentationSvc: LoadPresentation(ctx, path)
    
    PresentationSvc->>PluginSvc: ProcessSlides(ctx, slides)
    
    loop For each slide
        PluginSvc->>PluginCache: Check cache (slide content hash)
        alt Cache Hit
            PluginCache-->>PluginSvc: Cached processed slide
        else Cache Miss
            PluginSvc->>PluginSvc: Execute slide plugins
            PluginSvc->>PluginCache: Cache result
        end
    end
    
    PluginSvc-->>PresentationSvc: All slides processed
    PresentationSvc->>PresentationSvc: Extract slide titles
    PresentationSvc->>PresentationSvc: HTML sanitization
    PresentationSvc-->>HTTPServer: Slides JSON
    HTTPServer-->>Client: 200 application/json
    
    Note over Client,PluginCache: Typical response: 25-100ms depending on cache
```

## Presenter Mode API Flows

### POST /api/presenter/navigate (Presenter Navigation)

```mermaid
sequenceDiagram
    participant Presenter as Presenter Interface
    participant HTTPServer as HTTP Server
    participant WebSocket as WebSocket Hub
    participant Clients as Connected Clients
    
    Presenter->>HTTPServer: POST /api/presenter/navigate
    Note right of Presenter: {"action": "next|prev|goto", "slide": number}
    
    HTTPServer->>HTTPServer: Input validation
    HTTPServer->>HTTPServer: Update presenter state
    HTTPServer->>WebSocket: Broadcast navigation event
    
    par
        WebSocket->>Clients: Navigation update
    and
        HTTPServer-->>Presenter: 200 {"success": true, "currentSlide": N}
    end
    
    Note over Presenter,Clients: Real-time presenter synchronization
```

### GET /api/presenter/notes (Speaker Notes)

```mermaid
sequenceDiagram
    participant Presenter as Presenter Interface
    participant HTTPServer as HTTP Server
    participant PresentationSvc as PresentationService
    participant PluginCache as Plugin Cache
    
    Presenter->>HTTPServer: GET /api/presenter/notes?slide=N
    HTTPServer->>HTTPServer: Parameter validation
    HTTPServer->>PresentationSvc: GetSlideNotes(ctx, slideIndex)
    
    PresentationSvc->>PluginCache: Check notes cache
    alt Notes Cached
        PluginCache-->>PresentationSvc: Cached notes HTML
    else Notes Not Cached
        PresentationSvc->>PresentationSvc: Process speaker notes markdown
        PresentationSvc->>PluginCache: Cache processed notes
    end
    
    PresentationSvc-->>HTTPServer: Processed notes
    HTTPServer-->>Presenter: 200 {"notes": "HTML content"}
    
    Note over Presenter,PluginCache: Notes processed with same plugin pipeline
```

## Export API Flow

### POST /api/export (Export Request)

```mermaid
sequenceDiagram
    participant Client as Web Client
    participant HTTPServer as HTTP Server
    participant ExportSvc as ExportService
    participant BrowserAuto as Browser Automation
    participant Chrome as Chrome/Chromium
    participant TempStorage as Temp Storage
    
    Client->>HTTPServer: POST /api/export
    Note right of Client: {"format": "pdf|html|images", "options": {...}}
    
    HTTPServer->>HTTPServer: Input validation & sanitization
    HTTPServer->>ExportSvc: Export(ctx, presentation, options)
    
    ExportSvc->>ExportSvc: Create export operation ID
    ExportSvc->>ExportSvc: Initialize retry config (3 attempts, exponential backoff)
    
    loop Retry Logic (max 3 attempts)
        ExportSvc->>BrowserAuto: Launch browser instance
        BrowserAuto->>Chrome: Start headless Chrome
        Chrome-->>BrowserAuto: Browser ready
        
        BrowserAuto->>Chrome: Navigate to presentation URL
        BrowserAuto->>Chrome: Wait for page load
        
        alt PDF Export
            BrowserAuto->>Chrome: Print to PDF (options: A4, quality)
        else HTML Export
            BrowserAuto->>Chrome: Extract full HTML
        else Images Export
            BrowserAuto->>Chrome: Screenshot each slide
        end
        
        Chrome-->>BrowserAuto: Export data
        BrowserAuto->>TempStorage: Write temporary files
        
        alt Export Success
            ExportSvc->>ExportSvc: Calculate file size & metrics
            ExportSvc-->>HTTPServer: Export result
            break Retry loop
        else Export Failure (network/browser)
            ExportSvc->>ExportSvc: Categorize error (retryable?)
            ExportSvc->>ExportSvc: Wait (exponential backoff)
            Note over ExportSvc: Retry if network/browser/timeout error
        end
    end
    
    HTTPServer-->>Client: 200 {"success": true, "downloadUrl": "/api/export/download/ID"}
    
    Note over Client,TempStorage: Export with retry: 2-15 seconds depending on complexity
```

## WebSocket Live Reload Flow

### WebSocket /ws (Live Reload Connection)

```mermaid
sequenceDiagram
    participant Client as Browser Client
    participant WebSocket as WebSocket Server
    participant FileWatcher as File Watcher
    participant FileSystem as File System
    participant PresentationSvc as PresentationService
    
    Client->>WebSocket: Upgrade HTTP to WebSocket
    WebSocket->>WebSocket: CORS origin validation (allows all in dev)
    WebSocket-->>Client: Connection established
    
    FileWatcher->>FileSystem: Poll files (1-second interval)
    FileSystem-->>FileWatcher: File stats (size, modtime)
    
    alt File Changed Detection
        FileWatcher->>FileSystem: Calculate SHA256 checksum
        FileSystem-->>FileWatcher: File checksum
        FileWatcher->>FileWatcher: Compare with cached state
        
        alt File Actually Changed
            FileWatcher->>FileWatcher: Debounce logic (prevent rapid-fire)
            FileWatcher->>PresentationSvc: File change event
            PresentationSvc->>WebSocket: Broadcast reload event
            WebSocket->>Client: {"type": "reload", "path": "presentation.md"}
            Client->>Client: Reload presentation
        end
    end
    
    Note over Client,FileSystem: File watching: SHA256 calculated on every poll (optimization opportunity)
```

## Performance API Flows

### GET /api/performance/health (Health Check)

```mermaid
sequenceDiagram
    participant Client as Monitoring Client
    participant HTTPServer as HTTP Server
    participant PluginSvc as PluginService
    participant ThemeSvc as ThemeService
    participant PluginCache as Plugin Cache
    participant ThemeCache as Theme Cache
    
    Client->>HTTPServer: GET /api/performance/health
    HTTPServer->>PluginSvc: GetPluginHealth()
    PluginSvc->>PluginCache: GetCacheStats()
    PluginCache-->>PluginSvc: {"hits": 850, "misses": 150, "hitRate": 0.85}
    
    HTTPServer->>ThemeSvc: GetThemeHealth()
    ThemeSvc->>ThemeCache: GetCacheStats()
    ThemeCache-->>ThemeSvc: {"hits": 950, "misses": 50, "hitRate": 0.95}
    
    HTTPServer->>HTTPServer: Aggregate health metrics
    HTTPServer-->>Client: 200 {"status": "healthy", "cachePerformance": {...}}
    
    Note over Client,ThemeCache: Health check includes cache hit rates and system status
```

### POST /api/performance/optimize (Cache Optimization)

```mermaid
sequenceDiagram
    participant Admin as Admin Client
    participant HTTPServer as HTTP Server
    participant PluginCache as Plugin Cache
    participant ThemeCache as Theme Cache
    
    Admin->>HTTPServer: POST /api/performance/optimize
    Note right of Admin: {"action": "clear-cache|warm-cache|optimize"}
    
    HTTPServer->>HTTPServer: Admin action validation
    
    alt Clear Cache
        HTTPServer->>PluginCache: ClearCache()
        HTTPServer->>ThemeCache: ClearCache()
    else Warm Cache
        HTTPServer->>HTTPServer: Preload common themes
        HTTPServer->>HTTPServer: Preprocess common plugins
    else Optimize
        HTTPServer->>PluginCache: OptimizeEviction() 
        Note over PluginCache: Future: Replace O(n) with heap-based
        HTTPServer->>ThemeCache: Resize cache based on usage
    end
    
    HTTPServer-->>Admin: 200 {"optimized": true, "metrics": {...}}
    
    Note over Admin,ThemeCache: Cache optimization operations
```

## Error Response Patterns

### Consistent Error Handling

```mermaid
sequenceDiagram
    participant Client as Client
    participant HTTPServer as HTTP Server
    participant Service as Any Service
    participant Logger as Error Logger
    
    Client->>HTTPServer: Any API request
    HTTPServer->>Service: Service operation
    Service-->>HTTPServer: Error (with details)
    
    HTTPServer->>HTTPServer: Sanitize error message
    HTTPServer->>Logger: Log full error with context
    HTTPServer-->>Client: 500 {"error": "Internal server error"}
    
    Note over Client,Logger: Security: No internal details exposed to client
    Note over Logger: Full error details logged for debugging
```

## Key Performance Notes

- **Plugin Cache**: 85% hit rate, LRU + TTL + Size eviction (O(n) complexity issue)
- **Theme Cache**: 95% hit rate, LRU + TTL eviction (count-based only)
- **Sequential Plugin Execution**: Major bottleneck (650ms vs 300ms potential)
- **File Polling**: SHA256 checksum on every poll (95% unnecessary I/O)
- **Export Retry Logic**: Exponential backoff with 3 attempts for reliability
- **WebSocket**: No authentication, CORS validation allows all origins (dev setting)

All flows respect clean architecture boundaries with domain services isolated from infrastructure concerns.