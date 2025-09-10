# AIMem Best Practices Guide

This guide provides best practices for using AIMem effectively in various scenarios.

## 🎯 Session Management Best Practices

### 1. Project-Based Organization
```bash
# ✅ Good: Work within project directories
cd /path/to/your/project
# AIMem automatically detects project and creates appropriate session

# ❌ Avoid: Working from random directories
cd /tmp
# May create generic sessions without project context
```

### 2. Session Types Usage

#### Main Sessions
Use for primary development work:
```json
{
  "tool": "get_or_create_project_session",
  "working_dir": "/path/to/project"
}
```

#### Feature Sessions
Create for specific features:
```json
{
  "tool": "create_feature_session", 
  "parent_session_id": "project-main",
  "feature_name": "user-authentication"
}
```

#### Debug Sessions
Use for troubleshooting:
```json
{
  "tool": "create_feature_session",
  "parent_session_id": "project-main", 
  "feature_name": "debug-performance-issue"
}
```

### 3. Session Lifecycle Management
```yaml
# Configure automatic cleanup
session_manager:
  auto_cleanup_inactive: true
  inactive_threshold: 168h  # 1 week
  max_sessions_per_project: 10
```

## 💾 Context Storage Best Practices

### 1. Content Importance Levels

#### High Importance
Use for critical information that should never be removed:
```json
{
  "tool": "store_context",
  "content": "API authentication uses JWT tokens with 24h expiry",
  "importance": "high",
  "session_id": "session-id"
}
```

#### Medium Importance
Use for general development context:
```json
{
  "tool": "store_context", 
  "content": "User model has email, name, and created_at fields",
  "importance": "medium",
  "session_id": "session-id"
}
```

#### Low Importance
Use for temporary notes and debug information:
```json
{
  "tool": "store_context",
  "content": "Testing database connection - works locally",
  "importance": "low", 
  "session_id": "session-id"
}
```

### 2. Content Structure Best Practices

#### ✅ Good Context Structure
```json
{
  "content": "## API Endpoint: /api/users/login\n\n**Method**: POST\n**Authentication**: None required\n**Request Body**:\n```json\n{\n  \"email\": \"user@example.com\",\n  \"password\": \"secure123\"\n}\n```\n\n**Response**: JWT token in Authorization header",
  "importance": "high"
}
```

#### ❌ Poor Context Structure
```json
{
  "content": "login works post email password returns token",
  "importance": "medium"  
}
```

### 3. Context Retrieval Optimization

#### Task-Aware Retrieval
```json
{
  "tool": "context_aware_retrieve",
  "session_id": "session-id",
  "current_task": "Debug user authentication failing with 401 error",
  "task_type": "debugging",  // This helps focus retrieval
  "auto_expand": true,
  "max_chunks": 10
}
```

#### Specific Queries
```json
{
  "tool": "retrieve_context",
  "session_id": "session-id", 
  "query": "authentication JWT token validation middleware",  // Specific terms
  "max_chunks": 5
}
```

## 🚀 Performance Optimization

### 1. Memory Management Strategies

#### Conservative (Default)
Best for critical production environments:
```yaml
session_manager:
  auto_cleanup_inactive: false  # Manual cleanup only
  max_sessions_per_project: 20
```

#### Balanced  
Good for most development scenarios:
```yaml
session_manager:
  auto_cleanup_inactive: true
  inactive_threshold: 168h  # 1 week
  max_sessions_per_project: 10
```

#### Aggressive
For resource-constrained environments:
```yaml
session_manager:
  auto_cleanup_inactive: true
  inactive_threshold: 24h   # 1 day
  max_sessions_per_project: 5
```

### 2. Smart Memory Management
Use phase-appropriate strategies:

#### Development Phase
```json
{
  "tool": "smart_memory_manager",
  "session_id": "session-id",
  "session_phase": "development", 
  "memory_strategy": "balanced",
  "preserve_important": true
}
```

#### Testing Phase
```json
{
  "tool": "smart_memory_manager",
  "session_id": "session-id",
  "session_phase": "testing",
  "memory_strategy": "aggressive",  // Clean up dev context
  "preserve_important": true
}
```

### 3. Performance Monitoring
Regular monitoring for optimization:
```json
{
  "tool": "get_performance_metrics",
  "metric_type": "all"
}
```

## 🏗️ Project Organization Best Practices

### 1. Git Repository Structure
```bash
my-project/
├── .git/                    # AIMem detects this for project ID
├── package.json            # Workspace marker for Node.js
├── src/
├── tests/
└── docs/
```

### 2. Multi-Project Workspaces
```bash
workspace/
├── frontend/               # Separate AIMem session
│   ├── .git/
│   └── package.json
├── backend/                # Separate AIMem session  
│   ├── .git/
│   └── go.mod
└── shared/                 # May need manual session management
```

### 3. Custom Workspace Markers
```yaml
project_detector:
  custom_workspace_markers:
    - "my-project.yaml"
    - "workspace.config"
    - "Dockerfile"
```

## 🔧 Development Workflows

### 1. Feature Development Workflow
```bash
# 1. Start in project directory
cd /path/to/project

# 2. Create or get main session
# AIMem automatically detects project and creates session

# 3. Store initial context about the feature
{
  "tool": "store_context",
  "content": "Working on user registration feature. Need to implement email validation, password hashing, and user model creation.",
  "importance": "high"
}

