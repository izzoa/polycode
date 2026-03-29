## 1. Session Metadata Extensions

- [ ] 1.1 Add `ParentSessionID string` and `BranchExchangeIndex int` fields to session metadata in `internal/config/session.go`
- [ ] 1.2 Ensure new fields persist to/from session metadata file
- [ ] 1.3 Add `Branches() []Session` helper that queries all sessions with matching ParentSessionID
- [ ] 1.4 Add `IsRoot() bool` helper (ParentSessionID is empty)

## 2. Branch Creation Logic

- [ ] 2.1 Add `/branch` and `/branch <name>` slash command parsing in updateChat
- [ ] 2.2 Implement `forkSession(parentID string, exchangeIndex int, name string)` in session management
- [ ] 2.3 Deep-copy exchanges 0..N from parent to new session
- [ ] 2.4 Set ParentSessionID and BranchExchangeIndex on new session
- [ ] 2.5 Auto-generate branch name if not provided (parent name + "branch N")
- [ ] 2.6 Switch active session to the new branch after creation

## 3. Session Picker Hierarchy

- [ ] 3.1 Build tree structure from flat session list: group children under parents
- [ ] 3.2 Render indented entries with tree connector characters in session picker
- [ ] 3.3 Cap visual indentation at 3 levels with depth indicator for deeper branches
- [ ] 3.4 Show branch count on parent entries
- [ ] 3.5 Sort: root sessions by recency, branches by creation time under parent

## 4. Parent Deletion Handling

- [ ] 4.1 When deleting a session, find all children and clear their ParentSessionID
- [ ] 4.2 Promote orphaned branches to root level

## 5. Verification

- [ ] 5.1 Run `go build ./...` and `go test ./...` -- all pass
- [ ] 5.2 Manual: create a branch, verify history is copied, new exchanges are independent
- [ ] 5.3 Manual: verify session picker hierarchy rendering with 2-3 levels of branches
- [ ] 5.4 Manual: delete parent, verify children promoted to root
