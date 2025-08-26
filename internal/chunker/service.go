package chunker

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

// Service provides intelligent context chunking with semantic awareness
type Service struct {
	config *Config
	logger *logrus.Logger
}

// Config contains configuration for the chunking service
type Config struct {
	MaxChunkSize    int     `yaml:"max_chunk_size"`
	OverlapSize     int     `yaml:"overlap_size"`
	MinChunkSize    int     `yaml:"min_chunk_size"`
	SentenceWeight  float64 `yaml:"sentence_weight"`
	ParagraphWeight float64 `yaml:"paragraph_weight"`
	CodeWeight      float64 `yaml:"code_weight"`
}

// ChunkInfo contains metadata about a chunk
type ChunkInfo struct {
	Content       string  `json:"content"`
	StartOffset   int     `json:"start_offset"`
	EndOffset     int     `json:"end_offset"`
	ChunkIndex    int     `json:"chunk_index"`
	ContentType   string  `json:"content_type"`
	SemanticScore float64 `json:"semantic_score"`
}

// ContentType represents different types of content for specialized chunking
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeCode     ContentType = "code"
	ContentTypeMarkdown ContentType = "markdown"
	ContentTypeJSON     ContentType = "json"
	ContentTypeXML      ContentType = "xml"
)

// NewService creates a new chunking service
func NewService(config *Config, logger *logrus.Logger) *Service {
	if config == nil {
		config = &Config{
			MaxChunkSize:    1024,
			OverlapSize:     100,
			MinChunkSize:    50,
			SentenceWeight:  1.0,
			ParagraphWeight: 0.8,
			CodeWeight:      0.9,
		}
	}

	if logger == nil {
		logger = logrus.New()
	}

	return &Service{
		config: config,
		logger: logger,
	}
}

// ChunkContent splits content into semantic chunks with overlap
func (s *Service) ChunkContent(content string, maxSize int) ([]string, error) {
	if content == "" {
		return []string{}, nil
	}

	if maxSize <= 0 {
		maxSize = s.config.MaxChunkSize
	}

	chunks, err := s.ChunkContentWithInfo(content, maxSize)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(chunks))
	for i, chunk := range chunks {
		result[i] = chunk.Content
	}

	return result, nil
}

// ChunkContentWithInfo splits content and returns detailed chunk information
func (s *Service) ChunkContentWithInfo(content string, maxSize int) ([]ChunkInfo, error) {
	if content == "" {
		return []ChunkInfo{}, nil
	}

	contentType := s.detectContentType(content)
	s.logger.WithFields(logrus.Fields{
		"content_length": len(content),
		"content_type":   contentType,
		"max_size":       maxSize,
	}).Debug("Starting content chunking")

	var chunks []ChunkInfo
	var err error

	switch contentType {
	case ContentTypeCode:
		chunks, err = s.chunkCode(content, maxSize)
	case ContentTypeMarkdown:
		chunks, err = s.chunkMarkdown(content, maxSize)
	case ContentTypeJSON:
		chunks, err = s.chunkJSON(content, maxSize)
	default:
		chunks, err = s.chunkText(content, maxSize)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to chunk content: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"chunk_count":    len(chunks),
		"content_type":   contentType,
		"total_content":  len(content),
		"avg_chunk_size": s.calculateAverageChunkSize(chunks),
	}).Debug("Content chunking completed")

	return chunks, nil
}

