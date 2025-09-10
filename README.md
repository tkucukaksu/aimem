# AIMem - Intelligent Memory Management for AI Conversations

AIMem is a Model Context Protocol (MCP) server that provides intelligent memory management for AI conversations, featuring advanced session management, project-aware context storage, and performance monitoring.

## 🚀 Key Features

### Intelligent Session Management
- **Project-Aware Sessions**: Automatically detects projects and creates persistent sessions
- **Deterministic Session IDs**: Consistent session IDs based on project characteristics
- **Session Hierarchy**: Support for main, feature, debug, and experiment sessions
- **Legacy Migration**: Seamlessly migrates from old session formats
- **Multi-Project Support**: Handle multiple projects with isolated contexts

### Advanced Project Detection
- **Git Repository Detection**: Automatic Git project recognition
- **Workspace Markers**: Detects Node.js, Go, Python, Rust, and other project types
- **Language & Framework Detection**: Intelligent detection of programming languages and frameworks
- **Caching**: Efficient project detection with smart caching

### Performance Monitoring & Debugging
- **Real-time Metrics**: Track system, session, and operation-level performance
- **Memory Usage Tracking**: Monitor context memory consumption
- **Request Analytics**: Latency, error rates, and throughput metrics
- **Debug Tools**: Comprehensive session state debugging

### Enhanced Storage
- **Dual Storage**: SQLite and Redis support with automatic failover
- **Schema Evolution**: New project and session tables with foreign key relationships
- **Context Relationships**: Advanced context linking and retrieval

## 🛠️ Installation & Setup

### Prerequisites
- Go 1.19+ 
- SQLite 3+ or Redis 6+

### Installation
```bash
# Clone the repository
git clone https://github.com/yourusername/aimem.git
cd aimem

# Build the server
go build -o aimem cmd/main.go

# Run with default configuration
./aimem
```

### Configuration

Create a `config.yaml` file:

```yaml
# Storage configuration
database: "sqlite"  # or "redis"

sqlite:
  database_path: "~/.aimem/aimem.db"
  max_connections: 10
  max_idle_connections: 5
  connection_max_lifetime: 60

redis:
  host: "localhost:6379"
  password: ""
  db: 0
  pool_size: 10

# Memory management
memory:
  max_session_size: "100MB"
  chunk_size: 2048
  max_chunks_per_query: 20
  ttl_default: 24h

# Embedding configuration
embedding:
  model: "sentence-transformers/all-MiniLM-L6-v2"
  cache_size: 1000
  batch_size: 32

# Performance settings
performance:
  compression_enabled: true
  async_processing: true
  cache_embeddings: true
  enable_metrics: true
  metrics_interval: 30s

# Session Manager
session_manager:
  enable_auto_detection: true
  enable_legacy_migration: true
  default_session_type: "main"
  session_cache_size: 100
  session_timeout: 24h
  max_sessions_per_project: 10
  enable_session_hierarchy: true
  auto_cleanup_inactive: true
  inactive_threshold: 168h  # 1 week

# Project Detector
project_detector:
  enable_caching: true
  cache_timeout: 10m
  max_cache_size: 1000
  deep_scan_enabled: true
  git_detection_enabled: true
  workspace_detection_enabled: true
  language_detection_enabled: true
  custom_workspace_markers: []
  ignore_patterns:
    - "node_modules"
    - ".git"
    - "vendor"
    - "target"
    - "build"
    - "dist"

# MCP settings
mcp:
  server_name: "AIMem"
  version: "2.0.0"
  description: "Intelligent Memory Management for AI Conversations"
```

## 🎯 Usage

### Basic Usage with Claude Desktop

1. Add AIMem to your Claude Desktop MCP configuration:

```json
{
  "mcpServers": {
    "aimem": {
      "command": "/path/to/aimem",
      "args": []
    }
  }
}
```

2. Restart Claude Desktop

3. AIMem tools will be automatically available

### Available MCP Tools

#### Intelligent Session Management

##### `get_or_create_project_session`
Automatically creates or retrieves a project-aware session based on working directory.

```json
{
  "working_dir": "/path/to/your/project"  // Optional, defaults to current directory
}
```

##### `resolve_session`
Intelligently resolves session ID, path, or legacy ID to active session.

