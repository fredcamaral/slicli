# System Interaction Flows

Service-to-service communication patterns and clean architecture layer interactions in slicli.

## Clean Architecture Layer Communication

### Hexagonal Architecture Flow

```mermaid
sequenceDiagram
    participant Client as External Client
    participant PrimaryAdapter as Primary Adapter<br/>(HTTP/CLI)
    participant DomainService as Domain Service<br/>(Business Logic)
    participant DomainPorts as Domain Ports<br/>(Interfaces)
    participant SecondaryAdapter as Secondary Adapter<br/>(Infrastructure)
    participant ExternalSystem as External System
    
    Client->>PrimaryAdapter: Request (HTTP/CLI command)
    PrimaryAdapter->>PrimaryAdapter: Input validation & transformation
    PrimaryAdapter->>DomainService: Call domain operation
    
    DomainService->>DomainService: Execute business logic
    DomainService->>DomainPorts: Call infrastructure interface
    DomainPorts->>SecondaryAdapter: Invoke adapter implementation
    
    SecondaryAdapter->>SecondaryAdapter: Handle infrastructure concerns
    SecondaryAdapter->>ExternalSystem: External system call
    ExternalSystem-->>SecondaryAdapter: External response
    
    SecondaryAdapter->>SecondaryAdapter: Transform external data
    SecondaryAdapter-->>DomainPorts: Domain model response
    DomainPorts-->>DomainService: Business data
    
    DomainService->>DomainService: Process business result
    DomainService-->>PrimaryAdapter: Domain response
    PrimaryAdapter->>PrimaryAdapter: Format for client
    PrimaryAdapter-->>Client: Client response
    
    Note over Client,ExternalSystem: Dependency Rule: Domain never depends on infrastructure
```

## Plugin System Architecture

### Plugin Loading & Execution Architecture

```mermaid
sequenceDiagram
    participant PluginService as Plugin Service<br/>(Domain)
    participant PluginPort as Plugin Port<br/>(Interface)
    participant PluginLoader as Plugin Loader<br/>(Adapter)
    participant PluginCache as Plugin Cache<br/>(Infrastructure)
    participant GoRuntime as Go Runtime<br/>(System)
    participant PluginBinary as Plugin Binary<br/>(.so file)
    participant ResourceMonitor as Resource Monitor<br/>(Infrastructure)
    
    PluginService->>PluginPort: LoadPlugin(name)
    PluginPort->>PluginLoader: Load shared object
    
    PluginLoader->>PluginCache: CheckCache(pluginName)
    alt Plugin Cached
        PluginCache-->>PluginLoader: Cached plugin instance
    else Plugin Not Cached
        PluginLoader->>GoRuntime: dlopen(pluginPath)
        GoRuntime->>PluginBinary: Load shared object
        PluginBinary-->>GoRuntime: Plugin symbols
        GoRuntime-->>PluginLoader: Plugin interface
        
        PluginLoader->>PluginLoader: Validate plugin interface
        PluginLoader->>PluginCache: Store(pluginName, instance)
    end
    
    PluginLoader-->>PluginPort: Plugin ready
    PluginPort-->>PluginService: Plugin loaded
    
    PluginService->>PluginPort: ExecutePlugin(input)
    PluginPort->>PluginLoader: Execute with sandbox
    
    PluginLoader->>ResourceMonitor: Set resource limits
    ResourceMonitor->>ResourceMonitor: Memory limit: 100MB
    ResourceMonitor->>ResourceMonitor: Timeout: 30 seconds
    
    PluginLoader->>PluginBinary: Execute(context, input)
    
    par
        PluginBinary->>PluginBinary: Process input
    and
        ResourceMonitor->>ResourceMonitor: Monitor memory usage
    and
        ResourceMonitor->>ResourceMonitor: Monitor execution time
    end
    
    PluginBinary-->>PluginLoader: Plugin output
    PluginLoader->>ResourceMonitor: Release resources
    PluginLoader-->>PluginPort: Execution result
    PluginPort-->>PluginService: Plugin output
    
    Note over PluginService,ResourceMonitor: Plugin architecture: Sandboxed execution with resource monitoring
```