// chunkText handles general text chunking with semantic awareness
func (s *Service) chunkText(content string, maxSize int) ([]ChunkInfo, error) {
	if len(content) <= maxSize {
		return []ChunkInfo{{
			Content:       content,
			StartOffset:   0,
			EndOffset:     len(content),
			ChunkIndex:    0,
			ContentType:   string(ContentTypeText),
			SemanticScore: 1.0,
		}}, nil
	}

	var chunks []ChunkInfo
	sentences := s.splitIntoSentences(content)
	
	currentChunk := ""
	currentOffset := 0
	chunkIndex := 0

	for _, sentence := range sentences {
		// Check if adding this sentence would exceed max size
		if len(currentChunk)+len(sentence) > maxSize && currentChunk != "" {
			// Finalize current chunk
			chunk := ChunkInfo{
				Content:       strings.TrimSpace(currentChunk),
				StartOffset:   currentOffset,
				EndOffset:     currentOffset + len(currentChunk),
				ChunkIndex:    chunkIndex,
				ContentType:   string(ContentTypeText),
				SemanticScore: s.calculateSemanticScore(currentChunk, ContentTypeText),
			}
			chunks = append(chunks, chunk)

			// Start new chunk with overlap
			overlap := s.getOverlap(currentChunk, s.config.OverlapSize)
			currentChunk = overlap + sentence
			currentOffset = chunk.EndOffset - len(overlap)
			chunkIndex++
		} else {
			currentChunk += sentence
		}
	}

	// Add final chunk if any content remains
	if strings.TrimSpace(currentChunk) != "" {
		chunk := ChunkInfo{
			Content:       strings.TrimSpace(currentChunk),
			StartOffset:   currentOffset,
			EndOffset:     currentOffset + len(currentChunk),
			ChunkIndex:    chunkIndex,
			ContentType:   string(ContentTypeText),
			SemanticScore: s.calculateSemanticScore(currentChunk, ContentTypeText),
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// chunkCode handles code-specific chunking with syntax awareness
func (s *Service) chunkCode(content string, maxSize int) ([]ChunkInfo, error) {
	if len(content) <= maxSize {
		return []ChunkInfo{{
			Content:       content,
			StartOffset:   0,
			EndOffset:     len(content),
			ChunkIndex:    0,
			ContentType:   string(ContentTypeCode),
			SemanticScore: 1.0,
		}}, nil
	}

	var chunks []ChunkInfo
	lines := strings.Split(content, "\n")
	
	currentChunk := ""
	currentOffset := 0
	chunkIndex := 0
	lineOffset := 0

	for i, line := range lines {
		lineWithNewline := line
		if i < len(lines)-1 {
			lineWithNewline += "\n"
		}

		// Check if adding this line would exceed max size
		if len(currentChunk)+len(lineWithNewline) > maxSize && currentChunk != "" {
			// Try to find a good breaking point (function boundary, class, etc.)
			breakPoint := s.findCodeBreakPoint(currentChunk)
			
			chunk := ChunkInfo{
				Content:       currentChunk[:breakPoint],
				StartOffset:   currentOffset,
				EndOffset:     currentOffset + breakPoint,
				ChunkIndex:    chunkIndex,
				ContentType:   string(ContentTypeCode),
				SemanticScore: s.calculateSemanticScore(currentChunk, ContentTypeCode),
			}
			chunks = append(chunks, chunk)

			// Start new chunk with remaining content plus overlap
			remaining := currentChunk[breakPoint:]
			overlap := s.getCodeOverlap(currentChunk[:breakPoint], s.config.OverlapSize)
			currentChunk = overlap + remaining + lineWithNewline
			currentOffset = currentOffset + breakPoint - len(overlap)
			chunkIndex++
		} else {
			currentChunk += lineWithNewline
		}
		lineOffset += len(lineWithNewline)
	}

	// Add final chunk
	if strings.TrimSpace(currentChunk) != "" {
		chunk := ChunkInfo{
			Content:       currentChunk,
			StartOffset:   currentOffset,
			EndOffset:     currentOffset + len(currentChunk),
			ChunkIndex:    chunkIndex,
			ContentType:   string(ContentTypeCode),
			SemanticScore: s.calculateSemanticScore(currentChunk, ContentTypeCode),
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// chunkMarkdown handles markdown-specific chunking preserving structure
func (s *Service) chunkMarkdown(content string, maxSize int) ([]ChunkInfo, error) {
	if len(content) <= maxSize {
		return []ChunkInfo{{
			Content:       content,
			StartOffset:   0,
			EndOffset:     len(content),
			ChunkIndex:    0,
			ContentType:   string(ContentTypeMarkdown),
			SemanticScore: 1.0,
		}}, nil
	}

	// Split by headers first, then by paragraphs
	sections := s.splitMarkdownSections(content)
	var chunks []ChunkInfo
	chunkIndex := 0
	offset := 0

	for _, section := range sections {
		if len(section) <= maxSize {
			chunk := ChunkInfo{
				Content:       section,
				StartOffset:   offset,
				EndOffset:     offset + len(section),
				ChunkIndex:    chunkIndex,
				ContentType:   string(ContentTypeMarkdown),
				SemanticScore: s.calculateSemanticScore(section, ContentTypeMarkdown),
			}
			chunks = append(chunks, chunk)
			chunkIndex++
		} else {
			// Further split large sections
			subChunks, err := s.chunkText(section, maxSize)
			if err != nil {
				return nil, err
			}
			for _, subChunk := range subChunks {
				chunk := ChunkInfo{
					Content:       subChunk.Content,
					StartOffset:   offset + subChunk.StartOffset,
					EndOffset:     offset + subChunk.EndOffset,
					ChunkIndex:    chunkIndex,
					ContentType:   string(ContentTypeMarkdown),
					SemanticScore: s.calculateSemanticScore(subChunk.Content, ContentTypeMarkdown),
				}
				chunks = append(chunks, chunk)
				chunkIndex++
			}
		}
		offset += len(section)
	}

	return chunks, nil
}

// chunkJSON handles JSON-specific chunking preserving structure
func (s *Service) chunkJSON(content string, maxSize int) ([]ChunkInfo, error) {
	if len(content) <= maxSize {
		return []ChunkInfo{{
			Content:       content,
			StartOffset:   0,
			EndOffset:     len(content),
			ChunkIndex:    0,
			ContentType:   string(ContentTypeJSON),
			SemanticScore: 1.0,
		}}, nil
	}

	// For JSON, we need to be careful to maintain valid structure
	// This is a simple approach - in production, you'd use a JSON parser
	return s.chunkText(content, maxSize)
}

// detectContentType determines the type of content for optimal chunking
func (s *Service) detectContentType(content string) ContentType {
	content = strings.TrimSpace(content)
	
	// Check for code patterns
	codePatterns := []string{
		`package\s+\w+`,     // Go
		`import\s+`,         // Go, Python, Java
		`function\s+\w+`,    // JavaScript
		`def\s+\w+`,         // Python
		`class\s+\w+`,       // Multiple languages
		`#include\s+`,       // C/C++
		`public\s+class`,    // Java
	}
	
	for _, pattern := range codePatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return ContentTypeCode
		}
	}
	
	// Check for markdown
	if strings.Contains(content, "# ") || strings.Contains(content, "## ") ||
		strings.Contains(content, "```") || strings.Contains(content, "**") {
		return ContentTypeMarkdown
	}
	
	// Check for JSON
	if (strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}")) ||
		(strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]")) {
		return ContentTypeJSON
	}
	
	// Check for XML
	if strings.HasPrefix(content, "<") && strings.Contains(content, ">") {
		return ContentTypeXML
	}
	
	return ContentTypeText
}

// splitIntoSentences splits text into sentences with better boundary detection
func (s *Service) splitIntoSentences(text string) []string {
	// Simple sentence splitting - in production, use a proper NLP library
	sentenceRegex := regexp.MustCompile(`[.!?]+\s+`)
	sentences := sentenceRegex.Split(text, -1)
	
	// Reconstruct with punctuation
	var result []string
	matches := sentenceRegex.FindAllString(text, -1)
	
	for i, sentence := range sentences {
		if i < len(matches) {
			result = append(result, sentence+matches[i])
		} else {
			result = append(result, sentence)
		}
	}
	
	return result
}

// splitMarkdownSections splits markdown into logical sections
func (s *Service) splitMarkdownSections(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var currentSection strings.Builder
	
	for _, line := range lines {
		// Check if this line starts a new section (header)
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			if currentSection.Len() > 0 {
				sections = append(sections, currentSection.String())
				currentSection.Reset()
			}
		}
		
		currentSection.WriteString(line)
		currentSection.WriteString("\n")
	}
	
	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}
	
	return sections
}

// findCodeBreakPoint finds a good place to break code
func (s *Service) findCodeBreakPoint(code string) int {
	lines := strings.Split(code, "\n")
	
	// Look for function/class boundaries
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "}" || line == "" {
			// Calculate byte offset
			offset := 0
			for j := 0; j <= i; j++ {
				offset += len(lines[j]) + 1 // +1 for newline
			}
			return offset
		}
	}
	
	return len(code)
}

