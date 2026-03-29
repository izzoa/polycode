package tui

import "github.com/atotto/clipboard"

// copyToClipboard copies text to the system clipboard and returns an error
// if the clipboard is unavailable.
func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}
