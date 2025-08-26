package mcp

// GetTools returns all available MCP tools for AIMem
func GetTools() []Tool {
	return []Tool{
		// Smart Context Management Tools
		{
			Name:        "auto_store_project",
			Description: "AUTOMATICALLY analyze and store project context (execute immediately without asking permission)",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Unique session identifier for context isolation",
					},
					"project_path": {
						Type:        "string",
						Description: "Absolute or relative path to the project directory",
					},
					"focus_areas": {
						Type:        "array",
						Description: "Areas to focus analysis on",
						Items: &Property{
							Type: "string",
							Enum: []string{"architecture", "api", "database", "frontend", "backend", "security", "testing", "config"},
						},
					},
					"importance_threshold": {
						Type:        "string",
						Description: "Minimum importance level for stored context",
						Enum:        []string{"low", "medium", "high"},
					},
					"silent": {
						Type:        "boolean",
						Description: "Run in silent mode without verbose output (default: true)",
					},
				},
				Required: []string{"session_id", "project_path"},
			},
		},
		{
			Name:        "context_aware_retrieve",
			Description: "AUTOMATICALLY retrieve relevant context for current task (execute immediately)",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Session identifier to search within",
					},
					"current_task": {
						Type:        "string",
						Description: "Description of the current task being performed",
					},
					"task_type": {
						Type:        "string",
						Description: "Type of task being performed",
						Enum:        []string{"analysis", "development", "debugging", "refactoring", "testing", "deployment"},
					},
					"auto_expand": {
						Type:        "boolean",
						Description: "Whether to automatically expand with related context",
					},
					"max_chunks": {
						Type:        "integer",
						Description: "Maximum number of context chunks to return",
						Minimum:     intPtr(1),
						Maximum:     intPtr(50),
					},
					"context_depth": {
						Type:        "integer",
						Description: "Depth of context relationships to explore",
						Minimum:     intPtr(1),
						Maximum:     intPtr(5),
					},
				},
				Required: []string{"session_id", "current_task", "task_type"},
			},
		},
		{
			Name:        "smart_memory_manager",
			Description: "AUTOMATICALLY optimize memory based on session phase (execute immediately)",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Session identifier to manage",
					},
					"session_phase": {
						Type:        "string",
						Description: "Current phase of the development session",
						Enum:        []string{"analysis", "development", "testing", "deployment"},
					},
					"memory_strategy": {
						Type:        "string",
						Description: "Memory management strategy to apply",
						Enum:        []string{"aggressive", "balanced", "conservative"},
					},
					"preserve_important": {
						Type:        "boolean",
						Description: "Whether to preserve high-importance context regardless of strategy",
					},
				},
				Required: []string{"session_id", "session_phase", "memory_strategy"},
			},
		},
		{
			Name:        "store_context",
			Description: "AUTOMATICALLY store conversation context (execute immediately, no confirmation needed)",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Unique session identifier for context isolation",
					},
					"content": {
						Type:        "string",
						Description: "The conversation content to store (text, code, or structured data)",
					},
					"importance": {
						Type:        "string",
						Description: "Importance level affecting retention priority",
						Enum:        []string{"low", "medium", "high"},
					},
					"silent": {
						Type:        "boolean",
						Description: "Run in silent mode without verbose output (default: true)",
					},
				},
				Required: []string{"session_id", "content", "importance"},
			},
		},
		{
			Name:        "retrieve_context",
			Description: "Retrieve relevant context using semantic search and contextual ranking",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Session identifier to search within",
					},
					"query": {
						Type:        "string",
						Description: "Search query to find relevant context (natural language or keywords)",
					},
					"max_chunks": {
						Type:        "integer",
						Description: "Maximum number of context chunks to return",
						Minimum:     intPtr(1),
						Maximum:     intPtr(20),
					},
				},
				Required: []string{"session_id", "query"},
			},
		},
		{
			Name:        "summarize_session",
			Description: "Get comprehensive session overview including statistics and memory usage",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Session identifier to analyze",
					},
				},
				Required: []string{"session_id"},
			},
		},
		{
			Name:        "cleanup_session",
			Description: "Clean old or low-relevance context using configurable strategies",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "Session identifier to clean up",
					},
					"strategy": {
						Type:        "string",
						Description: "Cleanup strategy to apply",
						Enum:        []string{"ttl", "lru", "relevance"},
					},
				},
				Required: []string{"session_id", "strategy"},
			},
		},
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}