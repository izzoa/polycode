## Context

Polycode manages sessions with metadata and conversation history stored per-session. The session picker displays a flat list of sessions sorted by recency. There is no parent-child relationship between sessions. Conversation history is stored as a sequence of exchanges (user query + provider responses + consensus).

## Goals / Non-Goals

**Goals:**
- Fork a session at the current exchange index via `/branch`
- Copy conversation history up to the branch point into the new session
- Track parent session ID and branch point in session metadata
- Display branch hierarchy in the session picker (indented children)
- Branch sessions are fully independent post-fork

**Non-Goals:**
- Merging branches back together
- Diffing between branch and parent
- Visual branch graph (tree view beyond indentation)
- Branching from a specific past exchange (always branches from current position)
- Limiting branch depth

## Decisions

### 1. Branch metadata on session struct

**Decision**: Add `ParentSessionID string` and `BranchExchangeIndex int` fields to session metadata. Empty ParentSessionID means root session.

**Rationale**: Minimal metadata addition. Enough to reconstruct the tree for display and to copy history on fork.

### 2. Deep copy of history on branch

**Decision**: When `/branch` is executed, create a new session and deep-copy all exchanges from index 0 through the current exchange index. The branch is then fully independent.

**Rationale**: Copy-on-branch is simpler than shared history with COW semantics. Session sizes are small (conversation text), so copying is cheap.

### 3. Session picker shows indented hierarchy

**Decision**: Session picker groups branches under their parent. Children are indented with a tree connector character. Sort: parents by recency, children by branch time under their parent.

**Rationale**: Indentation provides visual hierarchy without requiring a complex tree widget. Consistent with file-tree patterns.

### 4. /branch command with optional name

**Decision**: `/branch` creates a branch with an auto-generated name (parent name + "branch N"). `/branch <name>` uses the given name.

**Rationale**: Quick branching should require zero arguments. Optional name for power users.

## Risks / Trade-offs

- **[Risk] Large conversation history makes copy slow** -> Mitigation: Conversations are text-only, typically <100KB. Copy is effectively instant.
- **[Risk] Deep branch nesting confuses picker** -> Mitigation: Indent up to 3 levels; deeper branches show flat with a depth indicator.
- **[Risk] Orphaned branches when parent deleted** -> Mitigation: Deleting a parent promotes its children to root level (clear ParentSessionID).
