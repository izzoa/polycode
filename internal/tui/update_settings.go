package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/auth"
)

// updateSettings handles key events when in settings mode.
func (m Model) updateSettings(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// If confirming delete, only handle y/n
	if m.confirmDelete {
		switch key {
		case "y":
			m.confirmDelete = false
			return m.deleteSelectedProvider()
		case "n", "esc":
			m.confirmDelete = false
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	if m.mcpConfirmDelete {
		switch key {
		case "y":
			m.mcpConfirmDelete = false
			return m.deleteSelectedMCPServer()
		case "n", "esc":
			m.mcpConfirmDelete = false
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	providerCount := 0
	mcpCount := 0
	if m.cfg != nil {
		providerCount = len(m.cfg.Providers)
		mcpCount = len(m.cfg.MCP.Servers)
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = viewChat
		m.settingsMsg = ""
		m.mcpSettingsFocused = false
		m.textarea.Focus()
		return m, nil
	case "tab":
		// Toggle focus between provider and MCP sections
		m.mcpSettingsFocused = !m.mcpSettingsFocused
		m.settingsMsg = ""
		return m, nil
	case "j", "down":
		m.settingsMsg = ""
		if m.mcpSettingsFocused {
			if m.mcpSettingsCursor < mcpCount-1 {
				m.mcpSettingsCursor++
			}
		} else {
			if m.settingsCursor < providerCount-1 {
				m.settingsCursor++
			}
		}
		return m, nil
	case "k", "up":
		m.settingsMsg = ""
		if m.mcpSettingsFocused {
			if m.mcpSettingsCursor > 0 {
				m.mcpSettingsCursor--
			}
		} else {
			if m.settingsCursor > 0 {
				m.settingsCursor--
			}
		}
		return m, nil
	case "a":
		if m.mcpSettingsFocused {
			m.initMCPWizardForAdd()
		} else {
			m.initWizardForAdd()
		}
		return m, nil
	case "e":
		if m.mcpSettingsFocused {
			if mcpCount > 0 {
				m.initMCPWizardForEdit(m.mcpSettingsCursor)
			}
		} else {
			if providerCount > 0 {
				m.initWizardForEdit(m.settingsCursor)
			}
		}
		return m, nil
	case "x":
		// Toggle disable/enable on selected provider
		if !m.mcpSettingsFocused && providerCount > 0 && m.cfg != nil {
			p := &m.cfg.Providers[m.settingsCursor]
			if p.Primary {
				m.settingsMsg = m.styles.StatusUnhealthy.Render(
					"Cannot disable the primary provider.")
				return m, nil
			}
			p.Disabled = !p.Disabled
			_ = m.cfg.Save()
			if p.Disabled {
				m.settingsMsg = m.styles.Dimmed.Render(p.Name + " disabled")
			} else {
				m.settingsMsg = m.styles.StatusHealthy.Render(p.Name + " enabled")
			}
			if m.onConfigChanged != nil {
				m.onConfigChanged(m.cfg)
			}
			return m, nil
		}
	case "d":
		if m.mcpSettingsFocused {
			if mcpCount > 0 {
				m.mcpConfirmDelete = true
			}
		} else {
			if providerCount > 0 {
				if m.cfg.Providers[m.settingsCursor].Primary {
					m.settingsMsg = m.styles.StatusUnhealthy.Render(
						"Cannot remove the primary provider. Change primary first.")
					return m, nil
				}
				m.confirmDelete = true
			}
		}
		return m, nil
	case "t":
		if m.mcpSettingsFocused {
			if mcpCount > 0 {
				cfg := m.cfg.MCP.Servers[m.mcpSettingsCursor]
				m.mcpTestingServer = cfg.Name
				m.settingsMsg = ""
				if m.onTestMCP != nil {
					m.onTestMCP(cfg)
				}
				return m, m.spinner.Tick
			}
		} else {
			if providerCount > 0 {
				name := m.cfg.Providers[m.settingsCursor].Name
				m.testingProvider = name
				m.settingsMsg = ""
				if m.onTestProvider != nil {
					m.onTestProvider(name)
				}
				return m, m.spinner.Tick
			}
		}
		return m, nil
	}

	return m, nil
}

// updateMCPDashboard handles key events when the MCP dashboard is open.
func (m Model) updateMCPDashboard(msg tea.KeyMsg) (Model, tea.Cmd) {
	serverCount := len(m.mcpDashboardData)

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "m":
		m.showMCPDashboard = false
		m.textarea.Focus()
		return m, nil
	case "/":
		// Close dashboard and pass `/` to textarea for slash commands.
		m.showMCPDashboard = false
		m.textarea.SetValue("/")
		m.textarea.Focus()
		return m, nil
	case "j", "down":
		if m.mcpDashboardCursor < serverCount-1 {
			m.mcpDashboardCursor++
		}
		return m, nil
	case "k", "up":
		if m.mcpDashboardCursor > 0 {
			m.mcpDashboardCursor--
		}
		return m, nil
	case "r":
		if serverCount > 0 && m.onReconnectMCP != nil {
			name := m.mcpDashboardData[m.mcpDashboardCursor].Name
			m.onReconnectMCP(name)
			// Refresh dashboard after reconnect.
			if m.onMCPDashboardRefresh != nil {
				m.onMCPDashboardRefresh()
			}
		}
		return m, m.spinner.Tick
	case "t":
		if serverCount > 0 && m.onTestMCP != nil && m.cfg != nil {
			name := m.mcpDashboardData[m.mcpDashboardCursor].Name
			for _, s := range m.cfg.MCP.Servers {
				if s.Name == name {
					m.onTestMCP(s)
					break
				}
			}
		}
		return m, m.spinner.Tick
	}
	return m, nil
}

// deleteSelectedProvider removes the currently selected provider from the
// config, deletes its credentials, saves, and sends a ConfigChangedMsg.
func (m Model) deleteSelectedProvider() (Model, tea.Cmd) {
	if m.cfg == nil || m.settingsCursor >= len(m.cfg.Providers) {
		return m, nil
	}

	name := m.cfg.Providers[m.settingsCursor].Name

	// Remove from config
	m.cfg.Providers = append(
		m.cfg.Providers[:m.settingsCursor],
		m.cfg.Providers[m.settingsCursor+1:]...,
	)

	// Delete stored credentials
	store := auth.NewStore()
	_ = store.Delete(name)

	// Save config
	_ = m.cfg.Save()

	// Adjust cursor
	if m.settingsCursor >= len(m.cfg.Providers) && m.settingsCursor > 0 {
		m.settingsCursor--
	}

	m.settingsMsg = m.styles.StatusHealthy.Render("Provider '" + name + "' removed")

	// Rebuild panels
	m.rebuildPanelsFromConfig()

	// Notify app layer
	if m.onConfigChanged != nil {
		m.onConfigChanged(m.cfg)
	}

	return m, func() tea.Msg {
		return ConfigChangedMsg{Config: m.cfg}
	}
}

// updateSessionPicker handles key events when the session picker is open.
func (m Model) updateSessionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	sessions := m.filteredSessions()

	// Renaming mode
	if m.sessionPickerRenaming {
		switch msg.String() {
		case "enter":
			newName := strings.TrimSpace(m.sessionPickerRenameInput)
			if newName != "" && m.sessionPickerCursor < len(sessions) {
				s := sessions[m.sessionPickerCursor]
				if m.onSessions != nil {
					if s.IsCurrent {
						m.onSessions("name", newName)
					} else {
						// Use rename subcommand for non-current sessions
						m.onSessions("rename", s.Name+" "+newName)
					}
				}
			}
			m.sessionPickerRenaming = false
			m.sessionPickerRenameInput = ""
			return m, nil
		case "esc":
			m.sessionPickerRenaming = false
			m.sessionPickerRenameInput = ""
			return m, nil
		case "backspace":
			if len(m.sessionPickerRenameInput) > 0 {
				m.sessionPickerRenameInput = m.sessionPickerRenameInput[:len(m.sessionPickerRenameInput)-1]
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				m.sessionPickerRenameInput += key
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.sessionPickerCursor > 0 {
			m.sessionPickerCursor--
		}
	case "down", "j":
		if m.sessionPickerCursor < len(sessions)-1 {
			m.sessionPickerCursor++
		}
	case "enter":
		// Open selected session
		if len(sessions) > 0 && m.sessionPickerCursor < len(sessions) {
			s := sessions[m.sessionPickerCursor]
			if !s.IsCurrent && m.onSessions != nil {
				m.onSessions("show", s.Name)
			}
			m.showSessionPicker = false
		}
	case "d":
		// Delete selected session (current session protected)
		if len(sessions) > 0 && m.sessionPickerCursor < len(sessions) {
			s := sessions[m.sessionPickerCursor]
			if s.IsCurrent {
				m.chatStatusMsg = "Cannot delete the current session"
			} else if m.onSessions != nil {
				m.onSessions("delete", s.Name)
				// App handler sends SessionPickerMsg to refresh
			}
		}
	case "r":
		// Enter rename mode
		if len(sessions) > 0 && m.sessionPickerCursor < len(sessions) {
			m.sessionPickerRenaming = true
			m.sessionPickerRenameInput = sessions[m.sessionPickerCursor].Name
		}
	case "/":
		// Start filtering — clear existing filter, subsequent chars will be typed
		m.sessionPickerFilter = ""
	case "backspace":
		if len(m.sessionPickerFilter) > 0 {
			m.sessionPickerFilter = m.sessionPickerFilter[:len(m.sessionPickerFilter)-1]
			m.sessionPickerCursor = 0
		}
	case "esc":
		if m.sessionPickerFilter != "" {
			m.sessionPickerFilter = ""
			m.sessionPickerCursor = 0
		} else {
			m.showSessionPicker = false
		}
	default:
		// Typing into filter
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.sessionPickerFilter += key
			m.sessionPickerCursor = 0
		}
	}
	return m, nil
}
