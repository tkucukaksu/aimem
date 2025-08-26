package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tarkank/aimem/internal/types"
)

// SQLiteStorage implements SQLite-based storage for AIMem
type SQLiteStorage struct {
	db     *sql.DB
	config *types.SQLiteConfig
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(config *types.SQLiteConfig) (*SQLiteStorage, error) {
	// Open SQLite database
	db, err := sql.Open("sqlite3", config.DatabasePath+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=10000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %v", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(time.Duration(config.ConnectionMaxLifetime) * time.Minute)

	storage := &SQLiteStorage{
		db:     db,
		config: config,
	}

	// Test connection and initialize schema
	if err := storage.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize SQLite storage: %v", err)
	}

	return storage, nil
}

// initialize creates the database schema
func (s *SQLiteStorage) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping SQLite database: %v", err)
	}

	// Create schema
	schema := `
	CREATE TABLE IF NOT EXISTS context_chunks (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		content TEXT NOT NULL,
		summary TEXT,
		embedding BLOB,
		relevance REAL DEFAULT 1.0,
		importance TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		ttl DATETIME,
		UNIQUE(id)
	);

	CREATE INDEX IF NOT EXISTS idx_session_id ON context_chunks(session_id);
	CREATE INDEX IF NOT EXISTS idx_relevance ON context_chunks(relevance DESC);
	CREATE INDEX IF NOT EXISTS idx_importance ON context_chunks(importance);
	CREATE INDEX IF NOT EXISTS idx_created_at ON context_chunks(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_ttl ON context_chunks(ttl);

	-- Session statistics table
	CREATE TABLE IF NOT EXISTS session_stats (
		session_id TEXT PRIMARY KEY,
		chunk_count INTEGER DEFAULT 0,
		memory_usage INTEGER DEFAULT 0,
		average_relevance REAL DEFAULT 1.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_activity DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Triggers to update session stats automatically
	CREATE TRIGGER IF NOT EXISTS update_session_stats_insert
	AFTER INSERT ON context_chunks
	BEGIN
		INSERT OR REPLACE INTO session_stats (
			session_id, 
			chunk_count, 
			memory_usage,
			average_relevance,
			created_at,
			last_activity
		)
		SELECT 
			NEW.session_id,
			COUNT(*),
			SUM(LENGTH(content)),
			AVG(relevance),
			MIN(created_at),
			MAX(updated_at)
		FROM context_chunks 
		WHERE session_id = NEW.session_id;
	END;

	CREATE TRIGGER IF NOT EXISTS update_session_stats_delete
	AFTER DELETE ON context_chunks
	BEGIN
		UPDATE session_stats 
		SET 
			chunk_count = (SELECT COUNT(*) FROM context_chunks WHERE session_id = OLD.session_id),
			memory_usage = (SELECT COALESCE(SUM(LENGTH(content)), 0) FROM context_chunks WHERE session_id = OLD.session_id),
			average_relevance = (SELECT COALESCE(AVG(relevance), 0) FROM context_chunks WHERE session_id = OLD.session_id),
			last_activity = CURRENT_TIMESTAMP
		WHERE session_id = OLD.session_id;
		
		-- Delete session stats if no chunks remain
		DELETE FROM session_stats 
		WHERE session_id = OLD.session_id 
		AND NOT EXISTS (SELECT 1 FROM context_chunks WHERE session_id = OLD.session_id);
	END;
	`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to create schema: %v", err)
	}

	return nil
}

