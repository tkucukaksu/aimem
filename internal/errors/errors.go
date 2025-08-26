package errors

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// ErrorCode represents different types of errors
type ErrorCode string

const (
	// Infrastructure errors
	ErrCodeDatabase    ErrorCode = "DATABASE_ERROR"
	ErrCodeRedis       ErrorCode = "REDIS_ERROR"
	ErrCodeNetwork     ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeout     ErrorCode = "TIMEOUT_ERROR"
	
	// Application errors
	ErrCodeValidation  ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound    ErrorCode = "NOT_FOUND"
	ErrCodeConflict    ErrorCode = "CONFLICT_ERROR"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	
	// Service errors
	ErrCodeEmbedding   ErrorCode = "EMBEDDING_ERROR"
	ErrCodeChunking    ErrorCode = "CHUNKING_ERROR"
	ErrCodeSummarization ErrorCode = "SUMMARIZATION_ERROR"
	ErrCodeStorage     ErrorCode = "STORAGE_ERROR"
	
	// System errors
	ErrCodeInternal    ErrorCode = "INTERNAL_ERROR"
	ErrCodeRateLimit   ErrorCode = "RATE_LIMIT_ERROR"
	ErrCodeCapacity    ErrorCode = "CAPACITY_ERROR"
)

// AiMemError represents a structured error with context and metadata
type AiMemError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Cause       error                  `json:"-"`
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	Retryable   bool                   `json:"retryable"`
}

// Error implements the error interface
func (e *AiMemError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for error wrapping
func (e *AiMemError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for errors.Is
func (e *AiMemError) Is(target error) bool {
	if t, ok := target.(*AiMemError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext adds context information to the error
func (e *AiMemError) WithContext(ctx context.Context) *AiMemError {
	if ctx == nil {
		return e
	}
	
	// Create a copy to avoid modifying the original
	newErr := *e
	
	if requestID, ok := ctx.Value("request_id").(string); ok {
		newErr.RequestID = requestID
	}
	
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		newErr.SessionID = sessionID
	}
	
	return &newErr
}

// WithMetadata adds metadata to the error
func (e *AiMemError) WithMetadata(key string, value interface{}) *AiMemError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// WithStackTrace adds a stack trace to the error
func (e *AiMemError) WithStackTrace() *AiMemError {
	if e.StackTrace == "" {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		e.StackTrace = string(buf[:n])
	}
	return e
}

// New creates a new AiMemError
func New(code ErrorCode, message string) *AiMemError {
	return &AiMemError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// Newf creates a new AiMemError with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *AiMemError {
	return &AiMemError{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// Wrap wraps an existing error with AiMem error context
func Wrap(err error, code ErrorCode, message string) *AiMemError {
	if err == nil {
		return nil
	}
	
	return &AiMemError{
		Code:      code,
		Message:   message,
		Cause:     err,
		Details:   err.Error(),
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *AiMemError {
	if err == nil {
		return nil
	}
	
	return &AiMemError{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Cause:     err,
		Details:   err.Error(),
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// isRetryable determines if an error type is retryable
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeNetwork, ErrCodeRateLimit, ErrCodeCapacity:
		return true
	case ErrCodeDatabase, ErrCodeRedis:
		return true // Often transient
	case ErrCodeValidation, ErrCodeNotFound, ErrCodeUnauthorized:
		return false
	default:
		return false
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if aimemErr, ok := err.(*AiMemError); ok {
		return aimemErr.Retryable
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	if aimemErr, ok := err.(*AiMemError); ok {
		return aimemErr.Code
	}
	return ErrCodeInternal
}

// Common error constructors

// NewValidationError creates a validation error
func NewValidationError(message string, field string, value interface{}) *AiMemError {
	return New(ErrCodeValidation, message).WithMetadata("field", field).WithMetadata("value", value)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string, id string) *AiMemError {
	return Newf(ErrCodeNotFound, "%s not found", resource).WithMetadata("resource", resource).WithMetadata("id", id)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, timeout time.Duration) *AiMemError {
	return Newf(ErrCodeTimeout, "operation timed out: %s", operation).
		WithMetadata("operation", operation).
		WithMetadata("timeout_ms", timeout.Milliseconds())
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(resource string, limit int, window time.Duration) *AiMemError {
	return Newf(ErrCodeRateLimit, "rate limit exceeded for %s", resource).
		WithMetadata("resource", resource).
		WithMetadata("limit", limit).
		WithMetadata("window_ms", window.Milliseconds())
}

// NewCapacityError creates a capacity error
func NewCapacityError(resource string, current, max int) *AiMemError {
	return Newf(ErrCodeCapacity, "capacity exceeded for %s", resource).
		WithMetadata("resource", resource).
		WithMetadata("current", current).
		WithMetadata("max", max)
}

// Recovery functions

// Recover recovers from panics and converts them to errors
func Recover() error {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = fmt.Errorf("panic: %s", x)
		case error:
			err = x
		default:
			err = fmt.Errorf("panic: %v", x)
		}
		
		return Wrap(err, ErrCodeInternal, "panic recovered").WithStackTrace()
	}
	return nil
}

// SafeCall executes a function and recovers from panics
func SafeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var panicErr error
			switch x := r.(type) {
			case string:
				panicErr = fmt.Errorf("panic: %s", x)
			case error:
				panicErr = x
			default:
				panicErr = fmt.Errorf("panic: %v", x)
			}
			
			err = Wrap(panicErr, ErrCodeInternal, "panic in safe call").WithStackTrace()
		}
	}()
	
	return fn()
}

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error {
	var lastErr error
	
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}
		
		// Don't retry on last attempt
		if attempt == maxAttempts {
			break
		}
		
		// Calculate delay with exponential backoff
		delay := time.Duration(attempt) * baseDelay
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		
		select {
		case <-ctx.Done():
			return Wrap(ctx.Err(), ErrCodeTimeout, "retry cancelled by context")
		case <-time.After(delay):
			continue
		}
	}
	
	return Wrapf(lastErr, ErrCodeInternal, "retry failed after %d attempts", maxAttempts)
}

// Circuit breaker state
type CircuitState string

const (
	CircuitClosed    CircuitState = "closed"
	CircuitOpen      CircuitState = "open"
	CircuitHalfOpen  CircuitState = "half_open"
)

// CircuitBreaker provides circuit breaker functionality
type CircuitBreaker struct {
	maxFailures   int
	resetTimeout  time.Duration
	failures      int
	lastFailTime  time.Time
	state         CircuitState
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
	}
}

// Call executes a function through the circuit breaker
func (cb *CircuitBreaker) Call(fn func() error) error {
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
		} else {
			return New(ErrCodeCapacity, "circuit breaker is open")
		}
	}
	
	err := fn()
	
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		
		if cb.failures >= cb.maxFailures {
			cb.state = CircuitOpen
		}
		
		return err
	}
	
	// Success - reset circuit breaker
	cb.failures = 0
	cb.state = CircuitClosed
	
	return nil
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.state
}