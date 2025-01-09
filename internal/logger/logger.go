package logger

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"fmt"

	internalRedis "github.com/bashx3r0/scala-applogs-client/internal/redis"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type RedisClient interface {
	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Pipeline() redis.Pipeliner
}

var (
	logger              *zap.Logger
	rdb                 RedisClient
	ctx                 = context.Background()
	redisAddr           string
	serviceName         string
	instanceID          string
	facilityID          string
	instanceType        string
	fallbackPath        string
	syslogsPath         string
	fallbackResyncTime  int // Time (in seconds) to attempt fallback log resend
	syslogKeepTime      int // Time (in hours) to keep syslog records
	ErrRedisUnavailable = errors.New("redis is unavailable")
)

// Ensure logs directory exists
func ensureLogDirectory() {
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		_ = os.MkdirAll("logs", 0755)
	}

	// Ensure fallback directory exists
	if fallbackPath == "" {
		fallbackPath = filepath.Join("logs", "fallback")
	}
	if _, err := os.Stat(fallbackPath); os.IsNotExist(err) {
		_ = os.MkdirAll(fallbackPath, 0755)
	}

	// Ensure syslogs directory exists
	syslogsPath = filepath.Join("logs", "syslogs")
	if _, err := os.Stat(syslogsPath); os.IsNotExist(err) {
		_ = os.MkdirAll(syslogsPath, 0755)
	}
}

// Initialize logger and Redis client
func InitApplogs() {

	fmt.Println("Initializing applogs...")

	fmt.Println("Loading environment variables...")

	_ = godotenv.Load(".env")
	ensureLogDirectory()

	serviceName = os.Getenv("SERVICE_NAME")
	instanceID = os.Getenv("INSTANCE_ID")
	facilityID = os.Getenv("FACILITY_ID")
	instanceType = os.Getenv("INSTANCE_TYPE")
	redisAddr = os.Getenv("APPLG_CORE_REDIS")

	fmt.Println(serviceName, instanceID, facilityID, instanceType, redisAddr)

	// Load fallback resync time (default: 30 seconds)
	fallbackResyncTime = getEnvAsInt("FALLBACK_RESYNC_TIME", 30)

	// Load syslog keep time (default: 72 hours)
	syslogKeepTime = getEnvAsInt("SYSLOG_KEEP_TIME", 72)

	logFile := generateLogFilePath()
	writeSyncer := getLogWriter(logFile)

	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel),                // File logging
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel), // Console logging
	)

	log := zap.New(core, zap.AddCaller())
	logger = log
	zap.ReplaceGlobals(logger) // Replace global logger

	logger.Info("Logger initialized successfully",
		zap.Int("fallback_resync_time", fallbackResyncTime),
		zap.Int("syslog_keep_time", syslogKeepTime))

	rdb = internalRedis.NewRedisClient(redisAddr)

	if rdb != nil {
		logger.Info("Checking Redis connection")
		checkRedisConnection()
	} else {
		logger.Error("Failed to initialize Redis client. Redis client is nil.")
	}

	// Start fallback recovery with dynamic interval
	StartRecoveryProcess(time.Duration(fallbackResyncTime) * time.Second)

	// Start periodic log cleanup
	go func() {
		for {
			time.Sleep(24 * time.Hour) // Run once per day
			cleanupOldLogs()
		}
	}()
}

// Logger returns the logger instance
func Logger() *zap.Logger {
	if logger == nil {
		InitApplogs()
	}
	return logger
}

// Check Redis connection and log status
func checkRedisConnection() {
	if rdb == nil {
		logger.Error("Redis client is nil. Skipping Redis connection check.")
		return
	}

	_, err := rdb.(*redis.Client).Ping(ctx).Result()
	if err != nil {
		logger.Error("Failed to connect to Redis Database",
			zap.String("address", redisAddr),
			zap.Error(err))
	} else {
		logger.Info("Connected to Redis successfully",
			zap.String("address", redisAddr))
	}
}

// General function to handle logging with fallback
func LogToRedis(level, message string, fields map[string]interface{}) {
	logData := map[string]interface{}{
		"timestamp":     time.Now().UTC(),
		"level":         level,
		"message":       message,
		"metadata":      fields,
		"service_name":  serviceName,
		"instance_id":   instanceID,
		"facility_id":   facilityID,
		"instance_type": instanceType,
	}

	key := "applogs:" + facilityID + ":" + instanceType + ":" + serviceName + ":" + instanceID

	// Marshal single log entry
	data, err := json.Marshal(logData)
	if err != nil {
		logger.Error("Failed to marshal log data to JSON", zap.Error(err))
		return
	}

	// Use LPUSH to append single log entry without overwriting
	err = rdb.LPush(ctx, key, data).Err()
	if err != nil {
		if isRedisUnavailable(err) {
			logger.Warn("Redis unavailable, saving to fallback", zap.Error(err))
			logToFallback(logData)
		} else {
			logger.Error("Failed to push log to Redis", zap.Error(err))
		}
	}
}

// Check if Redis is unavailable
func isRedisUnavailable(err error) bool {
	return errors.Is(err, ErrRedisUnavailable) || strings.Contains(err.Error(), "connection refused")
}

// Fallback mechanism to store logs locally if Redis fails
func logToFallback(logData map[string]interface{}) {
	filename := filepath.Join(fallbackPath, "fallback_"+time.Now().Format("20060102150405")+".log")
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("Failed to open fallback log file", zap.Error(err))
		return
	}
	defer file.Close()

	data, _ := json.Marshal(logData)
	file.WriteString(string(data) + "\n")
}

// Generate log file path with datetime for system logs
func generateLogFilePath() string {
	currentTime := time.Now().Format("020120061504")
	return filepath.Join(syslogsPath, "syslogs_"+currentTime+".log")
}

// Get log writer to write to log file
func getLogWriter(logFile string) zapcore.WriteSyncer {
	file, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return zapcore.AddSync(file)
}

// Cleanup logs older than syslogKeepTime
func cleanupOldLogs() {
	logDirs := []string{"logs", fallbackPath, syslogsPath}
	expiration := time.Now().Add(-time.Duration(syslogKeepTime) * time.Hour)

	for _, logDir := range logDirs {
		files, err := os.ReadDir(logDir)
		if err != nil {
			logger.Warn("Failed to read log directory for cleanup", zap.String("directory", logDir), zap.Error(err))
			continue
		}

		for _, file := range files {
			filePath := filepath.Join(logDir, file.Name())

			info, err := os.Stat(filePath)
			if err != nil {
				logger.Warn("Failed to fetch log file info", zap.String("file", filePath), zap.Error(err))
				continue
			}

			// Delete if the log is older than syslogKeepTime
			if info.ModTime().Before(expiration) {
				err := os.Remove(filePath)
				if err != nil {
					logger.Error("Failed to delete old log file", zap.String("file", filePath), zap.Error(err))
				} else {
					logger.Info("Deleted old log file", zap.String("file", filePath))
				}
			}
		}
	}
}

// Utility function to get environment variable as integer
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		logger.Warn("Invalid integer value for environment variable. Using default value.",
			zap.String("key", key), zap.String("value", valueStr), zap.Error(err))
		return defaultValue
	}
	return value
}

// SetFallbackPath allows testing to override the fallback path
func SetFallbackPath(path string) {
	fallbackPath = path
}

// SetRedisClient allows testing to inject a mock Redis client
func SetRedisClient(client RedisClient) {
	rdb = client
}