// getOverlap gets an appropriate overlap from the end of a chunk
func (s *Service) getOverlap(chunk string, overlapSize int) string {
	if overlapSize <= 0 || len(chunk) <= overlapSize {
		return ""
	}
	
	// Try to get overlap at sentence boundary
	sentences := s.splitIntoSentences(chunk)
	if len(sentences) > 1 {
		lastSentence := sentences[len(sentences)-1]
		if len(lastSentence) <= overlapSize {
			return lastSentence
		}
	}
	
	// Fall back to character-based overlap
	return chunk[len(chunk)-overlapSize:]
}

// getCodeOverlap gets appropriate code overlap preserving syntax
func (s *Service) getCodeOverlap(chunk string, overlapSize int) string {
	if overlapSize <= 0 {
		return ""
	}
	
	lines := strings.Split(chunk, "\n")
	if len(lines) < 2 {
		return s.getOverlap(chunk, overlapSize)
	}
	
	// Get last few lines that fit in overlap size
	var overlap strings.Builder
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i] + "\n"
		if overlap.Len()+len(line) > overlapSize {
			break
		}
		overlap.WriteString(line)
	}
	
	return overlap.String()
}

// calculateSemanticScore assigns a semantic importance score to a chunk
func (s *Service) calculateSemanticScore(content string, contentType ContentType) float64 {
	score := 0.5 // Base score
	
	// Adjust based on content type
	switch contentType {
	case ContentTypeCode:
		score += s.config.CodeWeight
		// Higher score for function definitions, class declarations
		if strings.Contains(content, "func ") || strings.Contains(content, "class ") {
			score += 0.2
		}
	case ContentTypeMarkdown:
		// Higher score for headers
		if strings.Contains(content, "# ") {
			score += 0.3
		}
	}
	
	// Adjust based on content characteristics
	wordCount := len(strings.Fields(content))
	if wordCount > 50 {
		score += 0.1
	}
	
	// Penalty for very short chunks
	if len(content) < s.config.MinChunkSize {
		score -= 0.2
	}
	
	// Normalize to [0, 1] range
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	
	return score
}

