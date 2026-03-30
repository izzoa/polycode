## 1. Wire /compact handler

- [ ] 1.1 Add `SetCompactHandler(func())` to TUI Model and store on model struct
- [ ] 1.2 Replace stub in `update.go` (`/compact` case) to call the compact handler
- [ ] 1.3 Add guard: only allow `/compact` when not actively querying
- [ ] 1.4 Implement compact handler closure in `app.go`: snapshot conversation, call `summarizeConversation()`, replace conversation state, send `TokenUpdateMsg`

## 2. User feedback

- [ ] 2.1 Send toast notification with before/after message count (e.g., "Compacted: 24 → 6 messages")
- [ ] 2.2 Update chat status message to show compaction result

## 3. Verification

- [ ] 3.1 Run `go build ./...` and `go test ./... -count=1` — all pass
- [ ] 3.2 Manual: send several messages, run `/compact`, verify conversation is summarized
- [ ] 3.3 Manual: verify `/compact` is blocked during active query
- [ ] 3.4 Manual: verify toast shows accurate message count delta
