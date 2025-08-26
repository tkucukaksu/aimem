package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/tarkank/aimem/internal/types"
)

// TokenEstimator provides token count estimation for text content
type TokenEstimator struct {
	// Average characters per token (GPT-style tokenization approximation)
	CharPerToken float64
}

// NewTokenEstimator creates a new token estimator
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		CharPerToken: 4.0, // Conservative estimate: ~4 characters per token
	}
}

// EstimateTokens estimates token count for text content
func (te *TokenEstimator) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	
	charCount := len(text)
	
	// Basic token estimation based on character count
	baseTokens := float64(charCount) / te.CharPerToken
	
	// Adjust for different content types
	tokenCount := te.adjustForContentType(text, baseTokens)
	
	return int(math.Ceil(tokenCount))
}

// EstimateTokensForResponse estimates tokens for entire MCP response structure
func (te *TokenEstimator) EstimateTokensForResponse(content interface{}) int {
	// Convert to JSON to get a realistic byte estimate
	jsonBytes, err := json.Marshal(content)
	if err != nil {
		// Fallback: estimate based on string representation
		return te.EstimateTokens(fmt.Sprintf("%v", content))
	}
	
	// Add overhead for JSON structure (~20% more tokens than raw content)
	rawTokens := te.EstimateTokens(string(jsonBytes))
	return int(float64(rawTokens) * 1.2)
}

// adjustForContentType applies content-specific token estimation adjustments
func (te *TokenEstimator) adjustForContentType(text string, baseTokens float64) float64 {
	// Code content tends to have more tokens per character
	codeIndicators := []string{"{", "}", "(", ")", "function", "class", "import", "const"}
	codeScore := 0
	
	lowerText := strings.ToLower(text)
	for _, indicator := range codeIndicators {
		if strings.Contains(lowerText, indicator) {
			codeScore++
		}
	}
	
	// If text seems to contain code, increase token estimate by 10-30%
	if codeScore >= 3 {
		return baseTokens * 1.3
	} else if codeScore >= 1 {
		return baseTokens * 1.1
	}
	
	return baseTokens
}

// ResponseLimiter handles response size limiting and pagination
type ResponseLimiter struct {
	estimator *TokenEstimator
	config    types.ResponseConfig
}

// NewResponseLimiter creates a new response limiter with default configuration
func NewResponseLimiter() *ResponseLimiter {
	return &ResponseLimiter{
		estimator: NewTokenEstimator(),
		config: types.ResponseConfig{
			MaxTokens:       20000, // Stay well below 25K limit
			EnablePaging:    true,
			PageSize:        10,    // Chunks per page
			TruncateContent: true,
		},
	}
}

// NewResponseLimiterWithConfig creates a response limiter with custom configuration
func NewResponseLimiterWithConfig(config types.ResponseConfig) *ResponseLimiter {
	return &ResponseLimiter{
		estimator: NewTokenEstimator(),
		config:    config,
	}
}

// LimitContextAwareRetrievalResponse limits the size of context aware retrieval responses
func (rl *ResponseLimiter) LimitContextAwareRetrievalResponse(
	primaryChunks []*types.ContextChunk,
	relatedChunks []*types.ContextChunk,
	relationships []types.ContextRelationship,
	retrievalReason string,
	totalRelevance float64,
	processingTime int64,
	page int,
) *types.PaginatedRetrievalResult {
	
	result := &types.PaginatedRetrievalResult{
		RetrievalReason:  retrievalReason,
		TotalRelevance:   totalRelevance,
		ProcessingTimeMs: processingTime,
		TokenLimits: types.TokenLimits{
			MaxResponseTokens: rl.config.MaxTokens,
		},
	}
	
	// Use iterative approach to fit content within token budget
	result.PrimaryChunks, result.RelatedChunks, result.Relationships, result.Paging = 
		rl.fitContentWithinBudget(primaryChunks, relatedChunks, relationships, page)
	
	// Calculate final token estimate
	result.TokenLimits.EstimatedTokens = rl.estimator.EstimateTokensForResponse(result)
	result.TokenLimits.TruncatedContent = len(result.PrimaryChunks) < len(primaryChunks) ||
		len(result.RelatedChunks) < len(relatedChunks) ||
		len(result.Relationships) < len(relationships)
	
	return result
}

