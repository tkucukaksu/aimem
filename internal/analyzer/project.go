package analyzer

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tarkank/aimem/internal/types"
)

// ProjectAnalyzer analyzes projects and extracts meaningful context
type ProjectAnalyzer struct {
	maxFileSizeBytes int64
	ignorePatterns   []*regexp.Regexp
	supportedExts    map[string]bool
}

// NewProjectAnalyzer creates a new project analyzer
func NewProjectAnalyzer() *ProjectAnalyzer {
	ignorePatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\.git`),
		regexp.MustCompile(`^node_modules`),
		regexp.MustCompile(`^vendor`),
		regexp.MustCompile(`^\.`),
		regexp.MustCompile(`^bin`),
		regexp.MustCompile(`^build`),
		regexp.MustCompile(`^dist`),
		regexp.MustCompile(`^target`),
		regexp.MustCompile(`\.log$`),
		regexp.MustCompile(`\.tmp$`),
		regexp.MustCompile(`\.cache`),
	}

	supportedExts := map[string]bool{
		".go":   true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".py":   true,
		".java": true,
		".cpp":  true,
		".c":    true,
		".rs":   true,
		".php":  true,
		".rb":   true,
		".sql":  true,
		".yaml": true,
		".yml":  true,
		".json": true,
		".xml":  true,
		".md":   true,
		".txt":  true,
		".sh":   true,
		".Dockerfile": true,
		".dockerfile": true,
	}

	return &ProjectAnalyzer{
		maxFileSizeBytes: 1024 * 1024, // 1MB max file size
		ignorePatterns:   ignorePatterns,
		supportedExts:    supportedExts,
	}
}

// AnalyzeProject performs comprehensive project analysis
func (pa *ProjectAnalyzer) AnalyzeProject(projectPath string, focusAreas []types.FocusArea) (*types.ProjectAnalysis, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	analysis := &types.ProjectAnalysis{
		ProjectPath: absPath,
		FocusAreas:  focusAreas,
		AnalyzedAt:  time.Now(),
	}

	// Detect language and framework
	if err := pa.detectLanguageAndFramework(absPath, analysis); err != nil {
		return nil, fmt.Errorf("language detection failed: %w", err)
	}

	// Analyze project structure
	if err := pa.analyzeStructure(absPath, analysis); err != nil {
		return nil, fmt.Errorf("structure analysis failed: %w", err)
	}

	// Extract key information based on focus areas
	if err := pa.extractFocusedInfo(absPath, analysis); err != nil {
		return nil, fmt.Errorf("focused analysis failed: %w", err)
	}

	// Calculate complexity score
	analysis.Complexity = pa.calculateComplexity(analysis)

	return analysis, nil
}

// detectLanguageAndFramework identifies the primary language and framework
func (pa *ProjectAnalyzer) detectLanguageAndFramework(projectPath string, analysis *types.ProjectAnalysis) error {
	languageCount := make(map[string]int)
	frameworks := make(map[string]bool)

	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		relPath := strings.TrimPrefix(path, projectPath)
		if pa.shouldIgnore(relPath) {
			return nil
		}

		ext := filepath.Ext(path)
		if !pa.supportedExts[ext] {
			return nil
		}

		// Count file extensions for language detection
		switch ext {
		case ".go":
			languageCount["Go"]++
		case ".js", ".jsx":
			languageCount["JavaScript"]++
		case ".ts", ".tsx":
			languageCount["TypeScript"]++
		case ".py":
			languageCount["Python"]++
		case ".java":
			languageCount["Java"]++
		case ".php":
			languageCount["PHP"]++
		case ".rb":
			languageCount["Ruby"]++
		case ".rs":
			languageCount["Rust"]++
		}

		// Detect frameworks from file names
		filename := strings.ToLower(d.Name())
		switch {
		case filename == "package.json":
			frameworks["Node.js"] = true
		case filename == "go.mod":
			frameworks["Go Modules"] = true
		case filename == "requirements.txt":
			frameworks["Python"] = true
		case filename == "dockerfile" || ext == ".dockerfile":
			frameworks["Docker"] = true
		case filename == "docker-compose.yml" || filename == "docker-compose.yaml":
			frameworks["Docker Compose"] = true
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Determine primary language
	maxCount := 0
	for lang, count := range languageCount {
		if count > maxCount {
			maxCount = count
			analysis.Language = lang
		}
	}

	// Set framework
	frameworkList := make([]string, 0, len(frameworks))
	for fw := range frameworks {
		frameworkList = append(frameworkList, fw)
	}
	if len(frameworkList) > 0 {
		analysis.Framework = strings.Join(frameworkList, ", ")
	}

	return nil
}

// analyzeStructure analyzes the project structure and identifies key files
func (pa *ProjectAnalyzer) analyzeStructure(projectPath string, analysis *types.ProjectAnalysis) error {
	var keyFiles []string
	var configFiles []string
	var entryPoints []string

	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		relPath := strings.TrimPrefix(path, projectPath)
		if pa.shouldIgnore(relPath) {
			return nil
		}

		filename := strings.ToLower(d.Name())

		// Identify configuration files
		switch {
		case strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml"):
			configFiles = append(configFiles, relPath)
		case strings.HasSuffix(filename, ".json") && (strings.Contains(filename, "config") || filename == "package.json"):
			configFiles = append(configFiles, relPath)
		case strings.HasSuffix(filename, ".toml"):
			configFiles = append(configFiles, relPath)
		case strings.HasSuffix(filename, ".env") || filename == ".env":
			configFiles = append(configFiles, relPath)
		}

		// Identify entry points
		switch {
		case filename == "main.go":
			entryPoints = append(entryPoints, relPath)
		case filename == "index.js" || filename == "app.js":
			entryPoints = append(entryPoints, relPath)
		case filename == "main.py" || filename == "__init__.py":
			entryPoints = append(entryPoints, relPath)
		case filename == "main.java":
			entryPoints = append(entryPoints, relPath)
		}

		// Identify key architectural files
		switch {
		case strings.Contains(filename, "router") || strings.Contains(filename, "route"):
			keyFiles = append(keyFiles, relPath)
		case strings.Contains(filename, "controller"):
			keyFiles = append(keyFiles, relPath)
		case strings.Contains(filename, "model") || strings.Contains(filename, "schema"):
			keyFiles = append(keyFiles, relPath)
		case strings.Contains(filename, "service"):
			keyFiles = append(keyFiles, relPath)
		case strings.Contains(filename, "middleware"):
			keyFiles = append(keyFiles, relPath)
		case strings.Contains(filename, "config"):
			keyFiles = append(keyFiles, relPath)
		case filename == "readme.md" || filename == "readme.txt":
			keyFiles = append(keyFiles, relPath)
		}

		return nil
	})

	analysis.KeyFiles = keyFiles
	analysis.ConfigFiles = configFiles
	analysis.EntryPoints = entryPoints

	return err
}

// extractFocusedInfo extracts information based on focus areas
func (pa *ProjectAnalyzer) extractFocusedInfo(projectPath string, analysis *types.ProjectAnalysis) error {
	for _, focus := range analysis.FocusAreas {
		switch focus {
		case types.FocusAPI:
			if err := pa.extractAPIInfo(projectPath, analysis); err != nil {
				return fmt.Errorf("API analysis failed: %w", err)
			}
		case types.FocusDatabase:
			if err := pa.extractDatabaseInfo(projectPath, analysis); err != nil {
				return fmt.Errorf("database analysis failed: %w", err)
			}
		case types.FocusArchitecture:
			if err := pa.extractArchitectureInfo(projectPath, analysis); err != nil {
				return fmt.Errorf("architecture analysis failed: %w", err)
			}
		}
	}

	return nil
}

// extractAPIInfo extracts API-related information
func (pa *ProjectAnalyzer) extractAPIInfo(projectPath string, analysis *types.ProjectAnalysis) error {
	var endpoints []string

	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if pa.shouldIgnore(strings.TrimPrefix(path, projectPath)) {
			return nil
		}

		ext := filepath.Ext(path)
		if !pa.supportedExts[ext] {
			return nil
		}

		// Read file and look for API patterns
		content, err := pa.readFileContent(path)
		if err != nil {
			return nil // Skip files that can't be read
		}

		// Look for common API patterns
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`@(GET|POST|PUT|DELETE|PATCH)\("([^"]+)"`),
			regexp.MustCompile(`\.(get|post|put|delete|patch)\("([^"]+)"`),
			regexp.MustCompile(`Route::\s*(get|post|put|delete|patch)\('([^']+)'`),
			regexp.MustCompile(`router\.(get|post|put|delete|patch)\("([^"]+)"`),
		}

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					endpoint := fmt.Sprintf("%s %s", strings.ToUpper(match[1]), match[2])
					endpoints = append(endpoints, endpoint)
				}
			}
		}

		return nil
	})

	analysis.APIEndpoints = endpoints
	return err
}