// Close closes the SQLite connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// StoreChunk stores a context chunk in SQLite
func (s *SQLiteStorage) StoreChunk(ctx context.Context, chunk *types.ContextChunk) error {
	// Serialize embedding as JSON
	var embeddingData []byte
	if chunk.Embedding != nil {
		var err error
		embeddingData, err = json.Marshal(chunk.Embedding)
		if err != nil {
			return fmt.Errorf("failed to serialize embedding: %v", err)
		}
	}

	// Calculate TTL timestamp
	var ttlTime *time.Time
	if chunk.TTL > 0 {
		ttl := time.Now().Add(chunk.TTL)
		ttlTime = &ttl
	}

	query := `
		INSERT OR REPLACE INTO context_chunks 
		(id, session_id, content, summary, embedding, relevance, importance, created_at, updated_at, ttl)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		chunk.ID,
		chunk.SessionID,
		chunk.Content,
		chunk.Summary,
		embeddingData,
		chunk.Relevance,
		string(chunk.Importance),
		chunk.Timestamp,
		time.Now(),
		ttlTime,
	)

	if err != nil {
		return fmt.Errorf("failed to store chunk: %v", err)
	}

	return nil
}

// GetChunk retrieves a context chunk by ID
func (s *SQLiteStorage) GetChunk(ctx context.Context, chunkID string) (*types.ContextChunk, error) {
	query := `
		SELECT id, session_id, content, summary, embedding, relevance, importance, created_at, ttl
		FROM context_chunks 
		WHERE id = ? AND (ttl IS NULL OR ttl > CURRENT_TIMESTAMP)
	`

	row := s.db.QueryRowContext(ctx, query, chunkID)
	
	var chunk types.ContextChunk
	var embeddingData []byte
	var importance string
	var ttlTime sql.NullTime

	err := row.Scan(
		&chunk.ID,
		&chunk.SessionID,
		&chunk.Content,
		&chunk.Summary,
		&embeddingData,
		&chunk.Relevance,
		&importance,
		&chunk.Timestamp,
		&ttlTime,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", chunkID)
		}
		return nil, fmt.Errorf("failed to get chunk: %v", err)
	}

	// Deserialize embedding
	if len(embeddingData) > 0 {
		if err := json.Unmarshal(embeddingData, &chunk.Embedding); err != nil {
			return nil, fmt.Errorf("failed to deserialize embedding: %v", err)
		}
	}

	// Convert importance
	chunk.Importance = types.Importance(importance)

	// Convert TTL
	if ttlTime.Valid {
		chunk.TTL = time.Until(ttlTime.Time)
	}

	return &chunk, nil
}

// SearchByEmbedding performs similarity search using embeddings
func (s *SQLiteStorage) SearchByEmbedding(ctx context.Context, sessionID string, queryEmbedding []float32, maxResults int) ([]*types.ContextChunk, error) {
	// First get all chunks for the session
	query := `
		SELECT id, session_id, content, summary, embedding, relevance, importance, created_at, ttl
		FROM context_chunks 
		WHERE session_id = ? AND (ttl IS NULL OR ttl > CURRENT_TIMESTAMP)
		ORDER BY relevance DESC, created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %v", err)
	}
	defer rows.Close()

	var candidates []*types.ContextChunk
	
	for rows.Next() {
		var chunk types.ContextChunk
		var embeddingData []byte
		var importance string
		var ttlTime sql.NullTime

		err := rows.Scan(
			&chunk.ID,
			&chunk.SessionID,
			&chunk.Content,
			&chunk.Summary,
			&embeddingData,
			&chunk.Relevance,
			&importance,
			&chunk.Timestamp,
			&ttlTime,
		)

		if err != nil {
			continue
		}

		// Deserialize embedding
		if len(embeddingData) > 0 {
			if err := json.Unmarshal(embeddingData, &chunk.Embedding); err != nil {
				continue
			}
		}

		chunk.Importance = types.Importance(importance)

		if ttlTime.Valid {
			chunk.TTL = time.Until(ttlTime.Time)
		}

		candidates = append(candidates, &chunk)
	}

	// Calculate similarities and sort
	type chunkWithSimilarity struct {
		chunk      *types.ContextChunk
		similarity float64
	}

	var results []chunkWithSimilarity
	
	for _, chunk := range candidates {
		if chunk.Embedding == nil {
			continue
		}
		
		similarity := cosineSimilaritySQLite(queryEmbedding, chunk.Embedding)
		results = append(results, chunkWithSimilarity{
			chunk:      chunk,
			similarity: similarity,
		})
	}

	// Sort by similarity (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].similarity > results[i].similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Extract top results
	var finalResults []*types.ContextChunk
	limit := maxResults
	if limit > len(results) {
		limit = len(results)
	}
	
	for i := 0; i < limit; i++ {
		finalResults = append(finalResults, results[i].chunk)
	}

	return finalResults, nil
}

