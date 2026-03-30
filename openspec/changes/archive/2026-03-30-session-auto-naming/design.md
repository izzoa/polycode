## Context

Polycode tracks sessions with metadata stored in config. Session picker displays sessions by creation timestamp. There is no descriptive name field. The `/sessions` command exists for session management but has no `name` sub-command. The primary model is always available after the first exchange completes (it just processed the consensus synthesis).

## Goals / Non-Goals

**Goals:**
- Auto-generate a 3-5 word session name after the first exchange
- Display name in status bar and session picker
- Persist name in session metadata
- Allow user override via `/sessions name`
- Zero additional latency on the user's critical path (naming runs in background)

**Non-Goals:**
- Rename based on subsequent exchanges (only first exchange triggers naming)
- AI-powered name suggestions for user to pick from (fully automatic)
- Session name search/filter (future enhancement)
- Renaming via keyboard shortcut (slash command only)

## Decisions

### 1. Background naming request after QueryDoneMsg

**Decision**: After the first QueryDoneMsg, fire a background goroutine that sends a short prompt to the primary model: "Summarize this conversation topic in 3-5 words. Reply with only the name." The result arrives as a `SessionNameMsg`.

**Rationale**: Running in the background ensures zero latency impact on the user. Using the primary model reuses existing auth. The prompt is cheap (~50 tokens).

### 2. Name stored as field in session metadata

**Decision**: Add a `Name string` field to the session metadata struct. Empty string means unnamed. The auto-generated name and user-override both write to this field.

**Rationale**: Single source of truth. Session picker reads this field; falls back to timestamp display when empty.

### 3. User override via /sessions name

**Decision**: Add `/sessions name <text>` sub-command. Setting a name marks the session as user-named (a `UserNamed bool` flag), which prevents auto-naming from overwriting it.

**Rationale**: Simple slash command pattern consistent with existing `/sessions` sub-commands. The flag prevents the auto-namer from clobbering intentional names.

## Risks / Trade-offs

- **[Risk] Primary model returns verbose name** -> Mitigation: Truncate to 40 characters; strip quotes and punctuation.
- **[Risk] Naming request fails** -> Mitigation: Silently fall back to no name; session remains timestamp-identified.
- **[Risk] Race between auto-name and user rename** -> Mitigation: UserNamed flag takes precedence; auto-name skips if flag is set.
