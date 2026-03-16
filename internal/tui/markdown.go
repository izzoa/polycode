package tui

import (
	"github.com/charmbracelet/glamour"
)

var mdRenderer *glamour.TermRenderer

func init() {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0), // we handle wrapping via viewport width
	)
	if err != nil {
		// Fall back to no rendering if glamour init fails.
		return
	}
	mdRenderer = r
}

// renderMarkdown renders a markdown string for terminal display.
// Falls back to raw text if the renderer is unavailable.
func renderMarkdown(content string) string {
	if mdRenderer == nil || content == "" {
		return content
	}
	rendered, err := mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return rendered
}
