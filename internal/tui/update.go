package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anthonyizzo/polycode/internal/tokens"
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
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.showIndividual = !m.showIndividual
			return m, nil
		case "enter":
			if !m.querying {
				prompt := strings.TrimSpace(m.textarea.Value())
				if prompt != "" {
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
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
		if m.querying {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update textarea
	if !m.querying {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewports
	for i := range m.panels {
		var cmd tea.Cmd
		m.panels[i].Viewport, cmd = m.panels[i].Viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	var cmd tea.Cmd
	m.consensusView, cmd = m.consensusView.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
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