// calculateAverageChunkSize calculates the average size of chunks
func (s *Service) calculateAverageChunkSize(chunks []ChunkInfo) float64 {
	if len(chunks) == 0 {
		return 0
	}
	
	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk.Content)
	}
	
	return float64(totalSize) / float64(len(chunks))
}

// ValidateChunk validates that a chunk meets quality criteria
func (s *Service) ValidateChunk(chunk string) error {
	if chunk == "" {
		return fmt.Errorf("chunk cannot be empty")
	}
	
	if !utf8.ValidString(chunk) {
		return fmt.Errorf("chunk contains invalid UTF-8")
	}
	
	if len(chunk) < s.config.MinChunkSize {
		return fmt.Errorf("chunk too small: %d < %d", len(chunk), s.config.MinChunkSize)
	}
	
	if len(chunk) > s.config.MaxChunkSize*2 {
		return fmt.Errorf("chunk too large: %d > %d", len(chunk), s.config.MaxChunkSize*2)
	}
	
	return nil
}

// GetChunkingStats returns statistics about chunking performance
func (s *Service) GetChunkingStats(chunks []ChunkInfo) ChunkingStats {
	if len(chunks) == 0 {
		return ChunkingStats{}
	}
	
	totalSize := 0
	minSize := len(chunks[0].Content)
	maxSize := len(chunks[0].Content)
	totalScore := 0.0
	
	for _, chunk := range chunks {
		size := len(chunk.Content)
		totalSize += size
		totalScore += chunk.SemanticScore
		
		if size < minSize {
			minSize = size
		}
		if size > maxSize {
			maxSize = size
		}
	}
	
	return ChunkingStats{
		TotalChunks:        len(chunks),
		AverageChunkSize:   float64(totalSize) / float64(len(chunks)),
		MinChunkSize:       minSize,
		MaxChunkSize:       maxSize,
		AverageSemanticScore: totalScore / float64(len(chunks)),
		TotalContentSize:   totalSize,
	}
}

// ChunkingStats provides statistics about chunking results
type ChunkingStats struct {
	TotalChunks          int     `json:"total_chunks"`
	AverageChunkSize     float64 `json:"average_chunk_size"`
	MinChunkSize         int     `json:"min_chunk_size"`
	MaxChunkSize         int     `json:"max_chunk_size"`
	AverageSemanticScore float64 `json:"average_semantic_score"`
	TotalContentSize     int     `json:"total_content_size"`
}