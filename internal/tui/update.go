package tui

import (
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
type ConfirmActionMsg struct {
	Description string
	OnConfirm   func()
	OnReject    func()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case splashDoneMsg:
		m.showSplash = false
		return m, nil

	case tea.KeyMsg:
		// Any keypress dismisses splash (except ctrl+c which quits)
		if m.showSplash && msg.String() != "ctrl+c" {
			m.showSplash = false
			return m, nil
		}

		// Help overlay toggle — available from any view
		if msg.String() == "?" && m.mode != viewAddProvider && m.mode != viewEditProvider {
			m.showHelp = !m.showHelp
			return m, nil
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

	case QueryStartMsg:
		m.querying = true
		m.consensusActive = false
		m.consensusContent.Reset()
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
				} else if msg.Done {
					m.panels[i].Status = StatusDone
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
			m.consensusContent.WriteString("\n[ERROR: " + msg.Error.Error() + "]")
		} else if !msg.Done {
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
		if m.querying || m.testingProvider != "" {
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
	}

	// Update viewports (only in chat mode)
	if m.mode == viewChat {
		for i := range m.panels {
			var cmd tea.Cmd
			m.panels[i].Viewport, cmd = m.panels[i].Viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

		var cmd tea.Cmd
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
	case "ctrl+s":
		if !m.querying {
			m.mode = viewSettings
			m.settingsCursor = 0
			m.confirmDelete = false
			m.settingsMsg = ""
			return m, nil
		}
	case "tab":
		m.showIndividual = !m.showIndividual
		return m, nil
	case "enter":
		if !m.querying {
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" {
				// Check for /settings command
				if strings.HasPrefix(prompt, "/settings") {
					m.textarea.Reset()
					m.mode = viewSettings
					m.settingsCursor = 0
					m.confirmDelete = false
					m.settingsMsg = ""
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
	statusBarHeight := 1
	availableHeight := m.height - inputHeight - statusBarHeight - 2

	// Update textarea width
	m.textarea.SetWidth(m.width - 4)

	panelWidth := m.width - 4

	if m.querying && m.showIndividual {
		// During a query with individual panels visible: split between
		// chat history, provider panels, and consensus panel
		chatHeight := availableHeight / 3
		panelAreaHeight := availableHeight - chatHeight
		panelCount := len(m.panels) + 1 // +1 for consensus
		perPanel := panelAreaHeight / max(panelCount, 1)

		m.chatView.Width = panelWidth
		m.chatView.Height = max(chatHeight-3, 1)

		for i := range m.panels {
			m.panels[i].Viewport.Width = panelWidth
			m.panels[i].Viewport.Height = max(perPanel-3, 1)
		}
		m.consensusView.Width = panelWidth
		m.consensusView.Height = max(perPanel-3, 1)
	} else if m.querying {
		// During a query, consensus-only view
		chatHeight := availableHeight / 2
		consensusHeight := availableHeight - chatHeight

		m.chatView.Width = panelWidth
		m.chatView.Height = max(chatHeight-3, 1)
		m.consensusView.Width = panelWidth
		m.consensusView.Height = max(consensusHeight-3, 1)
	} else {
		// Idle: chat log takes all the space
		m.chatView.Width = panelWidth
		m.chatView.Height = max(availableHeight-3, 1)
		m.consensusView.Width = panelWidth
		m.consensusView.Height = max(availableHeight/2-3, 1)
	}
}

// buildChatLog renders the full conversation history as a scrollable text log.
func (m Model) buildChatLog() string {
	var b strings.Builder

	for _, ex := range m.history {
		// User prompt
		b.WriteString("❯ ")
		b.WriteString(ex.Prompt)
		b.WriteString("\n\n")
		// Consensus response
		b.WriteString(ex.ConsensusResponse)
		b.WriteString("\n\n")
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
