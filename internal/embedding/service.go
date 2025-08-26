package embedding

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/mat"
)

// Service provides text embedding generation with caching and batch processing
type Service struct {
	model      *EmbeddingModel
	cache      *EmbeddingCache
	batchSize  int
	mu         sync.RWMutex
	logger     *logrus.Logger
}

// EmbeddingModel represents a lightweight embedding model
type EmbeddingModel struct {
	dimensions int
	vocab      map[string]int
	embeddings *mat.Dense
	mu         sync.RWMutex
}

// EmbeddingCache provides LRU caching for embeddings
type EmbeddingCache struct {
	cache    map[string]*CacheEntry
	lruOrder []string
	maxSize  int
	mu       sync.RWMutex
}

// CacheEntry represents a cached embedding with metadata
type CacheEntry struct {
	Embedding []float32
	CreatedAt time.Time
	AccessAt  time.Time
}

// Config contains configuration for the embedding service
type Config struct {
	Dimensions int    `yaml:"dimensions"`
	CacheSize  int    `yaml:"cache_size"`
	BatchSize  int    `yaml:"batch_size"`
	Model      string `yaml:"model"`
}

// NewService creates a new embedding service with local model
func NewService(config *Config, logger *logrus.Logger) (*Service, error) {
	if logger == nil {
		logger = logrus.New()
	}

	model, err := newEmbeddingModel(config.Dimensions)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding model: %w", err)
	}

	cache := newEmbeddingCache(config.CacheSize)

	return &Service{
		model:     model,
		cache:     cache,
		batchSize: config.BatchSize,
		logger:    logger,
	}, nil
}

