package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/tarkank/aimem/internal/types"
)

// RedisStorage implements Redis-based storage for AIMem
type RedisStorage struct {
	client *redis.Client
	config *types.RedisConfig
}

// NewRedisStorage creates a new Redis storage instance
func NewRedisStorage(config *types.RedisConfig) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Host,
		Password: config.Password,
		DB:       config.DB,
		PoolSize: config.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &RedisStorage{
		client: client,
		config: config,
	}, nil
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

// StoreChunk stores a context chunk in Redis
func (r *RedisStorage) StoreChunk(ctx context.Context, chunk *types.ContextChunk) error {
	// Serialize chunk to JSON
	data, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to marshal chunk: %v", err)
	}

	// Store in Redis with TTL
	chunkKey := r.getChunkKey(chunk.ID)
	err = r.client.Set(ctx, chunkKey, data, chunk.TTL).Err()
	if err != nil {
		return fmt.Errorf("failed to store chunk: %v", err)
	}

	// Add to session index
	sessionKey := r.getSessionKey(chunk.SessionID)
	err = r.client.SAdd(ctx, sessionKey, chunk.ID).Err()
	if err != nil {
		return fmt.Errorf("failed to add chunk to session index: %v", err)
	}

	// Update session metadata
	err = r.updateSessionMetadata(ctx, chunk.SessionID, chunk.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to update session metadata: %v", err)
	}

	return nil
}

// GetChunk retrieves a context chunk by ID
func (r *RedisStorage) GetChunk(ctx context.Context, chunkID string) (*types.ContextChunk, error) {
	chunkKey := r.getChunkKey(chunkID)
	data, err := r.client.Get(ctx, chunkKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("chunk not found: %s", chunkID)
		}
		return nil, fmt.Errorf("failed to get chunk: %v", err)
	}

	var chunk types.ContextChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chunk: %v", err)
	}

	return &chunk, nil
}

