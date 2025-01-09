# SCALA Applogs Client Documentation

## Overview
The SCALA Applogs Client provides a centralized and asynchronous logging solution. It supports logging to Redis and fallback mechanisms for reliability. This library is designed for scalable and distributed applications, ensuring that logs are efficiently processed and stored.

## Features
- **Asynchronous Logging**: Log entries are queued and processed asynchronously to improve application performance.
- **Redis Integration**: Logs are stored in Redis with support for structured data.
- **Fallback Mechanism**: When Redis is unavailable, logs are stored locally and automatically recovered later.
- **Customizable Logging Levels**: Supports multiple logging levels (`info`, `debug`, `warn`, `error`, `fatal`).
- **Request/Response Logging**: Easily log incoming HTTP requests and outgoing responses.
- **Panic Logging**: Automatically capture and log panic events for recovery.

---

## Installation
Add the SCALA Applogs Client to your project:
```bash
$ go get github.com/bashx3r0/scala-applogs-client
```

---

## Usage

### Initialize Logger
```go
package main

import (
	"github.com/bashx3r0/scala-applogs-client/pkg/applogs"
)

func main() {
	// Initialize the logger with a queue size of 10
	logger := applogs.NewLogger(10)
	defer logger.StopLogger()

	logger.Info("Application started", nil)
}
```

### Logging Levels

#### Info
```go
logger.Info("User logged in", map[string]interface{}{"user_id": 123})
```

#### Debug
```go
logger.Debug("Debugging user session", map[string]interface{}{"session_id": "abc123"})
```

#### Warn
```go
logger.Warn("Potential issue detected", map[string]interface{}{"disk_space": "low"})
```

#### Error
```go
logger.Error("Error processing request", map[string]interface{}{"error": "timeout"})
```

#### Fatal
```go
logger.Fatal("Critical failure", map[string]interface{}{"service": "database"})
```

### Request and Response Logging
#### Log Incoming Requests
```go
logger.LogRequest("GET", "/api/users", "192.168.1.1", map[string][]string{"User-Agent": {"curl/7.68.0"}})
```

#### Log Outgoing Responses
```go
logger.LogResponse(200, 120*time.Millisecond)
```

### Panic Logging
Capture panic details and log them for debugging:
```go
defer func() {
	if r := recover(); r != nil {
		logger.LogPanic(r, "POST", "/api/submit", "192.168.1.1")
	}
}()
```

---

## Advanced Configuration

### Set Fallback Path
Customize the path for storing fallback logs:
```go
logger.SetFallbackPath("/custom/path/to/logs")
```

### Set Redis Client (For Testing)
Inject a custom Redis client for testing purposes:
```go
mockRedis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
logger.SetRedisClient(mockRedis)
```

---

## Internal Workflow
1. **Log Entry Queuing**: Logs are queued in a buffered channel to ensure asynchronous processing.
2. **Redis Logging**: Logs are pushed to Redis for centralized storage.
3. **Fallback Mechanism**: If Redis is unavailable, logs are written to a local fallback file.
4. **Recovery Process**: A background process periodically scans and re-sends fallback logs to Redis.

---

## Error Handling
### Redis Unavailability
Logs are automatically stored locally if Redis becomes unavailable. The recovery process ensures that logs are re-sent to Redis when the connection is restored.

### Overflow Handling
If the log queue is full, additional log entries are dropped to maintain system performance. A warning message is logged.

---

## Limitations
- **Queue Size**: Ensure the queue size is large enough to handle peak log traffic.
- **Recovery Delays**: Fallback log recovery is performed at intervals. Ensure the interval is configured appropriately for your use case.

---

## Contributions
Contributions are welcome! Please open an issue or submit a pull request to the [GitHub repository](https://github.com/bashx3r0/scala-applogs-client).

---

## License
This project is licensed under the MIT License. See the LICENSE file for details.
