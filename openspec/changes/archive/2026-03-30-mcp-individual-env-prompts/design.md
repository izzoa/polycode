## Context

The wizard env step currently has one mode: freeform `KEY=value` input. When the user selects a server from the registry or hardcoded templates, env var metadata is available (name, isSecret, isRequired, description) via `mcpWizardEnvSecrets` and `mcpWizardEnv` (pre-populated with empty values for known vars).

The provider wizard already uses per-field input with `EchoPassword` for API keys (in `wizard.go`), so the UX pattern exists.

## Goals / Non-Goals

**Goals:**
- Known env vars prompted individually: name as label, value as input, secret vars masked
- Required vars prompted first, then optional, then freeform for extras
- Secret masking via `textinput.EchoPassword` per-var — toggled based on current var's secret flag
- Previously-entered values shown in the var list above the current prompt (secrets masked as `••••••••`)
- Smooth transition: after all known vars are prompted, switch to freeform mode for additional vars
- Esc during known-var prompting goes back to previous var (or to previous wizard step if at first var)

**Non-Goals:**
- Validating env var values (no format checking)
- Required var enforcement (don't block if user skips a required var — warn only)

## Decisions

### D1: Ordered env var list on Model

**Choice**: Add `mcpWizardEnvOrder []string` (ordered list of known var names) and `mcpWizardEnvIdx int` (current prompting index, -1 = freeform mode) to the Model. Populated when a server is selected from registry/template. Required vars sorted first.

**Rationale**: The existing `mcpWizardEnv` is a map (unordered). An ordered list ensures consistent prompting order and lets us track the current position.

### D2: Env step rendering branches on idx

**Choice**: In `mcpStepEnv` rendering:
- If `mcpWizardEnvIdx >= 0 && mcpWizardEnvIdx < len(mcpWizardEnvOrder)`: show current var name as title, description as subtitle, input field (EchoPassword if secret). Show already-entered vars above with values (masked for secrets).
- If `mcpWizardEnvIdx >= len(mcpWizardEnvOrder)` or `mcpWizardEnvOrder` is empty: show freeform `KEY=value` input (existing behavior).

### D3: Enter advances to next var or freeform

**Choice**: On Enter during known-var prompting:
- Save the value to `mcpWizardEnv[currentVarName]`
- Advance `mcpWizardEnvIdx++`
- Set EchoMode for the next var (or EchoNormal for freeform)
- If past last known var, switch to freeform mode

On Enter during freeform mode:
- If empty, advance to next wizard step (existing behavior)
- If `KEY=value`, parse and add (existing behavior)

### D4: Esc navigates backward through vars

**Choice**: Esc during known-var prompting:
- If at first var (idx=0), go back to previous wizard step
- If at later var (idx>0), go back to previous var
- If in freeform mode, go back to last known var (if any)
