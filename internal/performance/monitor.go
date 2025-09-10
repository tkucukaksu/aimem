package performance

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tarkank/aimem/internal/logger"
	"github.com/tarkank/aimem/internal/types"
)

// PerformanceMonitor tracks system performance metrics
type PerformanceMonitor struct {
	mu               sync.RWMutex
	logger           *logger.Logger
	config           *types.PerformanceConfig
	startTime        time.Time
	requestCount     int64
	errorCount       int64
	totalLatency     int64 // microseconds
	sessionMetrics   map[string]*SessionMetrics
	operationMetrics map[string]*OperationMetrics
	memoryStats      *MemoryStats
	enabled          bool
}

// SessionMetrics tracks per-session performance
type SessionMetrics struct {
	SessionID      string        `json:"session_id"`
	RequestCount   int64         `json:"request_count"`
	AverageLatency time.Duration `json:"average_latency"`
	LastActivity   time.Time     `json:"last_activity"`
	MemoryUsage    int64         `json:"memory_usage_bytes"`
	ChunkCount     int64         `json:"chunk_count"`
	EmbeddingTime  time.Duration `json:"embedding_time"`
	StorageTime    time.Duration `json:"storage_time"`
}

// OperationMetrics tracks per-operation performance
type OperationMetrics struct {
	OperationType  string        `json:"operation_type"`
	TotalRequests  int64         `json:"total_requests"`
	TotalErrors    int64         `json:"total_errors"`
	AverageLatency time.Duration `json:"average_latency"`
	MinLatency     time.Duration `json:"min_latency"`
	MaxLatency     time.Duration `json:"max_latency"`
	TotalLatency   time.Duration `json:"total_latency"`
	LastRequest    time.Time     `json:"last_request"`
}

