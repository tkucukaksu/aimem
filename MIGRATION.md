# AIMem Migration Guide

This guide helps you migrate from AIMem v1.x to v2.0 with intelligent session management.

## 🚨 Breaking Changes

### Session ID Format
- **v1.x**: Random UUIDs (e.g., `550e8400-e29b-41d4-a716-446655440000`)
- **v2.0**: Project-based deterministic IDs (e.g., `a1b2c3d4-main`, `a1b2c3d4-feature-e5f6`)

### Database Schema
- New `projects` table for project metadata
- New `sessions` table with foreign key relationships
- Enhanced `context_chunks` table with session references

### Configuration Changes
- New `session_manager` section
- New `project_detector` section
- Enhanced `performance` settings

## 🔄 Automatic Migration

AIMem v2.0 provides automatic migration for legacy sessions:

### 1. Enable Legacy Migration
```yaml
session_manager:
  enable_legacy_migration: true
```

### 2. Migration Process
When you access a legacy session, AIMem automatically:
1. Detects the legacy session format
2. Analyzes the current working directory for project info
3. Creates a new project-based session
4. Schedules background migration of context data

### 3. Using the Migration Tool
```json
{
  "tool": "resolve_session",
  "session_id_or_path": "550e8400-e29b-41d4-a716-446655440000"
}
```

## 📋 Manual Migration Steps

### Step 1: Backup Your Data
```bash
# Backup SQLite database
cp ~/.aimem/aimem.db ~/.aimem/aimem.db.backup

# Or backup Redis data if using Redis
redis-cli BGSAVE
```

### Step 2: Update Configuration
Create a new `config.yaml` with v2.0 format:

```yaml
# Previous v1.x config
database: "sqlite"
sqlite:
  database_path: "~/.aimem/aimem.db"

# New v2.0 config - add these sections:
session_manager:
  enable_auto_detection: true
  enable_legacy_migration: true
  default_session_type: "main"
  session_cache_size: 100
  session_timeout: 24h

project_detector:
  enable_caching: true
  cache_timeout: 10m
  git_detection_enabled: true
  workspace_detection_enabled: true
```

### Step 3: Test Migration
```bash
# Start AIMem v2.0 with legacy migration enabled
./aimem --config config.yaml

# Test with a legacy session ID
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "resolve_session",
      "arguments": {
        "session_id_or_path": "your-legacy-session-id"
      }
    }
  }'
```

## 🔍 Migration Verification

### Check Migration Status
Use the debug tool to verify migration:

```json
{
  "tool": "debug_session_state",
  "session_id": "new-session-id",
  "include_memory": true,
  "include_chunks": true
}
```

### Verify Project Detection
```json
{
  "tool": "get_or_create_project_session",
  "working_dir": "/path/to/your/project"
}
```

### Check Performance
```json
{
  "tool": "get_performance_metrics",
  "metric_type": "all"
}
```

## 🚀 Post-Migration Optimization

### 1. Clean Up Legacy Data
After successful migration, optionally clean up:

```sql
-- Check for orphaned legacy sessions
SELECT * FROM context_chunks 
WHERE session_id NOT IN (SELECT id FROM sessions);

-- Clean up after verification
DELETE FROM context_chunks 
WHERE session_id NOT IN (SELECT id FROM sessions)
AND timestamp < datetime('now', '-30 days');
```

### 2. Optimize Performance
Enable performance features:

```yaml
performance:
  enable_metrics: true
  async_processing: true
  cache_embeddings: true
```

### 3. Configure Session Management
```yaml
session_manager:
  max_sessions_per_project: 10
  auto_cleanup_inactive: true
  inactive_threshold: 168h  # 1 week
```

## 🛠️ Troubleshooting

### Common Migration Issues

#### Issue: Legacy Session Not Found
```
Error: could not resolve session: legacy-session-id
```

**Solutions**:
1. Check if the legacy database file exists
2. Ensure legacy migration is enabled
3. Verify the session ID format

#### Issue: Project Detection Failed
```
Error: failed to detect project
```

**Solutions**:
1. Ensure you're in a Git repository or workspace directory
2. Add custom workspace markers in config
3. Check ignore patterns aren't too restrictive

#### Issue: Permission Errors
```
Error: permission denied accessing database
```

**Solutions**:
1. Check file permissions on database
2. Ensure AIMem process has read/write access
3. Consider running with appropriate user permissions

### Debug Commands

#### Check Legacy Session Location
```bash
# Find legacy database files
find ~/.aimem -name "aimem_*.db" -type f

# Check database contents
sqlite3 ~/.aimem/aimem_legacy.db ".tables"
```

#### Verify New Schema
```bash
# Check new schema
sqlite3 ~/.aimem/aimem.db ".schema projects"
sqlite3 ~/.aimem/aimem.db ".schema sessions"
```

#### Monitor Migration Progress
```json
{
  "tool": "get_performance_metrics",
  "metric_type": "operation"
}
```

## 📊 Migration Performance

### Expected Timeline
- **Small sessions** (< 100 chunks): ~1-2 seconds
- **Medium sessions** (100-1000 chunks): ~5-30 seconds  
- **Large sessions** (1000+ chunks): ~1-5 minutes

### Memory Usage During Migration
- Temporary increase of ~2x normal usage
- Background processing to minimize impact
- Cleanup after completion

## ✅ Migration Checklist

- [ ] Backup existing data
- [ ] Update configuration file
- [ ] Enable legacy migration
- [ ] Test with legacy session IDs
- [ ] Verify project detection
- [ ] Check performance metrics
- [ ] Validate context retrieval
- [ ] Monitor for errors
- [ ] Optimize performance settings
- [ ] Clean up legacy data (optional)

## 🆘 Rollback Procedure

If migration fails, you can rollback:

### 1. Stop AIMem v2.0
```bash
# Stop the new version
pkill aimem
```

### 2. Restore Backup
```bash
# Restore database backup
cp ~/.aimem/aimem.db.backup ~/.aimem/aimem.db

# Or restore from Redis backup
redis-cli LASTSAVE  # Check backup timestamp
redis-cli DEBUG RESTART
```

### 3. Revert to v1.x
```bash
# Use previous AIMem version
./aimem-v1 --config old-config.yaml
```

## 🔗 Additional Resources

- [Configuration Reference](CONFIG.md)
- [API Documentation](API.md)
- [Best Practices](BEST_PRACTICES.md)
- [Troubleshooting Guide](TROUBLESHOOTING.md)

## 💬 Getting Help

If you encounter issues during migration:

1. Check the [troubleshooting section](#troubleshooting)
2. Review logs for detailed error messages
3. Use debug tools to inspect session state
4. Open an issue on [GitHub](https://github.com/yourusername/aimem/issues)
5. Join our [Discord community](https://discord.gg/aimem)

---

**Migration Support**: We're committed to making your migration smooth. Don't hesitate to reach out if you need assistance!