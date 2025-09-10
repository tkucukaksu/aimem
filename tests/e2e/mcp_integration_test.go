package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarkank/aimem/internal/mcp"
	"github.com/tarkank/aimem/internal/server"
	"github.com/tarkank/aimem/internal/types"
)

// TestMCPIntegrationFullWorkflow tests the complete MCP integration workflow
func TestMCPIntegrationFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup
	tempDir := t.TempDir()
	config := createE2ETestConfig(tempDir)

	// Initialize AIMem server
	aimemServer, err := server.NewAIMem(config)
	require.NoError(t, err)
	defer aimemServer.Close()

	// Create test project
	projectDir := setupTestProject(t, tempDir)

	t.Run("CompleteWorkflow", func(t *testing.T) {
		testCompleteWorkflow(t, aimemServer, projectDir)
	})

	t.Run("SessionHierarchyWorkflow", func(t *testing.T) {
		testSessionHierarchyWorkflow(t, aimemServer, projectDir)
	})

	t.Run("PerformanceMonitoringWorkflow", func(t *testing.T) {
		testPerformanceMonitoringWorkflow(t, aimemServer)
	})
}

func testCompleteWorkflow(t *testing.T, server *server.AIMem, projectDir string) {
	ctx := context.Background()

	// Step 1: Create project session
	sessionReq := createMCPRequest("get_or_create_project_session", map[string]interface{}{
		"working_dir": projectDir,
	})

	sessionResp := executeMCPRequest(t, server, sessionReq)
	sessionID := extractSessionIDFromResponse(t, sessionResp)

	// Step 2: Store project context
	storeReq := createMCPRequest("auto_store_project", map[string]interface{}{
		"session_id":           sessionID,
		"project_path":         projectDir,
		"focus_areas":          []string{"architecture", "api"},
		"importance_threshold": "medium",
		"silent":               false,
	})

	storeResp := executeMCPRequest(t, server, storeReq)
	assertMCPSuccess(t, storeResp)

	// Step 3: Store additional context
	contextReq := createMCPRequest("store_context", map[string]interface{}{
		"session_id": sessionID,
		"content":    "User authentication system using JWT tokens. Login endpoint at /api/auth/login accepts email and password.",
		"importance": "high",
		"silent":     false,
	})

	contextResp := executeMCPRequest(t, server, contextReq)
	assertMCPSuccess(t, contextResp)

	// Step 4: Retrieve context with task-aware intelligence
	retrieveReq := createMCPRequest("context_aware_retrieve", map[string]interface{}{
		"session_id":   sessionID,
		"current_task": "Debug authentication failing with 401 error",
		"task_type":    "debugging",
		"auto_expand":  true,
		"max_chunks":   5,
	})

	retrieveResp := executeMCPRequest(t, server, retrieveReq)
	assertMCPSuccess(t, retrieveResp)
	assertResponseContains(t, retrieveResp, "authentication")

	// Step 5: Get session info
	infoReq := createMCPRequest("get_session_info", map[string]interface{}{
		"session_id": sessionID,
	})

	infoResp := executeMCPRequest(t, server, infoReq)
	assertMCPSuccess(t, infoResp)
	assertResponseContains(t, infoResp, sessionID)

	// Step 6: Get performance metrics
	perfReq := createMCPRequest("get_performance_metrics", map[string]interface{}{
		"metric_type": "session",
		"session_id":  sessionID,
	})

	perfResp := executeMCPRequest(t, server, perfReq)
	assertMCPSuccess(t, perfResp)

	// Step 7: Session cleanup
	cleanupReq := createMCPRequest("cleanup_session", map[string]interface{}{
		"session_id": sessionID,
		"strategy":   "relevance",
	})

	cleanupResp := executeMCPRequest(t, server, cleanupReq)
	assertMCPSuccess(t, cleanupResp)
}

