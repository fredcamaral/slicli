# Error Handling & Recovery Flows

Error handling sequences and recovery mechanisms for robust operation in slicli.

## Plugin Execution Error Handling

### Plugin Sandbox Failure Recovery

```mermaid
sequenceDiagram
    participant PluginSvc as PluginService
    participant Sandbox as Plugin Sandbox
    participant ResourceMonitor as Resource Monitor
    participant TimeoutController as Timeout Controller
    participant PluginCache as Plugin Cache
    participant ErrorHandler as Error Handler
    participant Logger as Error Logger
    
    PluginSvc->>Sandbox: ExecutePlugin(input, limits)
    Sandbox->>ResourceMonitor: Set memory limit (100MB)
    Sandbox->>TimeoutController: Set timeout (30s)
    
    par Plugin Execution
        Sandbox->>Sandbox: Execute plugin code
    and Memory Monitoring
        ResourceMonitor->>ResourceMonitor: Monitor memory usage
        alt Memory Limit Exceeded
            ResourceMonitor->>ErrorHandler: MemoryLimitError
            ErrorHandler->>Sandbox: Kill plugin process
            ErrorHandler->>Logger: Log memory violation
            ErrorHandler-->>PluginSvc: Error: "Plugin memory limit exceeded"
        end
    and Timeout Monitoring
        TimeoutController->>TimeoutController: Monitor execution time
        alt Timeout Exceeded
            TimeoutController->>ErrorHandler: TimeoutError
            ErrorHandler->>Sandbox: Cancel context
            ErrorHandler->>Logger: Log timeout violation
            ErrorHandler-->>PluginSvc: Error: "Plugin execution timeout"
        end
    and Panic Recovery
        Sandbox->>Sandbox: Recover from panics
        alt Plugin Panic
            Sandbox->>ErrorHandler: PanicError
            ErrorHandler->>Logger: Log panic details
            ErrorHandler-->>PluginSvc: Error: "Plugin crashed"
        end
    end
    
    alt Plugin Success
        Sandbox-->>PluginSvc: Plugin output
        PluginSvc->>PluginCache: Cache successful result
    else Plugin Failure
        ErrorHandler->>ErrorHandler: Categorize error type
        ErrorHandler->>PluginCache: Skip caching failed result
        ErrorHandler->>ErrorHandler: Generate fallback content
        ErrorHandler-->>PluginSvc: Fallback: Original markdown content
    end
    
    Note over PluginSvc,Logger: Graceful degradation: Plugin failures don't break presentation
```

### Plugin Loading Error Recovery

```mermaid
sequenceDiagram
    participant PluginSvc as PluginService
    participant PluginLoader as PluginLoader
    participant FileSystem as File System
    participant PluginCache as Plugin Cache
    participant FallbackRegistry as Fallback Registry
    participant ErrorReporter as Error Reporter
    
    PluginSvc->>PluginLoader: LoadPlugin(pluginName)
    PluginLoader->>FileSystem: Load shared object (.so file)
    
    alt Plugin File Not Found
        FileSystem-->>PluginLoader: Error: "File not found"
        PluginLoader->>ErrorReporter: PluginNotFoundError
        ErrorReporter->>ErrorReporter: Log missing plugin
        PluginLoader->>FallbackRegistry: GetFallback(pluginName)
        FallbackRegistry-->>PluginLoader: Fallback: NoOpPlugin
        PluginLoader-->>PluginSvc: Fallback plugin loaded
    else Plugin Interface Mismatch
        PluginLoader->>PluginLoader: Validate plugin interface
        PluginLoader-->>PluginLoader: Error: "Interface not implemented"
        PluginLoader->>ErrorReporter: InterfaceError
        ErrorReporter->>ErrorReporter: Log interface mismatch
        PluginLoader->>FallbackRegistry: GetFallback(pluginName)
        FallbackRegistry-->>PluginLoader: Fallback: PassthroughPlugin
    else Plugin Dependencies Missing
        PluginLoader->>PluginLoader: Check plugin dependencies
        PluginLoader-->>PluginLoader: Error: "Missing dependencies"
        PluginLoader->>ErrorReporter: DependencyError
        ErrorReporter->>ErrorReporter: Log dependency issue
        PluginLoader->>FallbackRegistry: GetFallback(pluginName)
        FallbackRegistry-->>PluginLoader: Fallback: ErrorMessagePlugin
    else Plugin Loads Successfully
        PluginLoader->>PluginCache: Cache loaded plugin
        PluginLoader-->>PluginSvc: Plugin ready
    end
    
    Note over PluginSvc,ErrorReporter: Robust plugin loading: Always provides fallback functionality
```

