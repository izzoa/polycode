## Context

The project has strong user-facing docs (README, CHANGELOG) and internal dev docs (CLAUDE.md). What's missing is contributor-facing documentation that helps new developers understand the project structure, make their first PR, and navigate the OpenSpec change workflow.

## Goals / Non-Goals

**Goals:**
- `CONTRIBUTING.md` that covers: prerequisites, building, testing, PR checklist, OpenSpec workflow, code conventions
- Godoc comments on all exported types and functions in core packages
- Package-level doc comments explaining each package's role

**Non-Goals:**
- API reference documentation (godoc is sufficient)
- User tutorials or guides (README covers this)
- Architecture Decision Records (ADRs) — the OpenSpec archive serves this role
- Exhaustive inline code comments

## Decisions

### 1. CONTRIBUTING.md structure

**Decision**: Follow the standard open-source CONTRIBUTING.md format:
1. Development Setup (Go version, `go build`, `go test`)
2. Project Structure (brief overview referencing CLAUDE.md)
3. Making Changes (branch, commit conventions, PR template)
4. Testing (how to run, what to test, race detector)
5. OpenSpec Workflow (how to propose, design, implement, archive changes)
6. Code Conventions (error wrapping, mutex patterns, provider interface)

**Rationale**: Standard format is immediately recognizable. References CLAUDE.md for deep details to avoid duplication.

### 2. Godoc scope: core packages only

**Decision**: Add godoc comments to exported types in 8 core packages: consensus, provider, tokens, auth, mcp, action, tui, config. Skip internal helpers and test utilities.

**Rationale**: These are the packages a contributor is most likely to work with. Diminishing returns beyond this set.

### 3. Package-level doc comments

**Decision**: Add a `doc.go` or top-of-file comment to each core package with a 2-3 sentence description of its responsibility and key types.

**Rationale**: `go doc ./internal/consensus` should return a useful summary without reading the code.

## Risks / Trade-offs

- **[Risk] Documentation drifts from code** — Mitigation: Keep docs minimal and reference code. CLAUDE.md is already maintained.
- **[Risk] Over-documenting trivial types** — Mitigation: Only document exported types that aren't self-explanatory from their name and method set.
