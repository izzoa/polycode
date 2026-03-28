## 1. Model State

- [x] 1.1 Add `mcpWizardEnvOrder []string` field to Model — ordered list of known env var names (required first, then optional)
- [x] 1.2 Add `mcpWizardEnvIdx int` field to Model — current prompting index (-1 or >= len means freeform mode)
- [x] 1.3 Add `mcpWizardEnvDescs map[string]string` field to Model — env var descriptions from registry metadata
- [x] 1.4 Initialize all three fields in `initMCPWizardForAdd` and `initMCPWizardForEdit`

## 2. Populate Env Order on Server Selection

- [x] 2.1 When selecting from hardcoded template (`updateMCPWizardBrowse` fallback path): build `mcpWizardEnvOrder` from `tmpl.EnvVars` (required first), set `mcpWizardEnvIdx = 0`
- [x] 2.2 When selecting from live registry (`updateMCPWizardBrowse` live path + `onMCPRegistrySelect`): build `mcpWizardEnvOrder` from returned `EnvVarMeta`, set descriptions in `mcpWizardEnvDescs`, set `mcpWizardEnvIdx = 0`
- [x] 2.3 When no env vars are known (custom server path): leave `mcpWizardEnvOrder` empty, set `mcpWizardEnvIdx = -1` (freeform only)

## 3. Env Step Rendering — Known Var Mode

- [x] 3.1 In `mcpStepEnv` rendering: if `mcpWizardEnvIdx >= 0 && mcpWizardEnvIdx < len(mcpWizardEnvOrder)`, render the individual prompt mode
- [x] 3.2 Show already-entered vars above the current prompt (secrets masked as `•••••••• (configured)`, non-secrets show value)
- [x] 3.3 Show current var name as prompt title: `"GITHUB_TOKEN (required, secret):"` or `"BASE_URL:"` based on metadata
- [x] 3.4 Set `mcpWizardInput.EchoMode = textinput.EchoPassword` if current var is secret, `textinput.EchoNormal` otherwise
- [x] 3.5 Show progress: `"Env var 1 of 2"` below the title

## 4. Env Step Rendering — Freeform Mode

- [x] 4.1 If `mcpWizardEnvIdx >= len(mcpWizardEnvOrder)` or order is empty: show all entered vars (with masking) + freeform input
- [x] 4.2 Prompt title: `"Add additional env vars? (KEY=value or Enter to continue)"`
- [x] 4.3 Ensure `mcpWizardInput.EchoMode = textinput.EchoNormal` for freeform mode

## 5. Env Step Input Handling — Known Var Mode

- [x] 5.1 In `updateMCPWizardInput` for `mcpStepEnv`: if in known-var mode (idx in range), on Enter save value to `mcpWizardEnv[currentVar]`, advance idx, reset input, set EchoMode for next var
- [x] 5.2 If empty Enter on a required var: stay (don't advance) — show a subtle hint
- [x] 5.3 If empty Enter on an optional var: advance (skip it)
- [x] 5.4 After last known var: switch to freeform (set idx = len(order))

## 6. Env Step Input Handling — Freeform Mode

- [x] 6.1 Keep existing freeform behavior: parse KEY=value, add to env map, stay for more; empty Enter advances to next wizard step

## 7. Esc Navigation

- [x] 7.1 In `updateMCPWizard` Esc handler for `mcpStepEnv`: if in known-var mode and idx > 0, go back to previous var (decrement idx, pre-fill with previous value)
- [x] 7.2 If in known-var mode and idx == 0, go back to previous wizard step (existing behavior)
- [x] 7.3 If in freeform mode, go back to last known var (if any) or previous wizard step

## 8. prepareMCPInput Update

- [x] 8.1 In `prepareMCPInput` for `mcpStepEnv`: if in known-var mode, pre-fill with existing value for current var and set appropriate EchoMode
- [x] 8.2 If in freeform mode, clear input and set EchoNormal

## 9. Final Verification

- [x] 9.1 Run `go build ./...` — clean compile
- [x] 9.2 Run `go test ./... -count=1` — all packages pass
- [x] 9.3 Manual test: select GitHub template → prompted individually for GITHUB_TOKEN with masking
- [x] 9.4 Manual test: custom server → freeform KEY=value only (no individual prompts)
- [x] 9.5 Manual test: Esc navigates backward through individual vars
