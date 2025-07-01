# Security Validation & Protection Flows

Security validation sequences for slicli (local CLI tool without traditional authentication).

## Input Sanitization Flow

### HTML Content Sanitization (BlueMonday)

```mermaid
sequenceDiagram
    participant Client as Client Input
    participant HTTPServer as HTTP Server
    participant Sanitizer as BlueMonday Sanitizer
    participant PresentationSvc as PresentationService
    participant PluginSvc as PluginService
    
    Client->>HTTPServer: Request with user content
    HTTPServer->>HTTPServer: Basic input validation
    HTTPServer->>PresentationSvc: Process content
    
    PresentationSvc->>PluginSvc: Execute plugins on markdown
    PluginSvc-->>PresentationSvc: Raw plugin output (potentially unsafe HTML)
    
    PresentationSvc->>Sanitizer: Sanitize HTML content
    
    Sanitizer->>Sanitizer: Remove script tags
    Sanitizer->>Sanitizer: Filter dangerous attributes (onclick, etc.)
    Sanitizer->>Sanitizer: Whitelist safe HTML elements
    Sanitizer->>Sanitizer: Validate URLs and links
    
    Sanitizer-->>PresentationSvc: Clean, safe HTML
    PresentationSvc-->>HTTPServer: Sanitized content
    HTTPServer-->>Client: Safe response
    
    Note over Client,PluginSvc: XSS Protection: All user/plugin content sanitized
```

## Plugin Sandboxing & Security

### Plugin Execution Sandboxing

```mermaid
sequenceDiagram
    participant PluginSvc as PluginService
    participant PluginLoader as PluginLoader
    participant Sandbox as Execution Sandbox
    participant Plugin as Go Plugin (.so)
    participant Monitor as Resource Monitor
    participant Timeout as Timeout Controller
    
    PluginSvc->>PluginLoader: LoadPlugin(pluginPath)
    PluginLoader->>PluginLoader: Path validation (prevent traversal)
    PluginLoader->>PluginLoader: Load shared object (.so file)
    
    PluginSvc->>Sandbox: ExecutePlugin(input, limits)
    
    Sandbox->>Monitor: Set memory limit (default: 100MB)
    Sandbox->>Timeout: Set execution timeout (default: 30s)
    
    par
        Sandbox->>Plugin: Execute(ctx, input)
        Plugin->>Plugin: Process input content
    and
        Monitor->>Monitor: Track memory usage
        alt Memory Limit Exceeded
            Monitor->>Sandbox: Kill plugin execution
            Sandbox-->>PluginSvc: Error: "Memory limit exceeded"
        end
    and
        Timeout->>Timeout: Monitor execution time
        alt Timeout Exceeded
            Timeout->>Sandbox: Cancel context
            Sandbox-->>PluginSvc: Error: "Execution timeout"
        end
    end
    
    Plugin-->>Sandbox: Plugin output
    Sandbox->>Sandbox: Validate output format
    Sandbox-->>PluginSvc: Safe plugin result
    
    Note over PluginSvc,Timeout: Sandboxed execution prevents resource exhaustion
```

### Code Execution Plugin Security

```mermaid
sequenceDiagram
    participant User as User Markdown
    participant CodeExecPlugin as Code Execution Plugin
    participant SecurityCheck as Security Validator
    participant Sandbox as Code Sandbox
    participant Shell as Shell Environment
    
    User->>CodeExecPlugin: Code block (```bash, ```go, etc.)
    CodeExecPlugin->>SecurityCheck: Validate code content
    
    SecurityCheck->>SecurityCheck: Check for dangerous commands
    SecurityCheck->>SecurityCheck: Validate language whitelist
    SecurityCheck->>SecurityCheck: Check resource requirements
    
    alt Dangerous Code Detected
        SecurityCheck-->>CodeExecPlugin: Error: "Unsafe code blocked"
        CodeExecPlugin-->>User: Safe error message
    else Code Safe for Execution
        SecurityCheck-->>CodeExecPlugin: Validation passed
        
        CodeExecPlugin->>Sandbox: Execute in isolated environment
        Sandbox->>Sandbox: Set working directory restrictions
        Sandbox->>Sandbox: Limit network access
        Sandbox->>Sandbox: Set memory/CPU limits
        
        Sandbox->>Shell: Execute code
        Shell-->>Sandbox: Execution result
        Sandbox->>Sandbox: Sanitize output
        Sandbox-->>CodeExecPlugin: Safe execution output
        CodeExecPlugin-->>User: Formatted result
    end
    
    Note over User,Shell: Code execution: Sandboxed with resource limits
```

