# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.6.2] - 2026-03-19

### Added
- **Focusable tab bar navigation**: Press ↑ (when input is empty) to focus the tab bar, then ←/→ to switch tabs. Press ↓, Enter, or Esc to return focus to input. Tab bar shows visual focus indicator (▸ prefix, underlined active tab, blurred textarea).

### Fixed
- **Arrow keys no longer hijack textarea**: Left/right arrows only switch tabs when the tab bar is focused, not when typing.
- **Removed grey cursor line highlight** from the textarea input.

## [1.6.1] - 2026-03-19

### Fixed
- **Typing `p`, `?`, and arrow keys in input no longer triggers shortcuts**: Single-character shortcuts (`p` for provenance, `?` for help) now only fire when the textarea is empty. Left/right arrow keys move the cursor; use **Shift+←/→** to switch tabs.

### Added
- **Per-directory session scoping**: Chat sessions are now stored per working directory. Starting polycode in different project folders resumes separate conversations. Sessions live in `~/.config/polycode/sessions/<hash>.json`.

## [1.6.0] - 2026-03-19

### Added
- **Tabbed TUI interface**: Each provider gets its own tab. The Consensus tab (default) shows the synthesized response, while provider tabs show individual responses. Navigate with ←/→ arrow keys.
- **Unified tab bar**: App title, mode, provider status icons, and token usage are all shown in a single tab bar — replaces the separate status bar.
- **Scrollable chat viewport**: PgUp/PgDn, Ctrl+U/Ctrl+D, Home/End for keyboard scrolling. Mouse scroll wheel enabled via `WithMouseCellMotion`.
- **Output height constrained**: Chat view no longer overflows the terminal height.

### Fixed
- **Provider tab content now populated**: `ProviderChunkMsg` with `Done: true` was discarding the `Delta` content. Individual provider responses now appear in their tabs.

## [1.5.3] - 2026-03-18

### Added
- **Inline slash command autocomplete**: Matching commands appear as hints above the input as you type (e.g., typing `/cl` shows `/clear`). Press Tab to accept the highlighted completion.

## [1.5.2] - 2026-03-18

### Fixed
- **Fix panic: "strings: illegal use of non-zero Builder copied by value"**: `strings.Builder` fields in the TUI model were copied by value when Bubble Tea copies the model on each Update. Changed `consensusContent` and `ProviderPanel.Content` to pointer types (`*strings.Builder`) to avoid the copy-after-use panic.

### Added
- **Edit base URL in config editor**: `polycode config edit` now shows a "Change base URL" option for providers that have one configured

## [1.5.1] - 2026-03-18

### Added
- **Rename provider in config editor**: `polycode config edit` now has a "Rename" option that migrates credentials to the new name
- **`polycode auth logout` interactive picker**: Running without args shows a selectable list of providers instead of requiring the exact name

### Fixed
- **`polycode auth logout` no longer errors on missing credentials**: "Credential not found" is now treated as success (already removed) instead of a hard error

## [1.5.0] - 2026-03-18

### Added
- **Slash command autocomplete**: Type `/` and press Tab to cycle through matching commands (e.g., `/cl` + Tab → `/clear`)
- **`polycode config edit`**: Interactive config editor — add, remove, or edit providers (change model, API key, primary designation) from the command line
- **`polycode config show`**: Print current configuration summary
- **`polycode config path`**: Print config file location
- **Prominent error display**: Provider and consensus errors now show as bold red text in the main chat area instead of being hidden in provider panels
- **Version-aware base URL handling**: OpenAI-compatible providers with versioned base URLs (e.g., `/v4`) no longer get `/v1` doubled in the request path

## [1.4.1] - 2026-03-18

### Fixed
- **TUI chat submission now works**: `tea.NewProgram(model)` was called before `Set*Handler()`, so Bubble Tea ran a copy of the model with all callbacks nil. Pressing Enter cleared the input but never sent a query. Fixed by moving `NewProgram` after all handler wiring so the model copy has all callbacks set.

## [1.4.0] - 2026-03-18

