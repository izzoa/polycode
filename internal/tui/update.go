package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/tokens"
)

// Messages for TUI updates from the query pipeline.

// ProviderChunkMsg delivers a streaming chunk from a provider.
type ProviderChunkMsg struct {
	ProviderName string
	Delta        string
	Done         bool
	Error        error
}

// ConsensusChunkMsg delivers a streaming chunk from the consensus synthesis.
type ConsensusChunkMsg struct {
	Delta string
	Done  bool
	Error error
}

// QueryStartMsg signals that a query has begun.
type QueryStartMsg struct{}

// QueryDoneMsg signals that the full pipeline (fan-out + consensus) is complete.
type QueryDoneMsg struct{}

// TokenUpdateMsg delivers a snapshot of token usage for all providers.
type TokenUpdateMsg struct {
	Usage []tokens.ProviderUsage
}

// ConfirmActionMsg asks the user to confirm an action.
// The ResponseCh is used to synchronously communicate the user's decision
// back to the goroutine that requested confirmation.
type ConfirmActionMsg struct {
	Description string
	ResponseCh  chan bool
}

// ToolCallMsg notifies the TUI that a tool is being executed.
type ToolCallMsg struct {
	ToolName    string
	Description string // e.g., "Reading main.go" or "Running `go test`"
}

// ConsensusAnalysisMsg delivers structured provenance from the consensus synthesis.
type ConsensusAnalysisMsg struct {
	Confidence  string
	Agreements  []string
	Minorities  []string
	Evidence    []string
}

// ModeChangedMsg updates the current operating mode display.
type ModeChangedMsg struct {
	Mode string // "quick", "balanced", "thorough"
}

// MemoryDisplayMsg shows memory contents in the chat.
type MemoryDisplayMsg struct {
	Content string
}

// WorkerProgressMsg updates a worker's status in the TUI during /plan execution.
type WorkerProgressMsg struct {
	StageName    string
	Role         string
	ProviderName string
	Status       string // "pending", "running", "complete"
	Summary      string // one-line summary of output (set when complete)
}

