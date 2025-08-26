package mcp

import (
	"encoding/json"
)

// MCP Protocol types and structures

// Request represents an MCP JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC 2.0 response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents an MCP JSON-RPC 2.0 error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard MCP error codes
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
)

// MCP Tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema Schema      `json:"inputSchema"`
}

// Schema represents JSON Schema for tool parameters
type Schema struct {
	Type       string            `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string          `json:"required"`
}

// Property represents a JSON Schema property
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Enum        []string  `json:"enum,omitempty"`
	Minimum     *int      `json:"minimum,omitempty"`
	Maximum     *int      `json:"maximum,omitempty"`
	Items       *Property `json:"items,omitempty"`
}

// Tool call parameters for each AIMem tool

// StoreContextParams represents parameters for store_context tool
type StoreContextParams struct {
	SessionID  string `json:"session_id"`
	Content    string `json:"content"`
	Importance string `json:"importance"`
}

// RetrieveContextParams represents parameters for retrieve_context tool
type RetrieveContextParams struct {
	SessionID string `json:"session_id"`
	Query     string `json:"query"`
	MaxChunks int    `json:"max_chunks"`
}

// SummarizeSessionParams represents parameters for summarize_session tool
type SummarizeSessionParams struct {
	SessionID string `json:"session_id"`
}

// CleanupSessionParams represents parameters for cleanup_session tool
type CleanupSessionParams struct {
	SessionID string `json:"session_id"`
	Strategy  string `json:"strategy"`
}

// Tool call results

// StoreContextResult represents the result of storing context
type StoreContextResult struct {
	ChunkID   string `json:"chunk_id"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ChunkSize int    `json:"chunk_size_bytes"`
}

// RetrieveContextResult represents the result of retrieving context
type RetrieveContextResult struct {
	Chunks     []ContextChunk `json:"chunks"`
	TotalScore float64        `json:"total_score"`
	QueryTime  int64          `json:"query_time_ms"`
	HitCount   int            `json:"hit_count"`
}

// ContextChunk for MCP response (simplified)
type ContextChunk struct {
	ID        string  `json:"id"`
	Content   string  `json:"content"`
	Summary   string  `json:"summary"`
	Relevance float64 `json:"relevance"`
	Timestamp string  `json:"timestamp"`
}

// SummarizeSessionResult represents session summary and statistics
type SummarizeSessionResult struct {
	SessionID        string  `json:"session_id"`
	ChunkCount       int     `json:"chunk_count"`
	TotalSize        int64   `json:"total_size_bytes"`
	MemoryUsage      int64   `json:"memory_usage_bytes"`
	LastActivity     string  `json:"last_activity"`
	CreatedAt        string  `json:"created_at"`
	AverageRelevance float64 `json:"average_relevance"`
}

// CleanupSessionResult represents the result of cleanup operation
type CleanupSessionResult struct {
	Success        bool   `json:"success"`
	ChunksRemoved  int    `json:"chunks_removed"`
	BytesFreed     int64  `json:"bytes_freed"`
	Strategy       string `json:"strategy"`
	RemainingChunks int   `json:"remaining_chunks"`
}

// NewError creates a new MCP Error
func NewError(code int, message string, data interface{}) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewResponse creates a new MCP Response
func NewResponse(id interface{}, result interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates a new MCP Error Response
func NewErrorResponse(id interface{}, err *Error) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}
}

// ParseRequest parses a JSON-RPC request
func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}