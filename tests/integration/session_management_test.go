package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarkank/aimem/internal/project"
	"github.com/tarkank/aimem/internal/session"
	"github.com/tarkank/aimem/internal/storage"
	"github.com/tarkank/aimem/internal/types"
)

// TestSessionManagementIntegration tests the complete session management flow
func TestSessionManagementIntegration(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	config := createTestConfig(tempDir)

	// Initialize components
	storageInstance, err := storage.NewStorage(config)
	require.NoError(t, err)
	defer storageInstance.Close()

	projectDetector := project.NewProjectDetector()
	sessionManager := session.NewSessionManager(projectDetector, storageInstance, config)

	t.Run("ProjectBasedSessionCreation", func(t *testing.T) {
		testProjectBasedSessionCreation(t, sessionManager, tempDir)
	})

	t.Run("SessionHierarchy", func(t *testing.T) {
		testSessionHierarchy(t, sessionManager, tempDir)
	})

	t.Run("LegacySessionMigration", func(t *testing.T) {
		testLegacySessionMigration(t, sessionManager, tempDir)
	})

	t.Run("SessionDiscovery", func(t *testing.T) {
		testSessionDiscovery(t, sessionManager, tempDir)
	})
}

func testProjectBasedSessionCreation(t *testing.T, sm *session.SessionManager, tempDir string) {
	// Create a mock Git project
	projectDir := filepath.Join(tempDir, "test-project")
	err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	// Create package.json to indicate Node.js project
	packageJSON := `{"name": "test-project", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Test session creation
	session1, err := sm.GetOrCreateProjectSession(projectDir)
	require.NoError(t, err)
	assert.NotEmpty(t, session1.ID)
	assert.Equal(t, types.SessionTypeMain, session1.Type)
	assert.Equal(t, types.SessionStatusActive, session1.Status)
	assert.Contains(t, session1.ID, "main")

	// Test deterministic behavior - should return same session
	session2, err := sm.GetOrCreateProjectSession(projectDir)
	require.NoError(t, err)
	assert.Equal(t, session1.ID, session2.ID)

	// Test session metadata
	assert.Equal(t, projectDir, session1.WorkingDir)
	assert.Contains(t, session1.Metadata, "project_name")
	assert.Contains(t, session1.Metadata, "language")
	assert.Equal(t, "test-project", session1.Metadata["project_name"])
}

func testSessionHierarchy(t *testing.T, sm *session.SessionManager, tempDir string) {
	ctx := context.Background()

	// Create main session first
	projectDir := filepath.Join(tempDir, "hierarchy-project")
	err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	mainSession, err := sm.GetOrCreateProjectSession(projectDir)
	require.NoError(t, err)

	// Create feature session
	featureSession, err := sm.CreateFeatureSession(ctx, mainSession.ID, "user-auth")
	require.NoError(t, err)

	assert.Equal(t, types.SessionTypeFeature, featureSession.Type)
	assert.Equal(t, mainSession.ID, *featureSession.ParentSessionID)
	assert.Contains(t, featureSession.ID, "feature")
	assert.Contains(t, featureSession.Name, "user-auth")
	assert.Equal(t, mainSession.ProjectID, featureSession.ProjectID)

	// Verify hierarchy relationship
	assert.Equal(t, "user-auth", featureSession.Metadata["feature_name"])
	assert.Equal(t, mainSession.ID, featureSession.Metadata["parent_session"])
}

func testLegacySessionMigration(t *testing.T, sm *session.SessionManager, tempDir string) {
	// Create a mock legacy session ID (UUID format)
	legacySessionID := "550e8400-e29b-41d4-a716-446655440000"

	// Create legacy database file (simplified simulation)
	aimemDir := filepath.Join(tempDir, ".aimem")
	err := os.MkdirAll(aimemDir, 0755)
	require.NoError(t, err)

	legacyDBPath := filepath.Join(aimemDir, "aimem_"+legacySessionID+".db")
	err = os.WriteFile(legacyDBPath, []byte("dummy"), 0644)
	require.NoError(t, err)

	// Create current working directory with project
	projectDir := filepath.Join(tempDir, "legacy-project")
	err = os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	// Change to project directory for migration test
	originalWD, _ := os.Getwd()
	err = os.Chdir(projectDir)
	require.NoError(t, err)
	defer os.Chdir(originalWD)

	// Test legacy session resolution
	resolvedSession, err := sm.ResolveSession(legacySessionID)
	require.NoError(t, err)

	// Should create a new project-based session
	assert.NotEqual(t, legacySessionID, resolvedSession.ID)
	assert.Contains(t, resolvedSession.ID, "main")
	assert.Equal(t, types.SessionTypeMain, resolvedSession.Type)
}

func testSessionDiscovery(t *testing.T, sm *session.SessionManager, tempDir string) {
	ctx := context.Background()

	// Create multiple project directories
	projects := []string{"project-a", "project-b", "project-c"}
	var sessions []*types.SessionInfo

	for _, projName := range projects {
		projectDir := filepath.Join(tempDir, projName)
		err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
		require.NoError(t, err)

		session, err := sm.GetOrCreateProjectSession(projectDir)
		require.NoError(t, err)
		sessions = append(sessions, session)
	}

	// Test active sessions listing
	activeSessions, err := sm.ListActiveSessions(ctx)
	require.NoError(t, err)

	// Should find at least the sessions we created (might be more from other tests)
	assert.GreaterOrEqual(t, len(activeSessions), len(projects))

	// Verify session discovery in specific directory
	discoveredSessions, err := sm.ListActiveSessions(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, discoveredSessions)
}

// TestPerformanceUnderLoad tests session management under concurrent load
func TestPerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()
	config := createTestConfig(tempDir)

	storageInstance, err := storage.NewStorage(config)
	require.NoError(t, err)
	defer storageInstance.Close()

	projectDetector := project.NewProjectDetector()
	sessionManager := session.NewSessionManager(projectDetector, storageInstance, config)

	// Create base project
	projectDir := filepath.Join(tempDir, "load-test-project")
	err = os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	// Test concurrent session creation
	concurrency := 10
	iterations := 50

	start := time.Now()

	results := make(chan error, concurrency*iterations)
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < iterations; j++ {
				_, err := sessionManager.GetOrCreateProjectSession(projectDir)
				results <- err
			}
		}(i)
	}

	// Collect results
	for i := 0; i < concurrency*iterations; i++ {
		err := <-results
		assert.NoError(t, err)
	}

	duration := time.Since(start)
	avgLatency := duration / time.Duration(concurrency*iterations)

	t.Logf("Performance test completed:")
	t.Logf("  Total time: %v", duration)
	t.Logf("  Average latency: %v", avgLatency)
	t.Logf("  Operations/sec: %.2f", float64(concurrency*iterations)/duration.Seconds())

	// Performance assertions
	assert.Less(t, avgLatency, 100*time.Millisecond, "Average latency should be under 100ms")
}

func createTestConfig(tempDir string) *types.Config {
	return &types.Config{
		Database: "sqlite",
		SQLite: types.SQLiteConfig{
			DatabasePath:          filepath.Join(tempDir, "test.db"),
			MaxConnections:        10,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 60,
		},
		Memory: types.MemoryConfig{
			MaxSessionSize:    "10MB",
			ChunkSize:         1024,
			MaxChunksPerQuery: 10,
			TTLDefault:        1 * time.Hour,
		},
		Embedding: types.EmbeddingConfig{
			Model:     "test-model",
			CacheSize: 100,
			BatchSize: 16,
		},
		Performance: types.PerformanceConfig{
			CompressionEnabled: true,
			AsyncProcessing:    false, // Disable for testing
			CacheEmbeddings:    true,
			EnableMetrics:      true,
			MetricsInterval:    1 * time.Second,
		},
		MCP: types.MCPConfig{
			ServerName:  "AIMem-Test",
			Version:     "2.0.0-test",
			Description: "Test instance",
		},
		SessionManager: types.SessionManagerConfig{
			EnableAutoDetection:    true,
			EnableLegacyMigration:  true,
			DefaultSessionType:     types.SessionTypeMain,
			SessionCacheSize:       50,
			SessionTimeout:         1 * time.Hour,
			MaxSessionsPerProject:  5,
			EnableSessionHierarchy: true,
			AutoCleanupInactive:    false, // Disable for testing
			InactiveThreshold:      24 * time.Hour,
		},
		ProjectDetector: types.ProjectDetectorConfig{
			EnableCaching:             true,
			CacheTimeout:              1 * time.Minute,
			MaxCacheSize:              100,
			DeepScanEnabled:           true,
			GitDetectionEnabled:       true,
			WorkspaceDetectionEnabled: true,
			LanguageDetectionEnabled:  true,
			CustomWorkspaceMarkers:    []string{},
			IgnorePatterns:            []string{"node_modules", ".git"},
		},
	}
}