## Export Error Handling & Retry Logic

### Browser Automation Failure Recovery

```mermaid
sequenceDiagram
    participant ExportSvc as ExportService
    participant BrowserAuto as Browser Automation
    participant Chrome as Chrome/Chromium
    participant RetryManager as Retry Manager
    participant FallbackRenderer as Fallback Renderer
    participant ErrorCategorizer as Error Categorizer
    
    ExportSvc->>BrowserAuto: ExportToPDF(presentation)
    BrowserAuto->>Chrome: Launch headless browser
    
    alt Browser Launch Failure
        Chrome-->>BrowserAuto: Error: "Chrome not found"
        BrowserAuto->>ErrorCategorizer: CategorizeError(error)
        ErrorCategorizer-->>BrowserAuto: ErrorType: Browser
        
        BrowserAuto->>RetryManager: ShouldRetry(BrowserError, attempt=1)
        RetryManager-->>BrowserAuto: Retry: true (attempt 1/3)
        
        BrowserAuto->>BrowserAuto: Wait 2 seconds (exponential backoff)
        BrowserAuto->>Chrome: Try alternative Chrome path
        
        alt Alternative Browser Found
            Chrome-->>BrowserAuto: Browser ready
            BrowserAuto->>BrowserAuto: Continue export process
        else No Browser Available
            BrowserAuto->>FallbackRenderer: FallbackPDFGeneration()
            FallbackRenderer->>FallbackRenderer: Generate PDF using go-pdf
            FallbackRenderer-->>ExportSvc: Fallback PDF (reduced quality)
        end
    else Browser Navigation Failure
        Chrome-->>BrowserAuto: Error: "Page load timeout"
        BrowserAuto->>ErrorCategorizer: CategorizeError(error)
        ErrorCategorizer-->>BrowserAuto: ErrorType: Network
        
        BrowserAuto->>RetryManager: ShouldRetry(NetworkError, attempt=1)
        RetryManager-->>BrowserAuto: Retry: true (attempt 1/3)
        
        BrowserAuto->>BrowserAuto: Wait 4 seconds (exponential backoff)
        BrowserAuto->>Chrome: Retry navigation with longer timeout
        
        alt Retry Success
            Chrome-->>BrowserAuto: Page loaded
            BrowserAuto->>BrowserAuto: Continue export
        else Retry Failed
            BrowserAuto->>RetryManager: ShouldRetry(NetworkError, attempt=2)
            RetryManager-->>BrowserAuto: Retry: true (attempt 2/3)
            BrowserAuto->>BrowserAuto: Wait 8 seconds
            BrowserAuto->>Chrome: Final retry attempt
            
            alt Final Retry Success
                Chrome-->>BrowserAuto: Success
            else All Retries Exhausted
                BrowserAuto-->>ExportSvc: Error: "Export failed after 3 attempts"
            end
        end
    end
    
    Note over ExportSvc,FallbackRenderer: Export reliability: Retry logic + fallback rendering
```

### Export Format Fallback Strategy

```mermaid
sequenceDiagram
    participant User as User
    participant ExportSvc as ExportService
    participant PDFRenderer as PDF Renderer
    parameter HTMLRenderer as HTML Renderer
    participant ImageRenderer as Image Renderer
    participant FallbackChain as Fallback Chain
    participant ErrorAggregator as Error Aggregator
    
    User->>ExportSvc: Export presentation (PDF, HTML, Images)
    
    par PDF Export
        ExportSvc->>PDFRenderer: GeneratePDF()
        PDFRenderer->>PDFRenderer: Browser automation PDF
        alt PDF Generation Failed
            PDFRenderer-->>ExportSvc: Error: "PDF generation failed"
            ExportSvc->>FallbackChain: GetPDFFallback()
            FallbackChain->>FallbackChain: Use HTML → PDF conversion
            FallbackChain-->>ExportSvc: Fallback PDF generated
        else PDF Success
            PDFRenderer-->>ExportSvc: PDF ready
        end
    and HTML Export
        ExportSvc->>HTMLRenderer: GenerateHTML()
        HTMLRenderer->>HTMLRenderer: Extract full HTML + assets
        alt HTML Generation Failed
            HTMLRenderer-->>ExportSvc: Error: "HTML extraction failed"
            ExportSvc->>FallbackChain: GetHTMLFallback()
            FallbackChain->>FallbackChain: Use static HTML template
            FallbackChain-->>ExportSvc: Basic HTML generated
        else HTML Success
            HTMLRenderer-->>ExportSvc: HTML bundle ready
        end
    and Images Export
        ExportSvc->>ImageRenderer: GenerateImages()
        ImageRenderer->>ImageRenderer: Screenshot each slide
        alt Image Generation Failed
            ImageRenderer-->>ExportSvc: Error: "Screenshot failed"
            ExportSvc->>FallbackChain: GetImageFallback()
            FallbackChain->>FallbackChain: Use HTML canvas rendering
            FallbackChain-->>ExportSvc: Canvas-based images
        else Images Success
            ImageRenderer-->>ExportSvc: All slide images ready
        end
    end
    
    ExportSvc->>ErrorAggregator: CollectExportResults()
    ErrorAggregator->>ErrorAggregator: Aggregate success/failure status
    ErrorAggregator-->>ExportSvc: Export summary
    
    ExportSvc-->>User: Export complete (with fallback details if any)
    
    Note over User,ErrorAggregator: Comprehensive fallback: Ensures some export format always succeeds
```

