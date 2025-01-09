package main

import (
	"net/http"

	"github.com/bashx3r0/scala-applogs-client/v1"
)

func main() {
	// Initialize the logger
	applogs := applogs.NewLogger(10)

	// Log informational message

	// Log warning message
	applogs.Warn("Cubaan menggodam dikesan", map[string]interface{}{
		"memory": "90%",
	})



	http.ListenAndServe(":8080", nil)
}
