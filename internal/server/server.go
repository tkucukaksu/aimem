package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tarkank/aimem/internal/analyzer"
	"github.com/tarkank/aimem/internal/chunker"
	"github.com/tarkank/aimem/internal/embedding"
	"github.com/tarkank/aimem/internal/errors"
	"github.com/tarkank/aimem/internal/logger"
	"github.com/tarkank/aimem/internal/mcp"
	"github.com/tarkank/aimem/internal/storage"
	"github.com/tarkank/aimem/internal/summarizer"
	"github.com/tarkank/aimem/internal/types"
)

// AIMem represents the main MCP server
type AIMem struct {
	storage    storage.Storage
	embedder   *embedding.Service
	chunker    *chunker.Service
	summarizer *summarizer.Service
	analyzer   *analyzer.ProjectAnalyzer
	config     *types.Config
	logger     *logger.Logger
}

// Performance metrics tracking
type PerformanceMetrics struct {
	StorageLatency    time.Duration
	EmbeddingLatency  time.Duration
	ChunkingLatency   time.Duration
	SummarizationLatency time.Duration
	TotalLatency      time.Duration
	MemoryUsage       int64
}

// NewAIMem creates a new AIMem server instance
func NewAIMem(config *types.Config) (*AIMem, error) {
	// Initialize logger
	loggerConfig := &logger.Config{
		Level:        "info",
		Format:       "json",
		Output:       "stdout",
		EnableCaller: false,
	}
	aimemLogger, err := logger.NewLogger(loggerConfig, "aimem")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to initialize logger")
	}

	// Initialize storage (SQLite or Redis based on config)
	storageInstance, err := storage.NewStorage(config)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to initialize storage")
	}

	// Initialize embedding service
	embeddingConfig := &embedding.Config{
		Dimensions: 384,
		CacheSize:  config.Embedding.CacheSize,
		BatchSize:  config.Embedding.BatchSize,
		Model:      config.Embedding.Model,
	}
	embeddingService, err := embedding.NewService(embeddingConfig, aimemLogger.Logger)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeEmbedding, "failed to initialize embedding service")
	}

	// Initialize chunking service
	chunkingConfig := &chunker.Config{
		MaxChunkSize:    config.Memory.ChunkSize,
		OverlapSize:     config.Memory.ChunkSize / 10, // 10% overlap
		MinChunkSize:    50,
		SentenceWeight:  1.0,
		ParagraphWeight: 0.8,
		CodeWeight:      0.9,
	}
	chunkingService := chunker.NewService(chunkingConfig, aimemLogger.Logger)

	// Initialize summarization service
	summarizationConfig := &summarizer.Config{
		CompressionRatio: 0.3,
		MinSummaryLength: 50,
		MaxSummaryLength: config.Memory.ChunkSize / 2,
		PreserveCode:     true,
		PreserveLinks:    true,
		KeywordWeight:    1.5,
	}
	summarizationService := summarizer.NewService(summarizationConfig, aimemLogger.Logger)

	// Initialize project analyzer
	projectAnalyzer := analyzer.NewProjectAnalyzer()

	return &AIMem{
		storage:    storageInstance,
		embedder:   embeddingService,
		chunker:    chunkingService,
		summarizer: summarizationService,
		analyzer:   projectAnalyzer,
		config:     config,
		logger:     aimemLogger,
	}, nil
}

// HandleRequest processes MCP requests
func (a *AIMem) HandleRequest(ctx context.Context, reader io.Reader, writer io.Writer) error {
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(writer)

	for {
		var req mcp.Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			// Send parse error
			resp := mcp.NewErrorResponse(nil, mcp.NewError(
				mcp.ErrorCodeParseError,
				"Parse error",
				err.Error(),
			))
			return encoder.Encode(resp)
		}

		// Process the request
		resp := a.processRequest(ctx, &req)
		if err := encoder.Encode(resp); err != nil {
			a.logger.WithError(err).Error("Error encoding response")
			return err
		}
	}
}

// processRequest handles individual MCP requests
func (a *AIMem) processRequest(ctx context.Context, req *mcp.Request) *mcp.Response {
	switch req.Method {
	case "tools/list":
		return a.handleListTools(req)
	case "tools/call":
		return a.handleToolCall(ctx, req)
	case "initialize":
		return a.handleInitialize(req)
	default:
		return mcp.NewErrorResponse(req.ID, mcp.NewError(
			mcp.ErrorCodeMethodNotFound,
			fmt.Sprintf("Method not found: %s", req.Method),
			nil,
		))
	}
}

// handleInitialize handles the MCP initialize method
func (a *AIMem) handleInitialize(req *mcp.Request) *mcp.Response {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    a.config.MCP.ServerName,
			"version": a.config.MCP.Version,
		},
	}
	return mcp.NewResponse(req.ID, result)
}

// handleListTools handles the tools/list method
func (a *AIMem) handleListTools(req *mcp.Request) *mcp.Response {
	tools := mcp.GetTools()
	result := map[string]interface{}{
		"tools": tools,
	}
	return mcp.NewResponse(req.ID, result)
}

// handleToolCall handles the tools/call method
func (a *AIMem) handleToolCall(ctx context.Context, req *mcp.Request) *mcp.Response {
	// Parse the tool call parameters
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return mcp.NewErrorResponse(req.ID, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"Invalid parameters",
			"Expected object parameters",
		))
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return mcp.NewErrorResponse(req.ID, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"Missing tool name",
			"Tool name must be a string",
		))
	}

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return mcp.NewErrorResponse(req.ID, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"Missing arguments",
			"Arguments must be an object",
		))
	}

	// Route to appropriate tool handler
	switch toolName {
	// Smart Context Management Tools
	case "auto_store_project":
		return a.handleAutoStoreProject(ctx, req.ID, arguments)
	case "context_aware_retrieve":
		return a.handleContextAwareRetrieve(ctx, req.ID, arguments)
	case "smart_memory_manager":
		return a.handleSmartMemoryManager(ctx, req.ID, arguments)
	// Original Tools
	case "store_context":
		return a.handleStoreContext(ctx, req.ID, arguments)
	case "retrieve_context":
		return a.handleRetrieveContext(ctx, req.ID, arguments)
	case "summarize_session":
		return a.handleSummarizeSession(ctx, req.ID, arguments)
	case "cleanup_session":
		return a.handleCleanupSession(ctx, req.ID, arguments)
	default:
		return mcp.NewErrorResponse(req.ID, mcp.NewError(
			mcp.ErrorCodeMethodNotFound,
			fmt.Sprintf("Unknown tool: %s", toolName),
			nil,
		))
	}
}