// fitContentWithinBudget uses iterative approach to fit content within token budget
func (rl *ResponseLimiter) fitContentWithinBudget(
	primaryChunks []*types.ContextChunk,
	relatedChunks []*types.ContextChunk,
	relationships []types.ContextRelationship,
	page int,
) ([]types.ContextChunk, []types.ContextChunk, []types.ContextRelationship, *types.ResponsePaging) {
	
	// Start with minimal structure to calculate base overhead
	baseStructure := &types.PaginatedRetrievalResult{
		PrimaryChunks:    []types.ContextChunk{},
		RelatedChunks:    []types.ContextChunk{},
		Relationships:    []types.ContextRelationship{},
		RetrievalReason:  "Base structure overhead calculation",
		TotalRelevance:   0.0,
		ProcessingTimeMs: 0,
		TokenLimits: types.TokenLimits{
			MaxResponseTokens: rl.config.MaxTokens,
		},
	}
	
	baseOverhead := rl.estimator.EstimateTokensForResponse(baseStructure)
	availableTokens := rl.config.MaxTokens - baseOverhead - 200 // Reserve 200 for safety
	
	if availableTokens <= 500 {
		// Not enough tokens for meaningful content
		return []types.ContextChunk{}, []types.ContextChunk{}, []types.ContextRelationship{}, nil
	}
	
	// Iteratively build response within token budget
	var resultPrimary []types.ContextChunk
	var resultRelated []types.ContextChunk
	var resultRelationships []types.ContextRelationship
	var paging *types.ResponsePaging
	
	// Phase 1: Add primary chunks (priority allocation: 60%)
	primaryBudget := int(float64(availableTokens) * 0.6)
	if rl.config.EnablePaging {
		resultPrimary, paging = rl.paginateChunks(primaryChunks, page, "primary", primaryBudget)
	} else {
		resultPrimary = rl.limitChunksToTokenBudget(primaryChunks, primaryBudget)
	}
	
	usedTokens := rl.estimateChunksTokens(resultPrimary)
	remainingBudget := availableTokens - usedTokens
	
	// Phase 2: Add related chunks if budget allows (priority allocation: 30%)
	if remainingBudget > 200 {
		relatedBudget := min(remainingBudget/2, int(float64(availableTokens)*0.3))
		if len(relatedChunks) > 0 {
			if rl.config.EnablePaging {
				resultRelated, _ = rl.paginateChunks(relatedChunks, 1, "related", relatedBudget)
			} else {
				resultRelated = rl.limitChunksToTokenBudget(relatedChunks, relatedBudget)
			}
			
			usedTokens += rl.estimateChunksTokens(resultRelated)
			remainingBudget = availableTokens - usedTokens
		}
	}
	
	// Phase 3: Add relationships if budget allows (priority allocation: 10%)
	if remainingBudget > 100 && len(relationships) > 0 {
		relationshipBudget := min(remainingBudget, int(float64(availableTokens)*0.1))
		resultRelationships = rl.limitRelationshipsToTokenBudget(relationships, relationshipBudget)
	}
	
	return resultPrimary, resultRelated, resultRelationships, paging
}

// paginateChunks implements pagination for chunk arrays
func (rl *ResponseLimiter) paginateChunks(
	chunks []*types.ContextChunk, 
	page int, 
	chunkType string,
	tokenBudget int,
) ([]types.ContextChunk, *types.ResponsePaging) {
	
	if len(chunks) == 0 {
		return []types.ContextChunk{}, nil
	}
	
	// First, try to fit chunks within token budget
	fittingChunks := rl.limitChunksToTokenBudget(chunks, tokenBudget)
	
	// If pagination is not needed (all chunks fit), return all
	if len(fittingChunks) == len(chunks) && len(fittingChunks) <= rl.config.PageSize {
		return fittingChunks, nil
	}
	
	// Calculate pagination
	pageSize := rl.config.PageSize
	totalItems := len(fittingChunks)
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	
	// Get page slice
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if endIdx > len(fittingChunks) {
		endIdx = len(fittingChunks)
	}
	
	pageChunks := make([]types.ContextChunk, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		pageChunks[i-startIdx] = fittingChunks[i]
	}
	
	paging := &types.ResponsePaging{
		PageSize:    pageSize,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  totalItems,
		HasMore:     page < totalPages,
	}
	
	if paging.HasMore {
		paging.NextPageToken = fmt.Sprintf("%s_page_%d", chunkType, page+1)
	}
	
	return pageChunks, paging
}

