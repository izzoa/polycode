## Context

The CLI setup wizard collects provider configuration sequentially. For `openai_compatible`, the current step order is: type → name → auth → API key → model → **base URL**. Because base URL comes after model selection, we can't query the server for available models. The OpenAI `/v1/models` endpoint (`GET /models`) is part of the OpenAI API spec and is implemented by Ollama, vLLM, LM Studio, OpenRouter, Together AI, and most compatible servers.

The litellm metadata JSON contains entries for many models served by these providers (e.g., `openrouter/anthropic/claude-sonnet-4`, `ollama/llama3`), but the keys use varied prefixes that don't map cleanly to a single `openai_compatible` matcher. Cross-referencing discovered model IDs against litellm provides capability data without needing a complete prefix mapping.

## Goals / Non-Goals

**Goals:**
- Query the user's configured endpoint to discover available models during wizard setup
- Show discovered models in a filterable selectable list (same UX as other provider types)
- Enrich discovered models with litellm capability metadata when a match is found
- Fall back gracefully to text input when discovery fails

**Non-Goals:**
- Caching discovered models beyond the current wizard session
- Supporting non-standard model listing endpoints (only `/v1/models`)
- Changing model discovery for `anthropic`, `openai`, or `google` provider types (they use litellm)
- Automatically detecting the provider type from a base URL

## Decisions

### 1. Reorder wizard steps: collect base URL before model selection

**Decision**: For `openai_compatible` only, move the base URL prompt to immediately after auth/API key, before model selection. Other provider types keep their current order.

**Rationale**: We need the base URL and credentials to query `/models`. The natural flow becomes: type → name → **base URL** → auth → API key → **discover models** → model selection. Base URL before auth makes sense because the user needs to know where they're pointing before deciding on auth method.

**Alternatives considered**:
- Collect base URL first (before name): Awkward — user typically thinks "I want to add Ollama" (name) then provides the URL.
- Add a separate "discover models" step after everything: Would require going back to change the model, more complex flow.

### 2. Query `{base_url}/models` with a short timeout

**Decision**: Issue `GET {base_url}/models` with a 10-second timeout. Include the Bearer API key if auth is `api_key`. Parse the standard OpenAI response format (`{ "data": [{ "id": "model-name", ... }] }`).

**Rationale**: 10 seconds is long enough for remote endpoints (OpenRouter, Together) but short enough to not block the wizard if the server is unreachable. The response format is standardized across all OpenAI-compatible implementations.

**Note**: Some servers use `{base_url}/v1/models` while others serve it at `{base_url}/models`. We should try the path as-is first (since base_url may already include `/v1`), and if that fails with 404, try appending `/v1/models`.

### 3. Cross-reference discovered models with litellm via fuzzy matching

**Decision**: For each discovered model ID, attempt to find a matching litellm entry using the existing `MetadataStore.Lookup()` method (which already does exact, prefixed, and suffix matching). If found, attach the capability metadata (context window, tools, vision, reasoning).

**Rationale**: Many models on OpenAI-compatible endpoints have direct litellm entries (e.g., `llama3` matches litellm's data). The existing `Lookup()` method handles prefix variations. Models without a litellm match are still shown — just without capability annotations.

### 4. Merge discovered + litellm models with deduplication

**Decision**: Show only models discovered from the endpoint (not all litellm models). Enrich them with litellm data where available. This avoids showing models the server doesn't actually serve.

**Rationale**: The `/models` response is the ground truth for what the endpoint offers. Showing litellm models not available on the server would be misleading.

## Risks / Trade-offs

- **Endpoint may require auth to list models**: Some providers (OpenRouter) require an API key for `/models`. → Mitigated by collecting auth/API key before discovery. If auth is `none` and the endpoint requires it, discovery fails gracefully to text input.
- **Non-standard `/models` path**: Some servers might not support it or use a different path. → Mitigated by graceful fallback to text input + trying both `{base_url}/models` and `{base_url}/v1/models`.
- **Large model lists**: OpenRouter lists 200+ models. → Mitigated by the filterable `huh.Select` with `Height(15)` and type-to-filter. Could also limit to top N if needed.
- **Slow endpoint**: Discovery adds a network round-trip to the wizard. → Mitigated by 10s timeout and showing a spinner/message during the request.