// handleStoreContext handles the store_context tool
func (a *AIMem) handleStoreContext(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	// Parse parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}

	content, ok := args["content"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"content must be a string",
			nil,
		))
	}

	importanceStr, ok := args["importance"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"importance must be a string",
			nil,
		))
	}

	importance := types.Importance(importanceStr)
	if importance != types.ImportanceLow && importance != types.ImportanceMedium && importance != types.ImportanceHigh {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"importance must be one of: low, medium, high",
			nil,
		))
	}

	// Get silent mode (default: true for seamless operation)
	silent := true
	if silentValue, exists := args["silent"]; exists {
		if silentBool, ok := silentValue.(bool); ok {
			silent = silentBool
		}
	}

	// Process and store the context
	chunkID, err := a.storeContext(ctx, sessionID, content, importance)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			"Failed to store context",
			err.Error(),
		))
	}

	// Prepare response based on silent mode
	var responseText string
	if silent {
		// Minimal response for seamless operation
		responseText = fmt.Sprintf("✅ Stored (%d bytes)", len(content))
	} else {
		// Detailed response for debugging/verbose mode
		responseText = fmt.Sprintf("✅ Context stored successfully\n\n**Chunk ID**: %s\n**Size**: %d bytes\n**Importance**: %s", 
			chunkID, len(content), importance)
	}

	return mcp.NewResponse(id, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": responseText,
			},
		},
	})
}

// handleRetrieveContext handles the retrieve_context tool
func (a *AIMem) handleRetrieveContext(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	// Parse parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}

	query, ok := args["query"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"query must be a string",
			nil,
		))
	}

	maxChunks := 5 // default
	if maxChunksFloat, exists := args["max_chunks"]; exists {
		if maxChunksVal, ok := maxChunksFloat.(float64); ok {
			maxChunks = int(maxChunksVal)
		}
	}

	// Retrieve context
	result, err := a.retrieveContext(ctx, sessionID, query, maxChunks)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			"Failed to retrieve context",
			err.Error(),
		))
	}

	// Format response
	responseText := fmt.Sprintf("🔍 Retrieved %d relevant context chunks (Query time: %dms)\n\n", 
		len(result.Chunks), result.QueryTime)
	
	for i, chunk := range result.Chunks {
		responseText += fmt.Sprintf("**Chunk %d** (ID: %s, Relevance: %.3f)\n%s\n\n", 
			i+1, chunk.ID, chunk.Relevance, chunk.Content)
	}

	return mcp.NewResponse(id, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": responseText,
			},
		},
	})
}

// handleSummarizeSession handles the summarize_session tool
func (a *AIMem) handleSummarizeSession(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	// Parse parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}

	// Get session summary
	summary, err := a.getSessionSummary(ctx, sessionID)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			"Failed to get session summary",
			err.Error(),
		))
	}

	responseText := fmt.Sprintf(`📊 **Session Summary**: %s

**Statistics:**
- Total chunks: %d
- Memory usage: %.2f MB
- Average relevance: %.3f
- Created: %s
- Last activity: %s`,
		sessionID, 
		summary.ChunkCount,
		float64(summary.MemoryUsage)/1024/1024,
		summary.AverageRelevance,
		summary.CreatedAt.Format("2006-01-02 15:04:05"),
		summary.LastActivity.Format("2006-01-02 15:04:05"))

	return mcp.NewResponse(id, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": responseText,
			},
		},
	})
}

// handleCleanupSession handles the cleanup_session tool
func (a *AIMem) handleCleanupSession(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	// Parse parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}

	strategyStr, ok := args["strategy"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"strategy must be a string",
			nil,
		))
	}

	strategy := types.CleanupStrategy(strategyStr)
	if strategy != types.CleanupTTL && strategy != types.CleanupLRU && strategy != types.CleanupRelevance {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"strategy must be one of: ttl, lru, relevance",
			nil,
		))
	}

	// Perform cleanup
	result, err := a.cleanupSession(ctx, sessionID, strategy)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			"Failed to cleanup session",
			err.Error(),
		))
	}

	responseText := fmt.Sprintf(`🧹 **Session Cleanup Complete**

**Strategy**: %s
**Results:**
- Chunks removed: %d
- Bytes freed: %.2f MB
- Remaining chunks: %d`,
		strategy,
		result.ChunksRemoved,
		float64(result.BytesFreed)/1024/1024,
		result.RemainingChunks)

	return mcp.NewResponse(id, map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": responseText,
			},
		},
	})
}

// Implementation methods with production logic

func (a *AIMem) storeContext(ctx context.Context, sessionID, content string, importance types.Importance) (string, error) {
	start := time.Now()
	ctx = logger.ContextWithSessionID(ctx, sessionID)
	log := a.logger.WithContext(ctx).WithField("operation", "store_context")

	log.WithFields(logger.Fields{
		"content_length": len(content),
		"importance":     importance,
	}).Info("Starting context storage")

	// Generate unique chunk ID
	chunkID := uuid.New().String()

	// Step 1: Chunk the content
	chunkStart := time.Now()
	chunks, err := a.chunker.ChunkContent(content, a.config.Memory.ChunkSize)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeChunking, "failed to chunk content")
	}
	chunkingLatency := time.Since(chunkStart)

	// For simplicity, we'll store the first chunk or combine if small enough
	var finalContent string
	if len(chunks) == 1 {
		finalContent = chunks[0]
	} else {
		// Combine small chunks or use the first significant chunk
		combinedLength := 0
		for _, chunk := range chunks {
			if combinedLength+len(chunk) <= a.config.Memory.ChunkSize {
				finalContent += chunk + " "
				combinedLength += len(chunk)
			} else {
				break
			}
		}
		if finalContent == "" {
			finalContent = chunks[0] // Use first chunk as fallback
		}
	}

	// Step 2: Generate embedding
	embeddingStart := time.Now()
	embedding, err := a.embedder.GenerateEmbedding(finalContent)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeEmbedding, "failed to generate embedding")
	}
	embeddingLatency := time.Since(embeddingStart)

	// Step 3: Create summary
	summaryStart := time.Now()
	summary, err := a.summarizer.SummarizeContent(finalContent, a.config.Memory.ChunkSize/3)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeSummarization, "failed to create summary")
	}
	summarizationLatency := time.Since(summaryStart)

	// Step 4: Calculate relevance based on importance
	relevance := a.calculateInitialRelevance(importance)

	// Step 5: Create context chunk
	chunk := &types.ContextChunk{
		ID:          chunkID,
		SessionID:   sessionID,
		Content:     finalContent,
		Summary:     summary,
		Embedding:   embedding,
		Relevance:   relevance,
		Timestamp:   time.Now(),
		TTL:         a.config.Memory.TTLDefault,
		Importance:  importance,
	}

	// Step 6: Store in Redis
	storageStart := time.Now()
	err = a.storage.StoreChunk(ctx, chunk)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeStorage, "failed to store chunk in Redis")
	}
	storageLatency := time.Since(storageStart)

	// Log performance metrics
	totalLatency := time.Since(start)
	a.logger.LogPerformance(ctx, "store_context", totalLatency, map[string]interface{}{
		"chunking_ms":      chunkingLatency.Milliseconds(),
		"embedding_ms":     embeddingLatency.Milliseconds(),
		"summarization_ms": summarizationLatency.Milliseconds(),
		"storage_ms":       storageLatency.Milliseconds(),
		"content_length":   len(content),
		"final_length":     len(finalContent),
		"chunk_count":      len(chunks),
		"importance":       importance,
	})

	log.WithFields(logger.Fields{
		"chunk_id":       chunkID,
		"total_latency_ms": totalLatency.Milliseconds(),
	}).Info("Context storage completed")

	return chunkID, nil
}