// PlanDoneMsg signals that a /plan job has completed.
type PlanDoneMsg struct {
	FinalOutput string
	Error       error
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case splashDoneMsg:
		m.showSplash = false
		return m, nil

	case ConfirmActionMsg:
		m.confirmPending = true
		m.confirmDescription = msg.Description
		m.confirmResponseCh = msg.ResponseCh
		return m, nil

	case ToolCallMsg:
		m.toolStatus = msg.Description
		m.consensusContent.WriteString("\n" + msg.Description + "\n")
		m.consensusView.SetContent(m.consensusContent.String())
		m.consensusView.GotoBottom()
		return m, nil

	case ModeChangedMsg:
		m.currentMode = msg.Mode
		return m, nil

	case MemoryDisplayMsg:
		// Show memory content in the chat view
		m.consensusContent.Reset()
		m.consensusContent.WriteString(msg.Content)
		m.consensusView.SetContent(m.consensusContent.String())
		m.consensusActive = true
		return m, nil

	case ConsensusAnalysisMsg:
		m.consensusConfidence = msg.Confidence
		m.consensusAgreements = msg.Agreements
		m.minorityReports = msg.Minorities
		m.consensusEvidence = msg.Evidence
		return m, nil

	case WorkerProgressMsg:
		// Update or add the worker in agentStages
		found := false
		for i, s := range m.agentStages {
			if s.Name == msg.StageName {
				for j, w := range s.Workers {
					if w.Role == msg.Role {
						m.agentStages[i].Workers[j].Status = msg.Status
						m.agentStages[i].Workers[j].Summary = msg.Summary
						found = true
						break
					}
				}
				if !found {
					m.agentStages[i].Workers = append(m.agentStages[i].Workers, agentWorkerDisplay{
						Role:     msg.Role,
						Provider: msg.ProviderName,
						Status:   msg.Status,
						Summary:  msg.Summary,
					})
					found = true
				}
				break
			}
		}
		if !found {
			m.agentStages = append(m.agentStages, agentStageDisplay{
				Name: msg.StageName,
				Workers: []agentWorkerDisplay{{
					Role:     msg.Role,
					Provider: msg.ProviderName,
					Status:   msg.Status,
					Summary:  msg.Summary,
				}},
			})
		}
		return m, nil

	case PlanDoneMsg:
		m.planRunning = false
		if msg.Error != nil {
			m.consensusContent.WriteString("\n[Plan Error: " + msg.Error.Error() + "]")
		} else if msg.FinalOutput != "" {
			m.consensusContent.WriteString(msg.FinalOutput)
		}
		m.consensusView.SetContent(m.consensusContent.String())
		m.consensusView.GotoBottom()
		m.consensusActive = true
		return m, nil

	case tea.KeyMsg:
		// Confirmation prompt takes priority over everything except quit
		if m.confirmPending {
			switch msg.String() {
			case "ctrl+c":
				if m.confirmResponseCh != nil {
					m.confirmResponseCh <- false
				}
				m.confirmPending = false
				return m, tea.Quit
			case "y", "Y":
				if m.confirmResponseCh != nil {
					m.confirmResponseCh <- true
				}
				m.confirmPending = false
				m.confirmDescription = ""
				return m, nil
			case "n", "N", "esc":
				if m.confirmResponseCh != nil {
					m.confirmResponseCh <- false
				}
				m.confirmPending = false
				m.confirmDescription = ""
				return m, nil
			}
			return m, nil // swallow all other keys during confirm
		}

		// Any keypress dismisses splash (except ctrl+c which quits)
		if m.showSplash && msg.String() != "ctrl+c" {
			m.showSplash = false
			return m, nil
		}

		// Help overlay toggle — only when textarea is empty (so ? can be typed)
		if msg.String() == "?" && m.mode != viewAddProvider && m.mode != viewEditProvider {
			if m.mode != viewChat || strings.TrimSpace(m.textarea.Value()) == "" {
				m.showHelp = !m.showHelp
				return m, nil
			}
		}
		if m.showHelp {
			if msg.String() == "esc" || msg.String() == "?" {
				m.showHelp = false
			}
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		// Route key events by mode
		switch m.mode {
		case viewSettings:
			return m.updateSettings(msg)
		case viewAddProvider, viewEditProvider:
			return m.updateWizard(msg)
		default:
			return m.updateChat(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil

	case ConfigChangedMsg:
		// Handled by the app layer callback; we also update local state.
		m.cfg = msg.Config
		m.rebuildPanelsFromConfig()
		return m, nil

	case TestResultMsg:
		m.testingProvider = ""
		if msg.Success {
			m.settingsMsg = m.styles.StatusHealthy.Render(
				"Connected to " + msg.ProviderName + " (" + msg.Duration + ")")
		} else {
			errMsg := "unknown error"
			if msg.Error != nil {
				errMsg = msg.Error.Error()
			}
			m.settingsMsg = m.styles.StatusUnhealthy.Render(
				"Error testing " + msg.ProviderName + ": " + errMsg)
		}
		return m, nil

	case WizardTestResultMsg:
		m.wizardTesting = false
		if msg.Success {
			m.wizardTestResult = m.styles.StatusHealthy.Render("\u2713 Connected successfully!")
			// Auto-advance to next step after successful test
			m.nextWizardStep()
		} else {
			errMsg := "unknown error"
			if msg.Error != nil {
				errMsg = msg.Error.Error()
			}
			m.wizardTestResult = m.styles.StatusUnhealthy.Render(
				"\u2715 Connection failed: "+errMsg) +
				"\n  (r)etry credentials  (s)kip validation"
		}
		return m, nil

	case QueryStartMsg:
		m.querying = true
		m.consensusActive = false
		m.consensusContent.Reset()
		m.lastError = ""
		return m, m.spinner.Tick

	case QueryDoneMsg:
		m.querying = false
		// Save completed exchange to history
		exchange := Exchange{
			Prompt:             m.currentPrompt,
			ConsensusResponse:  m.consensusContent.String(),
			IndividualResponse: make(map[string]string),
		}
		for _, p := range m.panels {
			exchange.IndividualResponse[p.Name] = p.Content.String()
		}
		m.history = append(m.history, exchange)
		m.currentPrompt = ""
		// Update the chat view with full conversation
		m.chatView.SetContent(m.buildChatLog())
		m.chatView.GotoBottom()
		return m, nil

	case ProviderChunkMsg:
		for i := range m.panels {
			if m.panels[i].Name == msg.ProviderName {
				if msg.Error != nil {
					m.panels[i].Status = StatusFailed
					m.panels[i].Content.WriteString("\n[ERROR: " + msg.Error.Error() + "]")
					// Surface provider errors so they're visible without Tab
					m.lastError = fmt.Sprintf("%s: %s", msg.ProviderName, msg.Error.Error())
				} else if msg.Done {
					m.panels[i].Status = StatusDone
					if msg.Delta != "" {
						m.panels[i].Content.WriteString(msg.Delta)
					}
				} else {
					m.panels[i].Status = StatusLoading
					m.panels[i].Content.WriteString(msg.Delta)
				}
				m.panels[i].Viewport.SetContent(m.panels[i].Content.String())
				m.panels[i].Viewport.GotoBottom()
				break
			}
		}
		return m, nil

	case ConsensusChunkMsg:
		m.consensusActive = true
		if msg.Error != nil {
			m.lastError = msg.Error.Error()
			m.consensusContent.WriteString("\n[ERROR: " + msg.Error.Error() + "]")
		} else if msg.Done {
			// Successful completion — clear any prior error
			if m.consensusContent.Len() > 0 && m.lastError == "" {
				// only clear if we actually got content (not just Done after error)
			}
		} else {
			m.lastError = "" // clear error on first successful content
			m.consensusContent.WriteString(msg.Delta)
		}
		m.consensusView.SetContent(m.consensusContent.String())
		m.consensusView.GotoBottom()
		return m, nil

	case TokenUpdateMsg:
		if m.tokenUsage == nil {
			m.tokenUsage = make(map[string]tokenDisplay)
		}
		for _, u := range msg.Usage {
			td := tokenDisplay{
				Used:    tokens.FormatTokenCount(u.InputTokens),
				Percent: u.Percent(),
				HasData: u.InputTokens > 0 || u.OutputTokens > 0,
			}
			if u.Limit > 0 {
				td.Limit = tokens.FormatTokenCount(u.Limit)
			}
			m.tokenUsage[u.ProviderID] = td
		}
		return m, nil

	case spinner.TickMsg:
		if m.querying || m.testingProvider != "" || m.wizardTesting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update textarea (only in chat mode and not querying)
	if m.mode == viewChat && !m.querying {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)

		// Update slash command completions as user types
		input := strings.TrimSpace(m.textarea.Value())
		if strings.HasPrefix(input, "/") && len(input) > 1 {
			m.slashMatches = nil
			m.slashCompIdx = 0
			for _, cmd := range m.slashCommands {
				if strings.HasPrefix(cmd, input) {
					m.slashMatches = append(m.slashMatches, cmd)
				}
			}
		} else {
			m.slashMatches = nil
		}
	}

	// Update viewports (only in chat mode)
	if m.mode == viewChat {
		var cmd tea.Cmd
		m.chatView, cmd = m.chatView.Update(msg)
		cmds = append(cmds, cmd)

		for i := range m.panels {
			m.panels[i].Viewport, cmd = m.panels[i].Viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

		m.consensusView, cmd = m.consensusView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateChat handles key events when in chat mode.
func (m Model) updateChat(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "pgup", "shift+pgup":
		m.chatView.PageUp()
		return m, nil
	case "pgdown", "shift+pgdown":
		m.chatView.PageDown()
		return m, nil
	case "ctrl+u":
		m.chatView.HalfPageUp()
		return m, nil
	case "ctrl+d":
		m.chatView.HalfPageDown()
		return m, nil
	case "home":
		m.chatView.GotoTop()
		return m, nil
	case "end":
		m.chatView.GotoBottom()
		return m, nil
	case "up":
		// If textarea is empty, focus the tab bar
		if strings.TrimSpace(m.textarea.Value()) == "" && !m.tabBarFocused {
			m.tabBarFocused = true
			m.textarea.Blur()
			return m, nil
		}
	case "down":
		// Return focus to textarea from tab bar
		if m.tabBarFocused {
			m.tabBarFocused = false
			m.textarea.Focus()
			return m, nil
		}
	case "left":
		// Switch tabs when tab bar is focused
		if m.tabBarFocused && m.activeTab > 0 {
			m.activeTab--
			return m, nil
		}
	case "right":
		// Switch tabs when tab bar is focused
		if m.tabBarFocused {
			maxTab := len(m.panels)
			if m.activeTab < maxTab {
				m.activeTab++
			}
			return m, nil
		}
	case "esc":
		// Return focus to textarea from tab bar
		if m.tabBarFocused {
			m.tabBarFocused = false
			m.textarea.Focus()
			return m, nil
		}
	case "ctrl+s":
		if !m.querying {
			m.mode = viewSettings
			m.settingsCursor = 0
			m.confirmDelete = false
			m.settingsMsg = ""
			return m, nil
		}
	case "tab":
		// Tab accepts the current slash completion if one is shown
		input := strings.TrimSpace(m.textarea.Value())
		if strings.HasPrefix(input, "/") && !m.querying && len(m.slashMatches) > 0 {
			m.textarea.Reset()
			m.textarea.SetValue(m.slashMatches[m.slashCompIdx])
			m.slashMatches = nil
			m.slashCompPrefix = ""
			return m, nil
		}
		// Otherwise toggle provider panels
		m.showIndividual = !m.showIndividual
		return m, nil
	case "p":
		if !m.querying && strings.TrimSpace(m.textarea.Value()) == "" {
			m.showProvenance = !m.showProvenance
			return m, nil
		}
	case "enter":
		// Return focus to textarea from tab bar
		if m.tabBarFocused {
			m.tabBarFocused = false
			m.textarea.Focus()
			return m, nil
		}
		if !m.querying {
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" {
				// Check for slash commands
				if strings.HasPrefix(prompt, "/settings") {
					m.textarea.Reset()
					m.mode = viewSettings
					m.settingsCursor = 0
					m.confirmDelete = false
					m.settingsMsg = ""
					return m, nil
				}
				if strings.HasPrefix(prompt, "/mode ") {
					modeName := strings.TrimSpace(strings.TrimPrefix(prompt, "/mode "))
					m.textarea.Reset()
					switch modeName {
					case "quick", "balanced", "thorough":
						m.currentMode = modeName
						if m.onModeChange != nil {
							m.onModeChange(modeName)
						}
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/memory") {
					m.textarea.Reset()
					if m.onMemory != nil {
						m.onMemory(strings.TrimSpace(strings.TrimPrefix(prompt, "/memory")))
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
				if strings.HasPrefix(prompt, "/clear") {
					m.textarea.Reset()
					m.history = nil
					m.currentPrompt = ""
					m.lastError = ""
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
				if prompt == "/save" {
					m.textarea.Reset()
					if m.onSave != nil {
						m.onSave()
					}
					return m, nil
				}
				if strings.HasPrefix(prompt, "/export") {
					path := strings.TrimSpace(strings.TrimPrefix(prompt, "/export"))
					m.textarea.Reset()
					if m.onExport != nil {
						m.onExport(path)
					}
					return m, nil
				}
				m.currentPrompt = prompt
				m.textarea.Reset()
				m.resetPanels()
				if m.onSubmit != nil {
					m.onSubmit(prompt)
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

	providerCount := 0
	if m.cfg != nil {
		providerCount = len(m.cfg.Providers)
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = viewChat
		m.settingsMsg = ""
		m.textarea.Focus()
		return m, nil
	case "j", "down":
		if m.settingsCursor < providerCount-1 {
			m.settingsCursor++
		}
		m.settingsMsg = ""
		return m, nil
	case "k", "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		m.settingsMsg = ""
		return m, nil
	case "a":
		m.initWizardForAdd()
		return m, nil
	case "e":
		if providerCount > 0 {
			m.initWizardForEdit(m.settingsCursor)
		}
		return m, nil
	case "d":
		if providerCount > 0 {
			if m.cfg.Providers[m.settingsCursor].Primary {
				m.settingsMsg = m.styles.StatusUnhealthy.Render(
					"Cannot remove the primary provider. Change primary first.")
				return m, nil
			}
			m.confirmDelete = true
		}
		return m, nil
	case "t":
		if providerCount > 0 {
			name := m.cfg.Providers[m.settingsCursor].Name
			m.testingProvider = name
			m.settingsMsg = ""
			if m.onTestProvider != nil {
				m.onTestProvider(name)
			}
			return m, m.spinner.Tick
		}
		return m, nil
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

func (m *Model) resetPanels() {
	for i := range m.panels {
		m.panels[i].Status = StatusLoading
		m.panels[i].Content.Reset()
		m.panels[i].Viewport.SetContent("")
	}
	m.consensusContent.Reset()
	m.consensusView.SetContent("")
	m.consensusActive = false
}

func (m *Model) updateLayout() {
	inputHeight := 5 // textarea + border
	tabBarHeight := 1
	availableHeight := m.height - inputHeight - tabBarHeight - 2

	// Update textarea width
	m.textarea.SetWidth(m.width - 4)

	panelWidth := m.width - 4

	// All views get full width; the active tab gets all the height
	m.chatView.Width = panelWidth
	m.chatView.Height = max(availableHeight-3, 1)
	m.consensusView.Width = panelWidth
	m.consensusView.Height = max(availableHeight-3, 1)
	for i := range m.panels {
		m.panels[i].Viewport.Width = panelWidth
		m.panels[i].Viewport.Height = max(availableHeight-3, 1)
	}
}

// buildChatLog renders the full conversation history as a scrollable text log.
func (m Model) buildChatLog() string {
	var b strings.Builder

	for _, ex := range m.history {
		// User prompt (rendered as-is, no markdown)
		b.WriteString("❯ ")
		b.WriteString(ex.Prompt)
		b.WriteString("\n\n")
		// Consensus response (rendered as markdown with syntax highlighting)
		b.WriteString(renderMarkdown(ex.ConsensusResponse))
		b.WriteString("\n")
	}

	// If currently querying, show the in-progress prompt
	if m.currentPrompt != "" {
		b.WriteString("❯ ")
		b.WriteString(m.currentPrompt)
		b.WriteString("\n")
	}

	return b.String()
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
