package logger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

var recoveryRedisClient RedisClient // Abstracted Redis client for recovery

// SetRecoveryRedisClient allows setting the Redis client for recovery
func SetRecoveryRedisClient(client RedisClient) {
	recoveryRedisClient = client
}

// StartRecoveryProcess initiates periodic fallback recovery
func StartRecoveryProcess(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			recoverFallbackLogs()
		}
	}()
}

// recoverFallbackLogs scans fallback logs and resends them to Redis
func recoverFallbackLogs() {
	if rdb == nil {
		logger.Error("Redis client is not set. Skipping recovery.")
		return
	}

	files, err := os.ReadDir(fallbackPath)
	if err != nil {
		logger.Error("Failed to scan fallback directory", zap.Error(err))
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".log" && strings.HasPrefix(file.Name(), "fallback_") {
			filePath := filepath.Join(fallbackPath, file.Name())
			f, err := os.Open(filePath)
			if err != nil {
				logger.Error("Failed to read fallback log", zap.String("file", filePath), zap.Error(err))
				continue
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			var batchLogs []map[string]interface{}
			corrupt := false
			redisPushFailed := false

			for scanner.Scan() {
				line := scanner.Text()
				if len(line) == 0 {
					continue
				}

				var logData map[string]interface{}
				if err := json.Unmarshal([]byte(line), &logData); err != nil {
					logger.Error("Invalid JSON in fallback log line",
						zap.String("file", filePath),
						zap.String("line", line))
					corrupt = true
					continue
				}

				batchLogs = append(batchLogs, logData)
			}

			// Push batch logs to Redis
			if len(batchLogs) > 0 {
				if err := pushBatchToRedis(batchLogs); err != nil {
					redisPushFailed = true // Do not log here; it's already logged inside pushBatchToRedis
				} else {
					logger.Info("Batch log successfully sent to Redis",
						zap.String("file", filePath),
						zap.Int("count", len(batchLogs)))
				}
			}

			if err := scanner.Err(); err != nil {
				logger.Error("Error reading fallback log line by line", zap.Error(err))
			}

			// Handle log file removal or renaming
			if corrupt {
				os.Rename(filePath, filePath+".corrupt")
			} else if !redisPushFailed {
				os.Remove(filePath) // Remove after successful batch resend
			}
		}
	}
}

// pushBatchToRedis sends logs in a single batch operation
func pushBatchToRedis(logs []map[string]interface{}) error {
	pipe := rdb.Pipeline()
	var finalErr error // Track overall pipeline error

	for _, logData := range logs {
		key := "applogs:" + logData["facility_id"].(string) + ":" +
			logData["instance_type"].(string) + ":" +
			logData["service_name"].(string) + ":" +
			logData["instance_id"].(string)

		// Marshal logData to JSON
		data, err := json.Marshal(logData)
		if err != nil {
			logger.Error("Failed to marshal log data to JSON", zap.Error(err))
			continue
		}

		// Append new log to the list
		pipe.LPush(ctx, key, data)
	}

	// Execute the pipeline commands
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		logger.Warn("Pipeline execution failed", zap.Error(err))
		return err // Avoid redundant per-command errors if pipeline failed
	}

	// Only check individual command errors if the pipeline didn't fail
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			logger.Warn("Failed to push individual log to Redis",
				zap.String("cmd", cmd.String()),
				zap.Error(cmd.Err()))
			finalErr = cmd.Err()
		}
	}

	return finalErr
}
