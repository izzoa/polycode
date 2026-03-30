## 1. ASCII Art & Splash View

- [x] 1.1 Create the ASCII art string constant for "polycode" in a clean block-letter style
- [x] 1.2 Add `showSplash bool` field to the TUI Model, defaulting to `true` on creation
- [x] 1.3 Add a `splashTimeout` tea.Tick command in Init() that fires after 1.5 seconds
- [x] 1.4 Handle the timeout message in Update() to set `showSplash = false`
- [x] 1.5 Handle any keypress during splash in Update() to set `showSplash = false` (except ctrl+c which still quits)
- [x] 1.6 Implement `renderSplash()` in View — center the ASCII art, render version below it, render tagline, apply Lip Gloss gradient coloring
- [x] 1.7 Update `View()` to return `renderSplash()` when `showSplash` is true, otherwise render the normal TUI

## 2. Version Passthrough

- [x] 2.1 Add a `version` field to the TUI Model
- [x] 2.2 Update `NewModel()` to accept a version string parameter
- [x] 2.3 Update `startTUI` in `app.go` to pass the version string to `NewModel()`
