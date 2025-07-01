# slicli System Sequence Diagrams

Visual documentation of system interactions, data flows, and business processes for the slicli CLI-based slide presentation generator.

## 📋 Diagram Categories

### 🔌 API Interactions
- [API Flows](api-flows.md) - REST endpoint sequences and WebSocket communication
- [Security Validation Flows](auth-flows.md) - Input sanitization and security validation sequences
- [Error Flows](error-flows.md) - Error handling and recovery mechanisms

### 💾 Data Processing
- [Data Flows](data-flows.md) - Markdown processing, plugin execution, and theme loading
- [System Interactions](system-interactions.md) - Clean architecture layer communication

### 🏢 Business Processes  
- [Business Flows](business-flows.md) - Complete development workflows and export processes

## 🎯 slicli-Specific Patterns

**Clean Architecture (Hexagonal)**: All flows respect dependency inversion with domain layer isolation
**Plugin System**: Go shared objects (.so) with sandboxed execution and caching (85% hit rate)
**Theme System**: TOML-based configuration with CSS processing and caching (95% hit rate)
**Export System**: Chrome/Chromium headless automation with retry logic and fallback
**Live Reload**: File watching with SHA256 checksums and WebSocket notifications

## 🔍 Key Performance Characteristics

- **Plugin Cache**: LRU + TTL + Size (85% hit rate, O(n) eviction complexity)
- **Theme Cache**: LRU + TTL (95% hit rate, count-based eviction)
- **Plugin Execution**: Sequential processing (650ms current, 300ms potential with concurrency)
- **File Watching**: Polling-based with checksum validation (optimization opportunity)
- **Export Performance**: Multi-format with browser automation and retry logic

## 🛠️ Architecture Components

**Primary Adapters (Driving)**:
- CLI Interface (Cobra commands)
- HTTP Server (Gorilla Mux + WebSocket)
- Web Interface (Live reload)

**Domain Services**:
- PresentationService (orchestrator)
- PluginService (execution management)
- ThemeService (theme processing)
- ConfigService (configuration)
- LiveReloadService (file watching)

**Secondary Adapters (Driven)**:
- PluginLoader (Go shared objects)
- ThemeLoader (TOML + CSS)
- ExportService (browser automation)
- FileWatcher (change detection)

## 🔍 How to Read These Diagrams

- **Participants**: Actual slicli components (services, caches, external systems)
- **Messages**: Real API calls, plugin executions, file operations
- **Activations**: Processing time including cache operations and sandboxing
- **Notes**: Performance characteristics, cache hit rates, and optimization opportunities
- **Error Paths**: Actual retry logic, fallback mechanisms, and recovery sequences

## 🛠️ Updating Diagrams

When system architecture changes:
1. Update participant names to match current service interfaces
2. Verify cache hit rates and performance characteristics
3. Add new plugin interactions or export formats
4. Update error handling sequences with new retry logic
5. Sync with code changes in internal/domain/services/ and internal/adapters/

Generated from slicli codebase analysis (Phase 1: Sequence Diagram Visualization) - keep synchronized with actual Go implementation.