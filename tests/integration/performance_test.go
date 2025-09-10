package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarkank/aimem/internal/logger"
	"github.com/tarkank/aimem/internal/performance"
	"github.com/tarkank/aimem/internal/types"
)

// TestPerformanceMonitoringIntegration tests performance monitoring functionality
func TestPerformanceMonitoringIntegration(t *testing.T) {
	config := &types.PerformanceConfig{
		CompressionEnabled: true,
		AsyncProcessing:    true,
		CacheEmbeddings:    true,
		EnableMetrics:      true,
		MetricsInterval:    100 * time.Millisecond,
	}

	loggerConfig := &logger.Config{
		Level:        "info",
		Format:       "json",
		Output:       "stdout",
		EnableCaller: false,
	}
	testLogger, err := logger.NewLogger(loggerConfig, "test")
	require.NoError(t, err)

	monitor := performance.NewPerformanceMonitor(config, testLogger)

	t.Run("BasicMetricsTracking", func(t *testing.T) {
		testBasicMetricsTracking(t, monitor)
	})

	t.Run("SessionMetricsTracking", func(t *testing.T) {
		testSessionMetricsTracking(t, monitor)
	})

	t.Run("OperationMetricsTracking", func(t *testing.T) {
		testOperationMetricsTracking(t, monitor)
	})

	t.Run("PerformanceUnderLoad", func(t *testing.T) {
		testPerformanceUnderLoad(t, monitor)
	})

	t.Run("MetricsCleanup", func(t *testing.T) {
		testMetricsCleanup(t, monitor)
	})
}

func testBasicMetricsTracking(t *testing.T, monitor *performance.PerformanceMonitor) {
	ctx := context.Background()

	// Simulate some requests
	for i := 0; i < 10; i++ {
		reqCtx := monitor.StartRequest(ctx, "test-session", "store_context")
		time.Sleep(5 * time.Millisecond) // Simulate work
		monitor.EndRequest(reqCtx, nil)
	}

	// Add one error
	reqCtx := monitor.StartRequest(ctx, "test-session", "store_context")
	time.Sleep(5 * time.Millisecond)
	monitor.EndRequest(reqCtx, assert.AnError)

	// Get system metrics
	metrics := monitor.GetSystemMetrics()
	require.NotNil(t, metrics)

	assert.True(t, metrics["enabled"].(bool))
	assert.Equal(t, int64(11), metrics["total_requests"])
	assert.Equal(t, int64(1), metrics["total_errors"])
	assert.InDelta(t, 9.09, metrics["error_rate_percent"], 0.1) // 1/11 * 100
	assert.Greater(t, metrics["average_latency_ms"], int64(0))
	assert.Greater(t, metrics["requests_per_second"], 0.0)
}

func testSessionMetricsTracking(t *testing.T, monitor *performance.PerformanceMonitor) {
	ctx := context.Background()
	sessionID := "test-session-123"

	// Simulate requests for specific session
	for i := 0; i < 5; i++ {
		reqCtx := monitor.StartRequest(ctx, sessionID, "retrieve_context")
		time.Sleep(10 * time.Millisecond)
		monitor.EndRequest(reqCtx, nil)
	}

	// Record some timing metrics
	monitor.RecordEmbeddingTime(sessionID, 50*time.Millisecond)
	monitor.RecordStorageTime(sessionID, 20*time.Millisecond)
	monitor.UpdateMemoryUsage(sessionID, 1024*1024, 42) // 1MB, 42 chunks

	// Get session metrics
	sessionMetrics := monitor.GetSessionMetrics(sessionID)
	require.NotNil(t, sessionMetrics)

	assert.Equal(t, sessionID, sessionMetrics.SessionID)
	assert.Equal(t, int64(5), sessionMetrics.RequestCount)
	assert.Greater(t, sessionMetrics.AverageLatency, time.Duration(0))
	assert.Equal(t, int64(1024*1024), sessionMetrics.MemoryUsage)
	assert.Equal(t, int64(42), sessionMetrics.ChunkCount)
	assert.Equal(t, 50*time.Millisecond, sessionMetrics.EmbeddingTime)
	assert.Equal(t, 20*time.Millisecond, sessionMetrics.StorageTime)
}

