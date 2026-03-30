## Context

`summarizeConversation()` in `cmd/polycode/app.go` already implements context compaction. It takes a message slice and produces a compressed version by asking the primary model to summarize early turns into a dense paragraph, keeping only the most recent exchanges verbatim. This is triggered automatically when context usage exceeds 80% (inside the `SetSubmitHandler` closure). The `/compact` entry exists in the command palette (`model.go:536`) but the handler in `update.go:1769` returns a stub message.

## Goals / Non-Goals

**Goals:**
- Wire `/compact` to call `summarizeConversation()` on the current conversation
- Show before/after token count delta as a toast notification
- Allow manual compaction at any time (not just at 80% threshold)

**Non-Goals:**
- Changing the summarization algorithm
- Adding compaction settings (threshold, aggressiveness)
- Auto-compacting on `/compact` without user seeing the result

## Decisions

### 1. New handler pattern: SetCompactHandler

**Decision**: Add `model.SetCompactHandler(func())` following the existing handler pattern. The handler closure in `app.go` calls `summarizeConversation()` on the conversation snapshot, replaces the conversation state, and sends a `TokenUpdateMsg` plus a toast showing "Compacted: N → M messages".

**Rationale**: Consistent with all other slash command handlers. Keeps TUI and pipeline logic separated.

### 2. Token snapshot before and after

**Decision**: Capture `tracker.Summary()` before compaction, run compaction, then capture again. Show the delta in a toast.

**Rationale**: Users want confirmation that compaction actually freed context. The token tracker already provides per-provider snapshots.

## Risks / Trade-offs

- **[Risk] Compaction during active query** — Mitigation: Only allow `/compact` when not querying (same guard as `/clear`).
- **[Risk] Summarization loses important context** — This is inherent to compaction. The toast showing message count delta makes the trade-off visible.
