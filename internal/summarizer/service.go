package summarizer

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

// Service provides intelligent text summarization with importance preservation
type Service struct {
	config *Config
	logger *logrus.Logger
}

// Config contains configuration for the summarization service
type Config struct {
	CompressionRatio float64 `yaml:"compression_ratio"`
	MinSummaryLength int     `yaml:"min_summary_length"`
	MaxSummaryLength int     `yaml:"max_summary_length"`
	PreserveCode     bool    `yaml:"preserve_code"`
	PreserveLinks    bool    `yaml:"preserve_links"`
	KeywordWeight    float64 `yaml:"keyword_weight"`
}

// SentenceInfo contains information about a sentence for ranking
type SentenceInfo struct {
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	Position  int     `json:"position"`
	Length    int     `json:"length"`
	HasCode   bool    `json:"has_code"`
	HasLinks  bool    `json:"has_links"`
	Keywords  []string `json:"keywords"`
}

// SummaryResult contains the summarization result with metadata
type SummaryResult struct {
	Summary           string             `json:"summary"`
	OriginalLength    int                `json:"original_length"`
	SummaryLength     int                `json:"summary_length"`
	CompressionRatio  float64            `json:"compression_ratio"`
	PreservedElements []string           `json:"preserved_elements"`
	KeySentences      []SentenceInfo     `json:"key_sentences"`
	Stats             SummarizationStats `json:"stats"`
}

// SummarizationStats provides statistics about the summarization process
type SummarizationStats struct {
	SentencesAnalyzed  int     `json:"sentences_analyzed"`
	SentencesSelected  int     `json:"sentences_selected"`
	CodeBlocksFound    int     `json:"code_blocks_found"`
	LinksFound         int     `json:"links_found"`
	KeywordsExtracted  int     `json:"keywords_extracted"`
	AverageScore       float64 `json:"average_score"`
}

