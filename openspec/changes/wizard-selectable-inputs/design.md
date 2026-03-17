## Context

The CLI setup wizard in `cmd/polycode/setup.go` currently uses `bufio.NewReader` for all user input — users must type exact strings for provider type, auth method, and model selection. The model step already shows a numbered list from litellm, but still requires typing a number. The TUI wizard (`internal/tui/wizard.go`) already implements arrow-key selectable lists using custom cursor logic, but the CLI wizard predates that pattern.

The project already depends on `charmbracelet/bubbletea`, `charmbracelet/bubbles`, and `charmbracelet/lipgloss`.

## Goals / Non-Goals

**Goals:**
- Replace text input with interactive arrow-key selectable lists for all constrained-choice fields (provider type, auth method, primary yes/no)
- Replace the numbered model list with a filterable, arrow-key navigable list pre-populated from litellm metadata
- Keep text input for free-form fields (provider name, API key, base URL)
- Maintain the existing connection test flow (auto-test after API key, retry/skip/quit)
- Preserve the fallback to text input when litellm model data is unavailable

**Non-Goals:**
- Rewriting the TUI wizard (`internal/tui/wizard.go`) — it already has selectable lists
- Adding new wizard steps or config fields
- Changing the config file format or validation logic
- Supporting mouse input in the CLI wizard

## Decisions

### 1. Use `charmbracelet/huh` for interactive form components

**Decision**: Add `charmbracelet/huh` as a dependency for the CLI wizard's select and input prompts.

**Rationale**: `huh` is the Charm ecosystem's forms library, purpose-built for exactly this use case — interactive terminal forms with select lists, text inputs, and confirms. It integrates naturally with the existing lipgloss styling and Bubble Tea runtime already in the project.

**Alternatives considered**:
- **Raw Bubble Tea mini-program**: Could build a standalone `tea.Program` for each prompt. More control but significantly more code for what `huh` already provides out of the box.
- **`manifoldco/promptui`**: Popular but outside the Charm ecosystem. Would add a separate styling system and different terminal handling.
- **Keep `bufio.Reader` with ANSI escape codes**: Possible to build custom selection with raw terminal manipulation, but fragile and reinventing what `huh` solves.

### 2. Model list uses `huh.Select` with type-to-filter and a "Custom..." option

**Decision**: Render models from `MetadataStore.ModelsForProvider()` as a `huh.Select` with filterable options. Append a "Custom model..." sentinel option that, when selected, opens a `huh.Input` for manual entry.

**Rationale**: This matches the TUI wizard's pattern (which already has "Custom model..." as the last list item). The `huh.Select` component supports built-in type-to-filter, so users can quickly narrow long model lists (e.g., typing "sonnet" to find Claude Sonnet variants).

### 3. Show model capabilities inline in the selection list

**Decision**: Each model option displays as `model-name  (128K context | tools | vision)` using the existing `config.FormatCapabilities()` helper.

**Rationale**: Users need to compare models at a glance. The capability formatting already exists and is used in the TUI wizard — reuse it directly.

### 4. Keep the wizard as a sequential prompt flow (not a single form)

**Decision**: Each wizard step remains a separate `huh.Run()` call rather than combining everything into one `huh.Form`.

**Rationale**: The wizard has conditional steps (base URL only for openai_compatible, API key only for api_key auth, model list depends on provider type). A single-form approach would require complex conditional visibility logic. Sequential prompts are simpler and match the existing flow.

## Risks / Trade-offs

- **New dependency (`huh`)**: Adds one more Charm library. → Mitigated by being in the same ecosystem already used (bubbletea, bubbles, lipgloss). Transitive deps overlap heavily.
- **Terminal compatibility**: `huh` uses alternate screen/raw mode features that may not work in all terminals. → Mitigated by `huh`'s built-in accessible mode fallback, and the existing Bubble Tea TUI already requires similar terminal capabilities.
- **Model list load time**: `MetadataStore` may need to fetch from GitHub on first run. → Already mitigated by existing 24h cache + stale-cache fallback. The wizard already calls this today.