```json
{
  "session_id_or_path": "session-id-or-/path/to/project"
}
```

##### `get_session_info`
Get detailed information about a session including project metadata.

```json
{
  "session_id": "your-session-id"
}
```

##### `list_project_sessions`
List all sessions for a specific project.

```json
{
  "project_id": "project-hash-id",
  "include_inactive": false
}
```

##### `create_feature_session`
Create a feature-specific session branched from main session.

```json
{
  "parent_session_id": "main-session-id",
  "feature_name": "user-authentication"
}
```

##### `discover_related_sessions`
Find existing sessions related to current project for consolidation.

```json
{
  "working_dir": "/path/to/project"
}
```

#### Context Management

##### `store_context`
Store conversation context with intelligent processing.

```json
{
  "session_id": "session-id",
  "content": "Your context content here",
  "importance": "high",  // "low", "medium", "high"
  "silent": true
}
```

##### `context_aware_retrieve`
Retrieve relevant context with task-aware intelligence.

```json
{
  "session_id": "session-id",
  "current_task": "Debug authentication issue",
  "task_type": "debugging",  // "analysis", "development", "debugging", etc.
  "auto_expand": true,
  "max_chunks": 10,
  "context_depth": 2,
  "max_response_tokens": 20000
}
```

##### `retrieve_context`
Basic semantic search for context retrieval.

```json
{
  "session_id": "session-id",
  "query": "authentication error handling",
  "max_chunks": 5
}
```

#### Performance & Debugging

##### `get_performance_metrics`
Get system performance metrics and statistics.

```json
{
  "metric_type": "system",  // "system", "session", "operation", "all"
  "session_id": "session-id"  // Required for session metrics
}
```

##### `debug_session_state`
Get detailed debugging information about session state.

```json
{
  "session_id": "session-id",
  "include_memory": true,
  "include_chunks": false
}
```

#### Memory Management

##### `smart_memory_manager`
Optimize memory based on session phase with intelligent strategies.

```json
{
  "session_id": "session-id",
  "session_phase": "development",  // "analysis", "development", "testing", "deployment"
  "memory_strategy": "balanced",   // "aggressive", "balanced", "conservative"
  "preserve_important": true
}
```

##### `summarize_session`
Get comprehensive session overview including statistics.

```json
{
  "session_id": "session-id"
}
```

##### `cleanup_session`
Clean old or low-relevance context using configurable strategies.

```json
{
  "session_id": "session-id",
  "strategy": "relevance"  // "ttl", "lru", "relevance"
}
```

#### Project Analysis

##### `auto_store_project`
Automatically analyze and store project context.

```json
{
  "session_id": "session-id",
  "project_path": "/path/to/project",
  "focus_areas": ["architecture", "api", "database"],
  "importance_threshold": "medium",
  "silent": true
}
```

