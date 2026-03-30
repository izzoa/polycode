## Why

Every polished CLI tool has a recognizable startup banner — it establishes brand identity and signals to the user that the app has loaded. Polycode currently launches into the TUI with no visual greeting. A brief ASCII art splash with the "polycode" name adds personality and professionalism, similar to how opencode, lazygit, and other TUIs greet users.

## What Changes

- **ASCII art banner**: A styled "polycode" ASCII art logo displayed briefly on TUI startup before the main interface renders
- **Tagline**: A one-line subtitle below the banner (e.g., "multi-model consensus coding assistant")
- **Version display**: Show the current version alongside the banner
- **Brief display**: The banner shows for ~1 second or until the first keypress, then transitions to the main TUI

## Capabilities

### New Capabilities
- `startup-banner`: ASCII art intro screen displayed on TUI launch

### Modified Capabilities
_(none)_

## Impact

- **`internal/tui/`**: New splash screen view state in the Bubble Tea model
- **`cmd/polycode/app.go`**: Minor change to pass version string to TUI