// GenerateEmbedding generates a single embedding for the given content
func (s *Service) GenerateEmbedding(content string) ([]float32, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Check cache first
	if cached := s.cache.Get(content); cached != nil {
		s.logger.Debug("Embedding cache hit")
		return cached, nil
	}

	start := time.Now()
	embedding, err := s.model.Encode(content)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Cache the result
	s.cache.Set(content, embedding)

	s.logger.WithFields(logrus.Fields{
		"content_length": len(content),
		"duration_ms":    time.Since(start).Milliseconds(),
	}).Debug("Generated embedding")

	return embedding, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple contents efficiently
func (s *Service) BatchGenerateEmbeddings(contents []string) ([][]float32, error) {
	if len(contents) == 0 {
		return [][]float32{}, nil
	}

	start := time.Now()
	results := make([][]float32, len(contents))
	uncachedIndices := make([]int, 0, len(contents))
	uncachedContents := make([]string, 0, len(contents))

	// Check cache for all contents
	for i, content := range contents {
		if cached := s.cache.Get(content); cached != nil {
			results[i] = cached
		} else {
			uncachedIndices = append(uncachedIndices, i)
			uncachedContents = append(uncachedContents, content)
		}
	}

	if len(uncachedContents) == 0 {
		s.logger.WithField("cache_hits", len(contents)).Debug("All embeddings from cache")
		return results, nil
	}

	// Process uncached contents in batches
	for i := 0; i < len(uncachedContents); i += s.batchSize {
		end := i + s.batchSize
		if end > len(uncachedContents) {
			end = len(uncachedContents)
		}

		batch := uncachedContents[i:end]
		batchEmbeddings, err := s.model.BatchEncode(batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate batch embeddings: %w", err)
		}

		// Store results and cache
		for j, embedding := range batchEmbeddings {
			originalIndex := uncachedIndices[i+j]
			results[originalIndex] = embedding
			s.cache.Set(uncachedContents[i+j], embedding)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"total_contents": len(contents),
		"cache_hits":     len(contents) - len(uncachedContents),
		"cache_misses":   len(uncachedContents),
		"duration_ms":    time.Since(start).Milliseconds(),
	}).Debug("Generated batch embeddings")

	return results, nil
}

// CosineSimilarity calculates cosine similarity between two embeddings
func (s *Service) CosineSimilarity(a, b []float32) float64 {
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

// FindMostSimilar finds the most similar embeddings to the query
func (s *Service) FindMostSimilar(query []float32, candidates [][]float32, topK int) []SimilarityResult {
	if topK <= 0 || len(candidates) == 0 {
		return []SimilarityResult{}
	}

	results := make([]SimilarityResult, 0, len(candidates))
	for i, candidate := range candidates {
		similarity := s.CosineSimilarity(query, candidate)
		results = append(results, SimilarityResult{
			Index:      i,
			Similarity: similarity,
		})
	}

	// Sort by similarity (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top K results
	if topK > len(results) {
		topK = len(results)
	}
	return results[:topK]
}

// SimilarityResult represents a similarity search result
type SimilarityResult struct {
	Index      int     `json:"index"`
	Similarity float64 `json:"similarity"`
}

// GetCacheStats returns cache statistics
func (s *Service) GetCacheStats() CacheStats {
	return s.cache.GetStats()
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
	HitRate float64 `json:"hit_rate"`
}

// newEmbeddingModel creates a simple embedding model for demonstration
func newEmbeddingModel(dimensions int) (*EmbeddingModel, error) {
	if dimensions <= 0 {
		dimensions = 384 // Default dimension similar to MiniLM
	}

	return &EmbeddingModel{
		dimensions: dimensions,
		vocab:      make(map[string]int),
		embeddings: mat.NewDense(1000, dimensions, nil), // Pre-allocated space
	}, nil
}

// Encode generates an embedding for a single text
func (m *EmbeddingModel) Encode(text string) ([]float32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple embedding generation using text hashing and feature extraction
	// In production, this would use a pre-trained model like sentence-transformers
	return m.generateSimpleEmbedding(text), nil
}

// BatchEncode generates embeddings for multiple texts
func (m *EmbeddingModel) BatchEncode(texts []string) ([][]float32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([][]float32, len(texts))
	for i, text := range texts {
		results[i] = m.generateSimpleEmbedding(text)
	}
	return results, nil
}

// generateSimpleEmbedding creates a simple but consistent embedding
func (m *EmbeddingModel) generateSimpleEmbedding(text string) []float32 {
	// Create a hash-based embedding that's consistent and captures some semantic info
	hash := sha256.Sum256([]byte(text))
	
	embedding := make([]float32, m.dimensions)
	
	// Use different parts of the hash to generate features
	for i := 0; i < m.dimensions; i++ {
		// Create features from different combinations of hash bytes
		byteIdx := i % 32
		feature := float32(hash[byteIdx]) / 255.0 // Normalize to [0,1]
		
		// Add some simple text-based features
		if i < len(text) {
			charFeature := float32(text[i%len(text)]) / 255.0
			feature = (feature + charFeature) / 2.0
		}
		
		// Center around 0 and add some variance
		embedding[i] = (feature - 0.5) * 2.0
	}
	
	// Normalize the embedding
	var norm float32
	for _, val := range embedding {
		norm += val * val
	}
	norm = float32(math.Sqrt(float64(norm)))
	
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	
	return embedding
}

// newEmbeddingCache creates a new LRU cache for embeddings
func newEmbeddingCache(maxSize int) *EmbeddingCache {
	if maxSize <= 0 {
		maxSize = 1000 // Default cache size
	}
	
	return &EmbeddingCache{
		cache:    make(map[string]*CacheEntry),
		lruOrder: make([]string, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Get retrieves an embedding from cache
func (c *EmbeddingCache) Get(key string) []float32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	entry, exists := c.cache[key]
	if !exists {
		return nil
	}
	
	// Update access time and move to front of LRU
	entry.AccessAt = time.Now()
	c.moveToFront(key)
	
	return entry.Embedding
}

// Set stores an embedding in cache with LRU eviction
func (c *EmbeddingCache) Set(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// If key exists, update it
	if entry, exists := c.cache[key]; exists {
		entry.Embedding = embedding
		entry.AccessAt = time.Now()
		c.moveToFront(key)
		return
	}
	
	// Add new entry
	now := time.Now()
	c.cache[key] = &CacheEntry{
		Embedding: embedding,
		CreatedAt: now,
		AccessAt:  now,
	}
	c.lruOrder = append([]string{key}, c.lruOrder...)
	
	// Evict if necessary
	if len(c.cache) > c.maxSize {
		c.evictLRU()
	}
}

// moveToFront moves a key to the front of LRU order
func (c *EmbeddingCache) moveToFront(key string) {
	for i, k := range c.lruOrder {
		if k == key {
			// Remove from current position
			c.lruOrder = append(c.lruOrder[:i], c.lruOrder[i+1:]...)
			break
		}
	}
	// Add to front
	c.lruOrder = append([]string{key}, c.lruOrder...)
}

// evictLRU removes the least recently used entry
func (c *EmbeddingCache) evictLRU() {
	if len(c.lruOrder) == 0 {
		return
	}
	
	// Remove the last (least recently used) entry
	lastKey := c.lruOrder[len(c.lruOrder)-1]
	c.lruOrder = c.lruOrder[:len(c.lruOrder)-1]
	delete(c.cache, lastKey)
}

// GetStats returns cache performance statistics
func (c *EmbeddingCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return CacheStats{
		Size:    len(c.cache),
		MaxSize: c.maxSize,
		HitRate: 0.0, // Would need hit/miss tracking for accurate rate
	}
}