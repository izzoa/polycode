package main

import (
	"fmt"
	"os"
)

// runHeadless executes a single prompt in non-interactive mode.
// For now this is a stub that validates the approach — full headless mode
// requires refactoring the pipeline to be reusable outside the TUI.
func runHeadless(prompt string, _ bool) error {
	if prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	fmt.Fprintf(os.Stderr, "polycode: headless mode not yet fully implemented\n")
	fmt.Fprintf(os.Stderr, "polycode: use the interactive TUI for now\n")
	return fmt.Errorf("headless mode coming soon")
}
