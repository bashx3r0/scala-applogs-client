package applogs

import (
	"fmt"
	"time"

	"github.com/bashx3r0/scala-applogs-client/internal/logger"
	"github.com/go-redis/redis/v8" // Importing redis package
	"go.uber.org/zap"
)

// logEntry represents a single log entry for asynchronous processing
type logEntry struct {
	level   string
	message string
	fields  map[string]interface{}
}

// Applogs client structure
type Applogs struct {
	logQueue chan logEntry // Buffered channel for asynchronous logging
}

// NewLogger initializes the logger and sets up the log queue
func NewLogger(queueSize int) *Applogs {
	fmt.Println("Initializing applogs...")
	logger.InitApplogs()
	applogs := &Applogs{
		logQueue: make(chan logEntry, queueSize), // Buffered log queue
	}
	go applogs.processLogs() // Start log processing in a separate goroutine
	return applogs
}

// SetFallbackPath allows the fallback path to be set dynamically for testing
func (a *Applogs) SetFallbackPath(path string) {
	logger.SetFallbackPath(path)
}

// SetRedisClient allows a mock Redis client to be injected for testing
func (a *Applogs) SetRedisClient(mockClient *redis.Client) {
	logger.SetRedisClient(mockClient)
}

// logAsync queues a log entry for asynchronous processing
func (a *Applogs) logAsync(level, message string, fields map[string]interface{}) {
	entry := logEntry{level: level, message: message, fields: fields}
	select {
	case a.logQueue <- entry:
		// Log successfully added to the queue
	default:
		// Log queue is full; optionally drop the log or handle the overflow
		logger.Logger().Warn("Log queue is full, dropping log", zap.String("level", level), zap.String("message", message))
	}
}

// processLogs handles asynchronous processing of logs from the queue
func (a *Applogs) processLogs() {
	for entry := range a.logQueue {
		// Log to Redis and Uber Zap
		logger.LogToRedis(entry.level, entry.message, entry.fields)
		switch entry.level {
		case "info":
			logger.Logger().Info(entry.message, zap.Any("metadata", entry.fields))
		case "debug":
			logger.Logger().Debug(entry.message, zap.Any("metadata", entry.fields))
		case "warn":
			logger.Logger().Warn(entry.message, zap.Any("metadata", entry.fields))
		case "error":
			logger.Logger().Error(entry.message, zap.Any("metadata", entry.fields))
		case "fatal":
			logger.Logger().Fatal(entry.message, zap.Any("metadata", entry.fields))
		}
	}
}

// StopLogger gracefully shuts down the logger, ensuring all logs are processed
func (a *Applogs) StopLogger() {
	close(a.logQueue) // Close the log queue to stop processing
	logger.Logger().Info("Logger stopped gracefully")
}

// Info log
func (a *Applogs) Info(message string, fields map[string]interface{}) {
	a.logAsync("info", message, fields)
}

// Debug log
func (a *Applogs) Debug(message string, fields map[string]interface{}) {
	a.logAsync("debug", message, fields)
}

// Warn log
func (a *Applogs) Warn(message string, fields map[string]interface{}) {
	a.logAsync("warn", message, fields)
}

// Error log
func (a *Applogs) Error(message string, fields map[string]interface{}) {
	a.logAsync("error", message, fields)
}

// Fatal log
func (a *Applogs) Fatal(message string, fields map[string]interface{}) {
	a.logAsync("fatal", message, fields)
}

// LogRequest logs details about an incoming request
func (a *Applogs) LogRequest(method, url, clientIP string, headers map[string][]string) {
	fields := map[string]interface{}{
		"method":    method,
		"url":       url,
		"client_ip": clientIP,
		"headers":   headers,
		"timestamp": time.Now().UTC(),
	}
	a.logAsync("info", "Incoming request", fields)
}

// LogResponse logs details about an outgoing response
func (a *Applogs) LogResponse(statusCode int, duration time.Duration) {
	fields := map[string]interface{}{
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
		"timestamp":   time.Now().UTC(),
	}
	a.logAsync("info", "Outgoing response", fields)
}

// LogPanic logs panic details for recovery
func (a *Applogs) LogPanic(panicData interface{}, method, url, clientIP string) {
	fields := map[string]interface{}{
		"panic":     panicData,
		"method":    method,
		"url":       url,
		"client_ip": clientIP,
		"timestamp": time.Now().UTC(),
	}
	a.logAsync("error", "Recovered from panic", fields)
}