// limitChunksToTokenBudget fits as many chunks as possible within token budget
func (rl *ResponseLimiter) limitChunksToTokenBudget(chunks []*types.ContextChunk, tokenBudget int) []types.ContextChunk {
	if len(chunks) == 0 || tokenBudget <= 0 {
		return []types.ContextChunk{}
	}
	
	result := make([]types.ContextChunk, 0)
	usedTokens := 0
	
	for _, chunk := range chunks {
		chunkTokens := rl.estimateChunkTokens(chunk)
		
		if usedTokens+chunkTokens > tokenBudget {
			// If this chunk would exceed budget, check if we can truncate it
			if rl.config.TruncateContent && len(result) == 0 {
				// If this is the first chunk and we can truncate, do so
				truncatedChunk := rl.truncateChunk(chunk, tokenBudget-50) // Reserve 50 tokens for structure
				if truncatedChunk != nil {
					result = append(result, *truncatedChunk)
				}
			}
			break
		}
		
		result = append(result, *chunk)
		usedTokens += chunkTokens
	}
	
	return result
}

// limitRelationshipsToTokenBudget fits relationships within token budget
func (rl *ResponseLimiter) limitRelationshipsToTokenBudget(relationships []types.ContextRelationship, tokenBudget int) []types.ContextRelationship {
	if len(relationships) == 0 || tokenBudget <= 0 {
		return []types.ContextRelationship{}
	}
	
	result := make([]types.ContextRelationship, 0)
	usedTokens := 0
	
	for _, rel := range relationships {
		relTokens := rl.estimator.EstimateTokens(fmt.Sprintf("%s %s %.3f", rel.ChunkID, rel.Reason, rel.Strength))
		
		if usedTokens+relTokens > tokenBudget {
			break
		}
		
		result = append(result, rel)
		usedTokens += relTokens
	}
	
	return result
}

// estimateChunksTokens estimates total tokens for chunk array
func (rl *ResponseLimiter) estimateChunksTokens(chunks []types.ContextChunk) int {
	total := 0
	for _, chunk := range chunks {
		total += rl.estimateChunkTokens(&chunk)
	}
	return total
}

// estimateChunkTokens estimates tokens for a single chunk
func (rl *ResponseLimiter) estimateChunkTokens(chunk *types.ContextChunk) int {
	contentTokens := rl.estimator.EstimateTokens(chunk.Content)
	summaryTokens := rl.estimator.EstimateTokens(chunk.Summary)
	
	// Add overhead for JSON structure (ID, metadata, etc.)
	overhead := 50
	
	return contentTokens + summaryTokens + overhead
}

// truncateChunk truncates a chunk to fit within token budget
func (rl *ResponseLimiter) truncateChunk(chunk *types.ContextChunk, tokenBudget int) *types.ContextChunk {
	if tokenBudget <= 100 { // Need minimum tokens for structure
		return nil
	}
	
	// Reserve tokens for summary and metadata
	summaryTokens := rl.estimator.EstimateTokens(chunk.Summary)
	overhead := 50
	
	availableForContent := tokenBudget - summaryTokens - overhead
	if availableForContent <= 50 {
		return nil
	}
	
	// Calculate maximum characters for content
	maxChars := int(float64(availableForContent) * rl.estimator.CharPerToken * 0.8) // 80% safety margin
	
	if len(chunk.Content) <= maxChars {
		return chunk
	}
	
	// Truncate content at word boundary
	truncatedContent := chunk.Content[:maxChars]
	if lastSpace := strings.LastIndex(truncatedContent, " "); lastSpace > maxChars/2 {
		truncatedContent = truncatedContent[:lastSpace]
	}
	
	truncatedContent += "... [truncated]"
	
	truncatedChunk := *chunk
	truncatedChunk.Content = truncatedContent
	
	return &truncatedChunk
}