# 4. Create feature-specific session if needed
{
  "tool": "create_feature_session",
  "parent_session_id": "main-session-id",
  "feature_name": "user-registration"
}

# 5. Work and store context as you go
# 6. Retrieve relevant context when stuck
# 7. Clean up when feature is complete
```

### 2. Debugging Workflow
```bash
# 1. Identify the issue
{
  "tool": "store_context",
  "content": "Bug: User login returns 500 error. Error message: 'Cannot read property email of undefined'. Occurs when user object is null.",
  "importance": "high"
}

# 2. Retrieve related context
{
  "tool": "context_aware_retrieve", 
  "current_task": "Debug login 500 error with null user object",
  "task_type": "debugging",
  "auto_expand": true
}

# 3. Store investigation findings
{
  "tool": "store_context",
  "content": "Found issue: Database query in getUserByEmail() returns null when email contains special characters. Need to fix SQL escaping.",
  "importance": "medium"
}

# 4. Store solution
{
  "tool": "store_context",
  "content": "Solution: Updated getUserByEmail() to use parameterized queries. Added input validation for email format. Issue resolved.",
  "importance": "high"
}
```

### 3. Code Review Workflow  
```bash
# Before review - summarize session
{
  "tool": "summarize_session",
  "session_id": "session-id"
}

# Store review feedback
{
  "tool": "store_context", 
  "content": "Code review feedback: Add error handling for database timeouts, improve variable naming in auth module, add unit tests for edge cases.",
  "importance": "high"
}
```

## 📊 Monitoring and Maintenance

### 1. Regular Health Checks
```bash
# Weekly performance check
{
  "tool": "get_performance_metrics",
  "metric_type": "system"
}

# Monthly cleanup
{
  "tool": "smart_memory_manager",
  "session_phase": "deployment", 
  "memory_strategy": "balanced"
}
```

### 2. Session Maintenance
```bash
# List all project sessions
{
  "tool": "list_project_sessions", 
  "project_id": "project-hash",
  "include_inactive": true
}

# Clean up inactive sessions
{
  "tool": "cleanup_session",
  "session_id": "old-session-id",
  "strategy": "ttl"
}
```

### 3. Debug and Troubleshooting
```bash
# Debug session issues
{
  "tool": "debug_session_state",
  "session_id": "problematic-session",
  "include_memory": true,
  "include_chunks": true
}

# Check for related sessions
{
  "tool": "discover_related_sessions",
  "working_dir": "/path/to/project"
}
```

## 🔒 Security Best Practices

### 1. Sensitive Information Handling
```json
// ❌ Don't store sensitive data
{
  "content": "Database password: secretpassword123",
  "importance": "high"
}

// ✅ Store references instead
{
  "content": "Database connection configured via environment variable DB_PASSWORD. Connection successful to production database.",
  "importance": "medium"
}
```

### 2. Context Sanitization
```json
// ✅ Sanitize before storing
{
  "content": "API call to /users endpoint failed. Authentication token format: Bearer <token>. Response: 401 Unauthorized", 
  "importance": "medium"
}
```

### 3. Access Control
```yaml
# Restrict session access
session_manager:
  enable_auto_detection: true  # Only detect in appropriate directories
  max_sessions_per_project: 10  # Limit session proliferation
```

## 📈 Performance Tuning

### 1. Embedding Configuration
```yaml
embedding:
  model: "sentence-transformers/all-MiniLM-L6-v2"  # Fast, good quality
  cache_size: 1000    # Adjust based on memory
  batch_size: 32      # Optimize for your use case
```

### 2. Storage Optimization  
```yaml
# For high-performance scenarios
performance:
  compression_enabled: true    # Reduce storage size
  async_processing: true      # Non-blocking operations
  cache_embeddings: true      # Speed up retrieval

# For memory-constrained scenarios  
memory:
  max_session_size: "50MB"    # Limit per session
  chunk_size: 1024           # Smaller chunks
  ttl_default: 12h           # Shorter retention
```

### 3. Database Tuning
```yaml
sqlite:
  max_connections: 20         # Higher for concurrent access
  connection_max_lifetime: 120 # Longer for persistent connections

# Or use Redis for high-performance scenarios
database: "redis"
redis:
  host: "localhost:6379" 
  pool_size: 20
```

## 🚫 Common Antipatterns

### 1. Session Management
```bash
# ❌ Creating too many sessions
# Don't create a new session for every small task

# ❌ Using generic session IDs
# Let AIMem generate project-based IDs automatically

# ❌ Never cleaning up sessions
# Configure automatic cleanup or use cleanup tools
```

### 2. Context Storage
```bash
# ❌ Storing everything as high importance
# Use appropriate importance levels

# ❌ Storing unstructured content
# Use markdown, clear headings, and organized format

# ❌ Storing duplicate information
# Check existing context before storing
```

### 3. Performance
```bash
# ❌ Retrieving too much context
# Use appropriate max_chunks limits

# ❌ Never monitoring performance
# Regular health checks prevent issues

# ❌ Ignoring memory usage warnings
# Use smart memory management tools
```

---

**Remember**: AIMem is designed to learn and adapt. These practices will help you get the most out of its intelligent features while maintaining optimal performance.