// DeleteChunk removes a context chunk
func (s *SQLiteStorage) DeleteChunk(ctx context.Context, chunkID string) error {
	query := `DELETE FROM context_chunks WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, chunkID)
	if err != nil {
		return fmt.Errorf("failed to delete chunk: %v", err)
	}
	return nil
}

// GetSessionSummary returns session statistics
func (s *SQLiteStorage) GetSessionSummary(ctx context.Context, sessionID string) (*types.SessionSummary, error) {
	query := `
		SELECT chunk_count, memory_usage, average_relevance, created_at, last_activity
		FROM session_stats 
		WHERE session_id = ?
	`

	row := s.db.QueryRowContext(ctx, query, sessionID)
	
	var summary types.SessionSummary
	summary.SessionID = sessionID
	
	err := row.Scan(
		&summary.ChunkCount,
		&summary.MemoryUsage,
		&summary.AverageRelevance,
		&summary.CreatedAt,
		&summary.LastActivity,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Return empty summary for non-existent sessions
			return &summary, nil
		}
		return nil, fmt.Errorf("failed to get session summary: %v", err)
	}

	return &summary, nil
}

// CleanupByTTL removes expired chunks
func (s *SQLiteStorage) CleanupByTTL(ctx context.Context, sessionID string) (int, error) {
	query := `
		DELETE FROM context_chunks 
		WHERE session_id = ? AND ttl IS NOT NULL AND ttl <= CURRENT_TIMESTAMP
	`
	
	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup by TTL: %v", err)
	}
	
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// CleanupByLRU removes least recently used chunks
func (s *SQLiteStorage) CleanupByLRU(ctx context.Context, sessionID string, keepCount int) (int, error) {
	query := `
		DELETE FROM context_chunks 
		WHERE session_id = ? AND id NOT IN (
			SELECT id FROM context_chunks 
			WHERE session_id = ? 
			ORDER BY updated_at DESC 
			LIMIT ?
		)
	`
	
	result, err := s.db.ExecContext(ctx, query, sessionID, sessionID, keepCount)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup by LRU: %v", err)
	}
	
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// CleanupByRelevance removes low relevance chunks
func (s *SQLiteStorage) CleanupByRelevance(ctx context.Context, sessionID string, minRelevance float64) (int, error) {
	query := `
		DELETE FROM context_chunks 
		WHERE session_id = ? AND relevance < ?
	`
	
	result, err := s.db.ExecContext(ctx, query, sessionID, minRelevance)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup by relevance: %v", err)
	}
	
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// CleanupSession removes all chunks for a session
func (s *SQLiteStorage) CleanupSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM context_chunks WHERE session_id = ?`
	_, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to cleanup session: %v", err)
	}
	return nil
}

// cosineSimilaritySQLite calculates cosine similarity between two vectors
func cosineSimilaritySQLite(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	
	var dotProduct, normA, normB float64
	
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	
	if normA == 0 || normB == 0 {
		return 0.0
	}
	
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Vacuum optimizes the SQLite database
func (s *SQLiteStorage) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %v", err)
	}
	return nil
}

// GetDatabaseSize returns the size of the SQLite database file in bytes
func (s *SQLiteStorage) GetDatabaseSize() (int64, error) {
	info, err := os.Stat(s.config.DatabasePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get database file info: %v", err)
	}
	return info.Size(), nil
}