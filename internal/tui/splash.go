package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
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

	frame := m.splashFrame
	artLines := strings.Split(strings.TrimPrefix(ASCIIArt, "\n"), "\n")

	// Calculate the maximum rune width of the logo
	maxCols := 0
	for _, line := range artLines {
		if r := len([]rune(line)); r > maxCols {
			maxCols = r
		}
	}

	// Animation phases (at 50ms/frame):
	//   0-16:  typewriter reveal (4 cols per frame)
	//   17-24: color wave sweep
	//   25+:   hold final state, fade in tagline
	revealCols := frame * 4 // how many columns are visible
	if revealCols > maxCols {
		revealCols = maxCols
	}

	// Wave position (0.0 = left edge, 1.0 = right edge)
	var wavePos float64
	if frame >= 17 && frame <= 24 {
		wavePos = float64(frame-17) / 7.0
	} else if frame > 24 {
		wavePos = 1.0
	}

	// Resolve theme colors for gradient
	fromHex := resolveToHex(m.theme.Cyan)
	toHex := resolveToHex(m.theme.Primary)
	c1, err1 := colorful.Hex(fromHex)
	c2, err2 := colorful.Hex(toHex)
	useGradient := err1 == nil && err2 == nil && trueColorSupported

	// Render art with animation
	var renderedLines []string
	for _, line := range artLines {
		runes := []rune(line)
		var b strings.Builder
		for i, r := range runes {
			// Typewriter: hide characters beyond reveal point
			if i >= revealCols {
				b.WriteRune(' ')
				continue
			}

			// Color: blend based on wave position
			var style lipgloss.Style
			if useGradient && frame >= 17 {
				// Wave: characters to the left of wavePos get the gradient color
				colPct := float64(i) / float64(maxCols)
				if colPct <= wavePos {
					t := colPct / max(wavePos, 0.01) // position within the wave
					blended := c1.BlendHcl(c2, t)
					style = lipgloss.NewStyle().Foreground(lipgloss.Color(blended.Hex())).Bold(true)
				} else {
					// Not yet reached by wave — use base cyan
					style = lipgloss.NewStyle().Foreground(m.theme.Cyan).Bold(true)
				}
			} else {
				// Pre-wave: solid cyan with cursor glow at reveal edge
				if i >= revealCols-3 && i < revealCols {
					// Cursor glow: bright white at the typing edge
					style = lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
				} else {
					style = lipgloss.NewStyle().Foreground(m.theme.Cyan).Bold(true)
				}
			}
			b.WriteString(style.Render(string(r)))
		}
		renderedLines = append(renderedLines, b.String())
	}
	art := strings.Join(renderedLines, "\n")

	// Version line (fades in at frame 25+)
	versionLine := ""
	if frame >= 25 {
		versionStyle := lipgloss.NewStyle().Foreground(m.theme.TextSubtle)
		ver := m.version
		if ver != "" && ver[0] != 'v' {
			ver = "v" + ver
		}
		versionLine = versionStyle.Render(ver)
	}

	// Tagline (fades in at frame 22+)
	tagline := ""
	if frame >= 22 {
		taglineStyle := lipgloss.NewStyle().
			Foreground(m.theme.Primary).
			Italic(true)
		tagline = taglineStyle.Render("multi-model consensus coding assistant")
	}

	// Session info (fades in at frame 26+)
	sessionInfo := ""
	if frame >= 26 && m.splashSessionMsg != "" {
		sessionStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
		sessionInfo = sessionStyle.Render(m.splashSessionMsg)
	}

	// Hint (fades in at frame 28+)
	hint := ""
	if frame >= 28 {
		hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextMuted)
		hint = hintStyle.Render("press any key to continue")
	}

	// Compose
	parts := []string{"", art, ""}
	if versionLine != "" {
		parts = append(parts, versionLine)
	}
	if tagline != "" {
		parts = append(parts, tagline)
	}
	if sessionInfo != "" {
		parts = append(parts, sessionInfo)
	}
	if hint != "" {
		parts = append(parts, "", hint)
	}
	content := strings.Join(parts, "\n")

	// Center
	contentLines := strings.Count(content, "\n") + 1
	topPad := (m.height - contentLines) / 2
	if topPad < 0 {
		topPad = 0
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		PaddingTop(topPad).
		Render(content)
}
