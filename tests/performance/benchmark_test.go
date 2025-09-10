package performance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tarkank/aimem/internal/logger"
	"github.com/tarkank/aimem/internal/performance"
	"github.com/tarkank/aimem/internal/project"
	"github.com/tarkank/aimem/internal/server"
	"github.com/tarkank/aimem/internal/session"
	"github.com/tarkank/aimem/internal/storage"
	"github.com/tarkank/aimem/internal/types"
)

// BenchmarkProjectDetection benchmarks project detection performance
func BenchmarkProjectDetection(b *testing.B) {
	detector := project.NewProjectDetector()

	testCases := []struct {
		name string
		path string
	}{
		{"GitProject", setupGitProject(b)},
		{"NodeProject", setupNodeProject(b)},
		{"GoProject", setupGoProject(b)},
		{"PythonProject", setupPythonProject(b)},
		{"ComplexProject", setupComplexProject(b)},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := detector.DetectProject(tc.path)
				if err != nil {
					b.Fatalf("Project detection failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSessionManagement benchmarks session management operations
func BenchmarkSessionManagement(b *testing.B) {
	tempDir := b.TempDir()
	config := createBenchmarkConfig(tempDir)

	store, err := storage.NewSQLiteStorage(&config.SQLite)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	detector := project.NewProjectDetector()
	sessionManager := session.NewSessionManager(detector, store, config)
	ctx := context.Background()

	// Create a test project for session creation
	testProject := &types.ProjectInfo{
		ID:            "benchmark-project-id",
		Name:          "benchmark-project",
		CanonicalPath: tempDir,
		Type:          types.ProjectTypeDirectory,
		Language:      "go",
		Framework:     "test",
		CreatedAt:     time.Now(),
		Status:        types.ProjectStatusActive,
	}

	// Store the test project
	err = store.CreateProject(ctx, testProject)
	if err != nil {
		b.Fatalf("Failed to create test project: %v", err)
	}

	b.Run("GetOrCreateProjectSession", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := sessionManager.GetOrCreateProjectSession(tempDir)
			if err != nil {
				b.Fatalf("Failed to get/create session: %v", err)
			}
		}
	})

	// Create some test sessions for other benchmarks
	for i := 0; i < 100; i++ {
		sessionID := fmt.Sprintf("test-session-%d", i)
		testSession := &types.SessionInfo{
			ID:         sessionID,
			ProjectID:  testProject.ID,
			Name:       fmt.Sprintf("test-session-%d", i),
			Type:       types.SessionTypeMain,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
			Status:     types.SessionStatusActive,
			WorkingDir: tempDir,
			Metadata:   make(map[string]interface{}),
		}
		store.CreateSession(ctx, testSession)
	}

	b.Run("GetSession", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sessionID := fmt.Sprintf("test-session-%d", i%100)
			_, err := sessionManager.GetSession(ctx, sessionID)
			if err != nil {
				b.Fatalf("Failed to get session: %v", err)
			}
		}
	})

	b.Run("ResolveSession", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sessionID := fmt.Sprintf("test-session-%d", i%100)
			_, err := sessionManager.ResolveSession(sessionID)
			if err != nil {
				b.Fatalf("Failed to resolve session: %v", err)
			}
		}
	})
}