// MemoryStats tracks system memory usage
type MemoryStats struct {
	HeapAlloc    uint64        `json:"heap_alloc"`
	HeapSys      uint64        `json:"heap_sys"`
	HeapIdle     uint64        `json:"heap_idle"`
	HeapInuse    uint64        `json:"heap_inuse"`
	StackSys     uint64        `json:"stack_sys"`
	GCCycles     uint32        `json:"gc_cycles"`
	LastGC       time.Time     `json:"last_gc"`
	GCPauseTotal time.Duration `json:"gc_pause_total"`
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(config *types.PerformanceConfig, logger *logger.Logger) *PerformanceMonitor {
	return &PerformanceMonitor{
		logger:           logger,
		config:           config,
		startTime:        time.Now(),
		sessionMetrics:   make(map[string]*SessionMetrics),
		operationMetrics: make(map[string]*OperationMetrics),
		memoryStats:      &MemoryStats{},
		enabled:          config.EnableMetrics,
	}
}

// StartRequest begins tracking a request
func (pm *PerformanceMonitor) StartRequest(ctx context.Context, sessionID, operation string) context.Context {
	if !pm.enabled {
		return ctx
	}

	startTime := time.Now()

	// Create request context with timing info
	reqCtx := context.WithValue(ctx, "start_time", startTime)
	reqCtx = context.WithValue(reqCtx, "session_id", sessionID)
	reqCtx = context.WithValue(reqCtx, "operation", operation)

	atomic.AddInt64(&pm.requestCount, 1)

	pm.logger.Debug("Request started", map[string]interface{}{
		"session_id": sessionID,
		"operation":  operation,
		"start_time": startTime,
	})

	return reqCtx
}

// EndRequest finishes tracking a request
func (pm *PerformanceMonitor) EndRequest(ctx context.Context, err error) {
	if !pm.enabled {
		return
	}

	startTime, ok := ctx.Value("start_time").(time.Time)
	if !ok {
		return
	}

	sessionID, _ := ctx.Value("session_id").(string)
	operation, _ := ctx.Value("operation").(string)

	latency := time.Since(startTime)
	latencyMicros := latency.Microseconds()

	atomic.AddInt64(&pm.totalLatency, latencyMicros)

	if err != nil {
		atomic.AddInt64(&pm.errorCount, 1)
	}

	// Update session metrics
	if sessionID != "" {
		pm.updateSessionMetrics(sessionID, latency)
	}

	// Update operation metrics
	if operation != "" {
		pm.updateOperationMetrics(operation, latency, err != nil)
	}

	pm.logger.Debug("Request completed", map[string]interface{}{
		"session_id": sessionID,
		"operation":  operation,
		"latency_ms": latency.Milliseconds(),
		"error":      err != nil,
	})
}

// updateSessionMetrics updates metrics for a specific session
func (pm *PerformanceMonitor) updateSessionMetrics(sessionID string, latency time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	metrics, exists := pm.sessionMetrics[sessionID]
	if !exists {
		metrics = &SessionMetrics{
			SessionID:    sessionID,
			LastActivity: time.Now(),
		}
		pm.sessionMetrics[sessionID] = metrics
	}

	metrics.RequestCount++
	metrics.LastActivity = time.Now()

	// Calculate running average
	totalLatency := time.Duration(int64(metrics.AverageLatency) * (metrics.RequestCount - 1))
	metrics.AverageLatency = (totalLatency + latency) / time.Duration(metrics.RequestCount)
}

// updateOperationMetrics updates metrics for a specific operation
func (pm *PerformanceMonitor) updateOperationMetrics(operation string, latency time.Duration, isError bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	metrics, exists := pm.operationMetrics[operation]
	if !exists {
		metrics = &OperationMetrics{
			OperationType: operation,
			MinLatency:    latency,
			MaxLatency:    latency,
		}
		pm.operationMetrics[operation] = metrics
	}

	metrics.TotalRequests++
	metrics.LastRequest = time.Now()
	metrics.TotalLatency += latency

	if isError {
		metrics.TotalErrors++
	}

	if latency < metrics.MinLatency {
		metrics.MinLatency = latency
	}
	if latency > metrics.MaxLatency {
		metrics.MaxLatency = latency
	}

	metrics.AverageLatency = metrics.TotalLatency / time.Duration(metrics.TotalRequests)
}

// RecordEmbeddingTime records time spent on embedding operations
func (pm *PerformanceMonitor) RecordEmbeddingTime(sessionID string, duration time.Duration) {
	if !pm.enabled {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if metrics, exists := pm.sessionMetrics[sessionID]; exists {
		metrics.EmbeddingTime += duration
	}
}

// RecordStorageTime records time spent on storage operations
func (pm *PerformanceMonitor) RecordStorageTime(sessionID string, duration time.Duration) {
	if !pm.enabled {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if metrics, exists := pm.sessionMetrics[sessionID]; exists {
		metrics.StorageTime += duration
	}
}

// UpdateMemoryUsage updates memory usage for a session
func (pm *PerformanceMonitor) UpdateMemoryUsage(sessionID string, memoryBytes int64, chunkCount int64) {
	if !pm.enabled {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if metrics, exists := pm.sessionMetrics[sessionID]; exists {
		metrics.MemoryUsage = memoryBytes
		metrics.ChunkCount = chunkCount
	}
}

// GetSystemMetrics returns overall system performance metrics
func (pm *PerformanceMonitor) GetSystemMetrics() map[string]interface{} {
	if !pm.enabled {
		return map[string]interface{}{"enabled": false}
	}

	uptime := time.Since(pm.startTime)
	requestCount := atomic.LoadInt64(&pm.requestCount)
	errorCount := atomic.LoadInt64(&pm.errorCount)
	totalLatency := atomic.LoadInt64(&pm.totalLatency)

	var averageLatency time.Duration
	if requestCount > 0 {
		averageLatency = time.Duration(totalLatency/requestCount) * time.Microsecond
	}

	errorRate := float64(0)
	if requestCount > 0 {
		errorRate = float64(errorCount) / float64(requestCount) * 100
	}

	return map[string]interface{}{
		"enabled":             pm.enabled,
		"uptime_seconds":      uptime.Seconds(),
		"total_requests":      requestCount,
		"total_errors":        errorCount,
		"error_rate_percent":  errorRate,
		"average_latency_ms":  averageLatency.Milliseconds(),
		"requests_per_second": float64(requestCount) / uptime.Seconds(),
		"active_sessions":     len(pm.sessionMetrics),
	}
}

// GetSessionMetrics returns performance metrics for a specific session
func (pm *PerformanceMonitor) GetSessionMetrics(sessionID string) *SessionMetrics {
	if !pm.enabled {
		return nil
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics, exists := pm.sessionMetrics[sessionID]
	if !exists {
		return nil
	}

	// Return a copy to avoid data races
	return &SessionMetrics{
		SessionID:      metrics.SessionID,
		RequestCount:   metrics.RequestCount,
		AverageLatency: metrics.AverageLatency,
		LastActivity:   metrics.LastActivity,
		MemoryUsage:    metrics.MemoryUsage,
		ChunkCount:     metrics.ChunkCount,
		EmbeddingTime:  metrics.EmbeddingTime,
		StorageTime:    metrics.StorageTime,
	}
}

// GetOperationMetrics returns performance metrics for operations
func (pm *PerformanceMonitor) GetOperationMetrics() map[string]*OperationMetrics {
	if !pm.enabled {
		return nil
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*OperationMetrics)
	for op, metrics := range pm.operationMetrics {
		result[op] = &OperationMetrics{
			OperationType:  metrics.OperationType,
			TotalRequests:  metrics.TotalRequests,
			TotalErrors:    metrics.TotalErrors,
			AverageLatency: metrics.AverageLatency,
			MinLatency:     metrics.MinLatency,
			MaxLatency:     metrics.MaxLatency,
			TotalLatency:   metrics.TotalLatency,
			LastRequest:    metrics.LastRequest,
		}
	}

	return result
}

// LogPerformanceSummary logs a comprehensive performance summary
func (pm *PerformanceMonitor) LogPerformanceSummary() {
	if !pm.enabled {
		return
	}

	systemMetrics := pm.GetSystemMetrics()
	operationMetrics := pm.GetOperationMetrics()

	pm.logger.Info("Performance Summary", map[string]interface{}{
		"system":     systemMetrics,
		"operations": operationMetrics,
	})

	// Log top 5 slowest operations
	pm.logSlowestOperations(5)
}

// logSlowestOperations logs the slowest operations
func (pm *PerformanceMonitor) logSlowestOperations(limit int) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	type OpLatency struct {
		Operation string
		Latency   time.Duration
	}

	var operations []OpLatency
	for _, metrics := range pm.operationMetrics {
		operations = append(operations, OpLatency{
			Operation: metrics.OperationType,
			Latency:   metrics.AverageLatency,
		})
	}

	// Simple selection sort for top N (good enough for small lists)
	for i := 0; i < len(operations) && i < limit; i++ {
		maxIdx := i
		for j := i + 1; j < len(operations); j++ {
			if operations[j].Latency > operations[maxIdx].Latency {
				maxIdx = j
			}
		}
		if maxIdx != i {
			operations[i], operations[maxIdx] = operations[maxIdx], operations[i]
		}
	}

	if len(operations) > limit {
		operations = operations[:limit]
	}

	pm.logger.Info("Slowest Operations", map[string]interface{}{
		"operations": operations,
	})
}

// Cleanup removes old metrics to prevent memory leaks
func (pm *PerformanceMonitor) Cleanup(maxAge time.Duration) {
	if !pm.enabled {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	// Clean old session metrics
	for sessionID, metrics := range pm.sessionMetrics {
		if metrics.LastActivity.Before(cutoff) {
			delete(pm.sessionMetrics, sessionID)
		}
	}

	pm.logger.Debug("Performance metrics cleanup completed", map[string]interface{}{
		"active_sessions": len(pm.sessionMetrics),
		"cutoff_time":     cutoff,
	})
}

// Enable enables performance monitoring
func (pm *PerformanceMonitor) Enable() {
	pm.enabled = true
	pm.logger.Info("Performance monitoring enabled")
}

// Disable disables performance monitoring
func (pm *PerformanceMonitor) Disable() {
	pm.enabled = false
	pm.logger.Info("Performance monitoring disabled")
}

// IsEnabled returns whether performance monitoring is enabled
func (pm *PerformanceMonitor) IsEnabled() bool {
	return pm.enabled
}
