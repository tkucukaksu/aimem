# AIMem Response Size Limiting System

## Overview

AIMem now includes an advanced response size limiting system that prevents MCP tool responses from exceeding Claude Code's 25,000 token limit. This system ensures reliable operation and prevents tool failures due to oversized responses.

## Features

### 1. Token Estimation
- **Smart Token Calculation**: Uses approximate 4 characters per token with content-type adjustments
- **Code-Aware**: Recognizes code content and adjusts token estimates accordingly
- **JSON Overhead**: Accounts for JSON structure overhead (~20% more tokens than raw content)

### 2. Response Size Limiting
- **Configurable Token Limits**: Default 20,000 tokens (safety margin below 25K limit)
- **Dynamic Content Allocation**: 60% for primary chunks, 30% for related chunks, 10% for relationships
- **Iterative Budget Management**: Builds response progressively within token constraints

### 3. Pagination Support
- **Automatic Pagination**: Splits large results across multiple pages
- **Page Navigation**: Supports page parameter for retrieving specific pages
- **Smart Page Sizing**: Configurable page sizes based on content and token budget

### 4. Content Truncation
- **Chunk-Level Truncation**: Individual chunks can be truncated to fit budget
- **Word Boundary Preservation**: Truncation occurs at word boundaries when possible
- **Truncation Indicators**: Clear markers when content has been truncated

## Configuration

The system can be configured through `ResponseConfig`:

```go
type ResponseConfig struct {
    MaxTokens       int  // Maximum tokens in response (default: 20000)
    EnablePaging    bool // Enable pagination (default: true)
    PageSize        int  // Items per page (default: varies by context)
    TruncateContent bool // Allow content truncation (default: true)
}
```

## Usage in context_aware_retrieve

The enhanced `context_aware_retrieve` tool now includes these new parameters:

- `max_response_tokens` (integer, 1000-24000): Maximum tokens allowed in response
- `page` (integer, ≥1): Page number for paginated results
- `enable_pagination` (boolean): Enable/disable pagination

### Example Usage

```json
{
  "name": "context_aware_retrieve",
  "arguments": {
    "session_id": "user_session_123",
    "current_task": "Implement user authentication",
    "task_type": "development",
    "max_chunks": 20,
    "auto_expand": true,
    "max_response_tokens": 15000,
    "page": 1,
    "enable_pagination": true
  }
}
```

## Response Structure

The enhanced response includes metadata about token usage and pagination:

```json
{
  "content": [{
    "type": "text",
    "text": "🎯 **Context-Aware Retrieval**: ...\n**Token Budget**: 15000 (Estimated: 12453)\n**Page**: 1 of 3 (Total items: 45)\n⚠️ **Content was truncated to fit token limits**\n..."
  }]
}
```

## Token Budget Allocation

1. **Base Overhead**: ~500-1000 tokens for JSON structure and metadata
2. **Primary Chunks**: 60% of available token budget
3. **Related Chunks**: 30% of available token budget
4. **Relationships**: 10% of available token budget
5. **Safety Buffer**: 200 tokens reserved for unforeseen overhead

## Error Prevention

The system prevents the original error:
```
Error: MCP tool "context_aware_retrieve" response (1416538 tokens) exceeds maximum allowed tokens (25000)
```

By:
- Enforcing strict token budgets
- Providing pagination for large datasets
- Truncating content when necessary
- Including clear indicators when content is limited

## Implementation Benefits

1. **Reliability**: Prevents MCP tool failures due to oversized responses
2. **Performance**: Reduces token usage and improves response times
3. **Usability**: Provides pagination for navigating large result sets
4. **Transparency**: Clear indication of token usage and content limitation
5. **Configurability**: Adjustable limits based on specific use cases

## Testing

Run the test suite to verify functionality:

```bash
go run test_response_limiting.go
```

This test suite validates:
- Token estimation accuracy
- Response size limiting
- Pagination functionality
- Content truncation behavior

## Future Enhancements

- Adaptive token estimation based on actual response measurements
- Content importance-based prioritization for truncation decisions
- Streaming responses for very large datasets
- Content compression for better token utilization