func (a *AIMem) retrieveContext(ctx context.Context, sessionID, query string, maxChunks int) (*types.RetrievalResult, error) {
	start := time.Now()
	ctx = logger.ContextWithSessionID(ctx, sessionID)
	log := a.logger.WithContext(ctx).WithField("operation", "retrieve_context")

	log.WithFields(logger.Fields{
		"query_length": len(query),
		"max_chunks":   maxChunks,
	}).Info("Starting context retrieval")

	// Step 1: Generate query embedding
	embeddingStart := time.Now()
	queryEmbedding, err := a.embedder.GenerateEmbedding(query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeEmbedding, "failed to generate query embedding")
	}
	embeddingLatency := time.Since(embeddingStart)

	// Step 2: Get all session chunks
	storageStart := time.Now()
	allChunks, err := a.storage.SearchByEmbedding(ctx, sessionID, nil, 1000)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeStorage, "failed to retrieve session chunks")
	}
	storageLatency := time.Since(storageStart)

	if len(allChunks) == 0 {
		return &types.RetrievalResult{
			Chunks:     []types.ContextChunk{},
			TotalScore: 0.0,
			QueryTime:  time.Since(start),
		}, nil
	}

	// Step 3: Calculate similarities and rank
	rankingStart := time.Now()
	type ChunkScore struct {
		Chunk *types.ContextChunk
		Score float64
	}

	scores := make([]ChunkScore, 0, len(allChunks))
	for _, chunk := range allChunks {
		// Calculate cosine similarity
		similarity := a.embedder.CosineSimilarity(queryEmbedding, chunk.Embedding)
		
		// Combine with other factors
		finalScore := a.calculateRetrievalScore(similarity, chunk, query)
		
		scores = append(scores, ChunkScore{
			Chunk: chunk,
			Score: finalScore,
		})
	}

	// Sort by score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	rankingLatency := time.Since(rankingStart)

	// Step 4: Select top results
	limit := maxChunks
	if limit > len(scores) {
		limit = len(scores)
	}

	resultChunks := make([]types.ContextChunk, limit)
	totalScore := 0.0
	for i := 0; i < limit; i++ {
		// Update relevance score
		scores[i].Chunk.Relevance = scores[i].Score
		resultChunks[i] = *scores[i].Chunk
		totalScore += scores[i].Score
	}

	totalLatency := time.Since(start)

	// Log performance metrics
	a.logger.LogPerformance(ctx, "retrieve_context", totalLatency, map[string]interface{}{
		"embedding_ms":    embeddingLatency.Milliseconds(),
		"storage_ms":      storageLatency.Milliseconds(),
		"ranking_ms":      rankingLatency.Milliseconds(),
		"chunks_analyzed": len(allChunks),
		"chunks_returned": len(resultChunks),
		"query_length":    len(query),
		"total_score":     totalScore,
	})

	log.WithFields(logger.Fields{
		"chunks_found":     len(allChunks),
		"chunks_returned":  len(resultChunks),
		"total_latency_ms": totalLatency.Milliseconds(),
		"total_score":      totalScore,
	}).Info("Context retrieval completed")

	return &types.RetrievalResult{
		Chunks:     resultChunks,
		TotalScore: totalScore,
		QueryTime:  totalLatency,
	}, nil
}

func (a *AIMem) getSessionSummary(ctx context.Context, sessionID string) (*types.SessionSummary, error) {
	ctx = logger.ContextWithSessionID(ctx, sessionID)
	log := a.logger.WithContext(ctx).WithField("operation", "get_session_summary")

	log.Debug("Getting session summary")

	// Use storage method to get comprehensive stats
	stats, err := a.storage.GetSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeStorage, "failed to get session statistics")
	}

	log.WithFields(logger.Fields{
		"chunk_count":       stats.ChunkCount,
		"total_size":        stats.MemoryUsage,
		"memory_usage":      stats.MemoryUsage,
		"average_relevance": stats.AverageRelevance,
	}).Info("Session summary retrieved")

	return stats, nil
}