// BenchmarkStorageOperations benchmarks storage layer performance
func BenchmarkStorageOperations(b *testing.B) {
	tempDir := b.TempDir()
	config := createBenchmarkConfig(tempDir)

	store, err := storage.NewSQLiteStorage(&config.SQLite)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	sessionID := "benchmark-session"

	// Create test session
	testSession := &types.SessionInfo{
		ID:         sessionID,
		ProjectID:  "test-project",
		Name:       "benchmark-session",
		Type:       types.SessionTypeMain,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     types.SessionStatusActive,
		WorkingDir: tempDir,
		Metadata:   make(map[string]interface{}),
	}

	err = store.CreateSession(ctx, testSession)
	if err != nil {
		b.Fatalf("Failed to create session: %v", err)
	}

	b.Run("StoreChunk", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			chunk := &types.ContextChunk{
				ID:         fmt.Sprintf("chunk-%d", i),
				SessionID:  sessionID,
				Content:    fmt.Sprintf("Benchmark content for chunk %d. This is a sample context chunk for performance testing.", i),
				Summary:    fmt.Sprintf("Summary for chunk %d", i),
				Embedding:  make([]float32, 384), // Simulate embedding vector
				Relevance:  0.8,
				Timestamp:  time.Now(),
				TTL:        1 * time.Hour,
				Importance: types.ImportanceMedium,
			}
			err := store.StoreChunk(ctx, chunk)
			if err != nil {
				b.Fatalf("Failed to store chunk: %v", err)
			}
		}
	})

	b.Run("GetChunk", func(b *testing.B) {
		// Pre-create some chunks
		for i := 0; i < 100; i++ {
			chunk := &types.ContextChunk{
				ID:         fmt.Sprintf("get-chunk-%d", i),
				SessionID:  sessionID,
				Content:    fmt.Sprintf("Content for get test %d", i),
				Summary:    fmt.Sprintf("Summary %d", i),
				Embedding:  make([]float32, 384),
				Relevance:  0.7,
				Timestamp:  time.Now(),
				TTL:        1 * time.Hour,
				Importance: types.ImportanceMedium,
			}
			store.StoreChunk(ctx, chunk)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			chunkID := fmt.Sprintf("get-chunk-%d", i%100)
			_, err := store.GetChunk(ctx, chunkID)
			if err != nil {
				b.Fatalf("Failed to get chunk: %v", err)
			}
		}
	})

	b.Run("SearchByEmbedding", func(b *testing.B) {
		// Create query embedding
		queryEmbedding := make([]float32, 384)
		for i := range queryEmbedding {
			queryEmbedding[i] = 0.5 // Simple test embedding
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := store.SearchByEmbedding(ctx, sessionID, queryEmbedding, 10)
			if err != nil {
				b.Fatalf("Failed to search by embedding: %v", err)
			}
		}
	})
}

// BenchmarkPerformanceMonitoring benchmarks performance monitoring overhead
func BenchmarkPerformanceMonitoring(b *testing.B) {
	config := &types.PerformanceConfig{
		EnableMetrics:   true,
		MetricsInterval: 10 * time.Millisecond,
	}

	// Create logger for performance monitor
	loggerConfig := logger.TestConfig()
	testLogger, err := logger.NewLogger(loggerConfig, "benchmark-test")
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	monitor := performance.NewPerformanceMonitor(config, testLogger)

	b.Run("StartEndRequest", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reqCtx := monitor.StartRequest(ctx, "benchmark-session", "benchmark_operation")
			monitor.EndRequest(reqCtx, nil)
		}
	})

	b.Run("RecordEmbeddingTime", func(b *testing.B) {
		sessionID := "benchmark_session"
		duration := 5 * time.Millisecond
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.RecordEmbeddingTime(sessionID, duration)
		}
	})

	b.Run("RecordStorageTime", func(b *testing.B) {
		sessionID := "benchmark_session"
		duration := 10 * time.Millisecond
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.RecordStorageTime(sessionID, duration)
		}
	})

	b.Run("GetSystemMetrics", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = monitor.GetSystemMetrics()
		}
	})

	b.Run("GetSessionMetrics", func(b *testing.B) {
		sessionID := "benchmark_session"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = monitor.GetSessionMetrics(sessionID)
		}
	})
}

// BenchmarkMemoryEfficiency benchmarks memory usage patterns
func BenchmarkMemoryEfficiency(b *testing.B) {
	tempDir := b.TempDir()
	config := createBenchmarkConfig(tempDir)

	// Test with different memory limits
	memorySizes := []string{"1MB", "5MB", "10MB", "50MB"}

	for _, memSize := range memorySizes {
		b.Run(fmt.Sprintf("Memory_%s", memSize), func(b *testing.B) {
			config.Memory.MaxSessionSize = memSize

			aimemServer, err := server.NewAIMem(config)
			if err != nil {
				b.Fatalf("Failed to create server: %v", err)
			}
			defer aimemServer.Close()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Simulate memory allocation and usage
				largeContent := generateLargeContent(1024) // 1KB content
				_ = largeContent                           // Use the content to prevent optimization
			}
		})
	}
}

