## 1. Model Discovery Function

- [x] 1.1 Add `DiscoverModels(baseURL, apiKey string) ([]string, error)` to `internal/tokens/metadata.go` (or a new `discovery.go` file in the same package) — issues `GET {baseURL}/models`, parses `{ "data": [{ "id": "..." }] }`, returns sorted model IDs
- [x] 1.2 Handle path fallback: if `{baseURL}/models` returns 404 and baseURL doesn't end with `/v1`, retry with `{baseURL}/v1/models`
- [x] 1.3 Set 10-second timeout on the HTTP request; include `Authorization: Bearer {key}` header when apiKey is non-empty
- [x] 1.4 Add `EnrichWithMetadata(modelIDs []string, store *MetadataStore) []config.ModelSummary` — cross-references each ID against litellm via `store.Lookup()` to attach capability info

## 2. Reorder Wizard Steps for openai_compatible

- [x] 2.1 In `cmd/polycode/setup.go`, move the base URL prompt to immediately after provider name for `openai_compatible` (before auth method selection)
- [x] 2.2 Move auth method + API key prompts to after base URL (unchanged position relative to each other)
- [x] 2.3 Verify that model selection now has access to baseURL, authMethod, and apiKey before it runs

## 3. Integrate Discovery into Model Selection

- [x] 3.1 In `selectModel()`, when providerType is `openai_compatible` and baseURL is non-empty, call `DiscoverModels(baseURL, apiKey)` before falling back to text input
- [x] 3.2 If discovery succeeds, pass results through `EnrichWithMetadata()` and display in `huh.Select` with `Filtering(true)` + "Custom model..." option (same pattern as other provider types)
- [x] 3.3 Show a brief "Discovering models..." message (fmt.Print) before the HTTP call so the user knows what's happening
- [x] 3.4 If discovery fails, print a short warning (e.g., "Could not discover models: <reason>") and fall back to text input with hint

## 4. Tests

- [x] 4.1 Add unit tests for `DiscoverModels` — mock HTTP server returning standard `/models` response, empty response, 404-then-retry, timeout, and auth header inclusion
- [x] 4.2 Add unit tests for `EnrichWithMetadata` — model IDs with and without litellm matches
- [x] 4.3 Verify `go build ./...` compiles cleanly
- [x] 4.4 Verify `go test ./... -count=1 -race` passes

## 5. Manual Verification

- [ ] 5.1 Test with a local Ollama instance (`http://localhost:11434/v1`) — verify models are discovered and displayed *(deferred: requires running Ollama instance)*
- [ ] 5.2 Test with an unreachable URL — verify graceful fallback to text input *(deferred: beta validation)*
- [ ] 5.3 Test with an endpoint requiring auth (e.g., OpenRouter) — verify API key is sent and models are listed *(deferred: requires API key)*