func (a *AIMem) cleanupSession(ctx context.Context, sessionID string, strategy types.CleanupStrategy) (*mcp.CleanupSessionResult, error) {
	start := time.Now()
	ctx = logger.ContextWithSessionID(ctx, sessionID)
	log := a.logger.WithContext(ctx).WithField("operation", "cleanup_session")

	log.WithField("strategy", strategy).Info("Starting session cleanup")

	// Get all chunks for the session
	allChunks, err := a.storage.SearchByEmbedding(ctx, sessionID, nil, 1000)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeStorage, "failed to get session chunks for cleanup")
	}

	if len(allChunks) == 0 {
		return &mcp.CleanupSessionResult{
			Success:         true,
			ChunksRemoved:   0,
			BytesFreed:      0,
			Strategy:        string(strategy),
			RemainingChunks: 0,
		}, nil
	}

	// Determine chunks to remove based on strategy
	toRemove := a.selectChunksForRemoval(allChunks, strategy)
	
	// Calculate bytes that will be freed
	bytesFreed := int64(0)
	for _, chunk := range toRemove {
		bytesFreed += int64(len(chunk.Content))
	}

	// Remove selected chunks
	removed := 0
	for _, chunk := range toRemove {
		err := a.storage.DeleteChunk(ctx, chunk.ID)
		if err != nil {
			log.WithError(err).WithField("chunk_id", chunk.ID).Warn("Failed to remove chunk")
			continue
		}
		removed++
	}

	remainingChunks := len(allChunks) - removed

	// Log performance
	a.logger.LogPerformance(ctx, "cleanup_session", time.Since(start), map[string]interface{}{
		"strategy":         string(strategy),
		"total_chunks":     len(allChunks),
		"chunks_removed":   removed,
		"remaining_chunks": remainingChunks,
		"bytes_freed":      bytesFreed,
	})

	log.WithFields(logger.Fields{
		"removed":   removed,
		"remaining": remainingChunks,
		"bytes_freed": bytesFreed,
	}).Info("Session cleanup completed")

	return &mcp.CleanupSessionResult{
		Success:         true,
		ChunksRemoved:   removed,
		BytesFreed:      bytesFreed,
		Strategy:        string(strategy),
		RemainingChunks: remainingChunks,
	}, nil
}

// Helper methods for scoring and cleanup logic

// calculateInitialRelevance calculates initial relevance based on importance
func (a *AIMem) calculateInitialRelevance(importance types.Importance) float64 {
	switch importance {
	case types.ImportanceHigh:
		return 0.9
	case types.ImportanceMedium:
		return 0.7
	case types.ImportanceLow:
		return 0.5
	default:
		return 0.5
	}
}

// calculateRetrievalScore combines similarity with other factors
func (a *AIMem) calculateRetrievalScore(similarity float64, chunk *types.ContextChunk, query string) float64 {
	// Base similarity score (60% weight)
	score := similarity * 0.6
	
	// Importance weight (20%)
	importanceScore := 0.0
	switch chunk.Importance {
	case types.ImportanceHigh:
		importanceScore = 1.0
	case types.ImportanceMedium:
		importanceScore = 0.7
	case types.ImportanceLow:
		importanceScore = 0.3
	}
	score += importanceScore * 0.2
	
	// Recency weight (10% - newer chunks score higher)
	age := time.Since(chunk.Timestamp)
	recencyScore := math.Max(0, 1.0-float64(age.Hours())/168.0) // Decay over a week
	score += recencyScore * 0.1
	
	// Current relevance weight (10%)
	score += chunk.Relevance * 0.1
	
	return math.Min(score, 1.0)
}

// selectChunksForRemoval determines which chunks to remove based on cleanup strategy
func (a *AIMem) selectChunksForRemoval(chunks []*types.ContextChunk, strategy types.CleanupStrategy) []*types.ContextChunk {
	if len(chunks) == 0 {
		return []*types.ContextChunk{}
	}
	
	// Don't remove more than 50% of chunks in one cleanup
	maxRemove := len(chunks) / 2
	if maxRemove == 0 {
		maxRemove = 1
	}
	
	switch strategy {
	case types.CleanupTTL:
		return a.selectExpiredChunks(chunks, maxRemove)
	case types.CleanupLRU:
		return a.selectLRUChunks(chunks, maxRemove)
	case types.CleanupRelevance:
		return a.selectLowRelevanceChunks(chunks, maxRemove)
	default:
		return a.selectLRUChunks(chunks, maxRemove)
	}
}

// selectExpiredChunks selects chunks that have exceeded their TTL
func (a *AIMem) selectExpiredChunks(chunks []*types.ContextChunk, maxRemove int) []*types.ContextChunk {
	var expired []*types.ContextChunk
	now := time.Now()
	
	for _, chunk := range chunks {
		if chunk.TTL > 0 && now.Sub(chunk.Timestamp) > chunk.TTL {
			expired = append(expired, chunk)
			if len(expired) >= maxRemove {
				break
			}
		}
	}
	
	return expired
}

// selectLRUChunks selects the least recently used chunks
func (a *AIMem) selectLRUChunks(chunks []*types.ContextChunk, maxRemove int) []*types.ContextChunk {
	// Sort by timestamp (oldest first)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Timestamp.Before(chunks[j].Timestamp)
	})
	
	if maxRemove > len(chunks) {
		maxRemove = len(chunks)
	}
	
	return chunks[:maxRemove]
}

// selectLowRelevanceChunks selects chunks with the lowest relevance scores
func (a *AIMem) selectLowRelevanceChunks(chunks []*types.ContextChunk, maxRemove int) []*types.ContextChunk {
	// Sort by relevance (lowest first)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Relevance < chunks[j].Relevance
	})
	
	if maxRemove > len(chunks) {
		maxRemove = len(chunks)
	}
	
	return chunks[:maxRemove]
}

// Smart Context Management Handlers

