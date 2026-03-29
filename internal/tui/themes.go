package tui

import "github.com/charmbracelet/lipgloss"

// PolycodeDefault preserves the exact current hardcoded color palette.
var PolycodeDefault = Theme{
	Name:          "polycode",
	Primary:       lipgloss.Color("214"),
	Secondary:     lipgloss.Color("63"),
	Tertiary:      lipgloss.Color("86"),
	Success:       lipgloss.Color("42"),
	Error:         lipgloss.Color("196"),
	Warning:       lipgloss.Color("214"),
	Info:          lipgloss.Color("82"),
	Text:          lipgloss.Color("252"),
	TextMuted:     lipgloss.Color("241"),
	TextHint:      lipgloss.Color("243"),
TextSubtle:    lipgloss.Color("245"),
	TextBright:    lipgloss.Color("250"),
	BgBase:        lipgloss.Color(""),
	BgPanel:       lipgloss.Color("235"),
	BgSelected:    lipgloss.Color("236"),
	BgFocused:     lipgloss.Color("238"),
	BorderNormal:  lipgloss.Color("240"),
	BorderFocused: lipgloss.Color("63"),
	BorderAccent:  lipgloss.Color("214"),
	DiffAdded:     lipgloss.Color("42"),
	DiffRemoved:   lipgloss.Color("196"),
	DiffContext:   lipgloss.Color("241"),
	DiffHeader:    lipgloss.Color("63"),
	ScrollTrack:   lipgloss.Color("237"),
	ScrollThumb:   lipgloss.Color("243"),
	Shadow:        lipgloss.Color("237"),
	Cyan:          lipgloss.Color("39"),
	YellowW:       lipgloss.Color("226"),
}

// CatppuccinMocha is a popular pastel dark theme.
var CatppuccinMocha = Theme{
	Name:          "catppuccin-mocha",
	Primary:       lipgloss.Color("#cba6f7"), // mauve
	Secondary:     lipgloss.Color("#89b4fa"), // blue
	Tertiary:      lipgloss.Color("#94e2d5"), // teal
	Success:       lipgloss.Color("#a6e3a1"), // green
	Error:         lipgloss.Color("#f38ba8"), // red
	Warning:       lipgloss.Color("#fab387"), // peach
	Info:          lipgloss.Color("#74c7ec"), // sapphire
	Text:          lipgloss.Color("#cdd6f4"), // text
	TextMuted:     lipgloss.Color("#6c7086"), // overlay0
	TextHint:      lipgloss.Color("#7f849c"), // overlay1
TextSubtle:    lipgloss.Color("#9399b2"), // subtext0
	TextBright:    lipgloss.Color("#bac2de"), // subtext1
	BgBase:        lipgloss.Color("#1e1e2e"), // base
	BgPanel:       lipgloss.Color("#313244"), // surface0
	BgSelected:    lipgloss.Color("#45475a"), // surface1
	BgFocused:     lipgloss.Color("#585b70"), // surface2
	BorderNormal:  lipgloss.Color("#45475a"), // surface1
	BorderFocused: lipgloss.Color("#89b4fa"), // blue
	BorderAccent:  lipgloss.Color("#cba6f7"), // mauve
	DiffAdded:     lipgloss.Color("#a6e3a1"), // green
	DiffRemoved:   lipgloss.Color("#f38ba8"), // red
	DiffContext:   lipgloss.Color("#6c7086"), // overlay0
	DiffHeader:    lipgloss.Color("#89b4fa"), // blue
	ScrollTrack:   lipgloss.Color("#313244"), // surface0
	ScrollThumb:   lipgloss.Color("#585b70"), // surface2
	Shadow:        lipgloss.Color("#11111b"), // crust
	Cyan:          lipgloss.Color("#89dceb"), // sky
	YellowW:       lipgloss.Color("#f9e2af"), // yellow
}