func testOperationMetricsTracking(t *testing.T, monitor *performance.PerformanceMonitor) {
	ctx := context.Background()

	operations := []string{"store_context", "retrieve_context", "summarize_session"}

	// Simulate different operations with varying latencies
	for _, op := range operations {
		for i := 0; i < 3; i++ {
			reqCtx := monitor.StartRequest(ctx, "session", op)
			// Different latencies for different operations
			switch op {
			case "store_context":
				time.Sleep(20 * time.Millisecond)
			case "retrieve_context":
				time.Sleep(15 * time.Millisecond)
			case "summarize_session":
				time.Sleep(5 * time.Millisecond)
			}

			var err error
			if i == 2 && op == "retrieve_context" {
				err = assert.AnError // Simulate one error
			}
			monitor.EndRequest(reqCtx, err)
		}
	}

	// Get operation metrics
	opMetrics := monitor.GetOperationMetrics()
	require.NotNil(t, opMetrics)
	assert.Len(t, opMetrics, 3)

	// Check store_context metrics
	storeMetrics, exists := opMetrics["store_context"]
	assert.True(t, exists)
	assert.Equal(t, int64(3), storeMetrics.TotalRequests)
	assert.Equal(t, int64(0), storeMetrics.TotalErrors)
	assert.Greater(t, storeMetrics.AverageLatency, 15*time.Millisecond)

	// Check retrieve_context metrics (has one error)
	retrieveMetrics, exists := opMetrics["retrieve_context"]
	assert.True(t, exists)
	assert.Equal(t, int64(3), retrieveMetrics.TotalRequests)
	assert.Equal(t, int64(1), retrieveMetrics.TotalErrors)

	// Check summarize_session metrics (should be fastest)
	summaryMetrics, exists := opMetrics["summarize_session"]
	assert.True(t, exists)
	assert.Equal(t, int64(3), summaryMetrics.TotalRequests)
	assert.Equal(t, int64(0), summaryMetrics.TotalErrors)
	assert.Less(t, summaryMetrics.AverageLatency, storeMetrics.AverageLatency)
}

func testPerformanceUnderLoad(t *testing.T, monitor *performance.PerformanceMonitor) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	ctx := context.Background()
	concurrency := 20
	requestsPerWorker := 100

	start := time.Now()

	// Channel to synchronize goroutines
	done := make(chan bool, concurrency)

	// Launch concurrent workers
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer func() { done <- true }()

			for j := 0; j < requestsPerWorker; j++ {
				sessionID := fmt.Sprintf("load-session-%d", workerID)
				reqCtx := monitor.StartRequest(ctx, sessionID, "load_test")

				// Simulate variable work
				workTime := time.Duration(1+j%10) * time.Millisecond
				time.Sleep(workTime)

				monitor.EndRequest(reqCtx, nil)
			}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}

	duration := time.Since(start)
	totalRequests := concurrency * requestsPerWorker

	// Get final metrics
	systemMetrics := monitor.GetSystemMetrics()
	require.NotNil(t, systemMetrics)

	t.Logf("Load test results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Throughput: %.2f req/sec", float64(totalRequests)/duration.Seconds())
	t.Logf("  Average latency: %v ms", systemMetrics["average_latency_ms"])

	// Performance assertions
	assert.GreaterOrEqual(t, systemMetrics["total_requests"], int64(totalRequests))
	assert.Equal(t, int64(0), systemMetrics["total_errors"])       // Should be no errors
	assert.Greater(t, systemMetrics["requests_per_second"], 100.0) // Should handle > 100 req/sec
}

