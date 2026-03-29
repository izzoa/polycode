package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/notify"
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
	Description    string
	ResponseCh     chan ConfirmResult
	ToolName       string // e.g., "file_write", "shell_exec"
	RiskLevel      string // "read-only", "mutating", "destructive"
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
	ToolName  string
	Duration  time.Duration
	Error     string // empty if no error
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
		}
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

	case MCPCallCountMsg:
		m.mcpCallCount = msg.Count
		return m, nil

	case MCPRegistryResultsMsg:
		if msg.Error != nil {
			m.mcpRegistryOffline = true
			// Fall back to hardcoded list — convert PopularMCPServers to results.
			m.mcpRegistryResults = nil
			for i, s := range PopularMCPServers {
				m.mcpRegistryResults = append(m.mcpRegistryResults, MCPRegistryResult{
					Name:           s.Name,
					Description:    s.Description,
					TransportLabel: "npm/stdio",
					PackageID:      s.Command + " " + strings.Join(s.Args, " "),
					ServerData:     i, // index into PopularMCPServers
				})
			}
		} else {
			m.mcpRegistryOffline = false
			m.mcpRegistryResults = msg.Servers
		}
		// Reset browse cursor.
		m.mcpWizardListCursor = 0
		return m, nil

	case MCPDashboardDataMsg:
		m.mcpDashboardData = msg.Servers
		m.mcpDashboardTotal = msg.TotalTools
		m.mcpDashboardCalls = msg.TotalCalls
		return m, nil

	case MCPToolsChangedMsg:
		// Update tool count for the changed server in mcpServers.
		for i, s := range m.mcpServers {
			if s.Name == msg.ServerName {
				m.mcpServers[i].ToolCount = msg.ToolCount
				break
			}
		}
		return m, nil

	case MCPTestResultMsg:
		m.mcpTestingServer = ""
		m.mcpWizardTesting = false
		if msg.Success {
			result := m.styles.StatusHealthy.Render(
				fmt.Sprintf("✓ Connected (%d tools)", msg.ToolCount))
			m.settingsMsg = result
			m.mcpWizardTestResult = result
		} else {
			result := m.styles.StatusUnhealthy.Render("✗ Failed: " + msg.Error)
			m.settingsMsg = result
			m.mcpWizardTestResult = result
		}
		return m, nil

	case editorFinishedMsg:
		if msg.tempFile != "" {
			if msg.err != nil {
				m.chatStatusMsg = "Editor error: " + msg.err.Error()
			} else {
				content, err := os.ReadFile(msg.tempFile)
				if err == nil {
					// Always load content back, even if empty (allows clearing draft)
					m.textarea.SetValue(string(content))
					m.updateLayout()
				}
			}
			os.Remove(msg.tempFile)
		}
		m.textarea.Focus()
		return m, nil

	case ShellContextMsg:
		m.toolStatus = ""
		// Only inject if user hasn't typed something new since the ! command
		if strings.TrimSpace(m.textarea.Value()) != "" {
			// User has typed — don't overwrite, show as status instead
			m.chatStatusMsg = "Shell output ready (input not empty, skipped injection)"
			return m, nil
		}
		var contextBlock string
		if msg.Error != nil {
			// Preserve output even on non-zero exit (output may contain useful stderr)
			if msg.Output != "" {
				contextBlock = fmt.Sprintf("$ %s\n%s\n(exit: %s)", msg.Command, msg.Output, msg.Error)
			} else {
				contextBlock = fmt.Sprintf("$ %s\n(error: %s)", msg.Command, msg.Error)
			}
		} else {
			contextBlock = fmt.Sprintf("$ %s\n%s", msg.Command, msg.Output)
		}
		// Set the context block into the textarea for the user to add their question
		m.textarea.SetValue(contextBlock + "\n\n")
		m.updateLayout()
		return m, nil

	case TerminalFocusMsg:
		m.terminalFocused = msg.Focused
		return m, nil

	case tea.FocusMsg:
		m.terminalFocused = true
		return m, nil

	case tea.BlurMsg:
		m.terminalFocused = false
		return m, nil

	case UndoSnapshotMsg:
		m.undoStack = append(m.undoStack, msg.Snapshot)
		m.redoStack = nil // clear redo stack on new action
		return m, nil

	case UndoAppliedMsg:
		if msg.Error != nil {
			m.chatStatusMsg = "Undo failed: " + msg.Error.Error()
		} else if msg.IsRedo {
			m.chatStatusMsg = "⟳ Redo: " + msg.Description
		} else {
			// Pop from undo stack, push to redo stack
			if len(m.undoStack) > 0 {
				popped := m.undoStack[len(m.undoStack)-1]
				m.undoStack = m.undoStack[:len(m.undoStack)-1]
				m.redoStack = append(m.redoStack, popped)
			}
			remaining := len(m.undoStack)
			m.chatStatusMsg = fmt.Sprintf("⟲ Undone: %s (%d more undoable)", msg.Description, remaining)
		}
		return m, nil

	case SessionPickerMsg:
		m.sessionPickerData = msg.Sessions
		m.showSessionPicker = true
		m.sessionPickerCursor = 0
		m.sessionPickerFilter = ""
		m.sessionPickerRenaming = false
		return m, nil

	case PlanDoneMsg:
		m.planRunning = false
		m.sendNotification("polycode", "Plan execution finished")
		if msg.Error != nil {
			m.consensusContent.WriteString("\n[Plan Error: " + msg.Error.Error() + "]")
		} else if msg.FinalOutput != "" {
			m.consensusContent.WriteString(msg.FinalOutput)
		}
		m.consensusView.SetContent(m.consensusContent.String())
		if !m.autoScrollLocked {
			m.consensusView.GotoBottom()
		}
		m.consensusActive = true
		return m, nil

	case tea.KeyMsg:
		// Confirmation prompt takes priority over everything except quit
		if m.confirmPending {
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
		exchange := Exchange{
			Prompt:             m.currentPrompt,
			ConsensusResponse:  rawResponse,
			IndividualResponse: make(map[string]string),
			ProviderStatuses:   make(map[string]ProviderStatus),
			ProviderTraces:     make(map[string][]TraceSection),
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
				// Adaptive markdown re-render during streaming:
				// - Small delta (<100 chars since last render): 800ms throttle
				// - Large delta (>1000 chars since last render): immediate re-render
				// - Normal: 500ms throttle
				// Skip entirely if content hash hasn't changed.
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
			// This is the actual context window usage, not accumulated total
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
					m.sessionNameGen++       // prevent stale auto-names but keep current name
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

// closePalette resets all command palette state.
func (m *Model) closePalette() {
	m.paletteOpen = false
	m.paletteViaCtrlP = false
	m.paletteCursor = 0
	m.paletteFilter = ""
	m.paletteFiles = nil
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
