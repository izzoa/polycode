// Package notify provides desktop notifications for polycode.
// Uses osascript on macOS and notify-send on Linux.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends a desktop notification with the given title and body.
// Returns nil if notifications are not available on the platform.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Run()
	case "linux":
		return exec.Command("notify-send", title, body).Run()
	default:
		return nil // silently skip on unsupported platforms
	}
}

// Available returns true if desktop notifications are supported on this platform.
func Available() bool {
	switch runtime.GOOS {
	case "darwin":
		return true
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil
	default:
		return false
	}
}
