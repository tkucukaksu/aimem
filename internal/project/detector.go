package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tarkank/aimem/internal/types"
)

// ProjectDetector detects project information from working directories
type ProjectDetector struct {
	cache        map[string]*DetectionResult
	cacheTimeout time.Duration
	cacheMu      sync.RWMutex
}

// DetectionResult contains the detection result with caching info
type DetectionResult struct {
	Project    *types.ProjectInfo `json:"project"`
	CachedAt   time.Time          `json:"cached_at"`
	Confidence float64            `json:"confidence"`
}

// NewProjectDetector creates a new project detector
func NewProjectDetector() *ProjectDetector {
	return &ProjectDetector{
		cache:        make(map[string]*DetectionResult),
		cacheTimeout: 10 * time.Minute,
	}
}

// DetectProject detects project information from a working directory
func (pd *ProjectDetector) DetectProject(workingDir string) (*types.ProjectInfo, error) {
	// Canonicalize working directory
	canonicalPath, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check cache with read lock
	pd.cacheMu.RLock()
	cached, exists := pd.cache[canonicalPath]
	pd.cacheMu.RUnlock()

	if exists {
		if time.Since(cached.CachedAt) < pd.cacheTimeout {
			return cached.Project, nil
		}
		// Remove expired cache with write lock
		pd.cacheMu.Lock()
		delete(pd.cache, canonicalPath)
		pd.cacheMu.Unlock()
	}

	// Initialize project info
	project := &types.ProjectInfo{
		CanonicalPath: canonicalPath,
		Name:          filepath.Base(canonicalPath),
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
		Status:        types.ProjectStatusActive,
	}

	// 1. Git repository detection
	if gitRoot, gitRemote := pd.detectGitProject(canonicalPath); gitRoot != nil {
		project.Type = types.ProjectTypeGitRepository
		project.GitRoot = gitRoot
		project.GitRemote = gitRemote
		project.Name = filepath.Base(*gitRoot)
		project.CanonicalPath = *gitRoot // Use git root as canonical path
	} else {
		// 2. Workspace detection
		if pd.detectWorkspaceProject(canonicalPath, project) {
			project.Type = types.ProjectTypeWorkspace
		} else {
			project.Type = types.ProjectTypeDirectory
		}
	}

	// 3. Language & framework detection
	pd.detectLanguageAndFramework(project.CanonicalPath, project)

	// 4. Generate unique project ID
	project.ID = pd.generateProjectID(project)

	// Cache result with write lock
	pd.cacheMu.Lock()
	pd.cache[canonicalPath] = &DetectionResult{
		Project:    project,
		CachedAt:   time.Now(),
		Confidence: 0.95,
	}
	pd.cacheMu.Unlock()

	return project, nil
}

// detectGitProject detects if the path is within a git repository
func (pd *ProjectDetector) detectGitProject(path string) (gitRoot *string, gitRemote *string) {
	current := path

	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil {
			if info.IsDir() {
				// Git repository found
				gitRoot = &current

				// Try to get remote URL
				if remote := pd.getGitRemoteURL(current); remote != "" {
					gitRemote = &remote
				}
				return
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // Reached filesystem root
		}
		current = parent
	}

	return nil, nil
}

// getGitRemoteURL attempts to read git remote URL from config
func (pd *ProjectDetector) getGitRemoteURL(gitRoot string) string {
	configPath := filepath.Join(gitRoot, ".git", "config")
	if content, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, `[remote "origin"]`) {
				// Look for url in next few lines
				for j := i + 1; j < len(lines) && j < i+5; j++ {
					if strings.Contains(lines[j], "url =") {
						parts := strings.Split(lines[j], "url =")
						if len(parts) > 1 {
							return strings.TrimSpace(parts[1])
						}
					}
				}
				break
			}
		}
	}
	return ""
}

