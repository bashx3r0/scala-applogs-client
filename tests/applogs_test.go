package applogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bashx3r0/scala-applogs-client/internal/logger"
	"github.com/bashx3r0/scala-applogs-client/pkg/applogs"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// Setup mock Redis using miniredis
func setupMockRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

// Helper function to create a mock fallback file path
func createMockFallbackFile() string {
	fallbackPath := "./logs/test_fallback.log"
	_ = os.Remove(fallbackPath) // Ensure a clean slate for each test
	return fallbackPath
}

// Helper function to read fallback logs
func readFallbackLogs(filepath string) []string {
	data, _ := ioutil.ReadFile(filepath)
	lines := bytes.Split(data, []byte("\n"))
	var logs []string
	for _, line := range lines {
		if len(line) > 0 {
			logs = append(logs, string(line))
		}
	}
	return logs
}

// Utility function to list all keys and values in miniredis
func listKeysAndValues(mr *miniredis.Miniredis) {
	fmt.Println("Listing all keys and values in miniredis:")
	keys := mr.Keys()
	for _, key := range keys {
		values, err := mr.List(key)
		if err != nil {
			fmt.Printf("Key: %s, Error: %v\n", key, err)
			continue
		}
		fmt.Printf("Key: %s, Values: %v\n", key, values)
	}
}

// Test Cases

func TestLogQueueProcessingWithMiniredis(t *testing.T) {
	mr, client := setupMockRedis(t)
	defer mr.Close()

	logger.SetRedisClient(client)

	applogs := applogs.NewLogger(10) // Initialize Applogs with queue size 10

	// Log entries
	applogs.Info("Test info log", map[string]interface{}{"key": "value1"})
	applogs.Warn("Test warn log", map[string]interface{}{"key": "value2"})
	applogs.Error("Test error log", map[string]interface{}{"key": "value3"})

	// Wait for logs to be processed asynchronously
	time.Sleep(500 * time.Millisecond)

	// List all keys and values in Redis
	listKeysAndValues(mr)

	// Validate Redis logs
	key := "key:value1"
	if !mr.Exists(key) {
		t.Fatalf("Key %s does not exist in Redis", key)
	}

	logs, err := mr.List(key)
	if err != nil {
		t.Fatalf("Failed to fetch logs from miniredis: %v", err)
	}

	assert.Equal(t, 3, len(logs), "Redis should have received 3 logs")

	// Validate content of the first log
	var logData map[string]interface{}
	json.Unmarshal([]byte(logs[0]), &logData)
	assert.Equal(t, "info", logData["level"])
	assert.Equal(t, "Test info log", logData["message"])
}

func TestFallbackMechanismWithMiniredis(t *testing.T) {
	mr, _ := setupMockRedis(t)
	defer mr.Close()

	// Simulate Redis failure by shutting down miniredis
	mr.Close()

	fallbackPath := createMockFallbackFile()
	logger.SetFallbackPath(fallbackPath)

	applogs := applogs.NewLogger(10)

	// Log entry
	applogs.Info("Fallback log test", map[string]interface{}{"key": "fallback1"})

	// Wait for fallback to occur
	time.Sleep(500 * time.Millisecond)

	// Validate fallback logs
	fallbackLogs := readFallbackLogs(fallbackPath)
	assert.Equal(t, 1, len(fallbackLogs), "Fallback should contain 1 log")

	// Validate content of the fallback log
	var logData map[string]interface{}
	json.Unmarshal([]byte(fallbackLogs[0]), &logData)
	assert.Equal(t, "info", logData["level"])
	assert.Equal(t, "Fallback log test", logData["message"])
}