// handleAutoStoreProject handles the auto_store_project tool
func (a *AIMem) handleAutoStoreProject(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	startTime := time.Now()
	
	// Extract parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}
	
	projectPath, ok := args["project_path"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"project_path must be a string",
			nil,
		))
	}
	
	// Parse focus areas
	var focusAreas []types.FocusArea
	if focusAreasInterface, exists := args["focus_areas"]; exists {
		if focusAreasSlice, ok := focusAreasInterface.([]interface{}); ok {
			for _, area := range focusAreasSlice {
				if areaStr, ok := area.(string); ok {
					focusAreas = append(focusAreas, types.FocusArea(areaStr))
				}
			}
		}
	} else {
		// Default focus areas if not specified
		focusAreas = []types.FocusArea{
			types.FocusArchitecture,
			types.FocusAPI,
			types.FocusDatabase,
		}
	}
	
	// Get importance threshold
	importanceThreshold := types.ImportanceMedium
	if thresholdStr, exists := args["importance_threshold"].(string); exists {
		importanceThreshold = types.Importance(thresholdStr)
	}
	
	// Get silent mode (default: true for seamless operation)
	silent := true
	if silentValue, exists := args["silent"]; exists {
		if silentBool, ok := silentValue.(bool); ok {
			silent = silentBool
		}
	}
	
	a.logger.Info("Starting project analysis", map[string]interface{}{
		"session_id":            sessionID,
		"project_path":          projectPath,
		"focus_areas":           focusAreas,
		"importance_threshold":  importanceThreshold,
		"operation":             "auto_store_project",
	})
	
	// Analyze project
	analysis, err := a.analyzer.AnalyzeProject(projectPath, focusAreas)
	if err != nil {
		a.logger.WithError(err).Error("Project analysis failed", map[string]interface{}{
			"operation":   "auto_store_project",
			"session_id":  sessionID,
			"project_path": projectPath,
		})
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			fmt.Sprintf("Project analysis failed: %v", err),
			nil,
		))
	}
	
	// Generate context chunks from analysis
	chunks, err := a.analyzer.GenerateContextChunks(analysis, sessionID)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			fmt.Sprintf("Context chunk generation failed: %v", err),
			nil,
		))
	}
	
	// Store chunks with appropriate embeddings
	var storedChunks []string
	for _, chunk := range chunks {
		// Skip if importance is below threshold
		if a.isImportanceBelowThreshold(chunk.Importance, importanceThreshold) {
			continue
		}
		
		// Generate ID and embedding
		chunk.ID = uuid.New().String()
		
		// Generate embedding for content
		embedding, err := a.embedder.GenerateEmbedding(chunk.Content)
		if err != nil {
			a.logger.WithError(err).Warn("Failed to generate embedding", map[string]interface{}{
				"chunk_id": chunk.ID,
				"session_id": sessionID,
			})
			continue
		}
		chunk.Embedding = embedding
		
		// Store chunk
		if err := a.storage.StoreChunk(ctx, chunk); err != nil {
			a.logger.WithError(err).Warn("Failed to store chunk", map[string]interface{}{
				"chunk_id": chunk.ID,
				"session_id": sessionID,
			})
			continue
		}
		
		storedChunks = append(storedChunks, chunk.ID)
	}
	
	// Update analysis with stored chunks
	analysis.StoredChunks = storedChunks
	
	// Log performance metrics
	totalLatency := time.Since(startTime)
	a.logger.Info("Project analysis completed", map[string]interface{}{
		"operation":         "auto_store_project",
		"session_id":        sessionID,
		"project_path":      projectPath,
		"language":          analysis.Language,
		"framework":         analysis.Framework,
		"architecture":      analysis.Architecture,
		"complexity_score":  analysis.Complexity,
		"chunks_stored":     len(storedChunks),
		"focus_areas":       len(focusAreas),
		"total_latency_ms":  totalLatency.Milliseconds(),
	})
	
	// Prepare response content based on silent mode
	var responseContent string
	
	if silent {
		// Minimal response for seamless operation
		responseContent = fmt.Sprintf("✅ Project analyzed: %d chunks stored (%dms)", 
			len(storedChunks), totalLatency.Milliseconds())
	} else {
		// Detailed response for debugging/verbose mode
		responseContent = fmt.Sprintf(`🚀 **Project Analysis Complete**

**Project**: %s
**Language**: %s
**Framework**: %s
**Architecture**: %s
**Complexity Score**: %.2f/1.0

**Stored Context Chunks**: %d
**Focus Areas**: %s

**Key Files** (%d):
%s

**API Endpoints** (%d):
%s

**Database Schema** (%d):
%s

✅ Project context is now available for intelligent retrieval
⏱️ Analysis time: %dms`,
		projectPath,
		analysis.Language,
		analysis.Framework,
		analysis.Architecture,
		analysis.Complexity,
		len(storedChunks),
		strings.Join(a.focusAreasToStrings(focusAreas), ", "),
		len(analysis.KeyFiles),
		strings.Join(analysis.KeyFiles[:min(5, len(analysis.KeyFiles))], "\n"),
		len(analysis.APIEndpoints),
		strings.Join(analysis.APIEndpoints[:min(10, len(analysis.APIEndpoints))], "\n"),
		len(analysis.DatabaseSchema),
		strings.Join(analysis.DatabaseSchema, "\n"),
		totalLatency.Milliseconds(),
	)
	}
	
	return mcp.NewResponse(id, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": responseContent,
			},
		},
	})
}

// handleContextAwareRetrieve handles the context_aware_retrieve tool
func (a *AIMem) handleContextAwareRetrieve(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	startTime := time.Now()
	
	// Extract parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}
	
	currentTask, ok := args["current_task"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"current_task must be a string",
			nil,
		))
	}
	
	taskTypeStr, ok := args["task_type"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"task_type must be a string",
			nil,
		))
	}
	taskType := types.TaskType(taskTypeStr)
	
	// Parse optional parameters
	autoExpand := false
	if expand, exists := args["auto_expand"].(bool); exists {
		autoExpand = expand
	}
	
	maxChunks := 5
	if max, exists := args["max_chunks"].(float64); exists {
		maxChunks = int(max)
	}
	
	contextDepth := 2
	if depth, exists := args["context_depth"].(float64); exists {
		contextDepth = int(depth)
	}
	
	a.logger.Info("Starting context-aware retrieval", map[string]interface{}{
		"session_id":     sessionID,
		"current_task":   currentTask,
		"task_type":      taskType,
		"auto_expand":    autoExpand,
		"max_chunks":     maxChunks,
		"context_depth":  contextDepth,
		"operation":      "context_aware_retrieve",
	})
	
	// Create enhanced query based on task type
	enhancedQuery := a.enhanceQueryByTaskType(currentTask, taskType)
	
	// Perform semantic search
	embedding, err := a.embedder.GenerateEmbedding(enhancedQuery)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			fmt.Sprintf("Failed to generate embedding: %v", err),
			nil,
		))
	}
	
	// Retrieve chunks
	chunks, err := a.storage.SearchByEmbedding(ctx, sessionID, embedding, maxChunks)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			fmt.Sprintf("Failed to retrieve context: %v", err),
			nil,
		))
	}
	
	// Calculate enhanced relevance scores
	for _, chunk := range chunks {
		chunk.Relevance = a.calculateTaskAwareRelevance(chunk, taskType, currentTask)
	}
	
	// Sort by enhanced relevance
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Relevance > chunks[j].Relevance
	})
	
	var relatedChunks []*types.ContextChunk
	var relationships []types.ContextRelationship
	
	// Auto-expand with related context if enabled
	if autoExpand && len(chunks) > 0 {
		relatedChunks, relationships = a.expandWithRelatedContext(chunks, contextDepth, maxChunks)
	}
	
	totalRelevance := 0.0
	for _, chunk := range chunks {
		totalRelevance += chunk.Relevance
	}
	
	totalLatency := time.Since(startTime)
	
	a.logger.Info("Context-aware retrieval completed", map[string]interface{}{
		"operation":             "context_aware_retrieve",
		"session_id":            sessionID,
		"task_type":             taskType,
		"chunks_found":          len(chunks),
		"related_chunks_found":  len(relatedChunks),
		"relationships_found":   len(relationships),
		"total_relevance":       totalRelevance,
		"total_latency_ms":      totalLatency.Milliseconds(),
		"auto_expand":           autoExpand,
	})
	
	// Build response
	var contentParts []string
	
	contentParts = append(contentParts, fmt.Sprintf(
		"🎯 **Context-Aware Retrieval**: %s\n\n**Task Type**: %s\n**Query Enhancement**: Applied\n**Auto-Expand**: %t\n\n",
		currentTask, taskType, autoExpand,
	))
	
	if len(chunks) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**Primary Context** (%d chunks):\n", len(chunks)))
		for i, chunk := range chunks {
			contentParts = append(contentParts, fmt.Sprintf(
				"**Chunk %d** (ID: %s, Relevance: %.3f)\n%s\n\n",
				i+1, chunk.ID, chunk.Relevance, chunk.Content,
			))
		}
	}
	
	if len(relatedChunks) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**Related Context** (%d chunks):\n", len(relatedChunks)))
		for i, chunk := range relatedChunks {
			contentParts = append(contentParts, fmt.Sprintf(
				"**Related %d** (ID: %s, Relevance: %.3f)\n%s\n\n",
				i+1, chunk.ID, chunk.Relevance, chunk.Content,
			))
		}
	}
	
	if len(relationships) > 0 {
		contentParts = append(contentParts, fmt.Sprintf("**Context Relationships** (%d found):\n", len(relationships)))
		for _, rel := range relationships {
			contentParts = append(contentParts, fmt.Sprintf(
				"- %s → Strength: %.3f (%s)\n",
				rel.ChunkID, rel.Strength, rel.Reason,
			))
		}
	}
	
	responseContent := strings.Join(contentParts, "")
	
	return mcp.NewResponse(id, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": responseContent,
			},
		},
	})
}