// detectWorkspaceProject checks for workspace markers
func (pd *ProjectDetector) detectWorkspaceProject(path string, project *types.ProjectInfo) bool {
	workspaceMarkers := []string{
		"package.json",     // Node.js
		"go.mod",           // Go
		"Cargo.toml",       // Rust
		"pom.xml",          // Java Maven
		"build.gradle",     // Java Gradle
		"requirements.txt", // Python
		"pyproject.toml",   // Python
		"Gemfile",          // Ruby
		"composer.json",    // PHP
		"mix.exs",          // Elixir
	}

	var foundMarkers []string

	for _, marker := range workspaceMarkers {
		markerPath := filepath.Join(path, marker)
		if _, err := os.Stat(markerPath); err == nil {
			foundMarkers = append(foundMarkers, marker)
		}
	}

	if len(foundMarkers) > 0 {
		project.WorkspaceMarkers = foundMarkers
		return true
	}

	return false
}

// detectLanguageAndFramework analyzes files to detect language and framework
func (pd *ProjectDetector) detectLanguageAndFramework(path string, project *types.ProjectInfo) {
	languageCount := make(map[string]int)
	frameworkHints := make(map[string]bool)

	filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		// Skip ignored files
		if pd.shouldIgnoreFile(filePath) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		fileName := strings.ToLower(d.Name())

		// Language detection
		switch ext {
		case ".go":
			languageCount["Go"]++
		case ".js", ".mjs":
			languageCount["JavaScript"]++
		case ".ts":
			languageCount["TypeScript"]++
		case ".py":
			languageCount["Python"]++
		case ".rs":
			languageCount["Rust"]++
		case ".java":
			languageCount["Java"]++
		case ".php":
			languageCount["PHP"]++
		case ".rb":
			languageCount["Ruby"]++
		case ".cs":
			languageCount["C#"]++
		}

		// Framework detection
		switch fileName {
		case "next.config.js", "next.config.ts":
			frameworkHints["Next.js"] = true
		case "nuxt.config.js", "nuxt.config.ts":
			frameworkHints["Nuxt.js"] = true
		case "vue.config.js":
			frameworkHints["Vue.js"] = true
		case "angular.json":
			frameworkHints["Angular"] = true
		case "svelte.config.js":
			frameworkHints["Svelte"] = true
		case "gatsby-config.js":
			frameworkHints["Gatsby"] = true
		case "remix.config.js":
			frameworkHints["Remix"] = true
		}

		return nil
	})

	// Determine dominant language
	maxCount := 0
	for lang, count := range languageCount {
		if count > maxCount {
			maxCount = count
			project.Language = lang
		}
	}

	// Set framework
	if len(frameworkHints) > 0 {
		frameworks := make([]string, 0, len(frameworkHints))
		for framework := range frameworkHints {
			frameworks = append(frameworks, framework)
		}
		project.Framework = strings.Join(frameworks, ", ")
	}
}

// generateProjectID creates a unique project ID
func (pd *ProjectDetector) generateProjectID(project *types.ProjectInfo) string {
	var identifier string

	if project.GitRemote != nil {
		// Use git remote URL (most stable identifier)
		identifier = *project.GitRemote
	} else if project.GitRoot != nil {
		// Use git root path
		identifier = *project.GitRoot
	} else {
		// Use canonical path
		identifier = project.CanonicalPath
	}

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(identifier))
	return hex.EncodeToString(hash[:])[:16] // First 16 characters (64-bit)
}

// shouldIgnoreFile checks if a file should be ignored during analysis
func (pd *ProjectDetector) shouldIgnoreFile(path string) bool {
	ignorePaths := []string{
		"/.git/",
		"/node_modules/",
		"/vendor/",
		"/.vscode/",
		"/.idea/",
		"/target/",
		"/build/",
		"/dist/",
		"/.next/",
		"/.nuxt/",
	}

	for _, ignore := range ignorePaths {
		if strings.Contains(path, ignore) {
			return true
		}
	}

	return false
}

// ClearCache clears the detection cache
func (pd *ProjectDetector) ClearCache() {
	pd.cacheMu.Lock()
	defer pd.cacheMu.Unlock()
	pd.cache = make(map[string]*DetectionResult)
}
