package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the semantic color palette for the TUI.
// All rendering code references these fields instead of hardcoded color numbers.
type Theme struct {
	Name string

	// Base palette
	Primary   lipgloss.Color // main accent (default: 214 orange)
	Secondary lipgloss.Color // interactive elements (default: 63 blue)
	Tertiary  lipgloss.Color // active/selected (default: 86 cyan)
	Success   lipgloss.Color // healthy/done (default: 42 green)
	Error     lipgloss.Color // failure/error (default: 196 red)
	Warning   lipgloss.Color // attention (default: 214 orange)
	Info      lipgloss.Color // informational (default: 82 bright green)

	// Text
	Text       lipgloss.Color // primary text (default: 252)
	TextMuted  lipgloss.Color // dimmed/secondary (default: 241)
	TextHint   lipgloss.Color // hints/descriptions (default: 243)
	TextSubtle lipgloss.Color // subtle/version text (default: 245)
	TextBright lipgloss.Color // highlights, selected items (default: 250)

	// Backgrounds
	BgBase     lipgloss.Color // app background
	BgPanel    lipgloss.Color // panel/card background (default: 235)
	BgSelected lipgloss.Color // selected row (default: 236)
	BgFocused  lipgloss.Color // focused element (default: 238)

	// Borders
	BorderNormal  lipgloss.Color // default borders (default: 240)
	BorderFocused lipgloss.Color // focused borders (default: 63)
	BorderAccent  lipgloss.Color // accent borders (default: 214)

	// Diff
	DiffAdded   lipgloss.Color // added lines (default: 42 green)
	DiffRemoved lipgloss.Color // removed lines (default: 196 red)
	DiffContext lipgloss.Color // context lines (default: 241)
	DiffHeader  lipgloss.Color // diff headers (default: 63)

	// Scrollbar
	ScrollTrack lipgloss.Color // scrollbar track (default: 237)
	ScrollThumb lipgloss.Color // scrollbar thumb (default: 243)

	// Shadow
	Shadow lipgloss.Color // drop shadow (default: 237)

	// Special
	Cyan    lipgloss.Color // splash art (default: 39)
	YellowW lipgloss.Color // yellow warning band (default: 226)
}

// ThemeByName returns a built-in theme by name, or the default if not found.
func ThemeByName(name string) Theme {
	switch name {
	case "catppuccin", "catppuccin-mocha":
		return CatppuccinMocha
	case "tokyo-night":
		return TokyoNight
	case "dracula":
		return Dracula
	case "gruvbox", "gruvbox-dark":
		return GruvboxDark
	case "nord":
		return Nord
	default:
		return PolycodeDefault
	}
}

// BuiltinThemeNames returns the names of all built-in themes in display order.
func BuiltinThemeNames() []string {
	return []string{
		"polycode",
		"catppuccin-mocha",
		"tokyo-night",
		"dracula",
		"gruvbox-dark",
		"nord",
	}
}
