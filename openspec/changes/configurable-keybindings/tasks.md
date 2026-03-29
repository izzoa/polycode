## 1. Keybinding Infrastructure

- [ ] 1.1 Create `internal/tui/keys.go` with Action type, Binding struct, and KeyMap struct
- [ ] 1.2 Define all default bindings as a `DefaultKeyMap()` function (~30-40 actions)
- [ ] 1.3 Implement reverse lookup map (key string -> Action) built from bindings
- [ ] 1.4 Implement `KeyMap.Action(keyStr string) (Action, bool)` lookup method
- [ ] 1.5 Implement conflict detection: scan for duplicate key strings, return warnings

## 2. Config Integration

- [ ] 2.1 Add `Keybindings map[string]string` field to Config struct with YAML tag `keybindings`
- [ ] 2.2 Add `LeaderKey string` field to Config struct with YAML tag `leader_key`
- [ ] 2.3 Implement merge logic: user bindings override defaults per-action
- [ ] 2.4 Expand `<leader>` prefix in binding values at parse time
- [ ] 2.5 Log conflict warnings during config load

## 3. Leader Key State Machine

- [ ] 3.1 Add `leaderArmed bool` and `leaderTimer tea.Cmd` fields to Model
- [ ] 3.2 On leader key press (when not in textarea): set leaderArmed, start 500ms tick
- [ ] 3.3 On next keypress while armed: resolve `<leader>+key` binding, execute action, clear state
- [ ] 3.4 On timeout tick: clear leaderArmed, process original key normally
- [ ] 3.5 Show "LEADER" indicator in status bar when armed

## 4. Handler Migration

- [ ] 4.1 Replace hardcoded key checks in updateChat with KeyMap.Action() lookups
- [ ] 4.2 Replace hardcoded key checks in updateSettings with KeyMap.Action() lookups
- [ ] 4.3 Replace hardcoded key checks in updateWizard and other mode handlers
- [ ] 4.4 Replace hardcoded key checks in global handlers (quit, mode switch, etc.)

## 5. Verification

- [ ] 5.1 Run `go build ./...` and `go test ./...` -- all pass
- [ ] 5.2 Verify all existing shortcuts work with default KeyMap (no config changes)
- [ ] 5.3 Manual: add custom binding in config, verify it overrides default
- [ ] 5.4 Manual: test leader key arm/timeout/complete cycle
- [ ] 5.5 Manual: verify leader key types normally in chat input textarea