## File System Security

### Path Validation & Traversal Prevention

```mermaid
sequenceDiagram
    participant Client as Client Request
    participant HTTPServer as HTTP Server
    participant PathValidator as Path Validator
    participant FileSystem as File System
    participant SecurityLogger as Security Logger
    
    Client->>HTTPServer: File operation request
    HTTPServer->>PathValidator: ValidateFilePath(path)
    
    PathValidator->>PathValidator: Check for ".." patterns
    PathValidator->>PathValidator: Validate against directory traversal
    PathValidator->>PathValidator: Clean path normalization
    PathValidator->>PathValidator: Verify path within allowed directories
    
    alt Path Traversal Detected
        PathValidator->>SecurityLogger: Log security violation
        PathValidator-->>HTTPServer: Error: "Invalid path"
        HTTPServer-->>Client: 400 "Bad Request"
    else Path Valid
        PathValidator-->>HTTPServer: Path validated
        HTTPServer->>FileSystem: Safe file operation
        FileSystem-->>HTTPServer: File content/result
        HTTPServer-->>Client: 200 Success
    end
    
    Note over Client,SecurityLogger: Security: Prevents access outside allowed directories
```

### Export File Security

```mermaid
sequenceDiagram
    participant ExportSvc as ExportService
    participant PathValidator as Path Validator
    participant TempManager as Temp File Manager
    participant Cleanup as Cleanup Service
    participant FileSystem as File System
    
    ExportSvc->>PathValidator: ValidateOutputPath(outputPath)
    PathValidator->>PathValidator: Validate export directory
    PathValidator->>PathValidator: Check write permissions
    PathValidator-->>ExportSvc: Path validation result
    
    ExportSvc->>TempManager: CreateSecureTempFile(prefix, extension)
    TempManager->>TempManager: Generate unique filename
    TempManager->>TempManager: Set secure file permissions (0600)
    TempManager->>FileSystem: Create temporary file
    FileSystem-->>TempManager: Secure temp file
    
    TempManager-->>ExportSvc: Temp file path
    ExportSvc->>ExportSvc: Generate export content
    ExportSvc->>FileSystem: Write to secure temp file
    
    ExportSvc->>Cleanup: ScheduleCleanup(filePath, TTL)
    Cleanup->>Cleanup: Set 24-hour cleanup timer
    
    Note over ExportSvc,FileSystem: Temporary files: Secure permissions, automatic cleanup
```

## CORS & Origin Validation

### WebSocket Origin Validation

```mermaid
sequenceDiagram
    participant Client as Browser Client
    participant WebSocketServer as WebSocket Server
    participant OriginValidator as Origin Validator
    participant ConfigSvc as Config Service
    participant SecurityLogger as Security Logger
    
    Client->>WebSocketServer: WebSocket upgrade request
    Note right of Client: Origin: http://localhost:3000
    
    WebSocketServer->>OriginValidator: CheckOrigin(request)
    OriginValidator->>ConfigSvc: GetAllowedOrigins()
    ConfigSvc-->>OriginValidator: ["localhost", "127.0.0.1"] (dev mode: all)
    
    alt Development Mode
        OriginValidator->>SecurityLogger: Log: "Dev mode - allowing all origins"
        OriginValidator-->>WebSocketServer: Allow connection
    else Production Mode
        OriginValidator->>OriginValidator: Validate origin against whitelist
        alt Origin Not Allowed
            OriginValidator->>SecurityLogger: Log security violation
            OriginValidator-->>WebSocketServer: Reject connection
            WebSocketServer-->>Client: 403 Forbidden
        else Origin Allowed
            OriginValidator-->>WebSocketServer: Allow connection
        end
    end
    
    WebSocketServer-->>Client: WebSocket connection established
    
    Note over Client,SecurityLogger: CORS: Configurable origin validation (dev vs prod)
```

### HTTP CORS Header Validation