// GetSessionChunks retrieves all chunks for a session
func (r *RedisStorage) GetSessionChunks(ctx context.Context, sessionID string) ([]*types.ContextChunk, error) {
	sessionKey := r.getSessionKey(sessionID)
	chunkIDs, err := r.client.SMembers(ctx, sessionKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session chunk IDs: %v", err)
	}

	var chunks []*types.ContextChunk
	for _, chunkID := range chunkIDs {
		chunk, err := r.GetChunk(ctx, chunkID)
		if err != nil {
			// Skip missing chunks (may have been expired)
			continue
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// DeleteChunk removes a context chunk
func (r *RedisStorage) DeleteChunk(ctx context.Context, chunkID string) error {
	chunkKey := r.getChunkKey(chunkID)
	
	// Get sessionID from chunk data before deleting
	sessionID, err := r.client.HGet(ctx, chunkKey, "session_id").Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get session ID from chunk: %v", err)
	}
	
	// Delete the chunk
	err = r.client.Del(ctx, chunkKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete chunk: %v", err)
	}

	// Remove from session index if we have sessionID
	if sessionID != "" {
		sessionKey := r.getSessionKey(sessionID)
		err = r.client.SRem(ctx, sessionKey, chunkID).Err()
		if err != nil {
			return fmt.Errorf("failed to remove chunk from session index: %v", err)
		}
	}

	return nil
}

// GetSessionStats retrieves session statistics
func (r *RedisStorage) GetSessionStats(ctx context.Context, sessionID string) (*types.SessionStats, error) {
	chunks, err := r.GetSessionChunks(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	stats := &types.SessionStats{
		SessionID:  sessionID,
		ChunkCount: len(chunks),
	}

	if len(chunks) == 0 {
		return stats, nil
	}

	var totalSize int64
	var totalRelevance float64
	var earliestTime, latestTime time.Time

	for i, chunk := range chunks {
		totalSize += int64(len(chunk.Content))
		totalRelevance += chunk.Relevance

		if i == 0 {
			earliestTime = chunk.Timestamp
			latestTime = chunk.Timestamp
		} else {
			if chunk.Timestamp.Before(earliestTime) {
				earliestTime = chunk.Timestamp
			}
			if chunk.Timestamp.After(latestTime) {
				latestTime = chunk.Timestamp
			}
		}
	}

	stats.TotalSize = totalSize
	stats.MemoryUsage = totalSize // Approximate memory usage
	stats.AverageRelevance = totalRelevance / float64(len(chunks))
	stats.CreatedAt = earliestTime
	stats.LastActivity = latestTime

	return stats, nil
}

// CleanupExpired removes expired chunks from session indexes
func (r *RedisStorage) CleanupExpired(ctx context.Context, sessionID string) (int, error) {
	sessionKey := r.getSessionKey(sessionID)
	chunkIDs, err := r.client.SMembers(ctx, sessionKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get session chunk IDs: %v", err)
	}

	removed := 0
	for _, chunkID := range chunkIDs {
		chunkKey := r.getChunkKey(chunkID)
		exists, err := r.client.Exists(ctx, chunkKey).Result()
		if err != nil {
			continue
		}

		if exists == 0 {
			// Chunk has expired, remove from session index
			r.client.SRem(ctx, sessionKey, chunkID)
			removed++
		}
	}

	return removed, nil
}

// SearchByEmbedding performs vector similarity search
func (r *RedisStorage) SearchByEmbedding(ctx context.Context, sessionID string, queryEmbedding []float32, maxResults int) ([]*types.ContextChunk, error) {
	// Get all session chunks - in production, this could use Redis modules like RediSearch
	// For now, we'll do in-memory similarity calculation
	chunks, err := r.GetSessionChunks(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	// Calculate similarities
	type chunkSimilarity struct {
		chunk      *types.ContextChunk
		similarity float64
	}

	similarities := make([]chunkSimilarity, len(chunks))
	for i, chunk := range chunks {
		similarity := cosineSimilarity(queryEmbedding, chunk.Embedding)
		similarities[i] = chunkSimilarity{
			chunk:      chunk,
			similarity: similarity,
		}
	}

	// Sort by similarity (descending)
	for i := 0; i < len(similarities)-1; i++ {
		for j := i + 1; j < len(similarities); j++ {
			if similarities[j].similarity > similarities[i].similarity {
				similarities[i], similarities[j] = similarities[j], similarities[i]
			}
		}
	}

	// Return top results
	limit := maxResults
	if limit > len(similarities) {
		limit = len(similarities)
	}

	results := make([]*types.ContextChunk, limit)
	for i := 0; i < limit; i++ {
		results[i] = similarities[i].chunk
	}

	return results, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// GetSessionSummary returns session statistics
func (r *RedisStorage) GetSessionSummary(ctx context.Context, sessionID string) (*types.SessionSummary, error) {
	// Get all chunks for the session
	pattern := fmt.Sprintf("chunk:%s:*", sessionID)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session keys: %v", err)
	}

	if len(keys) == 0 {
		return &types.SessionSummary{SessionID: sessionID}, nil
	}

	// Get chunks and calculate statistics
	var totalSize int64
	var totalRelevance float64
	var createdAt, lastActivity time.Time
	validChunks := 0

	for _, key := range keys {
		data, err := r.client.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		// Parse chunk data
		if content, exists := data["content"]; exists {
			totalSize += int64(len(content))
		}
		
		if relevanceStr, exists := data["relevance"]; exists {
			var relevance float64
			if n, err := fmt.Sscanf(relevanceStr, "%f", &relevance); err == nil && n == 1 && relevance >= 0 {
				totalRelevance += relevance
			}
		}
		
		if timestampStr, exists := data["timestamp"]; exists {
			if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
				if createdAt.IsZero() || timestamp.Before(createdAt) {
					createdAt = timestamp
				}
				if lastActivity.IsZero() || timestamp.After(lastActivity) {
					lastActivity = timestamp
				}
			}
		}
		
		validChunks++
	}

	avgRelevance := 0.0
	if validChunks > 0 {
		avgRelevance = totalRelevance / float64(validChunks)
	}

	return &types.SessionSummary{
		SessionID:        sessionID,
		ChunkCount:       validChunks,
		MemoryUsage:      totalSize,
		AverageRelevance: avgRelevance,
		CreatedAt:        createdAt,
		LastActivity:     lastActivity,
	}, nil
}

// CleanupByTTL removes expired chunks
func (r *RedisStorage) CleanupByTTL(ctx context.Context, sessionID string) (int, error) {
	// Redis handles TTL automatically, so we just need to count expired keys
	pattern := fmt.Sprintf("chunk:%s:*", sessionID)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get session keys: %v", err)
	}

	// Check which keys still exist (non-expired)
	existingCount := 0
	for _, key := range keys {
		exists, _ := r.client.Exists(ctx, key).Result()
		if exists > 0 {
			existingCount++
		}
	}

	// Return the number of keys that were cleaned up
	return len(keys) - existingCount, nil
}

// CleanupByLRU removes least recently used chunks
func (r *RedisStorage) CleanupByLRU(ctx context.Context, sessionID string, keepCount int) (int, error) {
	pattern := fmt.Sprintf("chunk:%s:*", sessionID)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get session keys: %v", err)
	}

	if len(keys) <= keepCount {
		return 0, nil // Nothing to cleanup
	}

	// Get chunks with timestamps for LRU sorting
	type chunkWithTime struct {
		key       string
		timestamp time.Time
	}
	
	var chunks []chunkWithTime
	for _, key := range keys {
		timestampStr, err := r.client.HGet(ctx, key, "timestamp").Result()
		if err != nil {
			continue
		}
		
		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			continue
		}
		
		chunks = append(chunks, chunkWithTime{
			key:       key,
			timestamp: timestamp,
		})
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(chunks)-1; i++ {
		for j := i + 1; j < len(chunks); j++ {
			if chunks[j].timestamp.Before(chunks[i].timestamp) {
				chunks[i], chunks[j] = chunks[j], chunks[i]
			}
		}
	}

	// Remove oldest chunks
	toRemove := len(chunks) - keepCount
	removed := 0
	
	for i := 0; i < toRemove && i < len(chunks); i++ {
		if err := r.client.Del(ctx, chunks[i].key).Err(); err == nil {
			removed++
		}
	}

	return removed, nil
}

// CleanupByRelevance removes low relevance chunks
func (r *RedisStorage) CleanupByRelevance(ctx context.Context, sessionID string, minRelevance float64) (int, error) {
	pattern := fmt.Sprintf("chunk:%s:*", sessionID)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get session keys: %v", err)
	}

	removed := 0
	for _, key := range keys {
		relevanceStr, err := r.client.HGet(ctx, key, "relevance").Result()
		if err != nil {
			continue
		}
		
		var relevance float64
		if _, err := fmt.Sscanf(relevanceStr, "%f", &relevance); err != nil {
			continue
		}
		
		if relevance < minRelevance {
			if err := r.client.Del(ctx, key).Err(); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// CleanupSession removes all chunks for a session
func (r *RedisStorage) CleanupSession(ctx context.Context, sessionID string) error {
	pattern := fmt.Sprintf("chunk:%s:*", sessionID)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get session keys: %v", err)
	}

	if len(keys) > 0 {
		err = r.client.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("failed to delete session chunks: %v", err)
		}
	}

	return nil
}

// Helper methods for Redis key generation

func (r *RedisStorage) getChunkKey(chunkID string) string {
	return fmt.Sprintf("aimem:chunk:%s", chunkID)
}

func (r *RedisStorage) getSessionKey(sessionID string) string {
	return fmt.Sprintf("aimem:session:%s", sessionID)
}

func (r *RedisStorage) getSessionMetaKey(sessionID string) string {
	return fmt.Sprintf("aimem:session:meta:%s", sessionID)
}

func (r *RedisStorage) updateSessionMetadata(ctx context.Context, sessionID string, timestamp time.Time) error {
	metaKey := r.getSessionMetaKey(sessionID)
	
	// Store last activity timestamp
	return r.client.HSet(ctx, metaKey, map[string]interface{}{
		"last_activity": timestamp.Unix(),
		"session_id":    sessionID,
	}).Err()
}