package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tarkank/aimem/internal/config"
	"github.com/tarkank/aimem/internal/server"
	"github.com/tarkank/aimem/internal/types"
)

// Default config path will be determined dynamically

func main() {
	// Parse command line flags
	var (
		configPath = flag.String("config", config.GetDefaultConfigPath(), "Path to configuration file")
		showHelp   = flag.Bool("help", false, "Show help message")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showHelp {
		showUsage()
		return
	}

	if *version {
		showVersion()
		return
	}

	// Load configuration
	cfg, err := loadConfiguration(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize AIMem server
	aimemServer, err := server.NewAIMem(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize AIMem server: %v", err)
	}

	// Ensure graceful shutdown
	defer func() {
		if closeErr := aimemServer.Close(); closeErr != nil {
			log.Printf("Error during server shutdown: %v", closeErr)
		}
	}()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// Start MCP server
	log.Printf("Starting AIMem MCP Server v%s", cfg.MCP.Version)
	log.Printf("Server: %s", cfg.MCP.Description)
	if cfg.Database == "sqlite" {
		log.Printf("Database: SQLite (%s)", cfg.SQLite.DatabasePath)
	} else {
		log.Printf("Database: Redis (%s)", cfg.Redis.Host)
	}

	// Handle MCP requests from stdin/stdout
	if err := aimemServer.HandleRequest(ctx, os.Stdin, os.Stdout); err != nil {
		if ctx.Err() != nil {
			log.Println("Server shutdown completed")
		} else {
			log.Fatalf("Server error: %v", err)
		}
	}
}

// loadConfiguration loads configuration with fallbacks
func loadConfiguration(configPath string) (*types.Config, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Configuration file %s not found, using defaults", configPath)
		
		// Generate default config file
		cfg := config.GetDefaultConfig()
		if err := config.SaveConfig(cfg, configPath); err != nil {
			log.Printf("Warning: Could not save default config: %v", err)
		} else {
			log.Printf("Generated default configuration file: %s", configPath)
		}
		
		return cfg, nil
	}

	// Load from file
	return config.LoadConfig(configPath)
}

// showUsage displays usage information
func showUsage() {
	fmt.Printf(`AIMem - AI Memory Management Server

A Model Context Protocol (MCP) server that provides intelligent context storage
and retrieval capabilities for AI systems.

USAGE:
    aimem [OPTIONS]

OPTIONS:
    -config string
        Path to configuration file (default: %s)
    -help
        Show this help message
    -version  
        Show version information

CONFIGURATION:
    AIMem uses a YAML configuration file stored in ~/.aimem/aimem.yaml
    If the file doesn't exist, a default configuration will be generated automatically.

    Example configuration structure:
    - Redis connection settings
    - Memory management parameters  
    - Embedding service configuration
    - Performance tuning options

MCP INTEGRATION:
    AIMem implements the Model Context Protocol and provides the following tools:
    - store_context:     Store conversation context with importance levels
    - retrieve_context:  Retrieve relevant context using semantic search
    - summarize_session: Get session overview and statistics
    - cleanup_session:   Clean old or low-relevance context

    To use with Claude Code, add AIMem to your MCP client configuration.

EXAMPLES:
    # Start with default configuration
    aimem

    # Start with custom configuration
    aimem -config /path/to/custom.yaml

    # Generate default configuration and exit
    aimem -help

For more information, visit: https://github.com/tarkank/aimem
`, config.GetDefaultConfigPath())
}

// showVersion displays version information
func showVersion() {
	cfg := config.GetDefaultConfig()
	fmt.Printf(`AIMem v%s
%s

Built with Go
Model Context Protocol 2024-11-05
Redis-powered context storage
Local embedding generation
`, cfg.MCP.Version, cfg.MCP.Description)
}