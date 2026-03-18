# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.3.2] - 2026-03-18

### Fixed
- **OAuth authentication now works out of the box**: Providers with `auth: oauth` previously failed because no OAuth endpoints were configured. Built-in default OAuth device flow endpoints are now supplied for Anthropic and Google when no explicit `oauth_client_id` is set in config. Providers without built-in defaults get a clear error message instead of a circular "run polycode auth login" loop.

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