// TokyoNight is a purple/blue dark theme.
var TokyoNight = Theme{
	Name:          "tokyo-night",
	Primary:       lipgloss.Color("#7aa2f7"), // blue
	Secondary:     lipgloss.Color("#bb9af7"), // purple
	Tertiary:      lipgloss.Color("#7dcfff"), // cyan
	Success:       lipgloss.Color("#9ece6a"), // green
	Error:         lipgloss.Color("#f7768e"), // red
	Warning:       lipgloss.Color("#e0af68"), // yellow
	Info:          lipgloss.Color("#2ac3de"), // teal
	Text:          lipgloss.Color("#c0caf5"), // fg
	TextMuted:     lipgloss.Color("#565f89"), // comment
	TextHint:      lipgloss.Color("#737aa2"), // dark5
TextSubtle:    lipgloss.Color("#a9b1d6"), // fg_dark
	TextBright:    lipgloss.Color("#a9b1d6"), // fg_dark
	BgBase:        lipgloss.Color("#1a1b26"), // bg
	BgPanel:       lipgloss.Color("#24283b"), // bg_highlight
	BgSelected:    lipgloss.Color("#292e42"), // bg_visual
	BgFocused:     lipgloss.Color("#33467c"), // bg_search
	BorderNormal:  lipgloss.Color("#3b4261"), // border
	BorderFocused: lipgloss.Color("#7aa2f7"), // blue
	BorderAccent:  lipgloss.Color("#bb9af7"), // purple
	DiffAdded:     lipgloss.Color("#9ece6a"), // green
	DiffRemoved:   lipgloss.Color("#f7768e"), // red
	DiffContext:   lipgloss.Color("#565f89"), // comment
	DiffHeader:    lipgloss.Color("#7aa2f7"), // blue
	ScrollTrack:   lipgloss.Color("#24283b"), // bg_highlight
	ScrollThumb:   lipgloss.Color("#3b4261"), // border
	Shadow:        lipgloss.Color("#16161e"), // bg_dark
	Cyan:          lipgloss.Color("#7dcfff"), // cyan
	YellowW:       lipgloss.Color("#e0af68"), // yellow
}

// Dracula is a green/purple/pink dark theme.
var Dracula = Theme{
	Name:          "dracula",
	Primary:       lipgloss.Color("#bd93f9"), // purple
	Secondary:     lipgloss.Color("#8be9fd"), // cyan
	Tertiary:      lipgloss.Color("#50fa7b"), // green
	Success:       lipgloss.Color("#50fa7b"), // green
	Error:         lipgloss.Color("#ff5555"), // red
	Warning:       lipgloss.Color("#ffb86c"), // orange
	Info:          lipgloss.Color("#8be9fd"), // cyan
	Text:          lipgloss.Color("#f8f8f2"), // fg
	TextMuted:     lipgloss.Color("#6272a4"), // comment
	TextHint:      lipgloss.Color("#6272a4"), // comment
TextSubtle:    lipgloss.Color("#6272a4"), // comment
	TextBright:    lipgloss.Color("#f8f8f2"), // fg
	BgBase:        lipgloss.Color("#282a36"), // bg
	BgPanel:       lipgloss.Color("#44475a"), // current_line
	BgSelected:    lipgloss.Color("#44475a"), // current_line
	BgFocused:     lipgloss.Color("#6272a4"), // comment
	BorderNormal:  lipgloss.Color("#44475a"), // current_line
	BorderFocused: lipgloss.Color("#bd93f9"), // purple
	BorderAccent:  lipgloss.Color("#ff79c6"), // pink
	DiffAdded:     lipgloss.Color("#50fa7b"), // green
	DiffRemoved:   lipgloss.Color("#ff5555"), // red
	DiffContext:   lipgloss.Color("#6272a4"), // comment
	DiffHeader:    lipgloss.Color("#8be9fd"), // cyan
	ScrollTrack:   lipgloss.Color("#383a46"), // slightly lighter bg
	ScrollThumb:   lipgloss.Color("#6272a4"), // comment
	Shadow:        lipgloss.Color("#21222c"), // darker bg
	Cyan:          lipgloss.Color("#8be9fd"), // cyan
	YellowW:       lipgloss.Color("#f1fa8c"), // yellow
}