```mermaid
sequenceDiagram
    participant Client as Browser Client
    participant CORSMiddleware as CORS Middleware
    participant ConfigSvc as Config Service
    participant HTTPServer as HTTP Server
    
    Client->>CORSMiddleware: HTTP Request with Origin header
    CORSMiddleware->>ConfigSvc: GetCORSConfig()
    ConfigSvc-->>CORSMiddleware: CORS configuration
    
    CORSMiddleware->>CORSMiddleware: Validate request origin
    CORSMiddleware->>CORSMiddleware: Check allowed methods
    CORSMiddleware->>CORSMiddleware: Validate headers
    
    alt Preflight Request (OPTIONS)
        CORSMiddleware->>CORSMiddleware: Set Access-Control-Allow headers
        CORSMiddleware-->>Client: 200 OK (preflight response)
    else Actual Request
        CORSMiddleware->>CORSMiddleware: Set CORS response headers
        CORSMiddleware->>HTTPServer: Forward request
        HTTPServer-->>CORSMiddleware: Response
        CORSMiddleware-->>Client: Response with CORS headers
    end
    
    Note over Client,HTTPServer: CORS: Proper header validation for browser security
```

## Resource Protection

### Rate Limiting & Resource Protection

```mermaid
sequenceDiagram
    participant Client as Client
    participant RateLimiter as Rate Limiter
    participant ResourceMonitor as Resource Monitor
    participant HTTPServer as HTTP Server
    participant ServiceLayer as Service Layer
    
    Client->>RateLimiter: HTTP Request
    RateLimiter->>RateLimiter: Check request rate (per IP)
    
    alt Rate Limit Exceeded
        RateLimiter-->>Client: 429 Too Many Requests
    else Rate OK
        RateLimiter->>ResourceMonitor: Check system resources
        ResourceMonitor->>ResourceMonitor: Check memory usage
        ResourceMonitor->>ResourceMonitor: Check CPU load
        ResourceMonitor->>ResourceMonitor: Check active exports
        
        alt Resources Exhausted
            ResourceMonitor-->>Client: 503 Service Unavailable
        else Resources Available
            ResourceMonitor->>HTTPServer: Forward request
            HTTPServer->>ServiceLayer: Process request
            ServiceLayer-->>HTTPServer: Response
            HTTPServer-->>Client: Success response
        end
    end
    
    Note over Client,ServiceLayer: Protection: Rate limiting and resource monitoring
```

## Security Configuration

### Security Headers & Configuration

```mermaid
sequenceDiagram
    participant Client as Browser Client
    participant SecurityMiddleware as Security Middleware
    participant ConfigSvc as Config Service
    participant HTTPServer as HTTP Server
    
    Client->>SecurityMiddleware: HTTP Request
    SecurityMiddleware->>ConfigSvc: GetSecurityConfig()
    ConfigSvc-->>SecurityMiddleware: Security configuration
    
    SecurityMiddleware->>SecurityMiddleware: Add security headers
    Note over SecurityMiddleware: X-Content-Type-Options: nosniff
    Note over SecurityMiddleware: X-Frame-Options: DENY
    Note over SecurityMiddleware: X-XSS-Protection: 1; mode=block
    Note over SecurityMiddleware: Content-Security-Policy: default-src 'self'
    
    SecurityMiddleware->>HTTPServer: Request with security context
    HTTPServer-->>SecurityMiddleware: Response
    SecurityMiddleware->>SecurityMiddleware: Add response security headers
    SecurityMiddleware-->>Client: Secure response
    
    Note over Client,HTTPServer: Security: Comprehensive security headers
```

## Key Security Notes

**Input Validation**:
- All user input sanitized with BlueMonday HTML sanitizer
- Path validation prevents directory traversal attacks  
- Plugin code execution sandboxed with memory/time limits

**Resource Protection**:
- Plugin execution: 100MB memory limit, 30-second timeout
- File operations: Restricted to safe directories
- Export files: Secure temp directory with auto-cleanup

**Network Security**:
- CORS validation configurable (dev vs production)
- WebSocket origin checking (permissive in dev mode)
- Security headers added to all responses

**Process Isolation**:
- Go plugin system with sandboxed execution
- Browser automation in separate processes
- Resource monitoring and rate limiting

**File System Security**:
- Temporary files with secure permissions (0600)
- Automatic cleanup after 24 hours
- Path validation for all file operations

slicli prioritizes security for a local development tool while maintaining usability for presentation development workflows.