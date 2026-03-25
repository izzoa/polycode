package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ASCIIArt is the polycode logo for use in splash screens and CLI output.
const ASCIIArt = `
 ██████╗  ██████╗ ██╗  ██╗   ██╗ ██████╗ ██████╗ ██████╗ ███████╗
 ██╔══██╗██╔═══██╗██║  ╚██╗ ██╔╝██╔════╝██╔═══██╗██╔══██╗██╔════╝
 ██████╔╝██║   ██║██║   ╚████╔╝ ██║     ██║   ██║██║  ██║█████╗
 ██╔═══╝ ██║   ██║██║    ╚██╔╝  ██║     ██║   ██║██║  ██║██╔══╝
 ██║     ╚██████╔╝███████╗██║   ╚██████╗╚██████╔╝██████╔╝███████╗
 ╚═╝      ╚═════╝ ╚══════╝╚═╝    ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝`

func (m Model) renderSplash() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Apply gradient coloring to the ASCII art
	gradientStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")). // cyan
		Bold(true)

	art := gradientStyle.Render(ASCIIArt)

	// Version line
	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	ver := m.version
	if ver != "" && ver[0] != 'v' {
		ver = "v" + ver
	}
	versionLine := versionStyle.Render(ver)

	// Tagline
	taglineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Italic(true)
	tagline := taglineStyle.Render("multi-model consensus coding assistant")

	// Hint
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	hint := hintStyle.Render("press any key to continue")

	// Compose the splash content
	content := strings.Join([]string{
		"",
		art,
		"",
		versionLine,
		tagline,
		"",
		hint,
	}, "\n")

	// Center horizontally and vertically
	contentLines := strings.Count(content, "\n") + 1
	topPad := (m.height - contentLines) / 2
	if topPad < 0 {
		topPad = 0
	}

	centered := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		PaddingTop(topPad).
		Render(content)

	return centered
}
