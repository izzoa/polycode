package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

// debugLogger writes MCP JSON-RPC traffic to a log file for debugging.
type debugLogger struct {
	mu      sync.Mutex
	file    *os.File
	enabled bool
}

// newDebugLogger creates a debug logger. If enabled is false, all operations
// are no-ops. The log file is created at ~/.config/polycode/mcp-debug.log.
func newDebugLogger(enabled bool) *debugLogger {
	dl := &debugLogger{enabled: enabled}
	if !enabled {
		return dl
	}

	logPath := filepath.Join(config.ConfigDir(), "mcp-debug.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		// Silently disable if we can't open the log file.
		dl.enabled = false
		return dl
	}
	dl.file = f

	// Write session header.
	dl.writeRaw("=== MCP debug session started at %s ===\n", time.Now().Format(time.RFC3339))
	return dl
}

// LogRequest logs an outgoing JSON-RPC request.
func (dl *debugLogger) LogRequest(serverName, method string, params string) {
	if dl == nil || !dl.enabled {
		return
	}
	dl.writeRaw("[%s] → %s %s  %s\n",
		time.Now().Format("15:04:05.000"),
		serverName,
		method,
		truncateLog(params, 500),
	)
}

// LogResponse logs an incoming JSON-RPC response.
func (dl *debugLogger) LogResponse(serverName, method string, result string, errMsg string) {
	if dl == nil || !dl.enabled {
		return
	}
	if errMsg != "" {
		dl.writeRaw("[%s] ← %s %s  ERROR: %s\n",
			time.Now().Format("15:04:05.000"),
			serverName,
			method,
			truncateLog(errMsg, 500),
		)
	} else {
		dl.writeRaw("[%s] ← %s %s  %s\n",
			time.Now().Format("15:04:05.000"),
			serverName,
			method,
			truncateLog(result, 500),
		)
	}
}

// LogNotification logs a received notification.
func (dl *debugLogger) LogNotification(serverName, method string) {
	if dl == nil || !dl.enabled {
		return
	}
	dl.writeRaw("[%s] ← %s NOTIFICATION %s\n",
		time.Now().Format("15:04:05.000"),
		serverName,
		method,
	)
}

// Close closes the log file.
func (dl *debugLogger) Close() {
	if dl.file != nil {
		dl.file.Close()
	}
}

func (dl *debugLogger) writeRaw(format string, args ...any) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.file != nil {
		fmt.Fprintf(dl.file, format, args...)
	}
}

// truncateLog shortens a string for logging.
func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