// extractDatabaseInfo extracts database schema information
func (pa *ProjectAnalyzer) extractDatabaseInfo(projectPath string, analysis *types.ProjectAnalysis) error {
	var schemas []string

	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		ext := filepath.Ext(path)
		filename := strings.ToLower(d.Name())

		// Look for SQL files or migration files
		if ext == ".sql" || strings.Contains(filename, "migration") || strings.Contains(filename, "schema") {
			relPath := strings.TrimPrefix(path, projectPath)
			schemas = append(schemas, relPath)
		}

		return nil
	})

	analysis.DatabaseSchema = schemas
	return err
}

// extractArchitectureInfo extracts architectural information
func (pa *ProjectAnalyzer) extractArchitectureInfo(projectPath string, analysis *types.ProjectAnalysis) error {
	// Analyze directory structure to infer architecture
	architecturePatterns := make(map[string]int)

	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		dirName := strings.ToLower(d.Name())
		switch {
		case strings.Contains(dirName, "controller"):
			architecturePatterns["MVC"]++
		case strings.Contains(dirName, "model"):
			architecturePatterns["MVC"]++
		case strings.Contains(dirName, "view"):
			architecturePatterns["MVC"]++
		case strings.Contains(dirName, "service"):
			architecturePatterns["Service Layer"]++
		case strings.Contains(dirName, "repository"):
			architecturePatterns["Repository Pattern"]++
		case strings.Contains(dirName, "handler"):
			architecturePatterns["Handler Pattern"]++
		case strings.Contains(dirName, "middleware"):
			architecturePatterns["Middleware Pattern"]++
		case strings.Contains(dirName, "component"):
			architecturePatterns["Component-based"]++
		}

		return nil
	})

	// Determine the most likely architecture
	maxCount := 0
	for arch, count := range architecturePatterns {
		if count > maxCount {
			maxCount = count
			analysis.Architecture = arch
		}
	}

	return err
}