// handleSmartMemoryManager handles the smart_memory_manager tool
func (a *AIMem) handleSmartMemoryManager(ctx context.Context, id interface{}, args map[string]interface{}) *mcp.Response {
	startTime := time.Now()
	
	// Extract parameters
	sessionID, ok := args["session_id"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_id must be a string",
			nil,
		))
	}
	
	sessionPhaseStr, ok := args["session_phase"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"session_phase must be a string",
			nil,
		))
	}
	sessionPhase := types.SessionPhase(sessionPhaseStr)
	
	memoryStrategyStr, ok := args["memory_strategy"].(string)
	if !ok {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInvalidParams,
			"memory_strategy must be a string",
			nil,
		))
	}
	memoryStrategy := types.MemoryStrategy(memoryStrategyStr)
	
	preserveImportant := true
	if preserve, exists := args["preserve_important"].(bool); exists {
		preserveImportant = preserve
	}
	
	a.logger.Info("Starting smart memory management", map[string]interface{}{
		"session_id":         sessionID,
		"session_phase":      sessionPhase,
		"memory_strategy":    memoryStrategy,
		"preserve_important": preserveImportant,
		"operation":          "smart_memory_manager",
	})
	
	// Get current session stats
	stats, err := a.storage.GetSessionSummary(ctx, sessionID)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			fmt.Sprintf("Failed to get session stats: %v", err),
			nil,
		))
	}
	
	// Apply smart memory management strategy
	managementResult, err := a.applySmartMemoryStrategy(sessionID, sessionPhase, memoryStrategy, preserveImportant, stats)
	if err != nil {
		return mcp.NewErrorResponse(id, mcp.NewError(
			mcp.ErrorCodeInternalError,
			fmt.Sprintf("Smart memory management failed: %v", err),
			nil,
		))
	}
	
	totalLatency := time.Since(startTime)
	
	a.logger.Info("Smart memory management completed", map[string]interface{}{
		"operation":         "smart_memory_manager",
		"session_id":        sessionID,
		"session_phase":     sessionPhase,
		"memory_strategy":   memoryStrategy,
		"chunks_cleaned":    managementResult.ChunksCleaned,
		"memory_freed":      managementResult.MemoryFreed,
		"total_latency_ms":  totalLatency.Milliseconds(),
	})
	
	responseContent := fmt.Sprintf(`🧠 **Smart Memory Management Complete**

**Session Phase**: %s
**Strategy**: %s
**Preserve Important**: %t

**Results**:
- Chunks cleaned: %d
- Memory freed: %d bytes
- Chunks remaining: %d
- Average relevance: %.3f

**Strategy Applied**: %s
⏱️ Processing time: %dms

✅ Memory optimized for %s phase`,
		sessionPhase,
		memoryStrategy,
		preserveImportant,
		managementResult.ChunksCleaned,
		managementResult.MemoryFreed,
		managementResult.ChunksRemaining,
		managementResult.AverageRelevance,
		managementResult.Description,
		totalLatency.Milliseconds(),
		sessionPhase,
	)
	
	return mcp.NewResponse(id, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": responseContent,
			},
		},
	})
}

// Helper functions for smart context management

// isImportanceBelowThreshold checks if importance is below threshold
func (a *AIMem) isImportanceBelowThreshold(importance, threshold types.Importance) bool {
	importanceValues := map[types.Importance]int{
		types.ImportanceLow:    1,
		types.ImportanceMedium: 2,
		types.ImportanceHigh:   3,
	}
	return importanceValues[importance] < importanceValues[threshold]
}

// focusAreasToStrings converts focus areas to strings
func (a *AIMem) focusAreasToStrings(areas []types.FocusArea) []string {
	result := make([]string, len(areas))
	for i, area := range areas {
		result[i] = string(area)
	}
	return result
}

// enhanceQueryByTaskType enhances queries based on task type
func (a *AIMem) enhanceQueryByTaskType(query string, taskType types.TaskType) string {
	taskEnhancements := map[types.TaskType]string{
		types.TaskAnalysis:     "architecture structure design patterns code organization",
		types.TaskDevelopment:  "implementation code examples functions methods API",
		types.TaskDebugging:    "error handling exceptions logging debugging troubleshooting",
		types.TaskRefactoring:  "code quality structure patterns refactor improve optimize",
		types.TaskTesting:      "tests testing unit integration end-to-end validation",
		types.TaskDeployment:   "deployment configuration infrastructure setup production",
	}
	
	if enhancement, exists := taskEnhancements[taskType]; exists {
		return query + " " + enhancement
	}
	return query
}

