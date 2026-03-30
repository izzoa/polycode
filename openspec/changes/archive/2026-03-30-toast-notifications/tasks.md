## 1. Toast Core

- [x] 1.1 Create `internal/tui/toast.go` with Toast struct (ID, Variant, Text, CreatedAt), ToastVariant enum (Info, Success, Warning, Error)
- [x] 1.2 Add ToastMsg and ToastDismissMsg message types
- [x] 1.3 Add `toasts []Toast` and `nextToastID int` fields to Model
- [x] 1.4 Implement toast creation in Update: on ToastMsg, append toast, cap at 3, return tick command
- [x] 1.5 Implement toast dismissal in Update: on ToastDismissMsg, remove matching toast by ID

## 2. Toast Rendering

- [x] 2.1 Implement `renderToasts()` function: stack layout, bottom-right positioning, variant-colored borders
- [x] 2.2 Integrate renderToasts() into View() as final overlay pass
- [x] 2.3 Verify toasts do not interfere with input focus or viewport scrolling

## 3. Event Wiring

- [x] 3.1 Emit Success ToastMsg on config save (in ConfigChangedMsg handler)
- [x] 3.2 Emit Info ToastMsg on MCP server reconnect (in MCPStatusMsg handler)
- [x] 3.3 Emit Success ToastMsg on clipboard copy
- [x] 3.4 Emit Error ToastMsg on provider/consensus errors (in QueryDoneMsg handler)

## 4. Verification

- [x] 4.1 Run `go build ./...` and `go test ./...` -- all pass
- [x] 4.2 Manual: trigger each toast variant and verify styling, positioning, auto-dismiss
- [x] 4.3 Manual: trigger 4+ rapid events and verify oldest-eviction behavior
