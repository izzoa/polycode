## Context

Polycode's TUI currently goes straight to the input prompt. Adding a startup banner is a small cosmetic change that lives entirely in the TUI layer.

## Goals / Non-Goals

**Goals:**
- Show a styled ASCII art "polycode" banner on startup
- Display version and tagline
- Auto-dismiss after ~1.5 seconds or on any keypress

**Non-Goals:**
- Animated intro sequences
- Configurable banner text or custom ASCII art
- Splash screen on every subcommand (only the main TUI)

## Decisions

### 1. ASCII art style: Block letters via Lip Gloss gradient

**Choice**: A hardcoded ASCII art string rendered with Lip Gloss color styling (gradient from cyan to purple). The art uses a clean block-letter font.

**Rationale**: Hardcoded is simplest — no figlet dependency needed. Lip Gloss gradient makes it visually striking in terminals that support 256 colors, and degrades gracefully to plain text.

### 2. Implementation: Splash view state in Bubble Tea model

**Choice**: Add a `showSplash bool` field to the TUI Model. When true, View() renders the banner. A `tea.Tick` command fires after 1.5s to set `showSplash = false`. Any keypress also dismisses it.

**Rationale**: Minimal code change — one new field, a few lines in Update and View. No new files needed.

## Risks / Trade-offs

- **Startup delay**: The 1.5s splash adds perceived latency. → **Mitigation**: Any keypress skips it immediately. The pipeline initialization happens concurrently.
