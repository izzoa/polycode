## ADDED Requirements

### Requirement: Theme struct defines semantic color slots
The system SHALL define a `Theme` struct with at minimum these semantic color categories: base palette (Primary, Secondary, Tertiary, Success, Error, Warning, Info), text (Text, TextMuted, TextHint, TextBright), backgrounds (BgBase, BgPanel, BgSelected, BgFocused), borders (BorderNormal, BorderFocused, BorderAccent), diff (DiffAdded, DiffRemoved, DiffContext, DiffAddedBg, DiffRemovedBg), and markdown (MdHeading, MdLink, MdCode, MdBlockquote).

#### Scenario: Theme struct is usable
- **WHEN** a Theme struct is instantiated with all fields set
- **THEN** each field SHALL be a valid `lipgloss.Color` value usable in lipgloss style methods

### Requirement: Six built-in themes are available
The system SHALL ship 6 built-in themes defined as struct literals: Polycode (default), Catppuccin Mocha, Tokyo Night, Dracula, Gruvbox Dark, and Nord.

#### Scenario: Default theme preserves current palette
- **WHEN** the Polycode default theme is applied
- **THEN** all colors SHALL match the current hardcoded values exactly (e.g., Primary=214, Secondary=63, Tertiary=86)

#### Scenario: Each built-in theme is complete
- **WHEN** any built-in theme is loaded
- **THEN** every field of the Theme struct SHALL have a non-zero color value

### Requirement: Theme selection persists to config
The system SHALL persist the selected theme name to the config YAML file under a `theme` key. On startup, the system SHALL load the persisted theme by name, falling back to the default if the name is unrecognized.

#### Scenario: Theme persists across restarts
- **WHEN** the user selects "tokyo-night" and restarts the application
- **THEN** the application SHALL load with the Tokyo Night theme applied

#### Scenario: Invalid theme name falls back to default
- **WHEN** the config contains `theme: "nonexistent"`
- **THEN** the application SHALL load with the Polycode default theme

### Requirement: Styles derive from active theme
The `defaultStyles(theme Theme)` function SHALL accept a Theme and return a Styles struct where every style uses colors from the theme. Switching themes SHALL rebuild Styles and invalidate all cached renders.

#### Scenario: Theme switch rebuilds styles
- **WHEN** the user switches from Polycode to Dracula theme
- **THEN** `m.styles` SHALL be rebuilt from the Dracula theme colors and `chatLogCache` SHALL be invalidated
