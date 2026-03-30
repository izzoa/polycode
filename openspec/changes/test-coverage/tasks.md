## 1. internal/notify tests

- [ ] 1.1 Create `internal/notify/notify_test.go`
- [ ] 1.2 Test `Notify()` on macOS: verify osascript command construction
- [ ] 1.3 Test `Notify()` with empty title/body (edge case)
- [ ] 1.4 Test `Notify()` graceful no-op when notification tool unavailable

## 2. TUI update handler tests

- [ ] 2.1 Test `Update()` message routing: verify correct handler called per view mode
- [ ] 2.2 Test chat mode: key messages (enter, ctrl+c, tab, escape) produce expected state transitions
- [ ] 2.3 Test settings mode: navigation keys, provider add/edit/delete flows
- [ ] 2.4 Test approval mode: accept/reject/edit key handling
- [ ] 2.5 Test mode transitions: chat→settings→chat, chat→approval→chat

## 3. Provider adapter error tests

- [ ] 3.1 Add error tests for Anthropic adapter: malformed SSE, HTTP 429/500, empty stream
- [ ] 3.2 Add error tests for OpenAI adapter: malformed SSE, HTTP errors, function call parsing failures
- [ ] 3.3 Add error tests for Gemini adapter: malformed SSE, auth refresh failure
- [ ] 3.4 Add error tests for OpenAI-compatible adapter: connection refused, non-standard error formats

## 4. CI mode tests

- [ ] 4.1 Test CI mode with mock providers: verify non-interactive pipeline
- [ ] 4.2 Test CI mode flag parsing: --provider, --confirm, --no-tools
- [ ] 4.3 Test CI mode output format: verify structured output on stdout

## 5. Verification

- [ ] 5.1 Run `go test ./... -count=1 -race` — all pass
- [ ] 5.2 Verify no production code was changed