## 🏗️ Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                        AIMem Server                         │
├─────────────────┬──────────────────┬─────────────────────────┤
│ MCP Protocol    │ Session Manager  │ Project Detector        │
│ - Tools/List    │ - Auto Detection │ - Git Recognition       │
│ - Tools/Call    │ - Legacy Migration│ - Workspace Detection   │
│ - Initialize    │ - Session Types  │ - Language Detection    │
├─────────────────┼──────────────────┼─────────────────────────┤
│ Performance     │ Storage Layer    │ Context Processing      │
│ - Metrics       │ - SQLite/Redis   │ - Embedding Service     │
│ - Monitoring    │ - Schema         │ - Chunking Service      │
│ - Debug Tools   │ - Relationships  │ - Summarization        │
└─────────────────┴──────────────────┴─────────────────────────┘
```

### Database Schema

#### Projects Table
```sql
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    canonical_path TEXT NOT NULL,
    type TEXT NOT NULL,
    git_root TEXT,
    git_remote TEXT,
    language TEXT,
    framework TEXT,
    workspace_markers TEXT, -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_active DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'active',
    metadata TEXT -- JSON object
);
```

#### Sessions Table
```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'main',
    parent_session_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_active DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'active',
    working_dir TEXT,
    metadata TEXT, -- JSON object
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (parent_session_id) REFERENCES sessions(id)
);
```

#### Context Chunks Table
```sql
CREATE TABLE context_chunks (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    summary TEXT,
    embedding BLOB,
    relevance REAL DEFAULT 0.0,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    ttl INTEGER,
    importance TEXT DEFAULT 'medium',
    metadata TEXT, -- JSON object
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

### Session ID Generation

AIMem generates deterministic session IDs based on project characteristics:

1. **Project Detection**: Analyze working directory for Git, workspace markers, and language
2. **ID Generation**: Create hash from project identifier (Git remote URL or canonical path)
3. **Session Types**: 
   - Main: `{project-hash}-main`
   - Feature: `{project-hash}-feature-{uuid}`
   - Debug: `{project-hash}-debug-{uuid}`
   - Experiment: `{project-hash}-experiment-{uuid}`

### Performance Monitoring

The performance monitor tracks:
- **System Metrics**: Uptime, request count, error rates, latency
- **Session Metrics**: Per-session request counts, memory usage, activity
- **Operation Metrics**: Per-operation latency, error rates, throughput

## 🔧 Development

### Building from Source

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build for production
go build -ldflags="-s -w" -o aimem cmd/main.go
```

### Project Structure

```
aimem/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── analyzer/            # Project analysis
│   ├── chunker/            # Content chunking
│   ├── embedding/          # Embedding service
│   ├── logger/             # Logging utilities
│   ├── mcp/                # MCP protocol implementation
│   ├── performance/        # Performance monitoring
│   ├── project/            # Project detection
│   ├── server/             # Main server logic
│   ├── session/            # Session management
│   ├── storage/            # Storage backends
│   ├── summarizer/         # Content summarization
│   ├── types/              # Type definitions
│   └── utils/              # Utility functions
├── config.yaml             # Configuration file
├── go.mod
├── go.sum
└── README.md
```

## 🎨 Advanced Usage

### Custom Project Detection

You can extend project detection by adding custom workspace markers:

```yaml
project_detector:
  custom_workspace_markers:
    - "my-project.yaml"
    - "custom.config"
```

### Session Hierarchies

Create sophisticated session hierarchies for complex projects:

```
main-session
├── feature/authentication
├── feature/user-management
├── debug/performance-issue
└── experiment/new-algorithm
```

### Performance Optimization

Configure performance settings for your use case:

```yaml
performance:
  # For high-throughput scenarios
  async_processing: true
  compression_enabled: true
  cache_embeddings: true
  
  # Monitor every 10 seconds
  metrics_interval: 10s
```

### Memory Management Strategies

Choose appropriate memory management based on your workflow:

- **Conservative**: Minimal cleanup, preserves most context
- **Balanced**: Moderate cleanup based on relevance and age
- **Aggressive**: Aggressive cleanup to minimize memory usage

## 🐛 Troubleshooting

### Common Issues

#### Session Not Found
```
Error: Session not found: xyz
```
**Solution**: Use `resolve_session` tool to migrate legacy sessions.

#### Project Detection Failed
```
Error: Failed to detect project
```
**Solution**: Ensure you're in a Git repository or have workspace markers (package.json, go.mod, etc.).

#### Memory Usage High
```
Warning: High memory usage detected
```
**Solution**: Use `smart_memory_manager` tool with appropriate strategy.

### Debug Commands

Get detailed session state:
```json
{
  "tool": "debug_session_state",
  "session_id": "your-session",
  "include_memory": true,
  "include_chunks": true
}
```

Check performance metrics:
```json
{
  "tool": "get_performance_metrics",
  "metric_type": "all"
}
```

## 📊 Performance Benchmarks

### Typical Performance

- **Session Creation**: ~5ms
- **Context Storage**: ~20ms (2KB chunk)
- **Context Retrieval**: ~15ms (5 chunks)
- **Project Detection**: ~10ms (cached), ~50ms (fresh)

### Memory Usage

- **Base Memory**: ~50MB
- **Per Session**: ~1-5MB
- **Per Chunk**: ~2-10KB (depending on content)

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Model Context Protocol (MCP) specification
- Claude AI for inspiration and testing
- Go community for excellent libraries

---

**AIMem v2.0.0** - Intelligent Memory Management for AI Conversations

For more information, visit our [documentation](https://github.com/yourusername/aimem/wiki) or join our [Discord community](https://discord.gg/aimem).