// GruvboxDark is a warm retro dark theme.
var GruvboxDark = Theme{
	Name:          "gruvbox-dark",
	Primary:       lipgloss.Color("#fe8019"), // orange
	Secondary:     lipgloss.Color("#83a598"), // blue
	Tertiary:      lipgloss.Color("#8ec07c"), // aqua
	Success:       lipgloss.Color("#b8bb26"), // green
	Error:         lipgloss.Color("#fb4934"), // red
	Warning:       lipgloss.Color("#fabd2f"), // yellow
	Info:          lipgloss.Color("#83a598"), // blue
	Text:          lipgloss.Color("#ebdbb2"), // fg
	TextMuted:     lipgloss.Color("#928374"), // gray
	TextHint:      lipgloss.Color("#a89984"), // gray_light
TextSubtle:    lipgloss.Color("#a89984"), // gray_light
	TextBright:    lipgloss.Color("#fbf1c7"), // fg_bright
	BgBase:        lipgloss.Color("#282828"), // bg
	BgPanel:       lipgloss.Color("#3c3836"), // bg1
	BgSelected:    lipgloss.Color("#504945"), // bg2
	BgFocused:     lipgloss.Color("#665c54"), // bg3
	BorderNormal:  lipgloss.Color("#504945"), // bg2
	BorderFocused: lipgloss.Color("#83a598"), // blue
	BorderAccent:  lipgloss.Color("#fe8019"), // orange
	DiffAdded:     lipgloss.Color("#b8bb26"), // green
	DiffRemoved:   lipgloss.Color("#fb4934"), // red
	DiffContext:   lipgloss.Color("#928374"), // gray
	DiffHeader:    lipgloss.Color("#83a598"), // blue
	ScrollTrack:   lipgloss.Color("#3c3836"), // bg1
	ScrollThumb:   lipgloss.Color("#665c54"), // bg3
	Shadow:        lipgloss.Color("#1d2021"), // bg_hard
	Cyan:          lipgloss.Color("#8ec07c"), // aqua
	YellowW:       lipgloss.Color("#fabd2f"), // yellow
}

// Nord is a cool blue-gray dark theme.
var Nord = Theme{
	Name:          "nord",
	Primary:       lipgloss.Color("#88c0d0"), // nord8
	Secondary:     lipgloss.Color("#81a1c1"), // nord9
	Tertiary:      lipgloss.Color("#8fbcbb"), // nord7
	Success:       lipgloss.Color("#a3be8c"), // nord14
	Error:         lipgloss.Color("#bf616a"), // nord11
	Warning:       lipgloss.Color("#ebcb8b"), // nord13
	Info:          lipgloss.Color("#5e81ac"), // nord10
	Text:          lipgloss.Color("#eceff4"), // nord6
	TextMuted:     lipgloss.Color("#616e88"), // muted
	TextHint:      lipgloss.Color("#7b88a1"), // hint
TextSubtle:    lipgloss.Color("#7b88a1"), // hint
	TextBright:    lipgloss.Color("#d8dee9"), // nord4
	BgBase:        lipgloss.Color("#2e3440"), // nord0
	BgPanel:       lipgloss.Color("#3b4252"), // nord1
	BgSelected:    lipgloss.Color("#434c5e"), // nord2
	BgFocused:     lipgloss.Color("#4c566a"), // nord3
	BorderNormal:  lipgloss.Color("#434c5e"), // nord2
	BorderFocused: lipgloss.Color("#81a1c1"), // nord9
	BorderAccent:  lipgloss.Color("#88c0d0"), // nord8
	DiffAdded:     lipgloss.Color("#a3be8c"), // nord14
	DiffRemoved:   lipgloss.Color("#bf616a"), // nord11
	DiffContext:   lipgloss.Color("#616e88"), // muted
	DiffHeader:    lipgloss.Color("#81a1c1"), // nord9
	ScrollTrack:   lipgloss.Color("#3b4252"), // nord1
	ScrollThumb:   lipgloss.Color("#4c566a"), // nord3
	Shadow:        lipgloss.Color("#242933"), // darker
	Cyan:          lipgloss.Color("#88c0d0"), // nord8
	YellowW:       lipgloss.Color("#ebcb8b"), // nord13
}