## File System Error Handling

### File Watching Error Recovery

```mermaid
sequenceDiagram
    participant FileWatcher as File Watcher
    participant FileSystem as File System
    participant ErrorDetector as Error Detector
    participant RecoveryManager as Recovery Manager
    participant WebSocket as WebSocket Hub
    participant Clients as Connected Clients
    
    loop File Monitoring
        FileWatcher->>FileSystem: Poll watched files
        
        alt File System Error
            FileSystem-->>FileWatcher: Error: "Permission denied"
            FileWatcher->>ErrorDetector: FileSystemError
            ErrorDetector->>ErrorDetector: Categorize error severity
            
            alt Critical Error (file deleted)
                ErrorDetector->>RecoveryManager: HandleCriticalError()
                RecoveryManager->>RecoveryManager: Remove from watch list
                RecoveryManager->>WebSocket: Notify clients of file removal
                WebSocket->>Clients: File no longer available
            else Temporary Error (permission, lock)
                ErrorDetector->>RecoveryManager: HandleTemporaryError()
                RecoveryManager->>RecoveryManager: Wait and retry (exponential backoff)
                RecoveryManager->>FileWatcher: Retry file access
            else Network Error (NFS/SMB mount)
                ErrorDetector->>RecoveryManager: HandleNetworkError()
                RecoveryManager->>RecoveryManager: Switch to degraded mode
                RecoveryManager->>WebSocket: Notify clients of degraded service
            end
        else Checksum Calculation Error
            FileSystem->>FileWatcher: File content (with read errors)
            FileWatcher->>ErrorDetector: ChecksumError
            ErrorDetector->>ErrorDetector: Assess file corruption
            
            alt File Corrupted
                ErrorDetector->>RecoveryManager: HandleCorruption()
                RecoveryManager->>WebSocket: Notify file corruption
                WebSocket->>Clients: File may be corrupted
            else Partial Read
                ErrorDetector->>RecoveryManager: HandlePartialRead()
                RecoveryManager->>FileWatcher: Use file size/modtime only
            end
        else File Access Success
            FileSystem-->>FileWatcher: File metadata + content
            FileWatcher->>FileWatcher: Continue normal monitoring
        end
    end
    
    Note over FileWatcher,Clients: Resilient monitoring: Graceful degradation for file system issues
```

### Configuration Loading Error Handling