### Added
- **Multi-provider wizard loop**: `polycode init` now asks "Add another provider?" after each provider, letting users configure multiple providers in one session. Auto-sets `min_responses` based on provider count.
- **`polycode provider add` command**: Add a new provider to an existing config without re-running init. Asks about primary designation and auto-bumps `min_responses`.
- **Slash commands**: `/help`, `/save`, `/exit`, `/quit`, `/export [path]` added to the TUI chat input alongside existing `/clear`, `/settings`, `/mode`, `/memory`, `/plan`.
- **"Thinking..." indicator**: Chat area shows an animated spinner with "Thinking..." while waiting for the first response chunk from providers.
- **Status bar phase indicator**: Shows "querying providers..." while providers are processing and "synthesizing..." during consensus, replacing the generic "querying..." label.
- **"Edit base URL" on connection failure**: `openai_compatible` providers now show an "Edit base URL" option in the connection test failure menu.

### Fixed
- **Connection test passes base URL**: `openai_compatible` connection tests no longer fail with "unsupported protocol scheme" due to missing base URL.
- **Lightweight connection test for openai_compatible**: Uses `GET /models` instead of a chat completion, avoiding 404s from unrecognized placeholder model names.

## [1.3.2] - 2026-03-18

### Fixed
- **Remove broken OAuth defaults for Anthropic and OpenAI**: These providers do not support OAuth for third-party apps. The wizard no longer offers `oauth` as an auth method for Anthropic or OpenAI — use `api_key` instead. Google retains OAuth support (requires explicit `oauth_client_id` in config).
- **Clear error when OAuth is misconfigured**: Selecting `auth: oauth` without providing `oauth_client_id`/`oauth_device_url`/`oauth_token_url` now gives an actionable error message instead of a circular "run polycode auth login" loop.

## [1.3.1] - 2026-03-18

### Fixed
- **`polycode auth login` now actually authenticates**: Previously was a stub that printed "Authentication successful" without running any auth flow. Now creates the provider and calls `Authenticate()`, which triggers the OAuth device flow or API key lookup as appropriate.
- **`polycode auth logout` now removes credentials**: Previously was a stub. Now calls `store.Delete()` to remove the credential from the OS keyring.

## [1.3.0] - 2026-03-18

### Added
- **Endpoint model discovery for OpenAI-compatible providers**: The setup wizard now queries `GET /models` on the configured base URL to discover available models, with automatic `/v1` path fallback
- Discovered models are cross-referenced against litellm metadata to show capability info (context window, tools, vision, reasoning)
- Wizard step reordering for `openai_compatible`: base URL is now collected before auth and model selection to enable discovery

### Changed
- `selectModel()` now accepts base URL and API key parameters for endpoint discovery

## [1.2.1] - 2026-03-17

### Fixed
- **litellm model matching**: Provider prefix matching now uses the actual litellm key format — bare keys for Anthropic (`claude-*`) and OpenAI (`gpt-*`, `o1*`, `o3*`), dot-prefixed for Bedrock (`anthropic.*`), and slash-prefixed for Gemini (`gemini/`). Previously returned zero models for Anthropic and OpenAI.

## [1.2.0] - 2026-03-17

### Added
- **Interactive selectable inputs for CLI setup wizard**: Replaced plain text input with arrow-key navigable `huh.Select` lists for provider type, auth method, and connection test recovery
- Filterable model picker pre-populated from litellm metadata with inline capabilities display
- "Custom model..." escape hatch for manual model name entry
- Password-masked API key input via `huh.Input`
- Added `charmbracelet/huh` dependency (Charm ecosystem forms library)

## [1.1.0] - 2026-03-17

### Added
- Smart wizard: litellm model browsing with numbered list, filtered auth methods by provider type, connection testing with retry/skip/quit
- ASCII logo banner on `polycode init` and CLI commands

## [1.0.0] - 2026-03-17

### Added
- Workflow platform and team adoption features (Phase 5)
- Adaptive routing and repo memory (Phase 4)
- Consensus-native agent teams (Phase 3)
- Evidence-backed consensus review (Phase 2)
- Execution core and eval harness (Phase 1)

## [0.2.0]

### Added
- `/clear` command, markdown rendering, and auto-resumable sessions
- 5-phase product roadmap

## [0.1.1]

### Fixed
- Module path corrected to match repo URL
- Removed AI editor config dirs from repo

## [0.1.0]

### Added
- Initial release: multi-model consensus coding assistant TUI
- Anthropic, OpenAI, Google Gemini, and OpenAI-compatible provider support
- Fan-out consensus pipeline with primary model synthesis
- Bubble Tea TUI with streaming provider panels
- Tool execution (file read/write, shell exec)
- OS keyring credential storage with encrypted file fallback
- CI/CD pipeline with GoReleaser cross-platform builds
