package storage

import (
	"context"

	"github.com/tarkank/aimem/internal/types"
)

// Storage defines the interface for context storage implementations
type Storage interface {
	// Context Chunk Operations
	StoreChunk(ctx context.Context, chunk *types.ContextChunk) error
	GetChunk(ctx context.Context, chunkID string) (*types.ContextChunk, error)
	SearchByEmbedding(ctx context.Context, sessionID string, queryEmbedding []float32, maxResults int) ([]*types.ContextChunk, error)
	DeleteChunk(ctx context.Context, chunkID string) error

	// Session Operations (Legacy)
	GetSessionSummary(ctx context.Context, sessionID string) (*types.SessionSummary, error)
	CleanupByTTL(ctx context.Context, sessionID string) (int, error)
	CleanupByLRU(ctx context.Context, sessionID string, keepCount int) (int, error)
	CleanupByRelevance(ctx context.Context, sessionID string, minRelevance float64) (int, error)
	CleanupSession(ctx context.Context, sessionID string) error

	// Project Management
	CreateProject(ctx context.Context, project *types.ProjectInfo) error
	GetProject(ctx context.Context, projectID string) (*types.ProjectInfo, error)
	UpdateProject(ctx context.Context, project *types.ProjectInfo) error
	ListActiveProjects(ctx context.Context) ([]*types.ProjectInfo, error)

	// Session Management
	CreateSession(ctx context.Context, session *types.SessionInfo) error
	GetSession(ctx context.Context, sessionID string) (*types.SessionInfo, error)
	UpdateSession(ctx context.Context, session *types.SessionInfo) error
	GetProjectSessions(ctx context.Context, projectID string) ([]*types.SessionInfo, error)
	ListActiveSessions(ctx context.Context) ([]*types.SessionInfo, error)

	// Legacy Support
	ListLegacyDatabases(ctx context.Context) ([]string, error)

	// Connection Management
	Close() error
}

// NewStorage creates a new storage instance based on configuration
func NewStorage(config *types.Config) (Storage, error) {
	switch config.Database {
	case "sqlite", "":
		return NewSQLiteStorage(&config.SQLite)
	case "redis":
		// TODO: Implement full RedisStorage with new interface methods
		return NewSQLiteStorage(&config.SQLite) // Fallback to SQLite for now
	default:
		// Default to SQLite for unknown database types
		return NewSQLiteStorage(&config.SQLite)
	}
}
