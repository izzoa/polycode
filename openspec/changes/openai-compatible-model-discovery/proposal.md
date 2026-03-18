## Why

When configuring an `openai_compatible` provider, the wizard currently falls back to a plain text input for the model name because litellm has no standard key prefix for these endpoints. Users must know their model name in advance. Meanwhile, every OpenAI-compatible server (Ollama, vLLM, LM Studio, OpenRouter, Together AI) implements the standard `GET /v1/models` endpoint that returns the list of available models. We should query it to pre-populate the model selector, optionally cross-referencing litellm metadata to show capabilities for known models.

## What Changes

- **Reorder the CLI wizard for `openai_compatible`**: Collect base URL and auth/API key *before* model selection (currently base URL comes after)
- **Query `/v1/models` on the remote endpoint**: After the user provides base URL + credentials, issue a `GET {base_url}/models` request to discover available models
- **Cross-reference with litellm metadata**: For each discovered model, attempt to match it against litellm entries to attach capability info (context window, tools, vision, reasoning)
- **Show a selectable model list**: Present discovered models in the same `huh.Select` filterable list used by other provider types, with a "Custom model..." fallback
- **Graceful degradation**: If the `/models` query fails (auth issue, endpoint not supported, timeout), fall back to the existing text input with hint text

## Capabilities

### New Capabilities
- `endpoint-model-discovery`: Query an OpenAI-compatible `/v1/models` endpoint to discover available models at a given base URL, with litellm cross-referencing for capability metadata

### Modified Capabilities
<!-- None — the existing wizard-selectable-inputs behavior is unchanged for other provider types -->

## Impact

- **Code**: `cmd/polycode/setup.go` — wizard step reordering for `openai_compatible` flow
- **Code**: `internal/tokens/metadata.go` or new file — model discovery HTTP call + litellm cross-referencing logic
- **UX**: `openai_compatible` providers go from "type your model name" to "pick from what's available on the server"
- **Network**: New outbound HTTP call to the user's configured base URL during wizard setup (with short timeout)
- **No breaking changes**: Other provider types, config format, and existing `openai_compatible` configs are unaffected
