package types

import (
	"time"
)

// Importance levels for context chunks
type Importance string

const (
	ImportanceLow    Importance = "low"
	ImportanceMedium Importance = "medium"
	ImportanceHigh   Importance = "high"
)

// Cleanup strategies for memory management
type CleanupStrategy string

const (
	CleanupTTL       CleanupStrategy = "ttl"
	CleanupLRU       CleanupStrategy = "lru"
	CleanupRelevance CleanupStrategy = "relevance"
)

// Task types for context-aware operations
type TaskType string

const (
	TaskAnalysis    TaskType = "analysis"
	TaskDevelopment TaskType = "development"
	TaskDebugging   TaskType = "debugging"
	TaskRefactoring TaskType = "refactoring"
	TaskTesting     TaskType = "testing"
	TaskDeployment  TaskType = "deployment"
)

// Session phases for smart memory management
type SessionPhase string

const (
	PhaseAnalysis    SessionPhase = "analysis"
	PhaseDevelopment SessionPhase = "development"
	PhaseTesting     SessionPhase = "testing"
	PhaseDeployment  SessionPhase = "deployment"
)

// Memory management strategies
type MemoryStrategy string

const (
	MemoryAggressive   MemoryStrategy = "aggressive"
	MemoryBalanced     MemoryStrategy = "balanced"
	MemoryConservative MemoryStrategy = "conservative"
)

// Focus areas for project analysis
type FocusArea string

const (
	FocusArchitecture FocusArea = "architecture"
	FocusAPI          FocusArea = "api"
	FocusDatabase     FocusArea = "database"
	FocusFrontend     FocusArea = "frontend"
	FocusBackend      FocusArea = "backend"
	FocusSecurity     FocusArea = "security"
	FocusTesting      FocusArea = "testing"
	FocusConfig       FocusArea = "config"
)

// Project analysis result
type ProjectAnalysis struct {
	ProjectPath    string      `json:"project_path"`
	Language       string      `json:"language"`
	Framework      string      `json:"framework"`
	Architecture   string      `json:"architecture"`
	Dependencies   []string    `json:"dependencies"`
	EntryPoints    []string    `json:"entry_points"`
	ConfigFiles    []string    `json:"config_files"`
	DatabaseSchema []string    `json:"database_schema"`
	APIEndpoints   []string    `json:"api_endpoints"`
	KeyFiles       []string    `json:"key_files"`
	Complexity     float64     `json:"complexity_score"`
	FocusAreas     []FocusArea `json:"focus_areas"`
	AnalyzedAt     time.Time   `json:"analyzed_at"`
	StoredChunks   []string    `json:"stored_chunks"`
}

// Context relationship for smart retrieval
type ContextRelationship struct {
	ChunkID       string   `json:"chunk_id"`
	RelatedChunks []string `json:"related_chunks"`
	Strength      float64  `json:"relationship_strength"`
	Reason        string   `json:"relationship_reason"`
}

// Smart retrieval request
type SmartRetrievalRequest struct {
	SessionID    string   `json:"session_id"`
	CurrentTask  string   `json:"current_task"`
	TaskType     TaskType `json:"task_type"`
	AutoExpand   bool     `json:"auto_expand"`
	MaxChunks    int      `json:"max_chunks"`
	ContextDepth int      `json:"context_depth"`
}

// Smart retrieval result
type SmartRetrievalResult struct {
	PrimaryChunks    []ContextChunk        `json:"primary_chunks"`
	RelatedChunks    []ContextChunk        `json:"related_chunks"`
	Relationships    []ContextRelationship `json:"relationships"`
	RetrievalReason  string                `json:"retrieval_reason"`
	TotalRelevance   float64               `json:"total_relevance"`
	ProcessingTimeMs int64                 `json:"processing_time_ms"`
}

