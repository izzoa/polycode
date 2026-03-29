package tui

import (
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

var mdRenderer *glamour.TermRenderer

func init() {
	mdRenderer = buildMarkdownRenderer(PolycodeDefault)
}

// buildMarkdownRenderer creates a glamour renderer with theme-derived colors.
func buildMarkdownRenderer(t Theme) *glamour.TermRenderer {
	// Build a style config that uses theme colors for key markdown elements.
	style := ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.Text)),
			},
			Margin: uintPtr(0),
		},
		Text: ansi.StylePrimitive{
			Color: stringPtr(string(t.Text)),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.Text)),
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:       stringPtr(string(t.Primary)),
				Bold:        boolPtr(true),
				BlockPrefix: "\n",
				BlockSuffix: "\n",
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(string(t.Primary)),
				Bold:   boolPtr(true),
				Prefix: "# ",
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(string(t.Secondary)),
				Bold:   boolPtr(true),
				Prefix: "## ",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(string(t.Tertiary)),
				Bold:   boolPtr(true),
				Prefix: "### ",
			},
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr(string(t.Secondary)),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(string(t.Tertiary)),
			Bold:  boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(string(t.TextBright)),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(1),
			},
			Chroma: &ansi.Chroma{},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(string(t.TextHint)),
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			StyleBlock: ansi.StyleBlock{},
			LevelIndent: 2,
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		// Fall back to auto style if custom fails
		r, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(0),
		)
	}
	return r
}

// rebuildMarkdownRenderer rebuilds the global renderer with the given theme.
func rebuildMarkdownRenderer(t Theme) {
	mdRenderer = buildMarkdownRenderer(t)
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

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }
