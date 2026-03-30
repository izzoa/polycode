## ADDED Requirements

### Requirement: All hardcoded colors replaced with theme references
Every `lipgloss.Color()` call in TUI rendering code SHALL be replaced with a reference to the active theme's semantic color fields. No rendering function SHALL contain hardcoded color numbers.

#### Scenario: No hardcoded colors remain in view rendering
- **WHEN** a grep for `lipgloss.Color("` is run across `internal/tui/*.go`
- **THEN** the only matches SHALL be within the theme definition files (`theme.go`, `themes.go`)

#### Scenario: Files migrated include all rendering files
- **WHEN** the migration is complete
- **THEN** the following files SHALL reference theme colors instead of hardcoded values: `view.go`, `settings.go`, `splash.go`, `wizard.go`, `mcp_wizard.go`, `diff.go`, `error_panel.go`, `status_bar.go`, `timeline.go`, `session_picker.go`

### Requirement: Glamour markdown rendering matches active theme
The glamour `TermRenderer` SHALL be rebuilt with an `ansi.StyleConfig` derived from the active theme whenever the theme changes. Heading, link, code block, and blockquote colors SHALL match theme slots.

#### Scenario: Markdown headings use theme color
- **WHEN** the Tokyo Night theme is active and markdown with `# Heading` is rendered
- **THEN** the heading color SHALL match `theme.MdHeading` from the Tokyo Night palette

#### Scenario: Theme switch rebuilds glamour renderer
- **WHEN** the user switches themes
- **THEN** the glamour renderer SHALL be rebuilt with the new theme's markdown colors

### Requirement: Diff rendering uses theme diff colors
The `renderColorizedDiff()` function SHALL use `theme.DiffAdded`, `theme.DiffRemoved`, and `theme.DiffContext` instead of hardcoded green/red/grey.

#### Scenario: Diff colors follow theme
- **WHEN** the Gruvbox Dark theme is active and a diff is rendered
- **THEN** added lines SHALL use `theme.DiffAdded` color and removed lines SHALL use `theme.DiffRemoved` color

### Requirement: Error panel uses theme error colors
The `renderErrorPanel()` function SHALL use `theme.Error` for the border and header instead of hardcoded red (196).

#### Scenario: Error panel follows theme
- **WHEN** the Catppuccin Mocha theme is active and an error panel is displayed
- **THEN** the error border and header SHALL use the Catppuccin error color
