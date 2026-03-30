package tui

import (
	"strings"

	"github.com/sahilm/fuzzy"
)

// updateFilePickerState checks the textarea content for an @ trigger and updates
// the file picker state. Called after every textarea update.
// Only triggers when @ is at a token boundary (start of line, after space/newline)
// to avoid false positives on email addresses and other @-containing text.
func (m *Model) updateFilePickerState() {
	if m.fileIdx == nil {
		m.filePickerOpen = false
		return
	}

	val := m.textarea.Value()

	// Find the last @ that is at a token boundary (preceded by space, newline, or BOL).
	atIdx := -1
	for i := len(val) - 1; i >= 0; i-- {
		if val[i] == '@' {
			if i == 0 || val[i-1] == ' ' || val[i-1] == '\n' || val[i-1] == '\t' {
				atIdx = i
				break
			}
			// @ in the middle of a word (e.g. user@example.com) — skip it
		}
	}
	if atIdx < 0 {
		m.filePickerOpen = false
		return
	}

	// Extract the text after the @
	afterAt := val[atIdx+1:]

	// If there's a space or newline after the @-text, this reference is completed — close picker.
	if strings.Contains(afterAt, " ") || strings.Contains(afterAt, "\n") {
		m.filePickerOpen = false
		return
	}

	m.filePickerOpen = true
	m.filePickerFilter = afterAt
	m.filePickerMatches = m.fileIdx.search(afterAt, 10)
	// Reset cursor if it's out of range
	if m.filePickerCursor >= len(m.filePickerMatches) {
		m.filePickerCursor = 0
	}
}

// acceptFilePick replaces the @filter text in the textarea with the selected
// file path and adds it to attachedFiles.
func (m *Model) acceptFilePick() {
	if len(m.filePickerMatches) == 0 {
		return
	}
	selected := m.filePickerMatches[m.filePickerCursor]

	// Find the token-boundary @ (same logic as updateFilePickerState)
	val := m.textarea.Value()
	atIdx := -1
	for i := len(val) - 1; i >= 0; i-- {
		if val[i] == '@' {
			if i == 0 || val[i-1] == ' ' || val[i-1] == '\n' || val[i-1] == '\t' {
				atIdx = i
				break
			}
		}
	}
	if atIdx < 0 {
		return
	}

	newVal := val[:atIdx] // text before @
	// Don't re-add the @ reference to the textarea — just remove the @filter
	// and add the file to the attached list.
	m.textarea.SetValue(strings.TrimRight(newVal, " "))

	// Add to attached files if not already present
	for _, f := range m.attachedFiles {
		if f == selected.Path {
			m.filePickerOpen = false
			return
		}
	}
	m.attachedFiles = append(m.attachedFiles, selected.Path)
	m.filePickerOpen = false
	m.filePickerCursor = 0
}

// closePalette resets all command palette state.
func (m *Model) closePalette() {
	m.paletteOpen = false
	m.paletteViaCtrlP = false
	m.paletteCursor = 0
	m.paletteFilter = ""
	m.paletteFiles = nil
}

// commandSearchSource adapts a slice of slashCommands for sahilm/fuzzy matching.
type commandSearchSource []slashCommand

func (s commandSearchSource) Len() int           { return len(s) }
func (s commandSearchSource) String(i int) string { return s[i].Name + " " + s[i].Description }

// filterPaletteCommands returns commands matching the given filter string
// using fuzzy matching. Subcommands are hidden until the filter includes
// their parent prefix.
func (m Model) filterPaletteCommands(filter string) []slashCommand {
	// Build candidate list, hiding subcommands when appropriate.
	// The filter has the leading "/" stripped, so normalize parent names the same way.
	lower := strings.ToLower(filter)
	var candidates []slashCommand
	for _, cmd := range m.slashCommands {
		if strings.Contains(cmd.Name, " ") {
			parent := cmd.Name[:strings.Index(cmd.Name, " ")]
			parent = strings.TrimPrefix(parent, "/") // normalize: filter is slashless
			if !strings.HasPrefix(lower, strings.ToLower(parent)) {
				continue
			}
		}
		candidates = append(candidates, cmd)
	}

	if filter == "" {
		return candidates
	}

	// Use fuzzy matching for score-based ranking.
	results := fuzzy.FindFrom(filter, commandSearchSource(candidates))
	matches := make([]slashCommand, len(results))
	for i, r := range results {
		matches[i] = candidates[r.Index]
	}
	return matches
}
