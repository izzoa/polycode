## 1. CONTRIBUTING.md

- [ ] 1.1 Create `CONTRIBUTING.md` with development setup section (Go version, build, test commands)
- [ ] 1.2 Add project structure overview (reference CLAUDE.md for details)
- [ ] 1.3 Add making changes section (branch conventions, commit format, PR checklist)
- [ ] 1.4 Add testing guidelines (race detector, mock providers, test patterns)
- [ ] 1.5 Add OpenSpec workflow section (propose → design → spec → implement → archive)
- [ ] 1.6 Add code conventions section (error wrapping, mutex, provider interface)

## 2. Godoc comments

- [ ] 2.1 Add package-level doc comments to `internal/consensus`, `internal/provider`, `internal/tokens`
- [ ] 2.2 Add package-level doc comments to `internal/auth`, `internal/mcp`, `internal/action`
- [ ] 2.3 Add package-level doc comments to `internal/tui`, `internal/config`
- [ ] 2.4 Add godoc to exported types in `consensus` package (Pipeline, Engine, SynthesisMode)
- [ ] 2.5 Add godoc to exported types in `provider` package (Provider, StreamChunk, QueryOpts)
- [ ] 2.6 Add godoc to exported types in `tokens` package (TokenTracker, MetadataStore, Usage)

## 3. Verification

- [ ] 3.1 Run `go build ./...` — verify doc comments don't break compilation
- [ ] 3.2 Run `go doc ./internal/consensus` and verify useful output
- [ ] 3.3 Review CONTRIBUTING.md for accuracy against current workflow
