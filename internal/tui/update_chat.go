package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/notify"
)

// updateChat handles key events when in chat mode.
func (m Model) updateChat(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Clear transient status message on any keypress
	m.chatStatusMsg = ""

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "pgup", "shift+pgup":
		if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			m.panels[m.activeTab-1].Viewport.PageUp()
		} else {
			m.chatView.PageUp()
		}
		return m, nil
	case "pgdown", "shift+pgdown":
		if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			m.panels[m.activeTab-1].Viewport.PageDown()
		} else {
			m.chatView.PageDown()
		}
		return m, nil
	case "ctrl+u":
		if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			m.panels[m.activeTab-1].Viewport.HalfPageUp()
		} else {
			m.chatView.HalfPageUp()
		}
		return m, nil
	case "ctrl+d":
		if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			m.panels[m.activeTab-1].Viewport.HalfPageDown()
		} else {
			m.chatView.HalfPageDown()
		}
		return m, nil
	case "home":
		if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			m.panels[m.activeTab-1].Viewport.GotoTop()
		} else {
			m.chatView.GotoTop()
		}
		return m, nil
	case "end":
		if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			m.panels[m.activeTab-1].Viewport.GotoBottom()
		} else {
			m.chatView.GotoBottom()
		}
		return m, nil
	case "up":
		// Navigate up in command palette when open
		if m.paletteOpen && !m.paletteViaCtrlP && m.paletteCursor > 0 {
			m.paletteCursor--
			return m, nil
		}
		if m.tabBarFocused {
			break // let default handling happen
		}
		// If textarea is empty or browsing history, cycle backward
		if strings.TrimSpace(m.textarea.Value()) == "" || m.inputHistIdx >= 0 {
			if len(m.inputHistory) > 0 && m.inputHistIdx < 0 {
				// First press: enter history mode — save draft and show last entry
				m.inputDraft = m.textarea.Value()
				m.inputHistIdx = len(m.inputHistory) - 1
				m.textarea.Reset()
				m.textarea.SetValue(m.inputHistory[m.inputHistIdx])
				return m, nil
			}
			if m.inputHistIdx > 0 {
				// Continue cycling backward through history
				m.inputHistIdx--
				m.textarea.Reset()
				m.textarea.SetValue(m.inputHistory[m.inputHistIdx])
				return m, nil
			}
			// At oldest entry or no history — focus tab bar
			m.inputHistIdx = -1
			m.inputDraft = ""
			m.tabBarFocused = true
			m.textarea.Blur()
			return m, nil
		}
	case "down":
		// Navigate down in command palette when open
		if m.paletteOpen && !m.paletteViaCtrlP && m.paletteCursor < len(m.paletteMatches)-1 {
			m.paletteCursor++
			return m, nil
		}
		// Return focus to textarea from tab bar
		if m.tabBarFocused {
			m.tabBarFocused = false
			if m.activeTab < 0 {
				m.activeTab = 0
			}
			m.textarea.Focus()
			return m, nil
		}
		// Clear textarea content on down arrow — only when cursor is on the
		// last line so multi-line inputs can scroll through all lines first.
		if m.inputHistIdx < 0 && strings.TrimSpace(m.textarea.Value()) != "" {
			val := m.textarea.Value()
			lineCount := strings.Count(val, "\n") + 1
			cursorLine := m.textarea.Line() // 0-indexed current cursor line
			if cursorLine >= lineCount-1 {
				// Already on last line — clear
				m.textarea.Reset()
				m.updateLayout()
				return m, nil
			}
			// Not on last line — let textarea handle cursor movement
		}
		// Cycle forward through input history
		if m.inputHistIdx >= 0 {
			if m.inputHistIdx < len(m.inputHistory)-1 {
				m.inputHistIdx++
				m.textarea.Reset()
				m.textarea.SetValue(m.inputHistory[m.inputHistIdx])
			} else {
				// Back to draft
				m.inputHistIdx = -1
				m.textarea.Reset()
				m.textarea.SetValue(m.inputDraft)
				m.inputDraft = ""
			}
			return m, nil
		}
	case "left":
		// Switch tabs when tab bar is focused (-1 = mode selector)
		if m.tabBarFocused && m.activeTab > -1 {
			m.activeTab--
			m.rerenderActivePanel()
			return m, nil
		}
	case "right":
		// Switch tabs when tab bar is focused. Extra stop for MCP if servers exist.
		if m.tabBarFocused {
			maxTab := len(m.panels)
			if len(m.mcpServers) > 0 {
				maxTab++ // MCP tab is one past last provider
			}
			if m.activeTab < maxTab {
				m.activeTab++
				m.rerenderActivePanel()
			}
			return m, nil
		}
	case "esc":
		// Return focus to textarea from tab bar
		if m.tabBarFocused {
			m.tabBarFocused = false
			if m.activeTab < 0 {
				m.activeTab = 0
			}
			m.textarea.Focus()
			return m, nil
		}
	case "ctrl+s":
		if !m.querying {
			m.closePalette()
			m.mode = viewSettings
			m.settingsCursor = 0
			m.confirmDelete = false
			m.settingsMsg = ""
			return m, nil
		}
	case "ctrl+e":
		if !m.querying {
			m.closePalette()
			return m, m.openExternalEditor()
		}
	case "ctrl+t":
		if !m.querying {
			m.themePickerOpen = true
			m.themePickerCursor = 0
			// Pre-select current theme
			for i, name := range m.themePickerItems {
				if name == m.theme.Name {
					m.themePickerCursor = i
					break
				}
			}
			return m, nil
		}
	case "ctrl+left":
		if m.splitPaneActive() && m.splitRatio > 30 {
			m.splitRatio -= 5
			return m, nil
		}
	case "ctrl+right":
		if m.splitPaneActive() && m.splitRatio < 80 {
			m.splitRatio += 5
			return m, nil
		}
	case "ctrl+g":
		m.autoScrollLocked = !m.autoScrollLocked
		if !m.autoScrollLocked {
			m.chatView.GotoBottom()
		}
		return m, nil
	case "ctrl+h":
		m.closePalette()
		m.concealTools = !m.concealTools
		m.rebuildChatLogCache()
		m.syncChatViewContent()
		return m, nil
	case "ctrl+p":
		if !m.querying {
			m.paletteOpen = true
			m.paletteViaCtrlP = true
			m.paletteFilter = ""
			m.paletteCursor = 0
			m.paletteMatches = m.filterPaletteCommands("")
			m.paletteFiles = nil
			if m.fileIdx != nil {
				m.paletteFiles = m.fileIdx.search("", 5)
			}
			return m, nil
		}
	}

	// Remove last attached file on backspace when input is empty
	if msg.String() == "backspace" && len(m.attachedFiles) > 0 && strings.TrimSpace(m.textarea.Value()) == "" {
		m.attachedFiles = m.attachedFiles[:len(m.attachedFiles)-1]
		return m, nil
	}

	// Command palette navigation when opened via Ctrl+P
	if m.paletteOpen && m.paletteViaCtrlP {
		totalItems := len(m.paletteMatches) + len(m.paletteFiles)
		switch msg.String() {
		case "up":
			if m.paletteCursor > 0 {
				m.paletteCursor--
			}
			return m, nil
		case "down":
			if m.paletteCursor < totalItems-1 {
				m.paletteCursor++
			}
			return m, nil
		case "enter", "tab":
			if totalItems > 0 {
				if m.paletteCursor < len(m.paletteMatches) {
					// Selected a command — insert it into textarea and execute
					selected := m.paletteMatches[m.paletteCursor]
					cmdName := selected.Name
					if idx := strings.IndexAny(cmdName, "<["); idx > 0 {
						cmdName = strings.TrimSpace(cmdName[:idx])
					}
					m.paletteOpen = false
					m.paletteViaCtrlP = false
					m.textarea.SetValue(cmdName)
					// Auto-submit if command has no args placeholder
					if !strings.ContainsAny(selected.Name, "<[") {
						return m, func() tea.Msg {
							return tea.KeyMsg{Type: tea.KeyEnter}
						}
					}
				} else {
					// Selected a file — attach it
					fileIdx := m.paletteCursor - len(m.paletteMatches)
					if fileIdx < len(m.paletteFiles) {
						selected := m.paletteFiles[fileIdx]
						// Add to attached files if not already present
						found := false
						for _, f := range m.attachedFiles {
							if f == selected.Path {
								found = true
								break
							}
						}
						if !found {
							m.attachedFiles = append(m.attachedFiles, selected.Path)
						}
					}
					m.paletteOpen = false
					m.paletteViaCtrlP = false
				}
			}
			return m, nil
		case "esc":
			m.paletteOpen = false
			m.paletteViaCtrlP = false
			m.paletteCursor = 0
			return m, nil
		case "backspace":
			// Remove last character from palette filter
			if len(m.paletteFilter) > 0 {
				m.paletteFilter = m.paletteFilter[:len(m.paletteFilter)-1]
				m.paletteMatches = m.filterPaletteCommands(m.paletteFilter)
				if m.fileIdx != nil {
					m.paletteFiles = m.fileIdx.search(m.paletteFilter, 5)
				}
				m.paletteCursor = 0
			}
			return m, nil
		default:
			// Any printable character: append to the palette filter.
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				m.paletteFilter += key
				m.paletteMatches = m.filterPaletteCommands(m.paletteFilter)
				if m.fileIdx != nil {
					m.paletteFiles = m.fileIdx.search(m.paletteFilter, 5)
				}
				m.paletteCursor = 0
				return m, nil
			}
		}
	}

	// File picker intercepts navigation keys when open
	if m.filePickerOpen {
		switch msg.String() {
		case "up":
			if m.filePickerCursor > 0 {
				m.filePickerCursor--
			}
			return m, nil
		case "down":
			if m.filePickerCursor < len(m.filePickerMatches)-1 {
				m.filePickerCursor++
			}
			return m, nil
		case "tab":
			if len(m.filePickerMatches) > 0 {
				m.acceptFilePick()
			}
			return m, nil
		case "enter":
			// Only accept if there are matches; otherwise close and let Enter submit
			if len(m.filePickerMatches) > 0 {
				m.acceptFilePick()
				return m, nil
			}
			// No matches — close picker and fall through to normal Enter handling
			m.filePickerOpen = false
			m.filePickerCursor = 0
		case "esc":
			// Close file picker without modifying input (non-destructive)
			m.filePickerOpen = false
			m.filePickerCursor = 0
			return m, nil
		}
	}

	switch msg.String() {
	case "tab":
		// Tab accepts the selected palette match when palette is showing
		if m.paletteOpen && len(m.paletteMatches) > 0 {
			idx := m.paletteCursor
			if idx >= len(m.paletteMatches) {
				idx = 0
			}
			selected := m.paletteMatches[idx]
			// Strip placeholder part (e.g., "/mode <name>" -> "/mode ")
			name := selected.Name
			if si := strings.IndexAny(name, "<["); si > 0 {
				name = name[:si]
			}
			m.textarea.Reset()
			m.textarea.SetValue(name)
			m.paletteCursor = 0
			return m, nil
		}
		m.showIndividual = !m.showIndividual
		return m, nil
	case "p":
		if m.tabBarFocused {
			m.showProvenance = !m.showProvenance
			return m, nil
		}
	case "m":
		if m.tabBarFocused && len(m.mcpServers) > 0 {
			m.showMCPDashboard = !m.showMCPDashboard
			if m.showMCPDashboard && m.onMCPDashboardRefresh != nil {
				m.mcpDashboardCursor = 0
				m.onMCPDashboardRefresh()
			}
			return m, nil
		}
	// Vim-style scroll keys — only active when tab bar is focused
	// (prevents intercepting first character of user input)
	case "j":
		if m.tabBarFocused {
			if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
				m.panels[m.activeTab-1].Viewport.ScrollDown(1)
			} else {
				m.chatView.ScrollDown(1)
			}
			return m, nil
		}
	case "k":
		if m.tabBarFocused {
			if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
				m.panels[m.activeTab-1].Viewport.ScrollUp(1)
			} else {
				m.chatView.ScrollUp(1)
			}
			return m, nil
		}
	case "d":
		if m.tabBarFocused {
			m.chatView.HalfPageDown()
			return m, nil
		}
	case "u":
		if m.tabBarFocused {
			m.chatView.HalfPageUp()
			return m, nil
		}
	case "g":
		if m.tabBarFocused {
			m.chatView.GotoTop()
			return m, nil
		}
	case "G":
		if m.tabBarFocused {
			m.chatView.GotoBottom()
			return m, nil
		}
	// Split pane: number keys select right panel provider (tab bar focused)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if m.tabBarFocused && m.splitPaneActive() {
			idx := int(msg.String()[0]-'0') - 1
			if idx < len(m.panels) {
				m.splitPanelIdx = idx
			}
			return m, nil
		}
	// Error panel actions — only when tab bar focused and error is displayed
	case "r":
		if m.errorRecord != nil && !m.querying && m.tabBarFocused {
			// Retry: re-submit the last prompt
			if len(m.inputHistory) > 0 {
				lastPrompt := m.inputHistory[len(m.inputHistory)-1]
				m.clearError()
				m.resetPanels()
				m.currentPrompt = lastPrompt
				if m.onSubmit != nil {
					m.onSubmit(lastPrompt)
				}
			}
			return m, nil
		}
	case "e":
		if m.errorRecord != nil && m.tabBarFocused {
			m.errorRecord.Collapsed = !m.errorRecord.Collapsed
			m.syncChatViewContent()
			return m, nil
		}
	case "c":
		// Cancel a loading provider mid-query
		if m.tabBarFocused && m.querying && m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
			panel := &m.panels[m.activeTab-1]
			if panel.Status == StatusLoading {
				panel.Status = StatusCancelled
				panel.Content.WriteString("\n[Cancelled by user]")
				panel.Viewport.SetContent(panel.Content.String())
				if m.onCancelProvider != nil {
					m.onCancelProvider(panel.Name)
				}
				m.chatStatusMsg = panel.Name + " cancelled"
				return m, nil
			}
		}
		if m.errorRecord != nil && m.tabBarFocused {
			errText := m.errorRecord.Summary + "\n" + m.errorRecord.Detail
			if err := copyToClipboard(errText); err == nil {
				m.chatStatusMsg = "Error copied to clipboard"
			}
			return m, nil
		}
	case "t":
		// Toggle trace expansion on current/last exchange
		if m.tabBarFocused {
			if len(m.history) > 0 {
				last := &m.history[len(m.history)-1]
				last.expandedTrace = !last.expandedTrace
				m.rebuildChatLogCache()
				m.syncChatViewContent()
			}
			return m, nil
		}
	case "y":
		if m.tabBarFocused {
			content := m.getClipboardContent()
			if content != "" {
				if err := copyToClipboard(content); err == nil {
					cmd := m.addToast(ToastSuccess, "Copied to clipboard")
					return m, cmd
				} else {
					m.chatStatusMsg = "Clipboard unavailable"
				}
			} else {
				m.chatStatusMsg = "Nothing to copy"
			}
			return m, nil
		}
	case "enter":
		// Palette open: if user hasn't typed arguments yet, Enter accepts the
		// palette selection (like Tab). If they already typed the full command
		// with arguments (e.g., "/mode thorough"), close palette and submit.
		if m.paletteOpen && len(m.paletteMatches) > 0 {
			currentVal := strings.TrimSpace(m.textarea.Value())
			ci := m.paletteCursor
			if ci >= len(m.paletteMatches) {
				ci = 0
			}
			selected := m.paletteMatches[ci]
			cmdName := selected.Name
			if idx := strings.IndexAny(cmdName, "<["); idx > 0 {
				cmdName = strings.TrimSpace(cmdName[:idx])
			}
			// If user typed more than just the command name, they've added
			// arguments — close palette and let Enter submit normally.
			if strings.Contains(currentVal, " ") && len(currentVal) > len(cmdName) {
				m.paletteOpen = false
				// Fall through to normal Enter handling below
			} else {
				// No arguments yet — accept the palette selection.
				hasArgs := strings.ContainsAny(selected.Name, "<[")
				m.textarea.Reset()
				m.textarea.SetValue(cmdName + " ")
				m.paletteOpen = false
				if !hasArgs {
					m.textarea.SetValue(cmdName)
					return m, func() tea.Msg {
						return tea.KeyMsg{Type: tea.KeyEnter}
					}
				}
				return m, nil
			}
		}
		// Tab bar focused: Enter on mode selector opens picker, MCP opens dashboard, otherwise returns to textarea
		if m.tabBarFocused {
			if m.activeTab == -1 {
				// Open mode picker
				m.modePickerOpen = true
				m.tabBarFocused = false
				for i, item := range m.modePickerItems {
					if item == m.currentMode {
						m.modePickerIdx = i
						break
					}
				}
				return m, nil
			}
			// MCP tab (one past last provider)
			mcpTabIdx := len(m.panels) + 1
			if len(m.mcpServers) > 0 && m.activeTab == mcpTabIdx {
				m.tabBarFocused = false
				m.showMCPDashboard = true
				m.mcpDashboardCursor = 0
				if m.onMCPDashboardRefresh != nil {
					m.onMCPDashboardRefresh()
				}
				return m, nil
			}
			m.tabBarFocused = false
			m.textarea.Focus()
			return m, nil
		}
		if !m.querying {
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" {
				// Shell command context injection: !command runs the command
				// and injects its output as context into the prompt.
				if strings.HasPrefix(prompt, "!") {
					shellCmd := strings.TrimSpace(prompt[1:])
					if shellCmd != "" {
						m.textarea.Reset()
						m.toolStatus = "Running: " + shellCmd
						if m.onShellContext != nil {
							m.onShellContext(shellCmd)
						}
						return m, nil
					}
				}
				// Check for slash commands
				if strings.HasPrefix(prompt, "/settings") {
					m.textarea.Reset()
					m.mode = viewSettings
					m.settingsCursor = 0
					m.confirmDelete = false
					m.settingsMsg = ""
					return m, nil
				}
				if prompt == "/mode" {
					// No args — open mode picker
					m.textarea.Reset()
					m.modePickerOpen = true
					// Pre-select current mode
					for i, item := range m.modePickerItems {
						if item == m.currentMode {
							m.modePickerIdx = i
							break
						}
					}
					return m, nil
				}
				if modeName, ok := strings.CutPrefix(prompt, "/mode "); ok {
					modeName = strings.TrimSpace(modeName)
					m.textarea.Reset()
					switch modeName {
					case "quick", "balanced", "thorough":
						m.currentMode = modeName
						if m.onModeChange != nil {
							go m.onModeChange(modeName)
						}
					}
					return m, nil
				}
				if rest, ok := strings.CutPrefix(prompt, "/memory"); ok {
					m.textarea.Reset()
					if m.onMemory != nil {
						m.onMemory(strings.TrimSpace(rest))
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/skill") {
					m.textarea.Reset()
					if m.onSkill != nil {
						rest := strings.TrimSpace(strings.TrimPrefix(prompt, "/skill"))
						parts := strings.SplitN(rest, " ", 2)
						sub := ""
						args := ""
						if len(parts) > 0 {
							sub = parts[0]
						}
						if len(parts) > 1 {
							args = parts[1]
						}
						m.onSkill(sub, args)
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/plan ") {
					request := strings.TrimPrefix(prompt, "/plan ")
					m.textarea.Reset()
					m.currentPrompt = prompt
					m.planRunning = true
					m.agentStages = nil
					if m.onPlan != nil {
						m.onPlan(request)
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/sessions") || prompt == "/sessions" {
					m.textarea.Reset()
					rest := strings.TrimSpace(strings.TrimPrefix(prompt, "/sessions"))
					parts := strings.SplitN(rest, " ", 2)
					sub := ""
					args := ""
					if len(parts) > 0 {
						sub = parts[0]
					}
					if len(parts) > 1 {
						args = parts[1]
					}
					// /sessions or /sessions list → open visual picker
					if sub == "" || sub == "list" {
						if m.onSessionPickerRefresh != nil {
							m.onSessionPickerRefresh()
						}
						return m, nil
					}
					// Other subcommands go through callback
					if m.onSessions != nil {
						m.onSessions(sub, args)
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/name ") {
					name := strings.TrimSpace(strings.TrimPrefix(prompt, "/name "))
					m.textarea.Reset()
					if name != "" {
						m.sessionName = name
						m.sessionNameGen++ // prevent stale auto-name from overwriting
						if m.onSessions != nil {
							m.onSessions("name", name)
						}
						cmd := m.addToast(ToastSuccess, "Session named: "+name)
						return m, cmd
					}
					return m, nil
				}
				if prompt == "/compact" {
					m.textarea.Reset()
					m.chatStatusMsg = "Context compaction not yet implemented — use /clear to reset"
					return m, nil
				}
				if prompt == "/compose" {
					m.textarea.Reset()
					return m, m.openExternalEditor()
				}
				if prompt == "/theme" {
					m.textarea.Reset()
					m.themePickerOpen = true
					m.themePickerCursor = 0
					for i, name := range m.themePickerItems {
						if name == m.theme.Name {
							m.themePickerCursor = i
							break
						}
					}
					return m, nil
				}
				if prompt == "/conceal" {
					m.textarea.Reset()
					m.concealTools = !m.concealTools
					m.rebuildChatLogCache()
					m.syncChatViewContent()
					if m.concealTools {
						m.chatStatusMsg = "Tool calls concealed"
					} else {
						m.chatStatusMsg = "Tool calls expanded"
					}
					return m, nil
				}
				if prompt == "/copy" {
					m.textarea.Reset()
					content := m.getClipboardContent()
					if content != "" {
						if err := copyToClipboard(content); err == nil {
							m.chatStatusMsg = "Copied to clipboard"
						} else {
							m.chatStatusMsg = "Clipboard unavailable"
						}
					} else {
						m.chatStatusMsg = "Nothing to copy"
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/clear") {
					m.textarea.Reset()
					m.history = nil
					m.currentPrompt = ""
					m.clearError()
					m.chatLogCache = ""
					m.attachedFiles = nil
					m.undoStack = nil
					m.redoStack = nil
					m.sessionAllowed = nil   // clear session-level approvals
					m.sessionName = ""       // clear session name
					m.sessionNameGen++       // prevent stale auto-names
					m.tokenUsage = nil       // clear status bar token display
					m.lastWarningBand = 0    // reset context pressure warnings
					m.consensusContent.Reset()
					m.consensusView.SetContent("")
					m.chatView.SetContent("")
					m.resetPanels()
					if m.onClear != nil {
						m.onClear()
					}
					return m, nil
				}
				if prompt == "/help" || prompt == "/?" {
					m.textarea.Reset()
					m.showHelp = !m.showHelp
					return m, nil
				}
				if prompt == "/exit" || prompt == "/quit" {
					return m, tea.Quit
				}
				if strings.HasPrefix(prompt, "/mcp") {
					m.textarea.Reset()
					rest := strings.TrimSpace(strings.TrimPrefix(prompt, "/mcp"))
					// Handle /mcp add shortcut
					if rest == "add" {
						m.initMCPWizardForAdd()
						return m, nil
					}
					if m.onMCP != nil {
						parts := strings.SplitN(rest, " ", 2)
						sub := ""
						args := ""
						if len(parts) > 0 {
							sub = parts[0]
						}
						if len(parts) > 1 {
							args = parts[1]
						}
						m.onMCP(sub, args)
					}
					return m, nil
				}
				if prompt == "/undo" {
					m.textarea.Reset()
					if len(m.undoStack) == 0 {
						m.chatStatusMsg = "Nothing to undo"
					} else if m.onUndo != nil {
						m.onUndo()
					}
					return m, nil
				}
				if prompt == "/redo" {
					m.textarea.Reset()
					if len(m.redoStack) == 0 {
						m.chatStatusMsg = "Nothing to redo"
					} else if m.onRedo != nil {
						m.onRedo()
					}
					return m, nil
				}
				if prompt == "/yolo" {
					m.textarea.Reset()
					m.yoloMode = !m.yoloMode
					if m.onYoloToggle != nil {
						m.onYoloToggle(m.yoloMode)
					}
					return m, nil
				}
				if prompt == "/save" {
					m.textarea.Reset()
					if m.onSave != nil {
						go m.onSave()
					}
					return m, nil
				}
				if prompt == "/share" {
					m.textarea.Reset()
					if m.onShareSession != nil {
						m.onShareSession()
					}
					return m, nil
				}
				if rest, ok := strings.CutPrefix(prompt, "/export"); ok {
					path := strings.TrimSpace(rest)
					m.textarea.Reset()
					if path == "md" {
						// Export as markdown
						if m.onExportMarkdown != nil {
							m.onExportMarkdown()
						}
					} else {
						if m.onExport != nil {
							m.onExport(path)
						}
					}
					return m, nil
				}
				// Prepend attached file contents to the prompt
				submitPrompt := prompt
				if len(m.attachedFiles) > 0 && m.fileIdx != nil {
					submitPrompt = m.buildPromptWithFiles(prompt)
					m.attachedFiles = nil
				}
				m.currentPrompt = prompt
				m.textarea.Reset()
				m.inputHistory = append(m.inputHistory, prompt)
				m.inputHistIdx = -1
				m.inputDraft = ""
				m.resetPanels()
				if m.onSubmit != nil {
					m.onSubmit(submitPrompt)
				}
			}
			return m, nil
		}
	}

	// Pass to textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// updateMouse handles mouse events.
func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Skip mouse events when any modal overlay is open
	if m.confirmPending || m.showHelp || m.modePickerOpen || m.showMCPDashboard ||
		m.showSessionPicker || m.paletteViaCtrlP || m.filePickerOpen {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.tabBarFocused || m.mode == viewChat {
			m.scrollViewportAt(msg.X, -3)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.tabBarFocused || m.mode == viewChat {
			m.scrollViewportAt(msg.X, 3)
		}
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Y == 0 && m.mode == viewChat {
			// Click on tab bar row — switch to the clicked tab
			// Simple heuristic: divide width by number of tabs
			totalTabs := 1 + len(m.panels) // consensus + providers
			tabWidth := m.width / totalTabs
			if tabWidth > 0 {
				clickedTab := msg.X / tabWidth
				if clickedTab >= 0 && clickedTab < totalTabs {
					m.activeTab = clickedTab
					m.tabBarFocused = true
				}
			}
			return m, nil
		}
		// Click on input area (bottom rows) — focus textarea
		inputAreaStart := m.height - m.textarea.Height() - 5
		if msg.Y >= inputAreaStart {
			m.tabBarFocused = false
			m.textarea.Focus()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) resetPanels() {
	for i := range m.panels {
		m.panels[i].Status = StatusIdle
		m.panels[i].Content.Reset()
		m.panels[i].TraceSections = nil
		m.panels[i].currentPhase = ""
		m.panels[i].Viewport.SetContent("")
	}
	m.consensusContent.Reset()
	m.consensusRendered = ""
	m.consensusView.SetContent("")
	m.consensusActive = false
}

// markPanelsQueried sets the named providers to loading status.
// Providers not in the list stay idle, making it clear which ones
// are actually participating in the current query.
func (m *Model) markPanelsQueried(names []string) {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for i := range m.panels {
		if nameSet[m.panels[i].Name] {
			m.panels[i].Status = StatusLoading
		}
	}
}

// syncChatViewContent rebuilds the chat view content from history + in-progress
// state. Must be called from Update() (not View()) so the viewport's internal
// scroll state stays consistent with its content.
func (m *Model) syncChatViewContent() {
	chatContent := m.buildChatLog()
	if m.querying || m.consensusActive {
		if m.consensusRendered != "" {
			chatContent += m.consensusRendered
		} else if m.consensusContent.Len() > 0 {
			chatContent += m.consensusContent.String()
		}
	}
	if m.lastError != "" && !m.querying {
		if m.errorRecord != nil {
			chatContent += "\n\n" + renderErrorPanel(*m.errorRecord, m.width, m.theme)
		} else {
			chatContent += "\n\n[Error: " + m.lastError + "]"
		}
	}
	m.chatView.SetContent(chatContent)
	if m.querying && !m.autoScrollLocked {
		m.chatView.GotoBottom()
	}
}

func (m *Model) updateLayout() {
	// Auto-enable split pane for wide terminals
	m.splitPaneEnabled = m.width >= splitMinWidth

	// Dynamic textarea height: grows from 1 to 8 lines based on content
	m.recalcTextareaHeight()

	taHeight := m.textarea.Height()
	inputHeight := taHeight + 3 // textarea + border + padding + prompt label
	tabBarHeight := 1
	statusBarHeight := 1 // persistent status bar
	borderPadding := 4   // top/bottom border + padding on chat panel

	// Account for attachment pills row
	if len(m.attachedFiles) > 0 {
		inputHeight++
	}
	// Account for chat status message row
	if m.chatStatusMsg != "" {
		inputHeight++
	}

	// Account for live task HUD rows during tool execution
	hudHeight := 0
	if m.querying && len(m.toolCalls) > 0 {
		hudHeight = len(m.toolCalls)
		if hudHeight > 5 {
			hudHeight = 5
		}
	}

	availableHeight := m.height - inputHeight - tabBarHeight - statusBarHeight - hudHeight - borderPadding

	// Update textarea width
	m.textarea.SetWidth(m.width - 4)

	panelWidth := m.width - 4
	viewportWidth := panelWidth - 1 // reserve 1 col for scrollbar indicator

	// In split pane mode, size viewports to their actual pane widths
	if m.splitPaneActive() {
		leftWidth := int(float64(m.width-2) * float64(m.splitRatio) / 100.0)
		rightWidth := m.width - 2 - leftWidth - 1
		if leftWidth < 20 {
			leftWidth = 20
		}
		if rightWidth < 20 {
			rightWidth = 20
		}
		// Chat/consensus gets the left pane width (minus scrollbar)
		m.chatView.Width = leftWidth - 1
		m.chatView.Height = max(availableHeight-3, 1)
		m.consensusView.Width = leftWidth - 1
		m.consensusView.Height = max(availableHeight-3, 1)
		// Provider panels get the right pane width (minus scrollbar)
		for i := range m.panels {
			m.panels[i].Viewport.Width = rightWidth - 1
			m.panels[i].Viewport.Height = max(availableHeight-3, 1)
		}
		// Use the narrower pane width for word wrap so content fits both panes
		wrapWidth := leftWidth - 1
		if rightWidth-1 < wrapWidth {
			wrapWidth = rightWidth - 1
		}
		setMarkdownWidth(wrapWidth, m.theme)
	} else {
		// All views get full width minus scrollbar; the active tab gets all the height
		m.chatView.Width = viewportWidth
		m.chatView.Height = max(availableHeight-3, 1)
		m.consensusView.Width = viewportWidth
		m.consensusView.Height = max(availableHeight-3, 1)
		for i := range m.panels {
			m.panels[i].Viewport.Width = viewportWidth
			m.panels[i].Viewport.Height = max(availableHeight-3, 1)
		}
		setMarkdownWidth(viewportWidth, m.theme)
	}
}

// recalcTextareaHeight adjusts the textarea height based on content line count.
func (m *Model) recalcTextareaHeight() {
	const minHeight = 1
	const maxHeight = 8

	content := m.textarea.Value()
	if content == "" {
		if m.textarea.Height() != minHeight {
			m.textarea.SetHeight(minHeight)
		}
		return
	}

	// Count hard newlines; each newline means another visible row.
	lineCount := strings.Count(content, "\n") + 1
	desired := lineCount
	if desired < minHeight {
		desired = minHeight
	}
	if desired > maxHeight {
		desired = maxHeight
	}
	if m.textarea.Height() != desired {
		m.textarea.SetHeight(desired)
	}
}

// rebuildChatLogCache re-renders the chat log from history. Call this whenever
// history changes (pointer receiver so it can mutate the cache).
func (m *Model) rebuildChatLogCache() {
	var b strings.Builder
	for i := range m.history {
		ex := &m.history[i]
		b.WriteString("❯ ")
		b.WriteString(ex.Prompt)
		b.WriteString("\n\n")

		// Trace summary — collapsed by default after completion
		if len(ex.ProviderTraces) > 0 {
			if ex.expandedTrace {
				// Show full traces
				for provider, sections := range ex.ProviderTraces {
					b.WriteString("── " + provider + " ──\n")
					for _, s := range sections {
						b.WriteString(s.Content)
					}
					b.WriteString("\n")
				}
			} else {
				// Collapsed summary
				totalLines := 0
				totalSections := 0
				for _, sections := range ex.ProviderTraces {
					totalSections += len(sections)
					for _, s := range sections {
						totalLines += strings.Count(s.Content, "\n")
					}
				}
				if totalSections > 0 {
					b.WriteString(fmt.Sprintf("▶ Trace: %d sections, %d lines (press t to expand)\n\n", totalSections, totalLines))
				}
			}
		}

		// Tool call summary (when concealed)
		if m.concealTools && len(ex.ToolCalls) > 0 {
			b.WriteString(renderToolCallSummary(ex.ToolCalls))
			b.WriteString("\n\n")
		}

		// Turn timeline — shows providers, tools, synthesis at a glance
		timeline := renderTurnTimeline(*ex, m.panels, m.theme)
		if timeline != "" {
			b.WriteString(timeline)
			b.WriteString("\n\n")
		}

		// Render markdown once per exchange and cache it
		if ex.renderedResponse == "" && ex.ConsensusResponse != "" {
			ex.renderedResponse = renderMarkdown(ex.ConsensusResponse)
		}
		b.WriteString(ex.renderedResponse)
		b.WriteString("\n")
	}
	m.chatLogCache = b.String()
	m.chatLogDirty = false
}

// buildChatLog returns the cached chat log plus any in-progress prompt.
// This is cheap to call from View() since the expensive rendering is cached.
func (m Model) buildChatLog() string {
	result := m.chatLogCache

	if m.currentPrompt != "" {
		result += "❯ " + m.currentPrompt + "\n"
	}

	return result
}

// openExternalEditor opens $EDITOR with the current textarea content in a temp
// file. Uses tea.ExecProcess to suspend the TUI while the editor runs.
// Parses $EDITOR into argv to support values like "code --wait" or "nvim -f".
func (m Model) openExternalEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "polycode-compose-*.md")
	if err != nil {
		return nil
	}

	// Write current textarea content to temp file
	content := m.textarea.Value()
	if content != "" {
		tmpFile.WriteString(content)
	}
	tmpFile.Close()

	// Parse editor command into argv (supports "code --wait", "nvim -f", etc.)
	parts := strings.Fields(editor)
	args := append(parts[1:], tmpFile.Name())
	c := exec.Command(parts[0], args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{tempFile: tmpFile.Name(), err: err}
	})
}

// setError records a structured error for display in the chat area.
func (m *Model) setError(summary, detail string) {
	m.lastError = summary
	m.errorRecord = &ErrorRecord{
		Summary:   summary,
		Detail:    detail,
		Timestamp: time.Now(),
		Collapsed: true,
	}
}

// clearError removes any displayed error.
func (m *Model) clearError() {
	m.lastError = ""
	m.errorRecord = nil
}

// getClipboardContent returns the appropriate content for copying to clipboard.
// If a provider tab is active, returns that provider's response.
// Otherwise returns the last consensus response.
func (m *Model) getClipboardContent() string {
	// Provider tab selected: copy that provider's response
	if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
		return m.panels[m.activeTab-1].Content.String()
	}
	// Consensus: use raw text from the last exchange or current content
	if m.consensusRaw != "" {
		return m.consensusRaw
	}
	if len(m.history) > 0 {
		return m.history[len(m.history)-1].ConsensusResponse
	}
	return ""
}

// buildPromptWithFiles prepends attached file contents to the user's prompt.
// Skips files that are too large (>100KB) or appear to be binary.
func (m *Model) buildPromptWithFiles(prompt string) string {
	if len(m.attachedFiles) == 0 || m.fileIdx == nil {
		return prompt
	}

	const maxFileSize = 100 * 1024 // 100KB

	var b strings.Builder
	for _, f := range m.attachedFiles {
		fullPath := filepath.Join(m.fileIdx.root, f)

		// Check file size before reading
		info, statErr := os.Stat(fullPath)
		if statErr != nil {
			b.WriteString(fmt.Sprintf("@%s:\n(error: %s)\n\n", f, statErr))
			continue
		}
		if info.Size() > maxFileSize {
			b.WriteString(fmt.Sprintf("@%s:\n(skipped: file too large — %d bytes, limit %d)\n\n", f, info.Size(), maxFileSize))
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			b.WriteString(fmt.Sprintf("@%s:\n(error reading file: %s)\n\n", f, err))
			continue
		}

		// Simple binary detection: check for null bytes in first 512 bytes
		sample := data
		if len(sample) > 512 {
			sample = sample[:512]
		}
		isBinary := false
		for _, byt := range sample {
			if byt == 0 {
				isBinary = true
				break
			}
		}
		if isBinary {
			b.WriteString(fmt.Sprintf("@%s:\n(skipped: binary file)\n\n", f))
			continue
		}

		ext := strings.TrimPrefix(filepath.Ext(f), ".")
		b.WriteString(fmt.Sprintf("@%s:\n```%s\n%s\n```\n\n", f, ext, string(data)))
	}
	b.WriteString(prompt)
	return b.String()
}

// fnvHash computes a fast FNV-1a hash of a string for content change detection.
func fnvHash(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// sendNotification sends a desktop notification if enabled and terminal is unfocused.
func (m *Model) sendNotification(title, body string) {
	if m.notifyEnabled && !m.terminalFocused {
		go notify.Send(title, body)
	}
}

// scrollViewportAt scrolls the viewport under the given X coordinate.
// In split pane mode, routes to left (chat) or right (provider) panel.
// In single-panel mode, routes to the active tab's viewport.
func (m *Model) scrollViewportAt(x int, delta int) {
	if m.splitPaneActive() && m.splitPanelIdx >= 0 && m.splitPanelIdx < len(m.panels) {
		// Split pane: determine which side the mouse is over
		leftWidth := int(float64(m.width-2) * float64(m.splitRatio) / 100.0)
		if x > leftWidth {
			// Right panel — scroll the split panel's provider viewport
			if delta > 0 {
				m.panels[m.splitPanelIdx].Viewport.ScrollDown(delta)
			} else {
				m.panels[m.splitPanelIdx].Viewport.ScrollUp(-delta)
			}
			return
		}
		// Left panel — scroll chat view
		if delta > 0 {
			m.chatView.ScrollDown(delta)
		} else {
			m.chatView.ScrollUp(-delta)
		}
		return
	}

	// Single-panel mode: route to active tab
	if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
		if delta > 0 {
			m.panels[m.activeTab-1].Viewport.ScrollDown(delta)
		} else {
			m.panels[m.activeTab-1].Viewport.ScrollUp(-delta)
		}
	} else {
		if delta > 0 {
			m.chatView.ScrollDown(delta)
		} else {
			m.chatView.ScrollUp(-delta)
		}
	}
}

// rerenderActivePanel re-renders the active provider panel's content as
// markdown. Called when switching tabs so the newly-visible panel is formatted.
func (m *Model) rerenderActivePanel() {
	if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
		idx := m.activeTab - 1
		raw := m.panels[idx].Content.String()
		if raw != "" {
			m.panels[idx].Viewport.SetContent(renderMarkdown(raw))
		}
	}
}

func (m Model) SendProviderChunk(name, delta string, done bool, err error) tea.Cmd {
	return func() tea.Msg {
		return ProviderChunkMsg{
			ProviderName: name,
			Delta:        delta,
			Done:         done,
			Error:        err,
		}
	}
}

func (m Model) SendConsensusChunk(delta string, done bool, err error) tea.Cmd {
	return func() tea.Msg {
		return ConsensusChunkMsg{
			Delta: delta,
			Done:  done,
			Error: err,
		}
	}
}