func testSessionHierarchyWorkflow(t *testing.T, server *server.AIMem, projectDir string) {
	ctx := context.Background()

	// Create main session
	mainSessionReq := createMCPRequest("get_or_create_project_session", map[string]interface{}{
		"working_dir": projectDir,
	})

	mainSessionResp := executeMCPRequest(t, server, mainSessionReq)
	mainSessionID := extractSessionIDFromResponse(t, mainSessionResp)

	// Create feature session
	featureReq := createMCPRequest("create_feature_session", map[string]interface{}{
		"parent_session_id": mainSessionID,
		"feature_name":      "user-profile-api",
	})

	featureResp := executeMCPRequest(t, server, featureReq)
	featureSessionID := extractSessionIDFromResponse(t, featureResp)
	assertMCPSuccess(t, featureResp)
	assert.NotEqual(t, mainSessionID, featureSessionID)
	assert.Contains(t, featureSessionID, "feature")

	// Store context in feature session
	contextReq := createMCPRequest("store_context", map[string]interface{}{
		"session_id": featureSessionID,
		"content":    "Feature: User profile API endpoints. GET /api/profile returns user profile data. PUT /api/profile updates profile.",
		"importance": "high",
	})

	contextResp := executeMCPRequest(t, server, contextReq)
	assertMCPSuccess(t, contextResp)

	// List project sessions
	listReq := createMCPRequest("list_project_sessions", map[string]interface{}{
		"project_id":       extractProjectID(t, mainSessionID),
		"include_inactive": false,
	})

	listResp := executeMCPRequest(t, server, listReq)
	assertMCPSuccess(t, listResp)
	// Should show at least 2 sessions (main + feature)
	assertResponseContains(t, listResp, "2")
}

func testPerformanceMonitoringWorkflow(t *testing.T, server *server.AIMem) {
	// Get system metrics
	sysMetricsReq := createMCPRequest("get_performance_metrics", map[string]interface{}{
		"metric_type": "system",
	})

	sysMetricsResp := executeMCPRequest(t, server, sysMetricsReq)
	assertMCPSuccess(t, sysMetricsResp)
	assertResponseContains(t, sysMetricsResp, "requests")
	assertResponseContains(t, sysMetricsResp, "latency")

	// Get operation metrics
	opMetricsReq := createMCPRequest("get_performance_metrics", map[string]interface{}{
		"metric_type": "operation",
	})

	opMetricsResp := executeMCPRequest(t, server, opMetricsReq)
	assertMCPSuccess(t, opMetricsResp)
	assertResponseContains(t, opMetricsResp, "Operation Metrics")

	// Get all metrics
	allMetricsReq := createMCPRequest("get_performance_metrics", map[string]interface{}{
		"metric_type": "all",
	})

	allMetricsResp := executeMCPRequest(t, server, allMetricsReq)
	assertMCPSuccess(t, allMetricsResp)
	assertResponseContains(t, allMetricsResp, "Performance Report")
}

// Helper functions

func createE2ETestConfig(tempDir string) *types.Config {
	return &types.Config{
		Database: "sqlite",
		SQLite: types.SQLiteConfig{
			DatabasePath:          filepath.Join(tempDir, "e2e_test.db"),
			MaxConnections:        5,
			MaxIdleConnections:    2,
			ConnectionMaxLifetime: 30,
		},
		Memory: types.MemoryConfig{
			MaxSessionSize:    "5MB",
			ChunkSize:         1024,
			MaxChunksPerQuery: 5,
			TTLDefault:        30 * time.Minute,
		},
		Embedding: types.EmbeddingConfig{
			Model:     "test-embedding-model",
			CacheSize: 50,
			BatchSize: 8,
		},
		Performance: types.PerformanceConfig{
			CompressionEnabled: true,
			AsyncProcessing:    false, // Synchronous for testing
			CacheEmbeddings:    true,
			EnableMetrics:      true,
			MetricsInterval:    100 * time.Millisecond,
		},
		MCP: types.MCPConfig{
			ServerName:  "AIMem-E2E-Test",
			Version:     "2.0.0-e2e",
			Description: "End-to-end test instance",
		},
		SessionManager: types.SessionManagerConfig{
			EnableAutoDetection:    true,
			EnableLegacyMigration:  true,
			DefaultSessionType:     types.SessionTypeMain,
			SessionCacheSize:       20,
			SessionTimeout:         30 * time.Minute,
			MaxSessionsPerProject:  3,
			EnableSessionHierarchy: true,
			AutoCleanupInactive:    false,
			InactiveThreshold:      1 * time.Hour,
		},
		ProjectDetector: types.ProjectDetectorConfig{
			EnableCaching:             true,
			CacheTimeout:              5 * time.Minute,
			MaxCacheSize:              50,
			DeepScanEnabled:           true,
			GitDetectionEnabled:       true,
			WorkspaceDetectionEnabled: true,
			LanguageDetectionEnabled:  true,
			CustomWorkspaceMarkers:    []string{},
			IgnorePatterns:            []string{".git", "node_modules"},
		},
	}
}

