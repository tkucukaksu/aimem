package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
	"github.com/tarkank/aimem/internal/types"
)

// LoadConfig loads configuration from YAML file
func LoadConfig(configPath string) (*types.Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Ensure AIMem directory exists
	if err := ensureAIMemDir(); err != nil {
		return nil, fmt.Errorf("failed to create AIMem directory: %v", err)
	}

	// Set defaults and validate
	if err := setDefaults(&config); err != nil {
		return nil, fmt.Errorf("failed to set config defaults: %v", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return &config, nil
}

// getAIMemHomeDir returns the AIMem home directory path
func getAIMemHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir is not available
		return ".aimem"
	}
	return filepath.Join(homeDir, ".aimem")
}

// getDefaultDatabasePath returns the default database path in user home
func getDefaultDatabasePath() string {
	aimemDir := getAIMemHomeDir()
	return filepath.Join(aimemDir, "aimem.db")
}

// ensureAIMemDir creates the AIMem directory if it doesn't exist
func ensureAIMemDir() error {
	aimemDir := getAIMemHomeDir()
	return os.MkdirAll(aimemDir, 0755)
}

// GetProjectDatabasePath returns database path for specific project/session
func GetProjectDatabasePath(sessionID string) string {
	aimemDir := getAIMemHomeDir()
	// Create a safe filename from session ID
	safeSessionID := filepath.Base(sessionID)
	if safeSessionID == "" || safeSessionID == "." {
		safeSessionID = "default"
	}
	return filepath.Join(aimemDir, fmt.Sprintf("aimem_%s.db", safeSessionID))
}

// GetDefaultConfigPath returns the default config file path in user home
func GetDefaultConfigPath() string {
	aimemDir := getAIMemHomeDir()
	return filepath.Join(aimemDir, "aimem.yaml")
}

// GetDefaultConfig returns a configuration with default values
func GetDefaultConfig() *types.Config {
	// Ensure AIMem directory exists
	ensureAIMemDir()
	config := &types.Config{
		Database: "sqlite", // Default to SQLite for zero-config startup
		Redis: types.RedisConfig{
			Host:     "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
		SQLite: types.SQLiteConfig{
			DatabasePath:          getDefaultDatabasePath(),
			MaxConnections:        10,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 60, // 60 minutes
		},
		Memory: types.MemoryConfig{
			MaxSessionSize:    "10MB",
			ChunkSize:         1024,
			MaxChunksPerQuery: 5,
			TTLDefault:        24 * time.Hour,
		},
		Embedding: types.EmbeddingConfig{
			Model:     "all-MiniLM-L6-v2",
			CacheSize: 1000,
			BatchSize: 32,
		},
		Performance: types.PerformanceConfig{
			CompressionEnabled: true,
			AsyncProcessing:    true,
			CacheEmbeddings:    true,
		},
		MCP: types.MCPConfig{
			ServerName:  "AIMem",
			Version:     "1.5.0",
			Description: "AI Memory Management Server - SQLite powered, zero external dependencies",
		},
	}

	return config
}

// setDefaults sets default values for missing configuration fields
func setDefaults(config *types.Config) error {
	defaults := GetDefaultConfig()

	// Database defaults
	if config.Database == "" {
		config.Database = defaults.Database
	}

	// Redis defaults
	if config.Redis.Host == "" {
		config.Redis.Host = defaults.Redis.Host
	}
	if config.Redis.PoolSize == 0 {
		config.Redis.PoolSize = defaults.Redis.PoolSize
	}

	// SQLite defaults
	if config.SQLite.DatabasePath == "" {
		config.SQLite.DatabasePath = getDefaultDatabasePath()
	}
	if config.SQLite.MaxConnections == 0 {
		config.SQLite.MaxConnections = defaults.SQLite.MaxConnections
	}
	if config.SQLite.MaxIdleConnections == 0 {
		config.SQLite.MaxIdleConnections = defaults.SQLite.MaxIdleConnections
	}
	if config.SQLite.ConnectionMaxLifetime == 0 {
		config.SQLite.ConnectionMaxLifetime = defaults.SQLite.ConnectionMaxLifetime
	}

	// Memory defaults
	if config.Memory.MaxSessionSize == "" {
		config.Memory.MaxSessionSize = defaults.Memory.MaxSessionSize
	}
	if config.Memory.ChunkSize == 0 {
		config.Memory.ChunkSize = defaults.Memory.ChunkSize
	}
	if config.Memory.MaxChunksPerQuery == 0 {
		config.Memory.MaxChunksPerQuery = defaults.Memory.MaxChunksPerQuery
	}
	if config.Memory.TTLDefault == 0 {
		config.Memory.TTLDefault = defaults.Memory.TTLDefault
	}

	// Embedding defaults
	if config.Embedding.Model == "" {
		config.Embedding.Model = defaults.Embedding.Model
	}
	if config.Embedding.CacheSize == 0 {
		config.Embedding.CacheSize = defaults.Embedding.CacheSize
	}
	if config.Embedding.BatchSize == 0 {
		config.Embedding.BatchSize = defaults.Embedding.BatchSize
	}

	// MCP defaults
	if config.MCP.ServerName == "" {
		config.MCP.ServerName = defaults.MCP.ServerName
	}
	if config.MCP.Version == "" {
		config.MCP.Version = defaults.MCP.Version
	}
	if config.MCP.Description == "" {
		config.MCP.Description = defaults.MCP.Description
	}

	return nil
}

// validateConfig validates configuration values
func validateConfig(config *types.Config) error {
	// Validate Redis config
	if config.Redis.Host == "" {
		return fmt.Errorf("redis.host is required")
	}
	if config.Redis.PoolSize <= 0 {
		return fmt.Errorf("redis.pool_size must be positive")
	}

	// Validate Memory config
	if config.Memory.ChunkSize <= 0 {
		return fmt.Errorf("memory.chunk_size must be positive")
	}
	if config.Memory.MaxChunksPerQuery <= 0 {
		return fmt.Errorf("memory.max_chunks_per_query must be positive")
	}
	if config.Memory.TTLDefault <= 0 {
		return fmt.Errorf("memory.ttl_default must be positive")
	}

	// Validate Embedding config
	if config.Embedding.Model == "" {
		return fmt.Errorf("embedding.model is required")
	}
	if config.Embedding.CacheSize < 0 {
		return fmt.Errorf("embedding.cache_size cannot be negative")
	}
	if config.Embedding.BatchSize <= 0 {
		return fmt.Errorf("embedding.batch_size must be positive")
	}

	// Validate MCP config
	if config.MCP.ServerName == "" {
		return fmt.Errorf("mcp.server_name is required")
	}
	if config.MCP.Version == "" {
		return fmt.Errorf("mcp.version is required")
	}

	return nil
}

// SaveConfig saves configuration to YAML file
func SaveConfig(config *types.Config, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}