func testMetricsCleanup(t *testing.T, monitor *performance.PerformanceMonitor) {
	ctx := context.Background()

	// Create some sessions with different activity times
	sessions := []string{"old-session", "recent-session"}

	for _, sessionID := range sessions {
		reqCtx := monitor.StartRequest(ctx, sessionID, "test_cleanup")
		time.Sleep(1 * time.Millisecond)
		monitor.EndRequest(reqCtx, nil)
	}

	// Verify sessions exist
	oldMetrics := monitor.GetSessionMetrics("old-session")
	recentMetrics := monitor.GetSessionMetrics("recent-session")
	assert.NotNil(t, oldMetrics)
	assert.NotNil(t, recentMetrics)

	// Wait a bit and then cleanup with very short max age
	time.Sleep(10 * time.Millisecond)
	monitor.Cleanup(5 * time.Millisecond)

	// Both sessions should be cleaned up due to short max age
	systemMetrics := monitor.GetSystemMetrics()
	activeSessionCount := systemMetrics["active_sessions"].(int)

	// Should have fewer active sessions after cleanup
	assert.LessOrEqual(t, activeSessionCount, 2)
}

// BenchmarkPerformanceMonitor benchmarks the performance monitoring overhead
func BenchmarkPerformanceMonitor(b *testing.B) {
	config := &types.PerformanceConfig{
		EnableMetrics:   true,
		MetricsInterval: 1 * time.Second,
	}

	loggerConfig := &logger.Config{
		Level:  "error", // Minimize logging overhead
		Format: "json",
		Output: "stdout",
	}
	testLogger, err := logger.NewLogger(loggerConfig, "benchmark")
	if err != nil {
		b.Fatal(err)
	}

	monitor := performance.NewPerformanceMonitor(config, testLogger)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sessionID := fmt.Sprintf("bench-session-%d", i%10)
			reqCtx := monitor.StartRequest(ctx, sessionID, "benchmark_op")
			monitor.EndRequest(reqCtx, nil)
			i++
		}
	})
}

// BenchmarkPerformanceMonitorDisabled benchmarks overhead when monitoring is disabled
func BenchmarkPerformanceMonitorDisabled(b *testing.B) {
	config := &types.PerformanceConfig{
		EnableMetrics: false, // Disabled
	}

	loggerConfig := &logger.Config{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}
	testLogger, err := logger.NewLogger(loggerConfig, "benchmark")
	if err != nil {
		b.Fatal(err)
	}

	monitor := performance.NewPerformanceMonitor(config, testLogger)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sessionID := fmt.Sprintf("bench-session-%d", i%10)
			reqCtx := monitor.StartRequest(ctx, sessionID, "benchmark_op")
			monitor.EndRequest(reqCtx, nil)
			i++
		}
	})
}

// TestPerformanceMonitoringToggle tests enabling/disabling monitoring
func TestPerformanceMonitoringToggle(t *testing.T) {
	config := &types.PerformanceConfig{
		EnableMetrics: false, // Start disabled
	}

	loggerConfig := &logger.Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	testLogger, err := logger.NewLogger(loggerConfig, "test")
	require.NoError(t, err)

	monitor := performance.NewPerformanceMonitor(config, testLogger)
	ctx := context.Background()

	// Should be disabled initially
	assert.False(t, monitor.IsEnabled())

	// Metrics should show disabled
	metrics := monitor.GetSystemMetrics()
	assert.False(t, metrics["enabled"].(bool))

	// Enable monitoring
	monitor.Enable()
	assert.True(t, monitor.IsEnabled())

	// Now metrics should work
	reqCtx := monitor.StartRequest(ctx, "test-session", "test_op")
	time.Sleep(1 * time.Millisecond)
	monitor.EndRequest(reqCtx, nil)

	metrics = monitor.GetSystemMetrics()
	assert.True(t, metrics["enabled"].(bool))
	assert.Equal(t, int64(1), metrics["total_requests"])

	// Disable again
	monitor.Disable()
	assert.False(t, monitor.IsEnabled())

	metrics = monitor.GetSystemMetrics()
	assert.False(t, metrics["enabled"].(bool))
}