```mermaid
sequenceDiagram
    participant ConfigSvc as Config Service
    participant FileSystem as File System
    participant TOMLParser as TOML Parser
    participant ConfigValidator as Config Validator
    participant DefaultConfig as Default Config
    participant ErrorReporter as Error Reporter
    
    ConfigSvc->>FileSystem: Load config file
    
    alt Config File Missing
        FileSystem-->>ConfigSvc: Error: "File not found"
        ConfigSvc->>ErrorReporter: ConfigNotFound
        ErrorReporter->>ErrorReporter: Log missing config
        ConfigSvc->>DefaultConfig: LoadDefaults()
        DefaultConfig-->>ConfigSvc: Default configuration
    else Config File Corrupted
        FileSystem-->>ConfigSvc: Partial/corrupted content
        ConfigSvc->>TOMLParser: Parse configuration
        TOMLParser-->>ConfigSvc: Error: "Invalid TOML syntax"
        ConfigSvc->>ErrorReporter: ConfigCorrupted
        ErrorReporter->>ErrorReporter: Log corruption details
        ConfigSvc->>ConfigValidator: RepairConfig(partialConfig)
        
        ConfigValidator->>ConfigValidator: Extract valid sections
        ConfigValidator->>DefaultConfig: Merge with defaults
        ConfigValidator-->>ConfigSvc: Repaired configuration
    else Config Validation Failed
        FileSystem-->>ConfigSvc: Valid TOML content
        ConfigSvc->>TOMLParser: Parse configuration
        TOMLParser-->>ConfigSvc: Parsed config object
        ConfigSvc->>ConfigValidator: ValidateConfig(config)
        
        ConfigValidator->>ConfigValidator: Check required fields
        ConfigValidator->>ConfigValidator: Validate value ranges
        ConfigValidator->>ConfigValidator: Check dependencies
        
        alt Validation Errors Found
            ConfigValidator-->>ConfigSvc: ValidationErrors
            ConfigSvc->>ErrorReporter: ConfigInvalid
            ErrorReporter->>ErrorReporter: Log validation errors
            ConfigSvc->>ConfigValidator: ApplyDefaults(invalidFields)
            ConfigValidator-->>ConfigSvc: Config with defaults applied
        else Validation Success
            ConfigValidator-->>ConfigSvc: Valid configuration
        end
    end
    
    ConfigSvc-->>ConfigSvc: Configuration ready
    
    Note over ConfigSvc,ErrorReporter: Robust configuration: Always provides valid config through defaults and repair
```

## Network & Connectivity Error Handling

### HTTP Server Error Recovery

```mermaid
sequenceDiagram
    participant Client as Client
    participant HTTPServer as HTTP Server
    participant Middleware as Error Middleware
    participant ServiceLayer as Service Layer
    participant ErrorSanitizer as Error Sanitizer
    participant Logger as Request Logger
    participant Monitor as Health Monitor
    
    Client->>HTTPServer: HTTP Request
    HTTPServer->>Middleware: Process request
    Middleware->>ServiceLayer: Forward to service
    
    alt Service Layer Error
        ServiceLayer-->>Middleware: Internal error (with details)
        Middleware->>ErrorSanitizer: SanitizeError(error)
        ErrorSanitizer->>ErrorSanitizer: Remove sensitive information
        ErrorSanitizer->>ErrorSanitizer: Generate user-safe message
        ErrorSanitizer-->>Middleware: Safe error message
        
        Middleware->>Logger: Log full error details
        Logger->>Logger: Include request context, user agent, etc.
        
        Middleware->>Monitor: RecordError(errorType)
        Monitor->>Monitor: Update error metrics
        
        Middleware-->>Client: 500 {"error": "Internal server error"}
    else Validation Error
        ServiceLayer-->>Middleware: Validation error
        Middleware->>ErrorSanitizer: SanitizeValidationError(error)
        ErrorSanitizer-->>Middleware: User-friendly validation message
        Middleware->>Logger: Log validation failure
        Middleware-->>Client: 400 {"error": "Invalid input"}
    else Rate Limit Error
        Middleware->>Middleware: Check rate limits
        Middleware-->>Client: 429 {"error": "Too many requests"}
        Middleware->>Logger: Log rate limit hit
    else Service Success
        ServiceLayer-->>Middleware: Success response
        Middleware->>Logger: Log successful request
        Middleware-->>Client: 200 Success
    end
    
    Note over Client,Monitor: HTTP error handling: Secure error responses with comprehensive logging
```

### WebSocket Connection Error Handling