// calculateComplexity calculates a complexity score for the project
func (pa *ProjectAnalyzer) calculateComplexity(analysis *types.ProjectAnalysis) float64 {
	score := 0.0

	// File count contribution
	totalFiles := len(analysis.KeyFiles) + len(analysis.ConfigFiles) + len(analysis.EntryPoints)
	score += float64(totalFiles) * 0.1

	// API endpoints contribution
	score += float64(len(analysis.APIEndpoints)) * 0.2

	// Database schema contribution
	score += float64(len(analysis.DatabaseSchema)) * 0.3

	// Dependencies contribution (estimated)
	score += float64(len(analysis.Dependencies)) * 0.05

	// Normalize to 0-1 scale
	return score / 100.0
}

// shouldIgnore checks if a path should be ignored
func (pa *ProjectAnalyzer) shouldIgnore(path string) bool {
	for _, pattern := range pa.ignorePatterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

// readFileContent safely reads file content with size limit
func (pa *ProjectAnalyzer) readFileContent(path string) (string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if fileInfo.Size() > pa.maxFileSizeBytes {
		return "", fmt.Errorf("file too large: %d bytes", fileInfo.Size())
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	lineCount := 0
	maxLines := 1000 // Limit to prevent memory issues

	for scanner.Scan() && lineCount < maxLines {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
		lineCount++
	}

	return content.String(), scanner.Err()
}

// GenerateContextChunks generates context chunks from project analysis
func (pa *ProjectAnalyzer) GenerateContextChunks(analysis *types.ProjectAnalysis, sessionID string) ([]*types.ContextChunk, error) {
	var chunks []*types.ContextChunk
	timestamp := time.Now()

	// Create summary chunk
	summary, err := pa.generateProjectSummary(analysis)
	if err != nil {
		return nil, err
	}

	summaryChunk := &types.ContextChunk{
		SessionID:  sessionID,
		Content:    summary,
		Summary:    "Project overview and architecture summary",
		Relevance:  1.0,
		Timestamp:  timestamp,
		TTL:        24 * time.Hour,
		Importance: types.ImportanceHigh,
	}
	chunks = append(chunks, summaryChunk)

	// Create architecture chunk if detected
	if analysis.Architecture != "" {
		archContent := pa.generateArchitectureContent(analysis)
		archChunk := &types.ContextChunk{
			SessionID:  sessionID,
			Content:    archContent,
			Summary:    "Project architecture and design patterns",
			Relevance:  0.9,
			Timestamp:  timestamp,
			TTL:        24 * time.Hour,
			Importance: types.ImportanceHigh,
		}
		chunks = append(chunks, archChunk)
	}

	// Create API chunk if endpoints found
	if len(analysis.APIEndpoints) > 0 {
		apiContent := pa.generateAPIContent(analysis)
		apiChunk := &types.ContextChunk{
			SessionID:  sessionID,
			Content:    apiContent,
			Summary:    "API endpoints and routes",
			Relevance:  0.8,
			Timestamp:  timestamp,
			TTL:        24 * time.Hour,
			Importance: types.ImportanceMedium,
		}
		chunks = append(chunks, apiChunk)
	}

	// Create database chunk if schema found
	if len(analysis.DatabaseSchema) > 0 {
		dbContent := pa.generateDatabaseContent(analysis)
		dbChunk := &types.ContextChunk{
			SessionID:  sessionID,
			Content:    dbContent,
			Summary:    "Database schema and data models",
			Relevance:  0.8,
			Timestamp:  timestamp,
			TTL:        24 * time.Hour,
			Importance: types.ImportanceMedium,
		}
		chunks = append(chunks, dbChunk)
	}

	return chunks, nil
}

// generateProjectSummary creates a comprehensive project summary
func (pa *ProjectAnalyzer) generateProjectSummary(analysis *types.ProjectAnalysis) (string, error) {
	summary := fmt.Sprintf(`Project Analysis Summary:

Path: %s
Language: %s
Framework: %s
Architecture: %s
Complexity Score: %.2f

Key Files (%d):
%s

Configuration Files (%d):
%s

Entry Points (%d):
%s

Focus Areas: %s
Analyzed: %s`,
		analysis.ProjectPath,
		analysis.Language,
		analysis.Framework,
		analysis.Architecture,
		analysis.Complexity,
		len(analysis.KeyFiles),
		strings.Join(analysis.KeyFiles, "\n"),
		len(analysis.ConfigFiles),
		strings.Join(analysis.ConfigFiles, "\n"),
		len(analysis.EntryPoints),
		strings.Join(analysis.EntryPoints, "\n"),
		pa.focusAreasToString(analysis.FocusAreas),
		analysis.AnalyzedAt.Format(time.RFC3339),
	)

	return summary, nil
}

// generateArchitectureContent creates architecture-focused content
func (pa *ProjectAnalyzer) generateArchitectureContent(analysis *types.ProjectAnalysis) string {
	return fmt.Sprintf(`Architecture Analysis:

Pattern: %s
Language: %s
Framework: %s

Key architectural files:
%s

Project structure indicates a %s architecture with the following characteristics:
- Entry points: %s
- Configuration management: %d config files
- Complexity level: %.2f/1.0`,
		analysis.Architecture,
		analysis.Language,
		analysis.Framework,
		strings.Join(analysis.KeyFiles, "\n"),
		analysis.Architecture,
		strings.Join(analysis.EntryPoints, ", "),
		len(analysis.ConfigFiles),
		analysis.Complexity,
	)
}

// generateAPIContent creates API-focused content
func (pa *ProjectAnalyzer) generateAPIContent(analysis *types.ProjectAnalysis) string {
	return fmt.Sprintf(`API Analysis:

Discovered %d API endpoints:
%s

Framework: %s
Language: %s

The API appears to follow RESTful conventions with the following patterns:
- Entry points: %s
- Route definitions spread across: %s`,
		len(analysis.APIEndpoints),
		strings.Join(analysis.APIEndpoints, "\n"),
		analysis.Framework,
		analysis.Language,
		strings.Join(analysis.EntryPoints, ", "),
		strings.Join(analysis.KeyFiles, ", "),
	)
}

// generateDatabaseContent creates database-focused content
func (pa *ProjectAnalyzer) generateDatabaseContent(analysis *types.ProjectAnalysis) string {
	return fmt.Sprintf(`Database Analysis:

Schema files found (%d):
%s

Language: %s
Framework: %s

Database integration appears to be managed through:
- Configuration files: %s
- Model/Schema files: %s`,
		len(analysis.DatabaseSchema),
		strings.Join(analysis.DatabaseSchema, "\n"),
		analysis.Language,
		analysis.Framework,
		strings.Join(analysis.ConfigFiles, ", "),
		strings.Join(analysis.KeyFiles, ", "),
	)
}

// focusAreasToString converts focus areas to readable string
func (pa *ProjectAnalyzer) focusAreasToString(areas []types.FocusArea) string {
	strs := make([]string, len(areas))
	for i, area := range areas {
		strs[i] = string(area)
	}
	return strings.Join(strs, ", ")
}