### Plugin Communication Patterns

```mermaid
sequenceDiagram
    participant PresentationSvc as Presentation Service
    participant PluginOrchestrator as Plugin Orchestrator
    participant CodeExecPlugin as Code Exec Plugin
    participant SyntaxPlugin as Syntax Highlight Plugin
    participant MermaidPlugin as Mermaid Plugin
    participant PluginCache as Plugin Cache
    participant ResultAggregator as Result Aggregator
    
    PresentationSvc->>PluginOrchestrator: ProcessSlide(slideContent)
    PluginOrchestrator->>PluginOrchestrator: Parse plugin markers
    
    Note over PluginOrchestrator: Sequential execution (performance bottleneck)
    
    PluginOrchestrator->>CodeExecPlugin: Execute(```python code)
    CodeExecPlugin->>PluginCache: Check cache(codeHash)
    alt Cache Miss
        CodeExecPlugin->>CodeExecPlugin: Sandboxed Python execution
        CodeExecPlugin->>PluginCache: Store result
    end
    CodeExecPlugin-->>PluginOrchestrator: Execution output
    
    PluginOrchestrator->>SyntaxPlugin: Highlight(```go code)
    SyntaxPlugin->>PluginCache: Check cache(codeHash)
    alt Cache Miss
        SyntaxPlugin->>SyntaxPlugin: Apply Chroma highlighting
        SyntaxPlugin->>PluginCache: Store result
    end
    SyntaxPlugin-->>PluginOrchestrator: Highlighted HTML
    
    PluginOrchestrator->>MermaidPlugin: Render(%%%mermaid diagram)
    MermaidPlugin->>PluginCache: Check cache(diagramHash)
    alt Cache Miss
        MermaidPlugin->>MermaidPlugin: Generate CDN-based HTML
        MermaidPlugin->>PluginCache: Store result
    end
    MermaidPlugin-->>PluginOrchestrator: Diagram HTML
    
    PluginOrchestrator->>ResultAggregator: CombineResults(outputs)
    ResultAggregator->>ResultAggregator: Merge plugin outputs into slide
    ResultAggregator-->>PluginOrchestrator: Final slide HTML
    
    PluginOrchestrator-->>PresentationSvc: Processed slide
    
    Note over PresentationSvc,ResultAggregator: Future optimization: Parallel plugin execution
```

## Theme System Architecture

### Theme Loading & Processing Flow

```mermaid
sequenceDiagram
    participant ThemeService as Theme Service<br/>(Domain)
    participant ThemePort as Theme Port<br/>(Interface)
    participant ThemeLoader as Theme Loader<br/>(Adapter)
    participant ThemeCache as Theme Cache<br/>(Infrastructure)
    participant FileSystem as File System<br/>(External)
    participant TOMLProcessor as TOML Processor<br/>(Infrastructure)
    participant CSSProcessor as CSS Processor<br/>(Infrastructure)
    
    ThemeService->>ThemePort: LoadTheme(themeName)
    ThemePort->>ThemeLoader: Load theme configuration
    
    ThemeLoader->>ThemeCache: GetTheme(themeName)
    alt Theme Cache Hit (95% probability)
        ThemeCache-->>ThemeLoader: Cached theme engine
        ThemeLoader-->>ThemePort: Theme ready
    else Theme Cache Miss (5% probability)
        ThemeLoader->>FileSystem: Read theme.toml
        FileSystem-->>ThemeLoader: Theme configuration
        
        ThemeLoader->>TOMLProcessor: ParseConfig(tomlContent)
        TOMLProcessor->>TOMLProcessor: Parse theme metadata
        TOMLProcessor->>TOMLProcessor: Extract CSS references
        TOMLProcessor->>TOMLProcessor: Parse inheritance chain
        TOMLProcessor-->>ThemeLoader: Parsed configuration
        
        ThemeLoader->>FileSystem: Read CSS files
        FileSystem-->>ThemeLoader: CSS content
        
        ThemeLoader->>CSSProcessor: ProcessCSS(cssContent, config)
        CSSProcessor->>CSSProcessor: Apply CSS variables
        CSSProcessor->>CSSProcessor: Process theme inheritance
        CSSProcessor->>CSSProcessor: Optimize CSS (minification)
        CSSProcessor-->>ThemeLoader: Processed CSS
        
        ThemeLoader->>ThemeLoader: Create theme engine
        ThemeLoader->>ThemeCache: Store(themeName, themeEngine)
        ThemeLoader-->>ThemePort: Theme loaded
    end
    
    ThemePort-->>ThemeService: Theme engine ready
    
    Note over ThemeService,CSSProcessor: Theme architecture: TOML configuration with CSS processing pipeline
```

## Export System Architecture

### Multi-Format Export Orchestration

```mermaid
sequenceDiagram
    participant ExportService as Export Service<br/>(Domain)
    participant ExportPort as Export Port<br/>(Interface)
    participant ExportOrchestrator as Export Orchestrator<br/>(Adapter)
    participant BrowserAdapter as Browser Adapter<br/>(Infrastructure)
    participant Chrome as Chrome/Chromium<br/>(External)
    participant FormatRenderer as Format Renderer<br/>(Infrastructure)
    participant TempStorage as Temp Storage<br/>(Infrastructure)
    participant RetryManager as Retry Manager<br/>(Infrastructure)
    
    ExportService->>ExportPort: Export(presentation, formats)
    ExportPort->>ExportOrchestrator: OrchestateExport(options)
    
    ExportOrchestrator->>ExportOrchestrator: Create export operation ID
    ExportOrchestrator->>RetryManager: Initialize retry config
    
    par PDF Export
        ExportOrchestrator->>BrowserAdapter: ExportToPDF()
        BrowserAdapter->>Chrome: Launch headless browser
        Chrome-->>BrowserAdapter: Browser ready
        BrowserAdapter->>Chrome: Navigate to presentation
        BrowserAdapter->>Chrome: Print to PDF
        Chrome-->>BrowserAdapter: PDF data
        BrowserAdapter->>TempStorage: Store PDF file
    and HTML Export
        ExportOrchestrator->>FormatRenderer: ExportToHTML()
        FormatRenderer->>BrowserAdapter: Get page HTML
        BrowserAdapter->>Chrome: Extract HTML + assets
        Chrome-->>BrowserAdapter: Complete HTML bundle
        FormatRenderer->>TempStorage: Store HTML files
    and Images Export
        ExportOrchestrator->>BrowserAdapter: ExportToImages()
        loop For each slide
            BrowserAdapter->>Chrome: Navigate to slide
            BrowserAdapter->>Chrome: Screenshot slide
            Chrome-->>BrowserAdapter: Image data
            BrowserAdapter->>TempStorage: Store image file
        end
    end
    
    ExportOrchestrator->>ExportOrchestrator: Aggregate export results
    ExportOrchestrator->>TempStorage: Generate export manifest
    ExportOrchestrator-->>ExportPort: Export complete
    ExportPort-->>ExportService: Export results
    
    Note over ExportService,TempStorage: Export orchestration: Parallel format generation with unified result handling
```

## File System Monitoring Architecture

### Live Reload System Integration

```mermaid
sequenceDiagram
    participant FileWatcher as File Watcher<br/>(Infrastructure)
    participant ChangeDetector as Change Detector<br/>(Infrastructure)
    participant EventBus as Event Bus<br/>(Domain)
    participant LiveReloadService as Live Reload Service<br/>(Domain)
    participant WebSocketHub as WebSocket Hub<br/>(Infrastructure)
    participant ConnectedClients as Connected Clients<br/>(External)
    participant PresentationService as Presentation Service<br/>(Domain)
    
    loop File Monitoring
        FileWatcher->>FileWatcher: Poll watched files (1s interval)
        FileWatcher->>ChangeDetector: CheckForChanges(filePath)
        
        ChangeDetector->>ChangeDetector: Compare size/modtime
        alt Metadata Changed
            ChangeDetector->>ChangeDetector: Calculate SHA256 checksum
            ChangeDetector->>ChangeDetector: Compare with cached checksum
            
            alt File Actually Changed
                ChangeDetector->>EventBus: FileChangeEvent
                EventBus->>LiveReloadService: ProcessFileChange(event)
                
                LiveReloadService->>LiveReloadService: Apply debounce logic
                LiveReloadService->>PresentationService: InvalidateCache(filePath)
                PresentationService->>PresentationService: Clear relevant caches
                
                LiveReloadService->>WebSocketHub: BroadcastReload(event)
                WebSocketHub->>ConnectedClients: Send reload notification
                ConnectedClients->>ConnectedClients: Reload presentation
            end
        end
    end
    
    Note over FileWatcher,ConnectedClients: Live reload: File system monitoring with WebSocket notification
```

## Cache System Architecture

### Multi-Level Cache Coordination

```mermaid
sequenceDiagram
    participant CacheManager as Cache Manager<br/>(Domain)
    participant PluginCache as Plugin Cache<br/>(Infrastructure)
    participant ThemeCache as Theme Cache<br/>(Infrastructure)
    participant MemoryMonitor as Memory Monitor<br/>(Infrastructure)
    participant EvictionEngine as Eviction Engine<br/>(Infrastructure)
    participant CacheStats as Cache Statistics<br/>(Infrastructure)
    
    CacheManager->>MemoryMonitor: CheckSystemMemory()
    MemoryMonitor-->>CacheManager: Memory status
    
    alt Memory Pressure Detected
        CacheManager->>PluginCache: GetCacheStats()
        PluginCache-->>CacheManager: {size: 90MB, hits: 850, misses: 150}
        
        CacheManager->>ThemeCache: GetCacheStats()
        ThemeCache-->>CacheManager: {count: 15, hits: 950, misses: 50}
        
        CacheManager->>EvictionEngine: OptimizeMemoryUsage()
        
        EvictionEngine->>PluginCache: EvictLRUEntries(targetSize)
        Note over PluginCache: O(n) eviction complexity (optimization needed)
        PluginCache->>PluginCache: Linear scan for LRU entries
        PluginCache->>PluginCache: Remove least recently used
        
        EvictionEngine->>ThemeCache: EvictLRUThemes(targetCount)
        ThemeCache->>ThemeCache: Find least recently used theme
        ThemeCache->>ThemeCache: Remove LRU theme
        
        EvictionEngine-->>CacheManager: Memory optimization complete
    end
    
    CacheManager->>CacheStats: UpdateStatistics()
    CacheStats->>CacheStats: Calculate hit rates
    CacheStats->>CacheStats: Track eviction patterns
    CacheStats->>CacheStats: Monitor memory efficiency
    
    CacheManager->>CacheManager: Schedule next optimization cycle
    
    Note over CacheManager,CacheStats: Cache coordination: Multi-level optimization with performance monitoring
```

## Service Communication Patterns

### Domain Service Interactions

```mermaid
sequenceDiagram
    participant HTTPHandler as HTTP Handler<br/>(Primary Adapter)
    participant PresentationSvc as Presentation Service<br/>(Domain)
    participant PluginSvc as Plugin Service<br/>(Domain)
    participant ThemeService as Theme Service<br/>(Domain)
    participant ConfigService as Config Service<br/>(Domain)
    participant LiveReloadSvc as Live Reload Service<br/>(Domain)
    
    HTTPHandler->>PresentationSvc: LoadPresentation(path)
    PresentationSvc->>ConfigService: GetPresentationConfig()
    ConfigService-->>PresentationSvc: Configuration settings
    
    PresentationSvc->>ThemeService: ApplyTheme(themeName)
    ThemeService-->>PresentationSvc: Theme engine
    
    PresentationSvc->>PluginSvc: ProcessPlugins(content)
    
    loop For each plugin
        PluginSvc->>PluginSvc: Execute plugin (sandboxed)
        PluginSvc->>PluginSvc: Cache result
    end
    
    PluginSvc-->>PresentationSvc: Processed content
    
    PresentationSvc->>PresentationSvc: Combine theme + content
    PresentationSvc->>PresentationSvc: Apply HTML sanitization
    PresentationSvc-->>HTTPHandler: Final presentation
    
    par Live Reload Setup
        HTTPHandler->>LiveReloadSvc: RegisterClient(clientId)
        LiveReloadSvc->>LiveReloadSvc: Add to notification list
    and File Watching
        LiveReloadSvc->>LiveReloadSvc: Monitor presentation files
        LiveReloadSvc->>LiveReloadSvc: Detect changes
        LiveReloadSvc->>HTTPHandler: Notify reload needed
    end
    
    Note over HTTPHandler,LiveReloadSvc: Service collaboration: Domain services coordinate through interfaces
```

## External System Integration

### Browser Automation Integration

```mermaid
sequenceDiagram
    participant ExportAdapter as Export Adapter<br/>(Infrastructure)
    participant BrowserManager as Browser Manager<br/>(Infrastructure)
    participant ProcessManager as Process Manager<br/>(System)
    participant Chrome as Chrome Browser<br/>(External)
    participant DevToolsAPI as DevTools API<br/>(External)
    participant ResourceMonitor as Resource Monitor<br/>(Infrastructure)
    
    ExportAdapter->>BrowserManager: RequestBrowser(purpose)
    BrowserManager->>ProcessManager: LaunchProcess(chromeArgs)
    ProcessManager->>Chrome: Start headless Chrome
    Chrome-->>ProcessManager: Process started
    ProcessManager-->>BrowserManager: Browser process info
    
    BrowserManager->>DevToolsAPI: Connect to DevTools
    DevToolsAPI-->>BrowserManager: DevTools session
    BrowserManager->>ResourceMonitor: RegisterBrowser(processId)
    ResourceMonitor->>ResourceMonitor: Monitor CPU/memory usage
    
    BrowserManager-->>ExportAdapter: Browser ready
    
    ExportAdapter->>DevToolsAPI: Page.navigate(url)
    DevToolsAPI->>Chrome: Navigate to presentation
    Chrome-->>DevToolsAPI: Navigation complete
    
    ExportAdapter->>DevToolsAPI: Page.printToPDF(options)
    DevToolsAPI->>Chrome: Generate PDF
    Chrome-->>DevToolsAPI: PDF data
    DevToolsAPI-->>ExportAdapter: Export result
    
    ExportAdapter->>BrowserManager: ReleaseBrowser()
    BrowserManager->>ResourceMonitor: UnregisterBrowser(processId)
    BrowserManager->>ProcessManager: TerminateProcess(processId)
    ProcessManager->>Chrome: Graceful shutdown
    
    Note over ExportAdapter,Chrome: Browser integration: Managed process lifecycle with resource monitoring
```

## Performance Optimization Opportunities

**Sequential Plugin Execution**:
- Current: Plugins execute one after another (650ms total)
- Optimization: Parallel execution where possible (300ms potential)
- Challenge: Dependency management between plugins

**Cache Eviction Complexity**:
- Current: O(n) linear scan for LRU eviction
- Optimization: Heap-based priority queue (O(log n))
- Impact: 90% faster eviction for large caches

**File System Monitoring**:
- Current: SHA256 checksum on every poll
- Optimization: Skip checksum if size/modtime unchanged
- Impact: 95% reduction in unnecessary I/O

**WebSocket Connection Management**:
- Current: Per-connection heartbeat monitoring
- Optimization: Batch heartbeat processing
- Impact: Reduced CPU overhead for many connections

## Architecture Quality Characteristics

**Dependency Inversion**: Domain layer never depends on infrastructure
**Interface Segregation**: Focused, single-purpose port interfaces
**Single Responsibility**: Each service handles one business concern
**Open/Closed**: Plugin system allows extension without modification
**Separation of Concerns**: Clear boundaries between layers
**Testability**: Interfaces enable comprehensive mocking for tests

slicli's system interactions demonstrate excellent clean architecture implementation with clear separation between business logic and infrastructure concerns.