```mermaid
sequenceDiagram
    participant Client as Browser Client
    participant WebSocketServer as WebSocket Server
    participant ConnectionManager as Connection Manager
    participant HeartbeatMonitor as Heartbeat Monitor
    participant ErrorHandler as Error Handler
    participant ReconnectManager as Reconnect Manager
    
    Client->>WebSocketServer: WebSocket upgrade request
    WebSocketServer->>ConnectionManager: RegisterConnection(client)
    ConnectionManager->>HeartbeatMonitor: StartHeartbeat(client)
    
    loop Connection Monitoring
        HeartbeatMonitor->>Client: Ping
        
        alt Client Responds
            Client-->>HeartbeatMonitor: Pong
            HeartbeatMonitor->>HeartbeatMonitor: Reset timeout counter
        else Client Timeout
            HeartbeatMonitor->>ErrorHandler: ClientTimeout
            ErrorHandler->>ConnectionManager: RemoveConnection(client)
            ConnectionManager->>ConnectionManager: Clean up resources
        end
    end
    
    alt Connection Error
        Client-->>WebSocketServer: Connection error
        WebSocketServer->>ErrorHandler: ConnectionError
        ErrorHandler->>ErrorHandler: Log connection failure
        ErrorHandler->>ReconnectManager: InitiateReconnect()
        
        ReconnectManager->>ReconnectManager: Wait (exponential backoff)
        ReconnectManager->>Client: Attempt reconnection
        
        alt Reconnection Success
            Client->>WebSocketServer: New WebSocket connection
            WebSocketServer->>ConnectionManager: RegisterConnection(client)
            ReconnectManager-->>Client: Reconnected successfully
        else Reconnection Failed
            ReconnectManager->>ReconnectManager: Increase backoff delay
            ReconnectManager->>Client: Schedule next retry
        end
    else Forced Disconnection
        WebSocketServer->>ConnectionManager: ForceDisconnect(client)
        ConnectionManager->>Client: Close connection gracefully
        ConnectionManager->>ConnectionManager: Clean up resources
    end
    
    Note over Client,ReconnectManager: WebSocket resilience: Automatic reconnection with backoff
```

## Cache Error Handling

### Cache Corruption Recovery

```mermaid
sequenceDiagram
    participant CacheManager as Cache Manager
    participant PluginCache as Plugin Cache
    participant ThemeCache as Theme Cache
    participant CorruptionDetector as Corruption Detector
    participant CacheRepairer as Cache Repairer
    participant BackupManager as Backup Manager
    
    CacheManager->>PluginCache: Get(key)
    PluginCache->>CorruptionDetector: ValidateEntry(entry)
    
    alt Cache Entry Corrupted
        CorruptionDetector-->>PluginCache: CorruptionDetected
        PluginCache->>BackupManager: HasBackup(key)
        
        alt Backup Available
            BackupManager-->>PluginCache: Backup entry found
            PluginCache->>CacheRepairer: RestoreFromBackup(key)
            CacheRepairer-->>PluginCache: Entry restored
            PluginCache-->>CacheManager: Restored cache entry
        else No Backup Available
            PluginCache->>CacheRepairer: InvalidateEntry(key)
            CacheRepairer->>CacheRepairer: Remove corrupted entry
            CacheRepairer-->>PluginCache: Entry removed
            PluginCache-->>CacheManager: Cache miss (entry removed)
        end
    else Cache Memory Pressure
        PluginCache->>CorruptionDetector: CheckMemoryHealth()
        CorruptionDetector->>CorruptionDetector: Detect memory issues
        
        alt Memory Corruption Detected
            CorruptionDetector->>CacheRepairer: EmergencyCleanup()
            CacheRepairer->>PluginCache: ClearCache()
            CacheRepairer->>ThemeCache: ClearCache()
            CacheRepairer->>BackupManager: RestoreCriticalEntries()
            BackupManager-->>CacheRepairer: Critical entries restored
            CacheRepairer-->>CacheManager: Cache rebuilt
        end
    else Cache Entry Valid
        CorruptionDetector-->>PluginCache: Entry valid
        PluginCache-->>CacheManager: Cache hit
    end
    
    Note over CacheManager,BackupManager: Cache resilience: Corruption detection with backup restoration
```

## Key Error Handling Principles

**Graceful Degradation**:
- Plugin failures don't break presentations (fallback to original content)
- Export failures provide alternative formats
- File system issues trigger degraded monitoring mode

**Comprehensive Retry Logic**:
- Exponential backoff for network and browser errors
- Maximum retry attempts (typically 3) with increasing delays
- Error categorization to determine retry eligibility

**Fallback Strategies**:
- Alternative renderers when browser automation fails
- Default configurations when config files are missing/corrupt
- Passthrough plugins when specific plugins can't load

**Security-Focused Error Handling**:
- Error sanitization prevents information disclosure
- Full error details logged for debugging
- Rate limiting to prevent abuse

**Resource Protection**:
- Memory limits and timeouts for plugin execution
- Connection limits and heartbeat monitoring for WebSocket
- Cache corruption detection with automatic cleanup

**User Experience**:
- Non-blocking error recovery where possible
- Clear error messages without technical details
- Automatic reconnection for network issues

slicli's error handling ensures robust operation even when individual components fail, maintaining presentation generation capability through comprehensive fallback mechanisms.