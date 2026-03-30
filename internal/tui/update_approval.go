package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// handleConfirmKey handles keyboard input during the approval prompt.
// Called from Update() when m.confirmPending is true.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Edit sub-mode: textarea captures all input
	if m.confirmEditing {
		switch msg.String() {
		case "ctrl+c":
			m.confirmEditing = false
			if m.confirmResponseCh != nil {
				m.confirmResponseCh <- ConfirmResult{Approved: false}
			}
			m.confirmPending = false
			return m, tea.Quit
		case "ctrl+s":
			// Submit edited content
			edited := m.confirmEditTextarea.Value()
			if m.confirmResponseCh != nil {
				m.confirmResponseCh <- ConfirmResult{Approved: true, EditedContent: &edited}
			}
			m.confirmEditing = false
			m.confirmPending = false
			m.confirmDescription = ""
			return m, nil
		case "esc":
			// Exit edit mode, return to normal confirm
			m.confirmEditing = false
			return m, nil
		default:
			// Forward to textarea
			var cmd tea.Cmd
			m.confirmEditTextarea, cmd = m.confirmEditTextarea.Update(msg)
			return m, cmd
		}
	}

	// Normal confirmation mode
	switch msg.String() {
	case "ctrl+c":
		if m.confirmResponseCh != nil {
			m.confirmResponseCh <- ConfirmResult{Approved: false}
		}
		m.confirmPending = false
		return m, tea.Quit
	case "y", "Y":
		if m.confirmResponseCh != nil {
			m.confirmResponseCh <- ConfirmResult{Approved: true}
		}
		m.confirmPending = false
		m.confirmDescription = ""
		return m, nil
	case "e", "E":
		// Enter edit mode
		if m.confirmEditContent != "" {
			ta := textarea.New()
			ta.SetValue(m.confirmEditContent)
			ta.Focus()
			ta.CharLimit = 0
			ta.ShowLineNumbers = false
			maxH := m.height * 60 / 100
			if maxH < 5 {
				maxH = 5
			}
			ta.SetHeight(maxH)
			ta.SetWidth(m.width - 6)
			m.confirmEditTextarea = ta
			m.confirmEditing = true
		}
		return m, nil
	case "a", "A":
		// Allow for session — approve and remember (blocked for destructive tools)
		if m.confirmRiskLevel == "destructive" {
			m.chatStatusMsg = "Cannot allow destructive tools for session — approve individually"
			return m, nil
		}
		if m.confirmResponseCh != nil {
			m.confirmResponseCh <- ConfirmResult{Approved: true}
		}
		if m.confirmToolName != "" {
			if m.sessionAllowed == nil {
				m.sessionAllowed = make(map[string]bool)
			}
			m.sessionAllowed[m.confirmToolName] = true
			m.chatStatusMsg = fmt.Sprintf("Allowed %s for this session", m.confirmToolName)
		}
		m.confirmPending = false
		m.confirmDescription = ""
		return m, nil
	case "n", "N", "esc":
		if m.confirmResponseCh != nil {
			m.confirmResponseCh <- ConfirmResult{Approved: false}
		}
		m.confirmPending = false
		m.confirmDescription = ""
		return m, nil
	}
	return m, nil // swallow all other keys during confirm
}
