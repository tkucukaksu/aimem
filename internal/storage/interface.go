package storage

import (
	"context"
	"github.com/tarkank/aimem/internal/types"
)

// Storage defines the interface for context storage implementations
type Storage interface {
	// StoreChunk stores a context chunk
	StoreChunk(ctx context.Context, chunk *types.ContextChunk) error
	
	// GetChunk retrieves a context chunk by ID
	GetChunk(ctx context.Context, chunkID string) (*types.ContextChunk, error)
	
	// SearchByEmbedding performs similarity search using embeddings
	SearchByEmbedding(ctx context.Context, sessionID string, queryEmbedding []float32, maxResults int) ([]*types.ContextChunk, error)
	
	// DeleteChunk removes a context chunk
	DeleteChunk(ctx context.Context, chunkID string) error
	
	// GetSessionSummary returns session statistics
	GetSessionSummary(ctx context.Context, sessionID string) (*types.SessionSummary, error)
	
	// CleanupByTTL removes expired chunks
	CleanupByTTL(ctx context.Context, sessionID string) (int, error)
	
	// CleanupByLRU removes least recently used chunks
	CleanupByLRU(ctx context.Context, sessionID string, keepCount int) (int, error)
	
	// CleanupByRelevance removes low relevance chunks
	CleanupByRelevance(ctx context.Context, sessionID string, minRelevance float64) (int, error)
	
	// CleanupSession removes all chunks for a session
	CleanupSession(ctx context.Context, sessionID string) error
	
	// Close closes the storage connection
	Close() error
}

// NewStorage creates a new storage instance based on configuration
func NewStorage(config *types.Config) (Storage, error) {
	switch config.Database {
	case "sqlite", "":
		return NewSQLiteStorage(&config.SQLite)
	case "redis":
		return NewRedisStorage(&config.Redis)
	default:
		// Default to SQLite for unknown database types
		return NewSQLiteStorage(&config.SQLite)
	}
}