// calculateTaskAwareRelevance calculates relevance based on task type
func (a *AIMem) calculateTaskAwareRelevance(chunk *types.ContextChunk, taskType types.TaskType, currentTask string) float64 {
	baseRelevance := chunk.Relevance
	
	// Task-specific boosters
	taskBoosts := map[types.TaskType]map[string]float64{
		types.TaskAnalysis: {
			"architecture": 1.2,
			"structure":    1.1,
			"design":       1.1,
			"pattern":      1.1,
		},
		types.TaskDevelopment: {
			"function":       1.2,
			"method":         1.2,
			"implementation": 1.3,
			"API":            1.1,
			"code":           1.1,
		},
		types.TaskDebugging: {
			"error":   1.3,
			"bug":     1.3,
			"issue":   1.2,
			"problem": 1.2,
			"fix":     1.1,
		},
	}
	
	// Apply task-specific boosts
	if boosts, exists := taskBoosts[taskType]; exists {
		content := strings.ToLower(chunk.Content)
		for keyword, boost := range boosts {
			if strings.Contains(content, keyword) {
				baseRelevance *= boost
				break // Apply only one boost per chunk
			}
		}
	}
	
	// Importance boost
	importanceBoost := map[types.Importance]float64{
		types.ImportanceHigh:   1.2,
		types.ImportanceMedium: 1.0,
		types.ImportanceLow:    0.8,
	}
	
	if boost, exists := importanceBoost[chunk.Importance]; exists {
		baseRelevance *= boost
	}
	
	// Recency boost (newer chunks get slight boost)
	age := time.Since(chunk.Timestamp).Hours()
	if age < 1 {
		baseRelevance *= 1.1
	} else if age < 24 {
		baseRelevance *= 1.05
	}
	
	return math.Min(baseRelevance, 1.0) // Cap at 1.0
}

// expandWithRelatedContext finds related context chunks
func (a *AIMem) expandWithRelatedContext(primaryChunks []*types.ContextChunk, depth, maxTotal int) ([]*types.ContextChunk, []types.ContextRelationship) {
	var relatedChunks []*types.ContextChunk
	var relationships []types.ContextRelationship
	
	// Simple implementation - in production, this would use more sophisticated relationship detection
	for _, primaryChunk := range primaryChunks {
		// Find chunks with similar keywords
		keywords := a.extractKeywords(primaryChunk.Content)
		
		for _, keyword := range keywords[:min(3, len(keywords))] {
			if len(relatedChunks) >= maxTotal/2 {
				break
			}
			
			// This is simplified - would use semantic similarity in production
			embedding, err := a.embedder.GenerateEmbedding(keyword)
			if err != nil {
				continue
			}
			
			chunks, err := a.storage.SearchByEmbedding(context.TODO(), primaryChunk.SessionID, embedding, 2)
			if err != nil {
				continue
			}
			
			for _, chunk := range chunks {
				if chunk.ID != primaryChunk.ID && !a.containsChunk(relatedChunks, chunk.ID) {
					relatedChunks = append(relatedChunks, chunk)
					relationships = append(relationships, types.ContextRelationship{
						ChunkID:       primaryChunk.ID,
						RelatedChunks: []string{chunk.ID},
						Strength:      chunk.Relevance * 0.8,
						Reason:        fmt.Sprintf("Keyword similarity: %s", keyword),
					})
					break
				}
			}
		}
	}
	
	return relatedChunks, relationships
}

// extractKeywords extracts key terms from content
func (a *AIMem) extractKeywords(content string) []string {
	// Simplified keyword extraction - would use NLP in production
	words := strings.Fields(strings.ToLower(content))
	keywords := make(map[string]int)
	
	// Skip common words
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true,
		"on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"must": true, "can": true, "a": true, "an": true, "this": true,
		"that": true, "these": true, "those": true,
	}
	
	for _, word := range words {
		if len(word) > 3 && !stopWords[word] {
			keywords[word]++
		}
	}
	
	// Convert to sorted slice
	type kv struct {
		key   string
		value int
	}
	
	var kvSlice []kv
	for k, v := range keywords {
		kvSlice = append(kvSlice, kv{k, v})
	}
	
	sort.Slice(kvSlice, func(i, j int) bool {
		return kvSlice[i].value > kvSlice[j].value
	})
	
	result := make([]string, len(kvSlice))
	for i, kv := range kvSlice {
		result[i] = kv.key
	}
	
	return result
}

// containsChunk checks if a chunk ID exists in slice
func (a *AIMem) containsChunk(chunks []*types.ContextChunk, id string) bool {
	for _, chunk := range chunks {
		if chunk.ID == id {
			return true
		}
	}
	return false
}

// SmartMemoryResult represents the result of smart memory management
type SmartMemoryResult struct {
	ChunksCleaned     int     `json:"chunks_cleaned"`
	MemoryFreed       int64   `json:"memory_freed"`
	ChunksRemaining   int     `json:"chunks_remaining"`
	AverageRelevance  float64 `json:"average_relevance"`
	Description       string  `json:"description"`
}

// applySmartMemoryStrategy applies memory management strategy
func (a *AIMem) applySmartMemoryStrategy(sessionID string, phase types.SessionPhase, strategy types.MemoryStrategy, preserveImportant bool, stats *types.SessionSummary) (*SmartMemoryResult, error) {
	// Get all chunks for the session
	chunks, err := a.storage.SearchByEmbedding(context.TODO(), sessionID, nil, 1000)
	if err != nil {
		return nil, err
	}
	
	originalCount := len(chunks)
	_ = stats.MemoryUsage // unused for now
	
	// Determine cleanup ratio based on strategy and phase
	cleanupRatio := a.getCleanupRatio(strategy, phase)
	
	// Calculate how many chunks to remove
	maxRemove := int(float64(len(chunks)) * cleanupRatio)
	if maxRemove == 0 && len(chunks) > 10 {
		maxRemove = 1 // Remove at least one if we have many chunks
	}
	
	var chunksToRemove []*types.ContextChunk
	
	// Select chunks to remove based on strategy
	switch strategy {
	case types.MemoryAggressive:
		chunksToRemove = a.selectAggressiveCleanup(chunks, maxRemove, preserveImportant)
	case types.MemoryBalanced:
		chunksToRemove = a.selectBalancedCleanup(chunks, maxRemove, preserveImportant)
	case types.MemoryConservative:
		chunksToRemove = a.selectConservativeCleanup(chunks, maxRemove, preserveImportant)
	}
	
	// Remove selected chunks
	var memoryFreed int64
	for _, chunk := range chunksToRemove {
		if err := a.storage.DeleteChunk(context.TODO(), chunk.ID); err != nil {
			a.logger.WithError(err).Warn("Failed to remove chunk during smart cleanup")
			continue
		}
		memoryFreed += int64(len(chunk.Content))
	}
	
	// Calculate remaining stats
	remainingChunks := originalCount - len(chunksToRemove)
	
	// Calculate average relevance of remaining chunks
	avgRelevance := 0.0
	if remainingChunks > 0 {
		totalRelevance := 0.0
		for _, chunk := range chunks {
			found := false
			for _, removed := range chunksToRemove {
				if chunk.ID == removed.ID {
					found = true
					break
				}
			}
			if !found {
				totalRelevance += chunk.Relevance
			}
		}
		avgRelevance = totalRelevance / float64(remainingChunks)
	}
	
	description := fmt.Sprintf("Applied %s strategy for %s phase, %s important chunks",
		strategy, phase, map[bool]string{true: "preserving", false: "not preserving"}[preserveImportant])
	
	return &SmartMemoryResult{
		ChunksCleaned:    len(chunksToRemove),
		MemoryFreed:      memoryFreed,
		ChunksRemaining:  remainingChunks,
		AverageRelevance: avgRelevance,
		Description:      description,
	}, nil
}

