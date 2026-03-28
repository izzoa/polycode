## 1. Env Var Metadata Type + Registry Mapping

- [x] 1.1 Add `EnvVarMeta` struct to `registry.go`: Name, Description, IsSecret, IsRequired
- [x] 1.2 Change `ToMCPServerConfig` to return `(config.MCPServerConfig, []EnvVarMeta)` — populate from `RegistryPackage.EnvVars`
- [x] 1.3 Update all `ToMCPServerConfig` call sites: `mcp.go` (browse), `mcp_wizard.go` (browse select), app.go (registry select handler)
- [x] 1.4 Add test: `TestToMCPServerConfig_EnvVarMeta` — verify IsSecret/IsRequired flags preserved in returned metadata

## 2. PopularMCPServers Typed Env Vars

- [x] 2.1 Add `MCPTemplateEnvVar` struct to `mcp_wizard.go`: Name, IsSecret
- [x] 2.2 Change `MCPServerTemplate.EnvVars` from `[]string` to `[]MCPTemplateEnvVar`
- [x] 2.3 Update all PopularMCPServers entries: mark GITHUB_TOKEN, BRAVE_API_KEY, SLACK_BOT_TOKEN, SLACK_TEAM_ID as IsSecret=true
- [x] 2.4 Update hardcoded fallback browse handler to set `mcpWizardEnvSecrets` from template

## 3. TUI Wizard Secret Input

- [x] 3.1 Add `mcpWizardEnvSecrets map[string]bool` field to Model — tracks which env vars are secrets
- [x] 3.2 Set `mcpWizardEnvSecrets` when selecting a server from: registry results (from EnvVarMeta), hardcoded templates (from MCPTemplateEnvVar), or wizard browse fallback
- [x] 3.3 In `mcpStepEnv` rendering: if current env var requires input has matching secret flag, display value as `••••••••` instead of plain text
- [x] 3.4 In `updateMCPWizardInput` for env step: set `mcpWizardInput.EchoMode = textinput.EchoPassword` when entering a secret var, `textinput.EchoNormal` otherwise
- [x] 3.5 In `renderMCPSummary`: display secret env vars as `•••••••• (configured)` instead of actual values

## 4. TUI Wizard Keyring Storage

- [x] 4.1 In `saveMCPWizard`: for each env var marked as secret, call `auth.NewStore().Set("mcp_{server}_{var}", value)` and replace the config value with `$KEYRING:mcp_{server}_{var}`
- [x] 4.2 Non-secret env vars remain as plain text in config.yaml (existing behavior)

## 5. CLI Browse Secret Input

- [x] 5.1 Change `runMCPBrowse` to receive `[]EnvVarMeta` from `ToMCPServerConfig`
- [x] 5.2 For secret env vars: use `huh.NewInput().EchoMode(huh.EchoModePassword)` instead of plain input
- [x] 5.3 For secret env vars: after value entered, call `auth.NewStore().Set("mcp_{server}_{var}", value)` and set config value to `$KEYRING:mcp_{server}_{var}`
- [x] 5.4 For non-secret env vars: keep plain input and plain config value

## 6. CLI Add Secret Awareness

- [x] 6.1 In `runMCPAdd`: when prompting for env vars, detect common secret patterns (names containing TOKEN, KEY, SECRET, PASSWORD) and use masked input for those
- [x] 6.2 Store detected secrets in keyring with `$KEYRING:` references

## 7. Tests

- [x] 7.1 Add test: `TestEnvVarMeta_SecretFlag` — verify metadata preserves IsSecret from registry
- [x] 7.2 Add test: `TestKeyringReference_Format` — verify `$KEYRING:mcp_{server}_{var}` format
- [x] 7.3 Verify existing `TestToMCPServerConfig_EnvVars` still passes

## 8. Final Verification

- [x] 8.1 Run `go build ./...` — clean compile
- [x] 8.2 Run `go test ./... -count=1 -race` — all packages pass
- [x] 8.3 Manual test: `polycode mcp browse` → select server with secret env → verify masked input
- [x] 8.4 Manual test: verify config.yaml contains `$KEYRING:` reference after saving
- [x] 8.5 Manual test: TUI wizard env step → verify masked input for secret vars