// NewService creates a new summarization service
func NewService(config *Config, logger *logrus.Logger) *Service {
	if config == nil {
		config = &Config{
			CompressionRatio: 0.3, // Compress to 30% of original
			MinSummaryLength: 50,
			MaxSummaryLength: 2000,
			PreserveCode:     true,
			PreserveLinks:    true,
			KeywordWeight:    1.5,
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

// SummarizeContent creates an intelligent summary of the content
func (s *Service) SummarizeContent(content string, maxLength int) (string, error) {
	result, err := s.SummarizeContentWithInfo(content, maxLength)
	if err != nil {
		return "", err
	}
	return result.Summary, nil
}

// SummarizeContentWithInfo creates a summary with detailed information
func (s *Service) SummarizeContentWithInfo(content string, maxLength int) (*SummaryResult, error) {
	if content == "" {
		return &SummaryResult{}, nil
	}

	originalLength := len(content)
	
	// If content is already short enough, return as-is
	if originalLength <= maxLength {
		return &SummaryResult{
			Summary:          content,
			OriginalLength:   originalLength,
			SummaryLength:    originalLength,
			CompressionRatio: 1.0,
		}, nil
	}

	s.logger.WithFields(logrus.Fields{
		"original_length": originalLength,
		"max_length":      maxLength,
	}).Debug("Starting content summarization")

	// Extract preserved elements first
	preservedElements := s.extractPreservedElements(content)
	
	// Split into sentences
	sentences := s.splitIntoSentences(content)
	if len(sentences) == 0 {
		return &SummaryResult{
			Summary:        content[:min(maxLength, len(content))],
			OriginalLength: originalLength,
			SummaryLength:  min(maxLength, len(content)),
		}, nil
	}

	// Analyze and score sentences
	sentenceInfos := s.analyzeSentences(sentences, content)
	
	// Extract keywords for context
	keywords := s.extractKeywords(content)
	
	// Update scores based on keywords
	s.updateScoresWithKeywords(sentenceInfos, keywords)
	
	// Select sentences for summary
	selectedSentences := s.selectSentences(sentenceInfos, maxLength, preservedElements)
	
	// Build final summary
	summary := s.buildSummary(selectedSentences, preservedElements)
	
	// Calculate statistics
	stats := s.calculateStats(sentenceInfos, selectedSentences, keywords, preservedElements)
	
	result := &SummaryResult{
		Summary:           summary,
		OriginalLength:    originalLength,
		SummaryLength:     len(summary),
		CompressionRatio:  float64(len(summary)) / float64(originalLength),
		PreservedElements: s.getPreservedElementTypes(preservedElements),
		KeySentences:      selectedSentences,
		Stats:             stats,
	}

	s.logger.WithFields(logrus.Fields{
		"original_length":   originalLength,
		"summary_length":    len(summary),
		"compression_ratio": result.CompressionRatio,
		"sentences_used":    len(selectedSentences),
	}).Debug("Summarization completed")

	return result, nil
}

// extractPreservedElements extracts elements that should be preserved in summaries
func (s *Service) extractPreservedElements(content string) []string {
	var preserved []string
	
	if s.config.PreserveCode {
		// Extract code blocks
		codeRegex := regexp.MustCompile("```[\\s\\S]*?```")
		codeBlocks := codeRegex.FindAllString(content, -1)
		preserved = append(preserved, codeBlocks...)
		
		// Extract inline code
		inlineCodeRegex := regexp.MustCompile("`[^`]+`")
		inlineCode := inlineCodeRegex.FindAllString(content, -1)
		preserved = append(preserved, inlineCode...)
	}
	
	if s.config.PreserveLinks {
		// Extract markdown links
		linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
		links := linkRegex.FindAllString(content, -1)
		preserved = append(preserved, links...)
		
		// Extract URLs
		urlRegex := regexp.MustCompile(`https?://[^\s]+`)
		urls := urlRegex.FindAllString(content, -1)
		preserved = append(preserved, urls...)
	}
	
	return preserved
}

// splitIntoSentences splits content into sentences with better boundary detection
func (s *Service) splitIntoSentences(content string) []string {
	// Handle code blocks separately
	codeBlockRegex := regexp.MustCompile("```[\\s\\S]*?```")
	codeBlocks := codeBlockRegex.FindAllString(content, -1)
	
	// Replace code blocks with placeholders
	contentWithoutCode := content
	for i, block := range codeBlocks {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", i)
		contentWithoutCode = strings.Replace(contentWithoutCode, block, placeholder, 1)
	}
	
	// Split sentences (improved regex)
	sentenceRegex := regexp.MustCompile(`[.!?]+(?:\s+|$)`)
	sentences := sentenceRegex.Split(contentWithoutCode, -1)
	
	// Restore code blocks
	var result []string
	for _, sentence := range sentences {
		restored := sentence
		for i, block := range codeBlocks {
			placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", i)
			if strings.Contains(restored, placeholder) {
				restored = strings.Replace(restored, placeholder, block, 1)
			}
		}
		if strings.TrimSpace(restored) != "" {
			result = append(result, strings.TrimSpace(restored))
		}
	}
	
	return result
}

// analyzeSentences analyzes each sentence and assigns scores
func (s *Service) analyzeSentences(sentences []string, fullContent string) []SentenceInfo {
	infos := make([]SentenceInfo, len(sentences))
	
	for i, sentence := range sentences {
		info := SentenceInfo{
			Text:     sentence,
			Position: i,
			Length:   len(sentence),
			HasCode:  s.containsCode(sentence),
			HasLinks: s.containsLinks(sentence),
			Keywords: s.extractSentenceKeywords(sentence),
		}
		
		// Calculate base score
		info.Score = s.calculateSentenceScore(info, len(sentences), fullContent)
		infos[i] = info
	}
	
	return infos
}

// calculateSentenceScore calculates the importance score of a sentence
func (s *Service) calculateSentenceScore(info SentenceInfo, totalSentences int, fullContent string) float64 {
	score := 0.0
	
	// Position score (first and last sentences are often important)
	positionScore := 0.0
	if info.Position == 0 || info.Position == totalSentences-1 {
		positionScore = 0.3
	} else if info.Position < 3 || info.Position >= totalSentences-3 {
		positionScore = 0.2
	} else {
		positionScore = 0.1
	}
	score += positionScore
	
	// Length score (medium-length sentences often contain more information)
	lengthScore := 0.0
	if info.Length > 50 && info.Length < 200 {
		lengthScore = 0.2
	} else if info.Length >= 20 {
		lengthScore = 0.1
	}
	score += lengthScore
	
	// Code preservation
	if info.HasCode && s.config.PreserveCode {
		score += 0.4
	}
	
	// Link preservation
	if info.HasLinks && s.config.PreserveLinks {
		score += 0.3
	}
	
	// Keyword density
	keywordScore := float64(len(info.Keywords)) * 0.05
	if keywordScore > 0.3 {
		keywordScore = 0.3
	}
	score += keywordScore
	
	// Numeric content (often important)
	if s.containsNumbers(info.Text) {
		score += 0.1
	}
	
	// Question sentences (often important)
	if strings.HasSuffix(strings.TrimSpace(info.Text), "?") {
		score += 0.2
	}
	
	// Capitalized words (proper nouns, acronyms)
	capitalWords := s.countCapitalizedWords(info.Text)
	if capitalWords > 0 {
		score += math.Min(float64(capitalWords)*0.05, 0.2)
	}
	
	// Normalize score
	return math.Min(score, 1.0)
}

// updateScoresWithKeywords updates sentence scores based on keyword relevance
func (s *Service) updateScoresWithKeywords(sentences []SentenceInfo, keywords []string) {
	if len(keywords) == 0 {
		return
	}
	
	keywordSet := make(map[string]bool)
	for _, keyword := range keywords {
		keywordSet[strings.ToLower(keyword)] = true
	}
	
	for i := range sentences {
		keywordMatches := 0
		words := strings.Fields(strings.ToLower(sentences[i].Text))
		
		for _, word := range words {
			if keywordSet[word] {
				keywordMatches++
			}
		}
		
		if keywordMatches > 0 {
			keywordBonus := float64(keywordMatches) * s.config.KeywordWeight * 0.1
			sentences[i].Score += math.Min(keywordBonus, 0.5)
		}
	}
}

// selectSentences selects the best sentences for the summary
func (s *Service) selectSentences(sentences []SentenceInfo, maxLength int, preservedElements []string) []SentenceInfo {
	// Sort by score (descending)
	sort.Slice(sentences, func(i, j int) bool {
		return sentences[i].Score > sentences[j].Score
	})
	
	var selected []SentenceInfo
	currentLength := 0
	
	// Reserve space for preserved elements
	preservedLength := 0
	for _, element := range preservedElements {
		preservedLength += len(element)
	}
	
	availableLength := maxLength - preservedLength
	if availableLength < s.config.MinSummaryLength {
		availableLength = maxLength
	}
	
	// Select sentences up to the length limit
	for _, sentence := range sentences {
		if currentLength+sentence.Length <= availableLength {
			selected = append(selected, sentence)
			currentLength += sentence.Length
		}
		
		if currentLength >= availableLength {
			break
		}
	}
	
	// Sort selected sentences back to original order
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Position < selected[j].Position
	})
	
	return selected
}

// buildSummary constructs the final summary from selected sentences
func (s *Service) buildSummary(sentences []SentenceInfo, preservedElements []string) string {
	var summary strings.Builder
	
	for i, sentence := range sentences {
		if i > 0 {
			summary.WriteString(" ")
		}
		summary.WriteString(sentence.Text)
	}
	
	// Add preserved elements at the end if they weren't included
	summaryText := summary.String()
	for _, element := range preservedElements {
		if !strings.Contains(summaryText, element) {
			summary.WriteString("\n\n")
			summary.WriteString(element)
		}
	}
	
	return strings.TrimSpace(summary.String())
}

// extractKeywords extracts important keywords from the content
func (s *Service) extractKeywords(content string) []string {
	// Simple keyword extraction - in production, use TF-IDF or more sophisticated methods
	words := strings.Fields(strings.ToLower(content))
	wordFreq := make(map[string]int)
	
	// Count word frequencies
	for _, word := range words {
		// Clean word
		cleaned := s.cleanWord(word)
		if len(cleaned) > 3 && !s.isStopWord(cleaned) {
			wordFreq[cleaned]++
		}
	}
	
	// Sort by frequency
	type wordCount struct {
		word  string
		count int
	}
	
	var sortedWords []wordCount
	for word, count := range wordFreq {
		if count > 1 { // Only include words that appear more than once
			sortedWords = append(sortedWords, wordCount{word, count})
		}
	}
	
	sort.Slice(sortedWords, func(i, j int) bool {
		return sortedWords[i].count > sortedWords[j].count
	})
	
	// Return top keywords
	maxKeywords := min(10, len(sortedWords))
	keywords := make([]string, maxKeywords)
	for i := 0; i < maxKeywords; i++ {
		keywords[i] = sortedWords[i].word
	}
	
	return keywords
}

// extractSentenceKeywords extracts keywords specific to a sentence
func (s *Service) extractSentenceKeywords(sentence string) []string {
	words := strings.Fields(strings.ToLower(sentence))
	var keywords []string
	
	for _, word := range words {
		cleaned := s.cleanWord(word)
		if len(cleaned) > 3 && !s.isStopWord(cleaned) {
			keywords = append(keywords, cleaned)
		}
	}
	
	return keywords
}

// Helper functions

func (s *Service) containsCode(text string) bool {
	// Check for code patterns
	codePatterns := []string{
		"`",           // Inline code
		"```",         // Code blocks
		"func ",       // Go functions
		"def ",        // Python functions
		"class ",      // Class definitions
		"import ",     // Import statements
		"#include",    // C/C++ includes
		"console.",    // JavaScript console
		"print(",      // Print statements
	}
	
	for _, pattern := range codePatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	
	return false
}

func (s *Service) containsLinks(text string) bool {
	linkPatterns := []string{
		"http://",
		"https://",
		"[",  // Markdown links
		"www.",
	}
	
	for _, pattern := range linkPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	
	return false
}

func (s *Service) containsNumbers(text string) bool {
	for _, char := range text {
		if unicode.IsDigit(char) {
			return true
		}
	}
	return false
}

func (s *Service) countCapitalizedWords(text string) int {
	words := strings.Fields(text)
	count := 0
	
	for _, word := range words {
		if len(word) > 1 && unicode.IsUpper(rune(word[0])) {
			count++
		}
	}
	
	return count
}

func (s *Service) cleanWord(word string) string {
	// Remove punctuation
	cleaned := regexp.MustCompile(`[^\w]`).ReplaceAllString(word, "")
	return strings.ToLower(cleaned)
}

func (s *Service) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
		"are": true, "was": true, "were": true, "be": true, "been": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"this": true, "that": true, "these": true, "those": true, "i": true,
		"you": true, "he": true, "she": true, "it": true, "we": true,
		"they": true, "them": true, "their": true, "what": true, "which": true,
		"who": true, "when": true, "where": true, "why": true, "how": true,
	}
	
	return stopWords[word]
}