// getCleanupRatio returns cleanup ratio based on strategy and phase
func (a *AIMem) getCleanupRatio(strategy types.MemoryStrategy, phase types.SessionPhase) float64 {
	ratios := map[types.MemoryStrategy]map[types.SessionPhase]float64{
		types.MemoryAggressive: {
			types.PhaseAnalysis:    0.3,
			types.PhaseDevelopment: 0.2,
			types.PhaseTesting:     0.4,
			types.PhaseDeployment:  0.1,
		},
		types.MemoryBalanced: {
			types.PhaseAnalysis:    0.2,
			types.PhaseDevelopment: 0.1,
			types.PhaseTesting:     0.25,
			types.PhaseDeployment:  0.05,
		},
		types.MemoryConservative: {
			types.PhaseAnalysis:    0.1,
			types.PhaseDevelopment: 0.05,
			types.PhaseTesting:     0.15,
			types.PhaseDeployment:  0.0,
		},
	}
	
	if phaseRatios, exists := ratios[strategy]; exists {
		if ratio, exists := phaseRatios[phase]; exists {
			return ratio
		}
	}
	
	return 0.1 // Default conservative cleanup
}

// selectAggressiveCleanup selects chunks for aggressive cleanup
func (a *AIMem) selectAggressiveCleanup(chunks []*types.ContextChunk, maxRemove int, preserveImportant bool) []*types.ContextChunk {
	var candidates []*types.ContextChunk
	
	for _, chunk := range chunks {
		if preserveImportant && chunk.Importance == types.ImportanceHigh {
			continue
		}
		candidates = append(candidates, chunk)
	}
	
	// Sort by relevance and age (prefer removing old, low-relevance chunks)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Relevance != candidates[j].Relevance {
			return candidates[i].Relevance < candidates[j].Relevance
		}
		return candidates[i].Timestamp.Before(candidates[j].Timestamp)
	})
	
	if maxRemove > len(candidates) {
		maxRemove = len(candidates)
	}
	
	return candidates[:maxRemove]
}

// selectBalancedCleanup selects chunks for balanced cleanup
func (a *AIMem) selectBalancedCleanup(chunks []*types.ContextChunk, maxRemove int, preserveImportant bool) []*types.ContextChunk {
	var candidates []*types.ContextChunk
	
	for _, chunk := range chunks {
		if preserveImportant && chunk.Importance == types.ImportanceHigh {
			continue
		}
		// In balanced mode, also preserve medium importance if very recent
		if chunk.Importance == types.ImportanceMedium && time.Since(chunk.Timestamp) < time.Hour {
			continue
		}
		candidates = append(candidates, chunk)
	}
	
	// Sort by combined score (relevance + age + importance)
	sort.Slice(candidates, func(i, j int) bool {
		scoreI := a.calculateCleanupScore(candidates[i])
		scoreJ := a.calculateCleanupScore(candidates[j])
		return scoreI < scoreJ // Lower score = more likely to be removed
	})
	
	if maxRemove > len(candidates) {
		maxRemove = len(candidates)
	}
	
	return candidates[:maxRemove]
}

// selectConservativeCleanup selects chunks for conservative cleanup
func (a *AIMem) selectConservativeCleanup(chunks []*types.ContextChunk, maxRemove int, preserveImportant bool) []*types.ContextChunk {
	var candidates []*types.ContextChunk
	
	for _, chunk := range chunks {
		if preserveImportant && chunk.Importance != types.ImportanceLow {
			continue
		}
		// Only remove very old, low-relevance, low-importance chunks
		if chunk.Relevance < 0.3 && chunk.Importance == types.ImportanceLow && 
		   time.Since(chunk.Timestamp) > 24*time.Hour {
			candidates = append(candidates, chunk)
		}
	}
	
	// Sort by age (oldest first in conservative mode)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Timestamp.Before(candidates[j].Timestamp)
	})
	
	if maxRemove > len(candidates) {
		maxRemove = len(candidates)
	}
	
	return candidates[:maxRemove]
}

// calculateCleanupScore calculates a score for cleanup prioritization
func (a *AIMem) calculateCleanupScore(chunk *types.ContextChunk) float64 {
	score := 0.0
	
	// Relevance component (lower is more likely to be removed)
	score += (1.0 - chunk.Relevance) * 0.4
	
	// Age component (older is more likely to be removed)
	ageHours := time.Since(chunk.Timestamp).Hours()
	ageScore := math.Min(ageHours/24.0, 1.0) // Normalize to 0-1 over 24 hours
	score += ageScore * 0.3
	
	// Importance component
	importanceScore := map[types.Importance]float64{
		types.ImportanceHigh:   0.0,
		types.ImportanceMedium: 0.5,
		types.ImportanceLow:    1.0,
	}
	score += importanceScore[chunk.Importance] * 0.3
	
	return score
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Close gracefully shuts down the AIMem server
func (a *AIMem) Close() error {
	a.logger.Info("Shutting down AIMem server")
	
	if err := a.storage.Close(); err != nil {
		a.logger.WithError(err).Error("Error closing storage connection")
		return err
	}
	
	a.logger.Info("AIMem server shutdown complete")
	return nil
}