// ContextChunk represents a semantic piece of conversation context
type ContextChunk struct {
	ID         string        `json:"id" redis:"id"`
	SessionID  string        `json:"session_id" redis:"session_id"`
	Content    string        `json:"content" redis:"content"`
	Summary    string        `json:"summary" redis:"summary"`
	Embedding  []float32     `json:"embedding" redis:"embedding"`
	Relevance  float64       `json:"relevance" redis:"relevance"`
	Timestamp  time.Time     `json:"timestamp" redis:"timestamp"`
	TTL        time.Duration `json:"ttl" redis:"ttl"`
	Importance Importance    `json:"importance" redis:"importance"`
}

// SessionStats provides session-level statistics
type SessionStats struct {
	SessionID        string    `json:"session_id"`
	ChunkCount       int       `json:"chunk_count"`
	TotalSize        int64     `json:"total_size_bytes"`
	LastActivity     time.Time `json:"last_activity"`
	CreatedAt        time.Time `json:"created_at"`
	MemoryUsage      int64     `json:"memory_usage_bytes"`
	AverageRelevance float64   `json:"average_relevance"`
}

// RetrievalResult contains retrieved context with relevance scoring
type RetrievalResult struct {
	Chunks     []ContextChunk `json:"chunks"`
	TotalScore float64        `json:"total_score"`
	QueryTime  time.Duration  `json:"query_time_ms"`
}

// SessionSummary provides session statistics
type SessionSummary struct {
	SessionID        string    `json:"session_id"`
	ChunkCount       int       `json:"chunk_count"`
	MemoryUsage      int64     `json:"memory_usage"`
	AverageRelevance float64   `json:"average_relevance"`
	CreatedAt        time.Time `json:"created_at"`
	LastActivity     time.Time `json:"last_activity"`
}

// Config holds all configuration for AIMem server
type Config struct {
	Database        string                `yaml:"database"` // "sqlite" or "redis"
	Redis           RedisConfig           `yaml:"redis"`
	SQLite          SQLiteConfig          `yaml:"sqlite"`
	Memory          MemoryConfig          `yaml:"memory"`
	Embedding       EmbeddingConfig       `yaml:"embedding"`
	Performance     PerformanceConfig     `yaml:"performance"`
	MCP             MCPConfig             `yaml:"mcp"`
	SessionManager  SessionManagerConfig  `yaml:"session_manager"`
	ProjectDetector ProjectDetectorConfig `yaml:"project_detector"`
}

