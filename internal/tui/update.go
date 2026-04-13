package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/tokens"
)

// Messages for TUI updates from the query pipeline.

// TracePhase identifies which phase of provider activity a trace event belongs to.
type TracePhase string

const (
	PhaseFanout    TracePhase = "fanout"
	PhaseSynthesis TracePhase = "synthesis"
	PhaseTool      TracePhase = "tool"
	PhaseVerify    TracePhase = "verify"
)

// ProviderTraceMsg delivers a phase-aware trace event for a provider tab.
type ProviderTraceMsg struct {
	ProviderName string
	Phase        TracePhase
	Delta        string
	Done         bool
	Error        error
}

// ProviderChunkMsg delivers a streaming chunk from a provider.
type ProviderChunkMsg struct {
	ProviderName string
	Delta        string
	Done         bool
	Error        error
}

// ConsensusChunkMsg delivers a streaming chunk from the consensus synthesis.
type ConsensusChunkMsg struct {
	Delta  string
	Done   bool
	Error  error
	Status bool // true for tool status text (suppressed when concealTools is on)
}

// QueryStartMsg signals that a query has begun.
type QueryStartMsg struct {
	// QueriedProviders lists the provider IDs being queried. If empty,
	// all panels are set to loading (backward compat). If set, only the
	// listed providers show as loading — others stay idle.
	QueriedProviders []string
	// RoutingReason is a human-readable explanation of why these providers
	// were selected (e.g., "balanced: primary + best-scoring secondary").
	RoutingReason string
}

// QueryDoneMsg signals that the full pipeline (fan-out + consensus) is complete.
type QueryDoneMsg struct{}

// TokenUpdateMsg delivers a snapshot of token usage for all providers.
type TokenUpdateMsg struct {
	Usage []tokens.ProviderUsage
}

// ConfirmActionMsg asks the user to confirm an action.
// The ResponseCh is used to synchronously communicate the user's decision
// back to the goroutine that requested confirmation.
// ConfirmResult carries the user's confirmation decision, optionally with edited content.
type ConfirmResult struct {
	Approved      bool
	EditedContent *string // nil = use original; non-nil = substitute this content
}

type ConfirmActionMsg struct {
	Description     string
	ResponseCh      chan ConfirmResult
	ToolName        string // e.g., "file_write", "shell_exec"
	RiskLevel       string // "read-only", "mutating", "destructive"
	EditableContent string // the content the user can edit (command, file content, etc.)
}

// ToolCallMsg notifies the TUI that a tool is being executed.
type ToolCallMsg struct {
	ToolName    string
	Description string    // e.g., "Reading main.go" or "Running `go test`"
	StartTime   time.Time // when the tool call started (zero if not tracked)
}

// ToolCallDoneMsg notifies the TUI that a tool call has completed.
type ToolCallDoneMsg struct {
	ToolName string
	Duration time.Duration
	Error    string // empty if no error
}

