package logger

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// Config contains logger configuration
type Config struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"` // json or text
	Output     string `yaml:"output"` // stdout, stderr, or file path
	EnableCaller bool `yaml:"enable_caller"`
}

// Fields is an alias for logrus.Fields for convenience
type Fields = logrus.Fields

// Logger wraps logrus.Logger with additional context and performance monitoring
type Logger struct {
	*logrus.Logger
	serviceName string
}

// NewLogger creates a new production-ready logger
func NewLogger(config *Config, serviceName string) (*Logger, error) {
	logger := logrus.New()
	
	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
	
	// Set formatter
	if config.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "caller",
			},
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}
	
	// Set output
	switch config.Output {
	case "stderr":
		logger.SetOutput(os.Stderr)
	case "stdout", "":
		logger.SetOutput(os.Stdout)
	default:
		// File output
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.SetOutput(file)
	}
	
	// Enable caller info if requested
	if config.EnableCaller {
		logger.SetReportCaller(true)
	}
	
	// Add global fields
	logger = logger.WithFields(logrus.Fields{
		"service": serviceName,
		"version": "1.0.0", // Could be injected from build
	}).Logger
	
	return &Logger{
		Logger:      logger,
		serviceName: serviceName,
	}, nil
}

// WithContext creates a logger with request context
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.WithFields(logrus.Fields{})
	
	// Add request ID if available
	if requestID := getRequestID(ctx); requestID != "" {
		entry = entry.WithField("request_id", requestID)
	}
	
	// Add session ID if available
	if sessionID := getSessionID(ctx); sessionID != "" {
		entry = entry.WithField("session_id", sessionID)
	}
	
	return entry
}

// WithOperation creates a logger for a specific operation
func (l *Logger) WithOperation(operation string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"operation": operation,
	})
}

// WithError creates a logger entry with error information
func (l *Logger) WithError(err error) *logrus.Entry {
	entry := l.WithField("error", err.Error())
	
	// Add stack trace for debugging
	if l.GetLevel() <= logrus.DebugLevel {
		entry = entry.WithField("stack_trace", getStackTrace())
	}
	
	return entry
}

// LogOperation logs the start and end of an operation with duration
func (l *Logger) LogOperation(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()
	logger := l.WithContext(ctx).WithField("operation", operation)
	
	logger.Debug("Operation started")
	
	err := fn()
	duration := time.Since(start)
	
	if err != nil {
		logger.WithFields(logrus.Fields{
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		}).Error("Operation failed")
		return err
	}
	
	logger.WithField("duration_ms", duration.Milliseconds()).Info("Operation completed")
	return nil
}

// LogPerformance logs performance metrics for an operation
func (l *Logger) LogPerformance(ctx context.Context, operation string, duration time.Duration, metadata map[string]interface{}) {
	fields := logrus.Fields{
		"operation":   operation,
		"duration_ms": duration.Milliseconds(),
	}
	
	// Add metadata
	for k, v := range metadata {
		fields[k] = v
	}
	
	l.WithContext(ctx).WithFields(fields).Info("Performance metric")
}

// LogMemoryUsage logs current memory usage
func (l *Logger) LogMemoryUsage(ctx context.Context, operation string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	l.WithContext(ctx).WithFields(logrus.Fields{
		"operation":     operation,
		"alloc_mb":      bToMb(m.Alloc),
		"total_alloc_mb": bToMb(m.TotalAlloc),
		"sys_mb":        bToMb(m.Sys),
		"num_gc":        m.NumGC,
	}).Debug("Memory usage")
}

// Helper functions for context values
func getRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return ""
}

func getSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value("session_id").(string); ok {
		return id
	}
	return ""
}

func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// Context helpers

// ContextWithRequestID adds a request ID to the context
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, "request_id", requestID)
}

// ContextWithSessionID adds a session ID to the context
func ContextWithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, "session_id", sessionID)
}

// Default logger configuration for development and production

// DevelopmentConfig returns a logger config suitable for development
func DevelopmentConfig() *Config {
	return &Config{
		Level:        "debug",
		Format:       "text",
		Output:       "stdout",
		EnableCaller: true,
	}
}

// ProductionConfig returns a logger config suitable for production
func ProductionConfig() *Config {
	return &Config{
		Level:        "info",
		Format:       "json",
		Output:       "stdout",
		EnableCaller: false,
	}
}

// TestConfig returns a logger config suitable for testing
func TestConfig() *Config {
	return &Config{
		Level:        "warn",
		Format:       "text",
		Output:       "stderr",
		EnableCaller: false,
	}
}