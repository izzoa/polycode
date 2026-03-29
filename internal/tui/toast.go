package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToastVariant determines the visual style of a toast notification.
type ToastVariant int

const (
	ToastInfo ToastVariant = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// Toast represents a single transient notification.
type Toast struct {
	ID        int
	Variant   ToastVariant
	Text      string
	CreatedAt time.Time
}

// ToastMsg triggers a new toast notification.
type ToastMsg struct {
	Variant ToastVariant
	Text    string
}

// toastDismissMsg dismisses a specific toast by ID.
type toastDismissMsg struct {
	ID int
}

// toastDuration returns the auto-dismiss duration for a variant.
func toastDuration(v ToastVariant) time.Duration {
	if v == ToastError {
		return 5 * time.Second
	}
	return 3 * time.Second
}

// addToast adds a toast to the model's stack, caps at 3, and returns a dismiss tick.
func (m *Model) addToast(variant ToastVariant, text string) tea.Cmd {
	m.nextToastID++
	id := m.nextToastID
	t := Toast{
		ID:        id,
		Variant:   variant,
		Text:      text,
		CreatedAt: time.Now(),
	}

	m.toasts = append(m.toasts, t)
	// Cap at 3 — evict oldest
	if len(m.toasts) > 3 {
		m.toasts = m.toasts[len(m.toasts)-3:]
	}

	dur := toastDuration(variant)
	return tea.Tick(dur, func(_ time.Time) tea.Msg {
		return toastDismissMsg{ID: id}
	})
}

// dismissToast removes a toast by ID.
func (m *Model) dismissToast(id int) {
	for i, t := range m.toasts {
		if t.ID == id {
			m.toasts = append(m.toasts[:i], m.toasts[i+1:]...)
			return
		}
	}
}

// renderToasts renders the toast stack as a bottom-right anchored overlay.
func (m Model) renderToasts() string {
	if len(m.toasts) == 0 {
		return ""
	}

	var rendered []string
	for _, t := range m.toasts {
		var borderColor lipgloss.Color
		var icon string
		switch t.Variant {
		case ToastSuccess:
			borderColor = m.theme.Success
			icon = "✓"
		case ToastWarning:
			borderColor = m.theme.Warning
			icon = "⚠"
		case ToastError:
			borderColor = m.theme.Error
			icon = "✕"
		default: // Info
			borderColor = m.theme.Info
			icon = "ℹ"
		}

		// Clamp toast width to terminal
		toastW := 40
		if m.width < 44 {
			toastW = m.width - 4
			if toastW < 10 {
				toastW = 10
			}
		}

		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			BorderLeft(true).
			BorderRight(false).
			BorderTop(false).
			BorderBottom(false).
			Foreground(m.theme.Text).
			Padding(0, 1).
			Width(toastW)

		content := fmt.Sprintf("%s %s", icon, t.Text)
		rendered = append(rendered, style.Render(content))
	}

	stack := strings.Join(rendered, "\n")

	// Position bottom-right
	toastWidth := 42
	if m.width < 44 {
		toastWidth = m.width - 2
	}
	leftPad := m.width - toastWidth - 2
	if leftPad < 0 {
		leftPad = 0
	}

	return lipgloss.NewStyle().PaddingLeft(leftPad).Render(stack)
}
