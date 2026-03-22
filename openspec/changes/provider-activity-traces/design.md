## Context

Provider tabs are currently fed only by `ProviderChunkMsg` events from the fan-out callback in `cmd/polycode/app.go`. That means the provider panels accumulate the initial model response text, get marked done when fan-out ends, and are then persisted as the exchange's only per-provider detail. The primary provider's later work, including synthesis, tool execution, follow-up generations, and verification, is streamed only to the consensus panel.

This creates three problems. First, the UI contract is misleading: the README promises provider visibility, but the tabs stop at fan-out. Second, the primary provider's tab status is wrong because it is marked complete before that provider finishes its real work. Third, session persistence and export only preserve `Individual` fan-out responses, so replay data loses the majority of the primary provider's activity.

## Goals / Non-Goals

**Goals:**
- Make each provider tab represent the full observable work that provider performed during a turn
- Mirror the primary provider's synthesis, tool-loop, and verification activity into its tab while preserving the consensus panel
- Make provider completion status reflect the end of that provider's full trace, not just fan-out
- Persist provider traces in session history and exports with backward-compatible fallback to legacy `Individual` summaries

**Non-Goals:**
- Running write-capable tool loops for every provider in fan-out
- Changing the consensus panel into a provider-specific view
- Altering tool approval, execution order, or verification semantics
- Removing the legacy `Individual` summary field in this change

## Decisions

### 1. Introduce a phase-aware provider trace event model

**Decision**: Replace the fan-out-only provider update path with a provider trace message that carries `ProviderName`, `Phase`, `Delta`, `Done`, `Error`, and optional status semantics. The phase set is `fanout`, `synthesis`, `tool`, and `verify`.

**Rationale**: The current `ProviderChunkMsg` shape cannot distinguish fan-out output from later primary-provider phases, so the TUI cannot render accurate phase boundaries or completion state. A phase-aware message lets the app stream all provider-visible activity through one channel while keeping consensus rendering independent.

**Alternative considered**: Keep `ProviderChunkMsg` and overload `Delta` with ad hoc headers. Rejected because the TUI would still be unable to reason about phase transitions, completion timing, or persistence structure.

### 2. Provider tabs show everything a provider actually participated in, not simulated work

**Decision**: Non-primary providers continue to show only fan-out activity because that is all they do today. The primary provider tab additionally receives synthesis chunks, tool-loop status/output/follow-up chunks, and verification messages.

**Rationale**: This matches the intended UX without multiplying side effects. The primary model is the only model that synthesizes and executes tools in the current architecture, so its tab must reflect those phases. Secondary providers should not show fabricated phases they never ran.

**Alternative considered**: Run a tool loop for every provider so each tab has a fully independent agent execution trace. Rejected because it would multiply cost, approvals, and workspace side effects, and could create conflicting writes.

### 3. Persist structured provider trace sections per exchange

**Decision**: Extend `SessionExchange` with a new provider-trace field that stores structured sections per provider, where each section has a `Phase` and accumulated `Content`. Keep `Individual map[string]string` as a backward-compatible summary field and fallback for older sessions.

**Rationale**: Persisting a single assembled string per provider would lose phase boundaries that the UI and export formats need. Persisting raw chunk-level events would inflate session files unnecessarily. Section-level persistence keeps the important structure while remaining compact.

**Alternative considered**: Replace `Individual` entirely with a provider trace map. Rejected because it would complicate compatibility with existing saved sessions and export code in one step.

### 4. Use one aggregation path for live TUI state and persisted traces

**Decision**: The TUI panel state and the session persistence path should share the same conceptual trace structure: phase-ordered sections with appended content. The TUI can maintain richer in-memory panel state, but the app should serialize from the same accumulated trace source rather than reconstructing traces from unrelated buffers.

**Rationale**: Today the live provider tab state and the saved exchange data are populated from different sources, which is why they diverge. Using the same trace model for both avoids drift between what the user saw live and what gets exported later.

**Alternative considered**: Continue saving `fanOutResult.Responses` and only improve the live provider tab. Rejected because session exports and restores would still discard most of the primary provider's work.

### 5. Provider completion is app-controlled, not inferred from fan-out

**Decision**: The app layer will explicitly mark each provider trace done only when that provider's final phase ends. Non-primary providers are done at the end of fan-out; the primary provider is done only after synthesis and any optional tool/verification phases complete or fail.

**Rationale**: The TUI cannot infer whether more provider work is coming from fan-out events alone. Completion belongs in the orchestration layer because it knows whether synthesis, tool execution, and verification will follow.

**Alternative considered**: Infer primary completion from missing future chunks. Rejected because it is race-prone and tightly couples TUI behavior to implicit stream timing.

## Risks / Trade-offs

- **Larger session files** → Mitigation: persist section-level traces instead of raw chunk logs
- **More orchestration complexity in `app.go`** → Mitigation: keep phase mapping centralized in helper functions instead of scattering tab updates across the submit handler
- **Consensus and primary tabs could drift visually** → Mitigation: mirror the same emitted strings into both outputs where phases overlap
- **Backward-compatibility pressure on export and session load** → Mitigation: retain `Individual` and make new trace fields additive and optional

## Migration Plan

1. Add the new provider-trace session fields as optional JSON members
2. Update save paths to write both the new provider traces and the legacy `Individual` summaries
3. Update load and export paths to prefer provider traces when present and fall back to legacy summaries otherwise
4. Verify older session files still load without migration scripts

## Open Questions

None. This change intentionally stops at representing actual provider participation; per-provider tool execution remains a separate decision.
