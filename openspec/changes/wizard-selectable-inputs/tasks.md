## 1. Dependency Setup

- [x] 1.1 Add `charmbracelet/huh` to `go.mod` (`go get github.com/charmbracelet/huh`)
- [x] 1.2 Verify build still passes with the new dependency (`go build ./...`)

## 2. Selectable Prompts for Constrained Fields

- [x] 2.1 Replace provider type text input with `huh.Select` — options: anthropic, openai, google, openai_compatible; default: anthropic
- [x] 2.2 Replace auth method text input with `huh.Select` — options filtered by `config.AuthMethodsByType[providerType]`; default: first valid method
- [x] 2.3 Replace primary provider text input with `huh.Select` (Yes/No) — skip step entirely when no other providers exist (auto-set primary=true)
- [x] 2.4 Keep provider name as `huh.Input` with placeholder "e.g., claude, gpt4"
- [x] 2.5 Keep API key as `huh.Input` with `EchoMode: huh.EchoModePassword`; preserve existing connection test + retry/skip/quit flow
- [x] 2.6 Keep base URL as `huh.Input` with placeholder "e.g., http://localhost:11434/v1" (shown only for openai_compatible)

## 3. Model Browser with litellm Pre-Population

- [x] 3.1 Replace `selectModel()` numbered list with `huh.Select` populated from `MetadataStore.ModelsForProvider(providerType)`
- [x] 3.2 Format each model option as `model-name  (capabilities)` using `config.FormatCapabilities()` — display capabilities inline in the list
- [x] 3.3 Pre-select the default model for the provider type (`config.DefaultModelByType`); fall back to first item if default not in list
- [x] 3.4 Enable type-to-filter on the model select (built-in `huh.Select` filtering)
- [x] 3.5 Append "Custom model..." as the last option in the model list; when selected, show a `huh.Input` for manual model name entry with provider-specific placeholder
- [x] 3.6 Implement text input fallback when litellm returns no models (no metadata or openai_compatible) — use `huh.Input` with hint text (e.g., "e.g. claude-sonnet-4-20250514")

## 4. Cleanup & Verification

- [x] 4.1 Remove the `bufio.NewReader` usage from `runSetupWizard()` (replace with huh prompts); remove `selectModel()` function
- [x] 4.2 Verify `go build ./...` compiles cleanly
- [x] 4.3 Verify `go test ./... -count=1` passes (existing tests should not break since setup.go has no tests — config/provider tests remain green)
- [ ] 4.4 Manual test: run `polycode init` (or delete config and run `polycode`) to verify the full interactive wizard flow end-to-end *(deferred: beta validation — requires interactive terminal)*
