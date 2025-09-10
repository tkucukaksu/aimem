package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tarkank/aimem/internal/project"
	"github.com/tarkank/aimem/internal/storage"
	"github.com/tarkank/aimem/internal/types"
)

// SessionManager manages intelligent session creation and lifecycle
type SessionManager struct {
	projectDetector *project.ProjectDetector
	storage         storage.Storage
	sessionCache    map[string]*types.SessionInfo
	config          *types.Config
	cacheMu         sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(projectDetector *project.ProjectDetector, storage storage.Storage, config *types.Config) *SessionManager {
	return &SessionManager{
		projectDetector: projectDetector,
		storage:         storage,
		sessionCache:    make(map[string]*types.SessionInfo),
		config:          config,
	}
}

// GetOrCreateProjectSession is the main entry point for intelligent session management
func (sm *SessionManager) GetOrCreateProjectSession(workingDir string) (*types.SessionInfo, error) {
	ctx := context.Background()

	// 1. Detect project from working directory
	projectInfo, err := sm.projectDetector.DetectProject(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect project: %w", err)
	}

	// 2. Find or create main session for this project
	session, err := sm.getOrCreateMainSession(ctx, projectInfo, workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create main session: %w", err)
	}

	// 3. Update session activity
	session.LastActive = time.Now()
	session.WorkingDir = workingDir // Update working directory
	if err := sm.updateSessionActivity(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session activity: %w", err)
	}

	// 4. Cache session
	sm.cacheMu.Lock()
	sm.sessionCache[session.ID] = session
	sm.cacheMu.Unlock()

	return session, nil
}

// getOrCreateMainSession finds existing main session or creates a new one
func (sm *SessionManager) getOrCreateMainSession(ctx context.Context, projectInfo *types.ProjectInfo, workingDir string) (*types.SessionInfo, error) {
	// Try to find existing main session
	if mainSession, err := sm.findMainSession(ctx, projectInfo.ID); err == nil && mainSession != nil {
		// Update working directory
		mainSession.WorkingDir = workingDir
		return mainSession, nil
	}

	// Create new main session
	return sm.createMainSession(ctx, projectInfo, workingDir)
}

// findMainSession searches for active main session of a project
func (sm *SessionManager) findMainSession(ctx context.Context, projectID string) (*types.SessionInfo, error) {
	// This would query the storage for existing sessions
	// For now, we'll implement a simple check
	sessions, err := sm.listProjectSessions(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.Type == types.SessionTypeMain && session.Status == types.SessionStatusActive {
			return session, nil
		}
	}

	return nil, nil // No main session found
}

// createMainSession creates a new main session for the project
func (sm *SessionManager) createMainSession(ctx context.Context, projectInfo *types.ProjectInfo, workingDir string) (*types.SessionInfo, error) {
	sessionID := sm.generateSessionID(projectInfo, types.SessionTypeMain)

	session := &types.SessionInfo{
		ID:         sessionID,
		ProjectID:  projectInfo.ID,
		Name:       fmt.Sprintf("%s-main", projectInfo.Name),
		Type:       types.SessionTypeMain,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     types.SessionStatusActive,
		WorkingDir: workingDir,
		Metadata: map[string]interface{}{
			"project_name":      projectInfo.Name,
			"project_type":      projectInfo.Type,
			"canonical_path":    projectInfo.CanonicalPath,
			"language":          projectInfo.Language,
			"framework":         projectInfo.Framework,
			"auto_created":      true,
			"creation_method":   "smart_detection",
			"workspace_markers": projectInfo.WorkspaceMarkers,
		},
	}

	// Add git information if available
	if projectInfo.GitRoot != nil {
		session.Metadata["git_root"] = *projectInfo.GitRoot
	}
	if projectInfo.GitRemote != nil {
		session.Metadata["git_remote"] = *projectInfo.GitRemote
	}

	// Store session (this will be implemented when we extend storage)
	if err := sm.createSessionInStorage(ctx, session, projectInfo); err != nil {
		return nil, fmt.Errorf("failed to create session in storage: %w", err)
	}

	return session, nil
}

// generateSessionID creates intelligent session IDs
func (sm *SessionManager) generateSessionID(projectInfo *types.ProjectInfo, sessionType types.SessionType) string {
	projectPrefix := projectInfo.ID[:8] // First 8 characters

	if sessionType == types.SessionTypeMain {
		// Main sessions use stable IDs based on project
		return fmt.Sprintf("%s-main", projectPrefix)
	}

	// Other session types get unique suffixes
	uniqueSuffix := uuid.New().String()[:8]
	return fmt.Sprintf("%s-%s-%s", projectPrefix, sessionType, uniqueSuffix)
}

// ResolveSession handles legacy session IDs and path-based resolution
func (sm *SessionManager) ResolveSession(sessionIDOrPath string) (*types.SessionInfo, error) {
	ctx := context.Background()

	// 1. Try direct session ID lookup
	if session, err := sm.GetSession(ctx, sessionIDOrPath); err == nil {
		return session, nil
	}

	// 2. Treat as path and create/find project session
	if strings.Contains(sessionIDOrPath, "/") || strings.Contains(sessionIDOrPath, "\\") {
		return sm.GetOrCreateProjectSession(sessionIDOrPath)
	}

	// 3. Try legacy session migration
	if legacySession, err := sm.migrateLegacySession(ctx, sessionIDOrPath); err == nil {
		return legacySession, nil
	}

	return nil, fmt.Errorf("could not resolve session: %s", sessionIDOrPath)
}

// migrateLegacySession attempts to migrate old session IDs
func (sm *SessionManager) migrateLegacySession(ctx context.Context, legacySessionID string) (*types.SessionInfo, error) {
	// Check if legacy database exists
	legacyDbPath := sm.getLegacyDatabasePath(legacySessionID)
	if _, err := os.Stat(legacyDbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("legacy session not found: %s", legacySessionID)
	}

	// Use current working directory for project detection
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create modern session for current project
	session, err := sm.GetOrCreateProjectSession(currentDir)
	if err != nil {
		return nil, err
	}

	// Schedule background migration of legacy contexts
	go sm.scheduleLegacyMigration(legacySessionID, session.ID)

	return session, nil
}

// getLegacyDatabasePath returns the path to legacy database file
func (sm *SessionManager) getLegacyDatabasePath(sessionID string) string {
	homeDir, _ := os.UserHomeDir()
	aimemDir := filepath.Join(homeDir, ".aimem")
	return filepath.Join(aimemDir, fmt.Sprintf("aimem_%s.db", sessionID))
}

// scheduleLegacyMigration schedules background migration of legacy contexts
func (sm *SessionManager) scheduleLegacyMigration(legacySessionID, newSessionID string) {
	// This would implement actual migration logic
	fmt.Printf("Scheduled legacy migration: %s -> %s\n", legacySessionID, newSessionID)
	// TODO: Implement actual context migration
}

// CreateFeatureSession creates a feature-specific session
func (sm *SessionManager) CreateFeatureSession(ctx context.Context, parentSessionID string, featureName string) (*types.SessionInfo, error) {
	// Get parent session
	parentSession, err := sm.GetSession(ctx, parentSessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent session: %w", err)
	}

	// Generate feature session ID
	projectInfo := &types.ProjectInfo{ID: parentSession.ProjectID}
	sessionID := sm.generateSessionID(projectInfo, types.SessionTypeFeature)

	session := &types.SessionInfo{
		ID:              sessionID,
		ProjectID:       parentSession.ProjectID,
		Name:            fmt.Sprintf("feature-%s", featureName),
		Type:            types.SessionTypeFeature,
		ParentSessionID: &parentSessionID,
		CreatedAt:       time.Now(),
		LastActive:      time.Now(),
		Status:          types.SessionStatusActive,
		WorkingDir:      parentSession.WorkingDir,
		Metadata: map[string]interface{}{
			"feature_name":   featureName,
			"parent_session": parentSessionID,
			"branched_from":  time.Now().Format(time.RFC3339),
		},
	}

	// Store feature session
	if err := sm.createSessionInStorage(ctx, session, nil); err != nil {
		return nil, fmt.Errorf("failed to create feature session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*types.SessionInfo, error) {
	// Check cache first
	sm.cacheMu.RLock()
	session, exists := sm.sessionCache[sessionID]
	sm.cacheMu.RUnlock()

	if exists {
		return session, nil
	}

	// Load from storage (this will be implemented when storage is extended)
	session, err := sm.loadSessionFromStorage(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Cache session
	sm.cacheMu.Lock()
	sm.sessionCache[sessionID] = session
	sm.cacheMu.Unlock()

	return session, nil
}

// Helper methods for storage integration
func (sm *SessionManager) createSessionInStorage(ctx context.Context, session *types.SessionInfo, projectInfo *types.ProjectInfo) error {
	// First ensure project exists in storage
	if projectInfo != nil {
		if err := sm.storage.CreateProject(ctx, projectInfo); err != nil {
			return fmt.Errorf("failed to create project in storage: %w", err)
		}
	}

	// Then create the session
	if err := sm.storage.CreateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to create session in storage: %w", err)
	}

	return nil
}

func (sm *SessionManager) loadSessionFromStorage(ctx context.Context, sessionID string) (*types.SessionInfo, error) {
	return sm.storage.GetSession(ctx, sessionID)
}

func (sm *SessionManager) listProjectSessions(ctx context.Context, projectID string) ([]*types.SessionInfo, error) {
	return sm.storage.GetProjectSessions(ctx, projectID)
}

func (sm *SessionManager) updateSessionActivity(ctx context.Context, session *types.SessionInfo) error {
	return sm.storage.UpdateSession(ctx, session)
}

// GetSessionInfo returns formatted session information
func (sm *SessionManager) GetSessionInfo(session *types.SessionInfo) string {
	projectName := "Unknown"
	if name, exists := session.Metadata["project_name"].(string); exists {
		projectName = name
	}

	return fmt.Sprintf("Session: %s | Project: %s | Type: %s | Path: %s",
		session.ID, projectName, session.Type, session.WorkingDir)
}

// ListActiveSessions returns all active sessions
func (sm *SessionManager) ListActiveSessions(ctx context.Context) ([]*types.SessionInfo, error) {
	// TODO: Implement when storage interface is extended
	return []*types.SessionInfo{}, nil
}