// RedisConfig contains Redis connection settings
type RedisConfig struct {
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// SQLiteConfig contains SQLite database settings
type SQLiteConfig struct {
	DatabasePath          string `yaml:"database_path"`
	MaxConnections        int    `yaml:"max_connections"`
	MaxIdleConnections    int    `yaml:"max_idle_connections"`
	ConnectionMaxLifetime int    `yaml:"connection_max_lifetime"` // in minutes
}

// MemoryConfig contains memory management settings
type MemoryConfig struct {
	MaxSessionSize    string        `yaml:"max_session_size"`
	ChunkSize         int           `yaml:"chunk_size"`
	MaxChunksPerQuery int           `yaml:"max_chunks_per_query"`
	TTLDefault        time.Duration `yaml:"ttl_default"`
}

// EmbeddingConfig contains embedding service settings
type EmbeddingConfig struct {
	Model     string `yaml:"model"`
	CacheSize int    `yaml:"cache_size"`
	BatchSize int    `yaml:"batch_size"`
}

// PerformanceConfig contains performance tuning settings
type PerformanceConfig struct {
	CompressionEnabled bool          `yaml:"compression_enabled"`
	AsyncProcessing    bool          `yaml:"async_processing"`
	CacheEmbeddings    bool          `yaml:"cache_embeddings"`
	EnableMetrics      bool          `yaml:"enable_metrics"`
	MetricsInterval    time.Duration `yaml:"metrics_interval"`
}

// MCPConfig contains MCP protocol settings
type MCPConfig struct {
	ServerName  string `yaml:"server_name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

// Response pagination and size limiting
type ResponsePaging struct {
	PageSize      int    `json:"page_size"`
	CurrentPage   int    `json:"current_page"`
	TotalPages    int    `json:"total_pages"`
	TotalItems    int    `json:"total_items"`
	HasMore       bool   `json:"has_more"`
	NextPageToken string `json:"next_page_token,omitempty"`
}

// TokenLimits for response size control
type TokenLimits struct {
	MaxResponseTokens int  `json:"max_response_tokens"` // Maximum tokens in response
	EstimatedTokens   int  `json:"estimated_tokens"`    // Current estimated token count
	TruncatedContent  bool `json:"truncated_content"`   // Whether content was truncated
}

// Response size configuration
type ResponseConfig struct {
	MaxTokens       int  `json:"max_tokens"`       // Default: 20000 (below 25000 limit)
	EnablePaging    bool `json:"enable_paging"`    // Enable pagination for large results
	PageSize        int  `json:"page_size"`        // Items per page
	TruncateContent bool `json:"truncate_content"` // Truncate individual chunks if needed
}

// PaginatedRetrievalResult with size limiting
type PaginatedRetrievalResult struct {
	PrimaryChunks    []ContextChunk        `json:"primary_chunks"`
	RelatedChunks    []ContextChunk        `json:"related_chunks"`
	Relationships    []ContextRelationship `json:"relationships"`
	RetrievalReason  string                `json:"retrieval_reason"`
	TotalRelevance   float64               `json:"total_relevance"`
	ProcessingTimeMs int64                 `json:"processing_time_ms"`
	Paging           *ResponsePaging       `json:"paging,omitempty"`
	TokenLimits      TokenLimits           `json:"token_limits"`
}

// Project and Session Management Types

// ProjectInfo contains detected project information
type ProjectInfo struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	CanonicalPath    string        `json:"canonical_path"`
	Type             ProjectType   `json:"type"`
	GitRoot          *string       `json:"git_root,omitempty"`
	GitRemote        *string       `json:"git_remote,omitempty"`
	Language         string        `json:"language"`
	Framework        string        `json:"framework"`
	WorkspaceMarkers []string      `json:"workspace_markers"`
	CreatedAt        time.Time     `json:"created_at"`
	LastActive       time.Time     `json:"last_active"`
	Status           ProjectStatus `json:"status"`
}

// ProjectType represents the type of project detected
type ProjectType string

const (
	ProjectTypeGitRepository ProjectType = "git"
	ProjectTypeWorkspace     ProjectType = "workspace"
	ProjectTypeDirectory     ProjectType = "directory"
	ProjectTypeMonorepo      ProjectType = "monorepo"
)

// ProjectStatus represents project lifecycle status
type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
	ProjectStatusMigrated ProjectStatus = "migrated"
)

// SessionInfo represents a memory session with project context
type SessionInfo struct {
	ID              string                 `json:"id"`
	ProjectID       string                 `json:"project_id"`
	Name            string                 `json:"name"`
	Type            SessionType            `json:"type"`
	ParentSessionID *string                `json:"parent_session_id,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	LastActive      time.Time              `json:"last_active"`
	Status          SessionStatus          `json:"status"`
	Metadata        map[string]interface{} `json:"metadata"`
	WorkingDir      string                 `json:"working_dir"`
}

// SessionType represents different types of sessions
type SessionType string

// SessionStatus represents session lifecycle status
type SessionStatus string

const (
	SessionTypeMain       SessionType = "main"
	SessionTypeFeature    SessionType = "feature"
	SessionTypeDebug      SessionType = "debug"
	SessionTypeExperiment SessionType = "experiment"
	SessionTypeMigration  SessionType = "migration"

	SessionStatusActive   SessionStatus = "active"
	SessionStatusMerged   SessionStatus = "merged"
	SessionStatusArchived SessionStatus = "archived"
)

// SessionManagerConfig holds intelligent session management configuration
type SessionManagerConfig struct {
	EnableAutoDetection    bool          `yaml:"enable_auto_detection" json:"enable_auto_detection"`
	EnableLegacyMigration  bool          `yaml:"enable_legacy_migration" json:"enable_legacy_migration"`
	DefaultSessionType     SessionType   `yaml:"default_session_type" json:"default_session_type"`
	SessionCacheSize       int           `yaml:"session_cache_size" json:"session_cache_size"`
	SessionTimeout         time.Duration `yaml:"session_timeout" json:"session_timeout"`
	MaxSessionsPerProject  int           `yaml:"max_sessions_per_project" json:"max_sessions_per_project"`
	EnableSessionHierarchy bool          `yaml:"enable_session_hierarchy" json:"enable_session_hierarchy"`
	AutoCleanupInactive    bool          `yaml:"auto_cleanup_inactive" json:"auto_cleanup_inactive"`
	InactiveThreshold      time.Duration `yaml:"inactive_threshold" json:"inactive_threshold"`
}

// ProjectDetectorConfig holds project detection configuration
type ProjectDetectorConfig struct {
	EnableCaching             bool          `yaml:"enable_caching" json:"enable_caching"`
	CacheTimeout              time.Duration `yaml:"cache_timeout" json:"cache_timeout"`
	MaxCacheSize              int           `yaml:"max_cache_size" json:"max_cache_size"`
	DeepScanEnabled           bool          `yaml:"deep_scan_enabled" json:"deep_scan_enabled"`
	GitDetectionEnabled       bool          `yaml:"git_detection_enabled" json:"git_detection_enabled"`
	WorkspaceDetectionEnabled bool          `yaml:"workspace_detection_enabled" json:"workspace_detection_enabled"`
	LanguageDetectionEnabled  bool          `yaml:"language_detection_enabled" json:"language_detection_enabled"`
	CustomWorkspaceMarkers    []string      `yaml:"custom_workspace_markers" json:"custom_workspace_markers"`
	IgnorePatterns            []string      `yaml:"ignore_patterns" json:"ignore_patterns"`
}

// DefaultConfig returns a default configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Database: "sqlite",
		Redis: RedisConfig{
			Host:     "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
		SQLite: SQLiteConfig{
			DatabasePath:          "~/.aimem/aimem.db",
			MaxConnections:        10,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 60,
		},
		Memory: MemoryConfig{
			MaxSessionSize:    "100MB",
			ChunkSize:         2048,
			MaxChunksPerQuery: 20,
			TTLDefault:        24 * time.Hour,
		},
		Embedding: EmbeddingConfig{
			Model:     "sentence-transformers/all-MiniLM-L6-v2",
			CacheSize: 1000,
			BatchSize: 32,
		},
		Performance: PerformanceConfig{
			CompressionEnabled: true,
			AsyncProcessing:    true,
			CacheEmbeddings:    true,
			EnableMetrics:      true,
			MetricsInterval:    30 * time.Second,
		},
		MCP: MCPConfig{
			ServerName:  "AIMem",
			Version:     "1.0.0",
			Description: "Intelligent Memory Management for AI Conversations",
		},
		SessionManager: SessionManagerConfig{
			EnableAutoDetection:    true,
			EnableLegacyMigration:  true,
			DefaultSessionType:     SessionTypeMain,
			SessionCacheSize:       100,
			SessionTimeout:         24 * time.Hour,
			MaxSessionsPerProject:  10,
			EnableSessionHierarchy: true,
			AutoCleanupInactive:    true,
			InactiveThreshold:      7 * 24 * time.Hour, // 1 week
		},
		ProjectDetector: ProjectDetectorConfig{
			EnableCaching:             true,
			CacheTimeout:              10 * time.Minute,
			MaxCacheSize:              1000,
			DeepScanEnabled:           true,
			GitDetectionEnabled:       true,
			WorkspaceDetectionEnabled: true,
			LanguageDetectionEnabled:  true,
			CustomWorkspaceMarkers:    []string{},
			IgnorePatterns: []string{
				"node_modules",
				".git",
				"vendor",
				"target",
				"build",
				"dist",
				".next",
				".nuxt",
				"__pycache__",
				".pytest_cache",
				".vscode",
				".idea",
			},
		},
	}
}