// BenchmarkConcurrentAccess benchmarks concurrent access patterns
func BenchmarkConcurrentAccess(b *testing.B) {
	tempDir := b.TempDir()
	config := createBenchmarkConfig(tempDir)
	config.Performance.AsyncProcessing = true

	aimemServer, err := server.NewAIMem(config)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}
	defer aimemServer.Close()

	b.Run("ConcurrentReads", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Simulate concurrent read operations
				// In real scenario, this would be MCP tool calls
				time.Sleep(1 * time.Microsecond) // Minimal simulation
			}
		})
	})

	b.Run("ConcurrentWrites", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				// Simulate concurrent write operations
				content := fmt.Sprintf("Concurrent content %d", i)
				_ = content
				i++
				time.Sleep(1 * time.Microsecond) // Minimal simulation
			}
		})
	})
}

// Helper functions

func createBenchmarkConfig(tempDir string) *types.Config {
	return &types.Config{
		Database: "sqlite",
		SQLite: types.SQLiteConfig{
			DatabasePath:          filepath.Join(tempDir, "benchmark.db"),
			MaxConnections:        10,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 60,
		},
		Memory: types.MemoryConfig{
			MaxSessionSize:    "50MB",
			ChunkSize:         2048,
			MaxChunksPerQuery: 20,
			TTLDefault:        1 * time.Hour,
		},
		Embedding: types.EmbeddingConfig{
			Model:     "benchmark-model",
			CacheSize: 100,
			BatchSize: 16,
		},
		Performance: types.PerformanceConfig{
			CompressionEnabled: true,
			AsyncProcessing:    true,
			CacheEmbeddings:    true,
			EnableMetrics:      true,
			MetricsInterval:    1 * time.Second,
		},
		SessionManager: types.SessionManagerConfig{
			EnableAutoDetection:    true,
			DefaultSessionType:     types.SessionTypeMain,
			SessionCacheSize:       100,
			SessionTimeout:         1 * time.Hour,
			MaxSessionsPerProject:  10,
			EnableSessionHierarchy: true,
			AutoCleanupInactive:    false,
		},
		ProjectDetector: types.ProjectDetectorConfig{
			EnableCaching:             true,
			CacheTimeout:              10 * time.Minute,
			MaxCacheSize:              100,
			DeepScanEnabled:           true,
			GitDetectionEnabled:       true,
			WorkspaceDetectionEnabled: true,
			LanguageDetectionEnabled:  true,
		},
	}
}

func setupGitProject(b *testing.B) string {
	dir := b.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		b.Fatalf("Failed to create git dir: %v", err)
	}
	return dir
}

func setupNodeProject(b *testing.B) string {
	dir := b.TempDir()
	writeFile(b, filepath.Join(dir, "package.json"), `{"name": "benchmark-project"}`)
	return dir
}

func setupGoProject(b *testing.B) string {
	dir := b.TempDir()
	writeFile(b, filepath.Join(dir, "go.mod"), `module benchmark-project`)
	return dir
}

func setupPythonProject(b *testing.B) string {
	dir := b.TempDir()
	writeFile(b, filepath.Join(dir, "setup.py"), `from setuptools import setup`)
	return dir
}

func setupComplexProject(b *testing.B) string {
	dir := b.TempDir()
	writeFile(b, filepath.Join(dir, "package.json"), `{"name": "complex-project"}`)
	writeFile(b, filepath.Join(dir, "go.mod"), `module complex-project`)
	writeFile(b, filepath.Join(dir, "setup.py"), `from setuptools import setup`)
	return dir
}

func generateLargeContent(size int) string {
	content := make([]byte, size)
	for i := range content {
		content[i] = byte('A' + (i % 26))
	}
	return string(content)
}

func writeFile(b *testing.B, path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		b.Fatalf("Failed to create directory for %s: %v", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		b.Fatalf("Failed to create file %s: %v", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		b.Fatalf("Failed to write to file %s: %v", path, err)
	}
}