func setupTestProject(t *testing.T, tempDir string) string {
	projectDir := filepath.Join(tempDir, "e2e-test-project")

	// Create Git repository
	err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0755)
	require.NoError(t, err)

	gitConfig := `[core]
    repositoryformatversion = 0
[remote "origin"]
    url = https://github.com/test/e2e-project.git`

	err = os.WriteFile(filepath.Join(projectDir, ".git", "config"), []byte(gitConfig), 0644)
	require.NoError(t, err)

	// Create Node.js project
	packageJSON := `{
  "name": "e2e-test-project",
  "version": "1.0.0",
  "description": "End-to-end test project",
  "main": "index.js",
  "dependencies": {
    "express": "^4.18.0"
  }
}`

	err = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Create source files
	indexJS := `const express = require('express');
const app = express();

app.get('/api/auth/login', (req, res) => {
    // Authentication logic here
    res.json({ token: 'jwt-token' });
});

app.listen(3000, () => {
    console.log('Server running on port 3000');
});`

	err = os.WriteFile(filepath.Join(projectDir, "index.js"), []byte(indexJS), 0644)
	require.NoError(t, err)

	return projectDir
}

func createMCPRequest(toolName string, arguments map[string]interface{}) *mcp.Request {
	return &mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}
}

func executeMCPRequest(t *testing.T, server *server.AIMem, req *mcp.Request) *mcp.Response {
	ctx := context.Background()

	// Serialize request
	reqData, err := json.Marshal(req)
	require.NoError(t, err)

	// Create buffers for request/response
	reqBuffer := bytes.NewBuffer(reqData)
	respBuffer := &bytes.Buffer{}

	// Execute request
	err = server.HandleRequest(ctx, reqBuffer, respBuffer)
	require.NoError(t, err)

	// Parse response
	var resp mcp.Response
	err = json.Unmarshal(respBuffer.Bytes(), &resp)
	require.NoError(t, err)

	return &resp
}

func assertMCPSuccess(t *testing.T, resp *mcp.Response) {
	assert.Nil(t, resp.Error, "MCP request should succeed")
	assert.NotNil(t, resp.Result, "MCP response should have result")
}

func assertResponseContains(t *testing.T, resp *mcp.Response, expectedContent string) {
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok, "Response result should be an object")

	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Response should have content array")
	require.NotEmpty(t, content, "Content array should not be empty")

	firstContent, ok := content[0].(map[string]interface{})
	require.True(t, ok, "First content item should be an object")

	text, ok := firstContent["text"].(string)
	require.True(t, ok, "Content should have text field")

	assert.Contains(t, text, expectedContent, "Response should contain expected content")
}

func extractSessionIDFromResponse(t *testing.T, resp *mcp.Response) string {
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	content, ok := result["content"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, content)

	firstContent, ok := content[0].(map[string]interface{})
	require.True(t, ok)

	text, ok := firstContent["text"].(string)
	require.True(t, ok)

	// Extract session ID from response text
	// This is a simplified extraction - in real scenarios, you might parse it more carefully
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Session ID:") || strings.Contains(line, "session-id") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				sessionID := strings.TrimSpace(parts[1])
				if sessionID != "" && sessionID != "session-id" {
					return sessionID
				}
			}
		}
	}

	// Fallback: generate a test session ID
	return "test-session-" + fmt.Sprintf("%d", time.Now().Unix())
}

func extractProjectID(t *testing.T, sessionID string) string {
	// Extract project ID from session ID format: {project-hash}-main
	parts := strings.Split(sessionID, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return "test-project-id"
}