// ConsensusAnalysisMsg delivers structured provenance from the consensus synthesis.
type ConsensusAnalysisMsg struct {
	Confidence string
	Agreements []string
	Minorities []string
	Evidence   []string
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

// TerminalFocusMsg reports terminal focus changes (from ANSI focus tracking).
type TerminalFocusMsg struct {
	Focused bool
}

// ShellContextMsg delivers the output of a ! shell command for context injection.
type ShellContextMsg struct {
	Command string
	Output  string
	Error   error
}

// SessionNameMsg delivers an auto-generated session name.
type SessionNameMsg struct {
	Name       string
	Generation int // must match m.sessionNameGen or message is stale
}

// editorFinishedMsg is sent when the external editor returns.
type editorFinishedMsg struct {
	tempFile string
	err      error
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case splashTickMsg:
		if m.showSplash {
			m.splashFrame++
			return m, tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
				return splashTickMsg{}
			})
		}
		return m, nil

	case SessionNameMsg:
		// Ignore stale auto-names from prior session/generation
		if msg.Generation != m.sessionNameGen {
			return m, nil
		}
		// Sanitize: collapse whitespace, truncate by rune count, strip punctuation
		name := strings.Join(strings.Fields(msg.Name), " ")
		runes := []rune(name)
		if len(runes) > 40 {
			runes = runes[:40]
		}
		name = strings.Trim(string(runes), "\"'`.,!?;:\n")
		name = strings.TrimSpace(name)
		if name != "" {
			m.sessionName = name
			cmd := m.addToast(ToastInfo, "Session: "+name)
			return m, cmd
		}
		return m, nil

	case ToastMsg:
		cmd := m.addToast(msg.Variant, msg.Text)
		return m, cmd

	case toastDismissMsg:
		m.dismissToast(msg.ID)
		return m, nil

	case ConfirmActionMsg:
		// Check session-level overrides first
		if m.sessionAllowed != nil && m.sessionAllowed[msg.ToolName] {
			msg.ResponseCh <- ConfirmResult{Approved: true}
			return m, nil
		}
		m.confirmPending = true
		m.confirmDescription = msg.Description
		m.confirmToolName = msg.ToolName
		m.confirmRiskLevel = msg.RiskLevel
		m.confirmResponseCh = msg.ResponseCh
		m.confirmEditContent = msg.EditableContent
		m.confirmEditing = false
		// Sanitize notification: only first line, truncated
		notifyBody := msg.Description
		if idx := strings.IndexByte(notifyBody, '\n'); idx >= 0 {
			notifyBody = notifyBody[:idx]
		}
		if len(notifyBody) > 80 {
			notifyBody = notifyBody[:77] + "..."
		}
		m.sendNotification("polycode", "Approval needed: "+notifyBody)
		return m, nil

	case ToolCallMsg:
		m.queryPhase = "executing"
		m.toolStatus = msg.Description
		startTime := msg.StartTime
		if startTime.IsZero() {
			startTime = time.Now()
		}
		m.toolCalls = append(m.toolCalls, ToolCallRecord{
			ToolName:    msg.ToolName,
			Description: msg.Description,
			StartTime:   startTime,
		})
		if !m.concealTools {
			m.consensusContent.WriteString("\n🔧 " + msg.Description + "\n")
			m.consensusView.SetContent(m.consensusContent.String())
			if !m.autoScrollLocked {
				m.consensusView.GotoBottom()
			}
		}
		return m, nil

	case ToolCallDoneMsg:
		// Mark the most recent matching tool call as done
		for i := len(m.toolCalls) - 1; i >= 0; i-- {
			if m.toolCalls[i].ToolName == msg.ToolName && !m.toolCalls[i].Done {
				m.toolCalls[i].Done = true
				m.toolCalls[i].Duration = msg.Duration
				m.toolCalls[i].Error = msg.Error
				break
			}
		}
		// Update the last tool call display with duration
		if !m.concealTools && msg.Duration > 0 {
			m.consensusContent.WriteString(fmt.Sprintf("  (%s)\n", msg.Duration.Round(time.Millisecond)))
			m.consensusView.SetContent(m.consensusContent.String())
			if !m.autoScrollLocked {
				m.consensusView.GotoBottom()
			}
		}
		return m, nil

	case ConsensusAnalysisMsg:
		m.consensusConfidence = msg.Confidence
		m.consensusAgreements = msg.Agreements
		m.minorityReports = msg.Minorities
		m.consensusEvidence = msg.Evidence
		return m, nil

	case ModeChangedMsg:
		m.currentMode = msg.Mode
		cmd := m.addToast(ToastInfo, "Mode: "+msg.Mode)
		return m, cmd

	case MemoryDisplayMsg:
		m.consensusContent.WriteString(msg.Content)
		m.consensusView.SetContent(m.consensusContent.String())
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
			m.setError("Plan failed", msg.Error.Error())
		} else if msg.FinalOutput != "" {
			m.consensusContent.WriteString(msg.FinalOutput)
			m.consensusRaw = msg.FinalOutput
			m.consensusRendered = renderMarkdown(msg.FinalOutput)
			m.consensusView.SetContent(m.consensusRendered)
		}
		return m, nil

	case UndoAppliedMsg:
		if msg.Error != nil {
			m.chatStatusMsg = "Undo failed: " + msg.Error.Error()
		} else {
			action := "Undo"
			if msg.IsRedo {
				action = "Redo"
			}
			m.chatStatusMsg = action + ": " + msg.Description
		}
		return m, nil

	case UndoSnapshotMsg:
		m.undoStack = append(m.undoStack, msg.Snapshot)
		return m, nil

	case ShellContextMsg:
		m.toolStatus = ""
		if msg.Error != nil {
			m.chatStatusMsg = "Shell error: " + msg.Error.Error()
		}
		// Inject shell output as context into next prompt
		contextBlock := fmt.Sprintf("Shell output of `%s`:\n```\n%s\n```\n", msg.Command, msg.Output)
		current := m.textarea.Value()
		if current != "" {
			m.textarea.SetValue(current + "\n\n" + contextBlock)
		} else {
			m.textarea.SetValue(contextBlock)
		}
		return m, nil

	case TerminalFocusMsg:
		m.terminalFocused = msg.Focused
		return m, nil

	case editorFinishedMsg:
		if msg.tempFile != "" {
			if msg.err != nil {
				m.chatStatusMsg = "Editor error: " + msg.err.Error()
			} else {
				content, err := os.ReadFile(msg.tempFile)
				if err == nil && len(content) > 0 {
					m.textarea.SetValue(string(content))
				}
			}
			os.Remove(msg.tempFile)
		}
		return m, nil

	case MCPStatusMsg:
		// Check if any server just connected (compare with previous state)
		for _, s := range msg.Servers {
			if s.Status == "connected" {
				wasConnected := false
				for _, old := range m.mcpServers {
					if old.Name == s.Name && old.Status == "connected" {
						wasConnected = true
						break
					}
				}
				if !wasConnected {
					cmd := m.addToast(ToastInfo, "MCP: "+s.Name+" connected")
					m.mcpServers = msg.Servers
					return m, cmd
				}
			}
		}
		m.mcpServers = msg.Servers
		return m, nil

	case MCPTestResultMsg:
		m.mcpTestingServer = ""
		if msg.Success {
			m.settingsMsg = m.styles.StatusHealthy.Render(
				fmt.Sprintf("✓ %s connected (%d tools)", msg.ServerName, msg.ToolCount))
		} else {
			m.settingsMsg = m.styles.StatusUnhealthy.Render(
				"✗ " + msg.ServerName + ": " + msg.Error)
		}
		return m, nil

	case MCPToolsChangedMsg:
		// Dynamic tool refresh — just update the status display
		cmd := m.addToast(ToastInfo, fmt.Sprintf("MCP tools updated: %s (%d tools)", msg.ServerName, msg.ToolCount))
		return m, cmd

	case MCPCallCountMsg:
		m.mcpCallCount = msg.Count
		return m, nil

	case MCPDashboardDataMsg:
		m.mcpDashboardData = msg.Servers
		m.mcpDashboardTotal = msg.TotalTools
		m.mcpDashboardCalls = msg.TotalCalls
		return m, nil

	case SessionPickerMsg:
		m.sessionPickerData = msg.Sessions
		m.showSessionPicker = true
		m.sessionPickerCursor = 0
		m.sessionPickerFilter = ""
		if msg.Error != nil {
			m.chatStatusMsg = "Error loading sessions: " + msg.Error.Error()
			m.showSessionPicker = false
		}
		return m, nil

	case tea.KeyMsg:
		// Confirmation prompt takes priority over everything except quit
		if m.confirmPending {
			return m.handleConfirmKey(msg)
		}

		// Any keypress dismisses splash (except ctrl+c which quits)
		if m.showSplash && msg.String() != "ctrl+c" {
			m.showSplash = false
			return m, nil
		}

		// Help overlay toggle — in chat mode, only when tab bar is focused
		if msg.String() == "?" && m.mode != viewAddProvider && m.mode != viewEditProvider {
			if m.mode != viewChat || m.tabBarFocused {
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

		// Theme picker intercepts all keys when open
		if m.themePickerOpen {
			switch msg.String() {
			case "up", "k":
				if m.themePickerCursor > 0 {
					m.themePickerCursor--
				}
			case "down", "j":
				if m.themePickerCursor < len(m.themePickerItems)-1 {
					m.themePickerCursor++
				}
			case "enter":
				// Apply selected theme
				name := m.themePickerItems[m.themePickerCursor]
				m.theme = ThemeByName(name)
				m.styles = defaultStyles(m.theme)
				rebuildMarkdownRenderer(m.theme)
				m.themePickerOpen = false
				// Invalidate all cached markdown so it re-renders with new theme
				for i := range m.history {
					m.history[i].renderedResponse = ""
				}
				m.rebuildChatLogCache()
				m.syncChatViewContent()
				m.chatStatusMsg = "Theme: " + m.theme.Name
				// Persist to config
				if m.cfg != nil {
					m.cfg.Theme = name
					_ = m.cfg.Save() // persist to disk
					if m.onConfigChanged != nil {
						m.onConfigChanged(m.cfg)
					}
				}
			case "esc":
				m.themePickerOpen = false
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		// Session picker intercepts all keys when open
		if m.showSessionPicker {
			return m.updateSessionPicker(msg)
		}

		// MCP dashboard intercepts all keys when open
		if m.showMCPDashboard {
			return m.updateMCPDashboard(msg)
		}

		// Mode picker overlay intercepts all keys when open
		if m.modePickerOpen {
			yoloIdx := len(m.modePickerItems) // yolo is the last item
			maxIdx := yoloIdx
			switch msg.String() {
			case "up", "k":
				if m.modePickerIdx > 0 {
					m.modePickerIdx--
				}
			case "down", "j":
				if m.modePickerIdx < maxIdx {
					m.modePickerIdx++
				}
			case "enter":
				if m.modePickerIdx == yoloIdx {
					// Toggle yolo
					m.yoloMode = !m.yoloMode
					if m.onYoloToggle != nil {
						m.onYoloToggle(m.yoloMode)
					}
				} else {
					// Select mode
					m.currentMode = m.modePickerItems[m.modePickerIdx]
					if m.onModeChange != nil {
						go m.onModeChange(m.currentMode)
					}
					m.modePickerOpen = false
					m.textarea.Focus()
				}
			case "esc", "ctrl+c":
				m.modePickerOpen = false
				m.textarea.Focus()
			}
			return m, nil
		}

		// Route key events by mode
		switch m.mode {
		case viewSettings:
			return m.updateSettings(msg)
		case viewAddProvider, viewEditProvider:
			return m.updateWizard(msg)
		case viewAddMCP, viewEditMCP:
			return m.updateMCPWizard(msg)
		default:
			return m.updateChat(msg)
		}

	case tea.MouseMsg:
		return m.updateMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil

	case ConfigChangedMsg:
		// Handled by the app layer callback; we also update local state.
		m.cfg = msg.Config
		m.rebuildPanelsFromConfig()
		cmd := m.addToast(ToastSuccess, "Config saved")
		return m, cmd

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
		m.queryPhase = "dispatching"
		m.consensusActive = false
		m.consensusContent.Reset()
		m.consensusRaw = ""
		m.consensusRendered = ""
		m.lastRenderLen = 0
		m.lastRenderHash = 0
		m.lastRenderTime = time.Time{} // allow immediate first render
		m.clearError()
		m.toolCalls = nil
		m.routingReason = msg.RoutingReason
		m.resetPanels()
		if len(msg.QueriedProviders) > 0 {
			m.markPanelsQueried(msg.QueriedProviders)
		} else {
			// Backward compat: mark all panels as loading
			for i := range m.panels {
				m.panels[i].Status = StatusLoading
			}
		}
		return m, m.spinner.Tick

	case QueryDoneMsg:
		m.querying = false
		m.queryPhase = ""
		m.sendNotification("polycode", "Consensus ready")
		// Save completed exchange to history using raw (pre-markdown) text
		rawResponse := m.consensusRaw
		if rawResponse == "" {
			rawResponse = m.consensusContent.String() // fallback if Done never fired
		}
		// Capture provider order and primary at time of exchange.
		provOrder := make([]string, len(m.panels))
		var primaryName string
		for i, p := range m.panels {
			provOrder[i] = p.Name
			if p.IsPrimary {
				primaryName = p.Name
			}
		}
		exchange := Exchange{
			Prompt:             m.currentPrompt,
			ConsensusResponse:  rawResponse,
			IndividualResponse: make(map[string]string),
			ProviderStatuses:   make(map[string]ProviderStatus),
			ProviderTraces:     make(map[string][]TraceSection),
			ProviderOrder:      provOrder,
			PrimaryProvider:    primaryName,
			ToolCalls:          append([]ToolCallRecord(nil), m.toolCalls...),
		}
		for _, p := range m.panels {
			exchange.IndividualResponse[p.Name] = p.Content.String()
			exchange.ProviderStatuses[p.Name] = p.Status
			if len(p.TraceSections) > 0 {
				sections := make([]TraceSection, len(p.TraceSections))
				copy(sections, p.TraceSections)
				exchange.ProviderTraces[p.Name] = sections
			}
		}
		m.history = append(m.history, exchange)
		m.currentPrompt = ""
		m.rebuildChatLogCache()
		m.syncChatViewContent()
		if !m.autoScrollLocked {
			m.chatView.GotoBottom()
		}
		// Auto-name session after first exchange if unnamed
		if len(m.history) == 1 && m.sessionName == "" && m.onAutoNameSession != nil {
			m.onAutoNameSession(exchange.Prompt, m.sessionNameGen)
		}
		return m, nil

	case ProviderTraceMsg:
		for i := range m.panels {
			if m.panels[i].Name == msg.ProviderName {
				if msg.Error != nil {
					m.panels[i].Status = StatusFailed
					m.panels[i].appendTraceContent(msg.Phase, "\n[ERROR: "+msg.Error.Error()+"]")
					m.setError(msg.ProviderName+" failed", msg.Error.Error())
				} else if msg.Done {
					m.panels[i].Status = StatusDone
					// Render markdown on completion for formatted display
					m.panels[i].Viewport.SetContent(renderMarkdown(m.panels[i].Content.String()))
				} else {
					// Don't downgrade from Failed back to Loading.
					if m.panels[i].Status != StatusFailed {
						m.panels[i].Status = StatusLoading
					}
					m.panels[i].appendTraceContent(msg.Phase, msg.Delta)
					// Per-panel throttled markdown rendering
					raw := m.panels[i].Content.String()
					now := time.Now()
					if now.Sub(m.panels[i].lastRenderTime) > 500*time.Millisecond {
						m.panels[i].Viewport.SetContent(renderMarkdown(raw))
						m.panels[i].lastRenderTime = now
					} else {
						m.panels[i].Viewport.SetContent(raw)
					}
				}
				if !m.autoScrollLocked {
					m.panels[i].Viewport.GotoBottom()
				}
				break
			}
		}
		return m, nil

	case ProviderChunkMsg:
		if m.queryPhase == "dispatching" {
			m.queryPhase = "thinking"
		}
		for i := range m.panels {
			if m.panels[i].Name == msg.ProviderName {
				if msg.Error != nil {
					m.panels[i].Status = StatusFailed
					m.panels[i].Content.WriteString("\n[ERROR: " + msg.Error.Error() + "]")
					// Surface provider errors so they're visible without Tab
					m.setError(msg.ProviderName+" failed", msg.Error.Error())
				} else if msg.Done {
					m.panels[i].Status = StatusDone
					if msg.Delta != "" {
						m.panels[i].Content.WriteString(msg.Delta)
					}
					// Render markdown on completion
					m.panels[i].Viewport.SetContent(renderMarkdown(m.panels[i].Content.String()))
				} else {
					m.panels[i].Status = StatusLoading
					m.panels[i].Content.WriteString(msg.Delta)
					// Per-panel throttled markdown rendering
					raw := m.panels[i].Content.String()
					now := time.Now()
					if now.Sub(m.panels[i].lastRenderTime) > 500*time.Millisecond {
						m.panels[i].Viewport.SetContent(renderMarkdown(raw))
						m.panels[i].lastRenderTime = now
					} else {
						m.panels[i].Viewport.SetContent(raw)
					}
				}
				if !m.autoScrollLocked {
					m.panels[i].Viewport.GotoBottom()
				}
				break
			}
		}
		return m, nil

	case ConsensusChunkMsg:
		// Suppress tool status text when concealed
		if msg.Status && m.concealTools {
			return m, nil
		}
		if m.queryPhase != "executing" {
			m.queryPhase = "synthesizing"
		}
		m.consensusActive = true
		if msg.Error != nil {
			m.setError("Consensus error", msg.Error.Error())
			m.consensusContent.WriteString("\n[ERROR: " + msg.Error.Error() + "]")
			m.consensusRendered = m.consensusContent.String()
			cmds = append(cmds, func() tea.Msg { return ToastMsg{ToastError, msg.Error.Error()} })
		} else {
			// Process Delta first — a message can carry both Delta and Done
			// (e.g., /mcp search sends Delta+Done in one message).
			if msg.Delta != "" {
				m.clearError()
				m.consensusContent.WriteString(msg.Delta)
			}

			if msg.Done {
				// Stream complete — final render of accumulated content.
				m.clearError()
				if m.consensusContent.Len() > 0 {
					m.consensusRaw = m.consensusContent.String()
					m.consensusRendered = renderMarkdown(m.consensusRaw)
				}
			} else if msg.Delta != "" {
				// Adaptive markdown re-render during streaming
				now := time.Now()
				currentLen := m.consensusContent.Len()
				deltaSinceRender := currentLen - m.lastRenderLen
				var throttle time.Duration
				switch {
				case deltaSinceRender > 1000:
					throttle = 0 // immediate
				case deltaSinceRender < 100:
					throttle = 800 * time.Millisecond
				default:
					throttle = 500 * time.Millisecond
				}
				if now.Sub(m.lastRenderTime) > throttle {
					content := m.consensusContent.String()
					h := fnvHash(content)
					if h != m.lastRenderHash {
						m.consensusRendered = renderMarkdown(content)
						m.lastRenderHash = h
					}
					m.lastRenderTime = now
					m.lastRenderLen = currentLen
				}
			}
		}
		m.consensusView.SetContent(m.consensusRendered)
		if !m.autoScrollLocked {
			m.consensusView.GotoBottom()
		}
		m.syncChatViewContent()
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case TokenUpdateMsg:
		if m.tokenUsage == nil {
			m.tokenUsage = make(map[string]tokenDisplay)
		}
		var maxPercent float64
		for _, u := range msg.Usage {
			// Show last request's input tokens as "Used" (matches context %)
			usedTokens := u.LastInputTokens
			if usedTokens == 0 {
				usedTokens = u.InputTokens // fallback for providers that don't report per-request
			}
			td := tokenDisplay{
				Used:    tokens.FormatTokenCount(usedTokens),
				Percent: u.Percent(),
				HasData: u.InputTokens > 0 || u.OutputTokens > 0,
				Cost:    tokens.FormatCost(u.Cost),
			}
			if u.Limit > 0 {
				td.Limit = tokens.FormatTokenCount(u.Limit)
			}
			m.tokenUsage[u.ProviderID] = td
			if td.Percent > maxPercent {
				maxPercent = td.Percent
			}
		}
		// Context pressure warning — only fire when crossing into a new band
		if maxPercent >= 95 && m.lastWarningBand < 95 {
			m.chatStatusMsg = fmt.Sprintf("⚠ Context at %.0f%% — approaching limit!", maxPercent)
			m.lastWarningBand = 95
		} else if maxPercent >= 80 && m.lastWarningBand < 80 {
			m.chatStatusMsg = fmt.Sprintf("⚠ Context at %.0f%% — consider /compact", maxPercent)
			m.lastWarningBand = 80
		} else if maxPercent < 80 && m.lastWarningBand > 0 {
			m.lastWarningBand = 0 // reset when usage drops
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

		// Update command palette based on textarea content.
		// Palette shows automatically when input starts with "/".
		// Skip if palette is in Ctrl+P modal mode.
		if !m.paletteViaCtrlP {
			input := strings.TrimSpace(m.textarea.Value())
			if strings.HasPrefix(input, "/") && !m.querying {
				// Extract filter text (everything after the leading /)
				filter := input[1:]
				// Only filter up to the first space (don't filter on arguments)
				if idx := strings.Index(filter, " "); idx >= 0 {
					filter = filter[:idx]
				}
				oldFilter := m.paletteFilter
				m.paletteOpen = true
				m.paletteFilter = filter
				m.paletteMatches = m.filterPaletteCommands(filter)
				m.paletteFiles = nil
				// Reset cursor when filter changes
				if filter != oldFilter {
					m.paletteCursor = 0
				}
			} else if !m.paletteViaCtrlP {
				m.paletteOpen = false
			}
		}

		// Update file picker based on @ trigger in textarea content.
		m.updateFilePickerState()

		// Recalculate layout if textarea height should change.
		m.updateLayout()
	}

	// Update viewports (only in chat mode)
	if m.mode == viewChat {
		// Sync content before viewport update so scroll calculations are accurate.
		m.syncChatViewContent()
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