func (s *Service) calculateStats(allSentences, selectedSentences []SentenceInfo, keywords []string, preservedElements []string) SummarizationStats {
	totalScore := 0.0
	codeBlocks := 0
	links := 0
	
	for _, sentence := range allSentences {
		totalScore += sentence.Score
		if sentence.HasCode {
			codeBlocks++
		}
		if sentence.HasLinks {
			links++
		}
	}
	
	avgScore := 0.0
	if len(allSentences) > 0 {
		avgScore = totalScore / float64(len(allSentences))
	}
	
	return SummarizationStats{
		SentencesAnalyzed:  len(allSentences),
		SentencesSelected:  len(selectedSentences),
		CodeBlocksFound:    codeBlocks,
		LinksFound:         links,
		KeywordsExtracted:  len(keywords),
		AverageScore:       avgScore,
	}
}

func (s *Service) getPreservedElementTypes(elements []string) []string {
	var types []string
	hasCode := false
	hasLinks := false
	
	for _, element := range elements {
		if strings.Contains(element, "```") || strings.Contains(element, "`") {
			hasCode = true
		}
		if strings.Contains(element, "http") || strings.Contains(element, "[") {
			hasLinks = true
		}
	}
	
	if hasCode {
		types = append(types, "code")
	}
	if hasLinks {
		types = append(types, "links")
	}
	
	return types
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}