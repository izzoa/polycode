package tui

// UndoSnapshot represents a git snapshot taken before a mutating tool call.
type UndoSnapshot struct {
	Tag         string // git tag name used for this snapshot
	Description string // human-readable description (e.g., "file_write view.go")
}

// UndoSnapshotMsg pushes a new undo snapshot onto the stack.
type UndoSnapshotMsg struct {
	Snapshot UndoSnapshot
}

// UndoAppliedMsg is sent when an undo/redo operation completes.
type UndoAppliedMsg struct {
	Description string
	Error       error
	IsRedo      bool
}
