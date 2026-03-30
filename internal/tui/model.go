package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/izzoa/polycode/internal/config"
)

// viewMode represents which screen the TUI is currently showing.
type viewMode int

const (
	viewChat         viewMode = iota
	viewSettings
	viewAddProvider
	viewEditProvider
	viewAddMCP
	viewEditMCP
)

// wizardStep represents a step in the add/edit provider wizard.
type wizardStep int

const (
	stepType    wizardStep = iota
	stepName
	stepAuth
	stepAPIKey
	stepModel
	stepBaseURL
	stepPrimary
	stepConfirm
)

// ProviderStatus tracks the state of a single provider's response.
type ProviderStatus int

const (
	StatusIdle ProviderStatus = iota
	StatusLoading
	StatusDone
	StatusFailed
	StatusTimedOut
)

// TraceSection holds accumulated content for one phase of provider activity.
type TraceSection struct {
	Phase   string // "fanout", "synthesis", "tool", "verify"
	Content string
}

// ProviderPanel holds the state for one provider's response panel.
// StatusCancelled indicates the user cancelled this provider mid-query.
const StatusCancelled ProviderStatus = 5

type ProviderPanel struct {
	Name      string
	IsPrimary bool
	Status    ProviderStatus
	Content   *strings.Builder
	Viewport  viewport.Model

	// Phase-ordered trace sections for structured provider activity.
	TraceSections  []TraceSection
	currentPhase   string    // phase of the most recently appended section
	lastRenderTime time.Time // per-panel markdown render throttle
}

// appendTraceContent appends delta text to the panel's trace, inserting a
// phase header when the phase changes. It also writes to Content so the
// viewport stays in sync.
func (p *ProviderPanel) appendTraceContent(phase TracePhase, delta string) {
	ph := string(phase)
	if ph != p.currentPhase {
		// Start a new section
		p.TraceSections = append(p.TraceSections, TraceSection{Phase: ph})
		p.currentPhase = ph
		// Write a visible phase label into Content
		label := phaseLabel(phase)
		if p.Content.Len() > 0 {
			p.Content.WriteString("\n")
		}
		p.Content.WriteString(label + "\n")
	}
	// Append to the current section
	if len(p.TraceSections) > 0 {
		idx := len(p.TraceSections) - 1
		p.TraceSections[idx].Content += delta
	}
	p.Content.WriteString(delta)
}

// phaseLabel returns a human-readable header for a trace phase.
func phaseLabel(phase TracePhase) string {
	switch phase {
	case PhaseFanout:
		return "── Fan-out ──"
	case PhaseSynthesis:
		return "── Synthesis ──"
	case PhaseTool:
		return "── Tool Execution ──"
	case PhaseVerify:
		return "── Verification ──"
	default:
		return "── " + string(phase) + " ──"
	}
}

// tokenDisplay holds pre-formatted token usage info for one provider.
type tokenDisplay struct {
	Used    string  // formatted used count, e.g. "12.4K"
	Limit   string  // formatted limit, e.g. "200K", or "" if unlimited
	Cost    string  // formatted cost, e.g. "$0.12", or "" if no pricing data
	Percent float64 // 0-100, 0 if unlimited
	HasData bool    // false if provider reported zero usage
}

// agentWorkerDisplay holds display info for a single worker in a /plan job.
type agentWorkerDisplay struct {
	Role     string
	Provider string
	Status   string // "pending", "running", "complete"
	Summary  string
}

// agentStageDisplay holds display info for a stage in a /plan job.
type agentStageDisplay struct {
	Name    string
	Workers []agentWorkerDisplay
}

// slashCommand defines a command available in the command palette.
type slashCommand struct {
	Name        string // e.g., "/clear"
	Description string // e.g., "Clear conversation and reset context"
	Shortcut    string // e.g., "ctrl+s" (optional)
}

// ToolCallRecord tracks a single tool call for display.
type ToolCallRecord struct {
	ToolName    string
	Description string
	StartTime   time.Time
	Duration    time.Duration
	Done        bool
	Error       string
}

// Exchange represents a completed prompt/response pair in history.
type Exchange struct {
	Prompt             string
	ConsensusResponse  string
	IndividualResponse map[string]string            // provider name → response
	ProviderStatuses   map[string]ProviderStatus     // provider name → final status
	ProviderTraces     map[string][]TraceSection     // provider name → ordered trace sections
	ToolCalls          []ToolCallRecord              // tool calls executed during this exchange
	expandedTrace      bool                          // whether trace sections are expanded (default: collapsed after completion)
	renderedResponse   string                        // cached glamour-rendered markdown (computed once)
}

// Model is the main Bubble Tea model for the polycode TUI.
type Model struct {
	// Input
	textarea textarea.Model

	// Provider panels
	panels    []ProviderPanel
	providers []string // provider names in order

	// Consensus panel
	consensusContent  *strings.Builder // accumulates raw streamed text
	consensusRaw      string           // raw text preserved for history (set on Done)
	consensusRendered string           // periodically re-rendered markdown for live display
	lastRenderTime    time.Time        // throttle: last time we ran glamour on streaming content
	lastRenderLen     int              // content length at last render (for delta tracking)
	lastRenderHash    uint64           // FNV hash of last rendered content (skip if unchanged)
	consensusView     viewport.Model
	consensusActive   bool

	// Consensus provenance
	showProvenance      bool
	consensusConfidence string   // "high", "medium", "low", ""
	consensusAgreements []string // key agreement points
	minorityReports     []string // dissenting views
	consensusEvidence   []string // cited evidence

	// Conversation state — full multi-turn dialogue
	history       []Exchange // completed exchanges for display
	currentPrompt string     // the prompt being processed right now
	chatLogCache  string     // cached rendered chat log (rebuilt only when history changes)
	chatLogDirty  bool       // true when history changed and cache needs rebuild

	// Input history — allows cycling through previous prompts with up/down
	inputHistory []string // all submitted prompts in order
	inputHistIdx int      // current position in history (-1 = not browsing)
	inputDraft   string   // saved draft when entering history browsing

	// Error display — surfaced prominently in the chat area
	lastError   string       // cleared on next successful query or /clear
	errorRecord *ErrorRecord // structured error for panel display

	// Toast notifications
	toasts      []Toast
	nextToastID int

	// Transient status message — shown briefly in chat input area
	chatStatusMsg string // e.g., "Copied to clipboard" — cleared on next keypress

	// Token usage (updated after each query)
	tokenUsage       map[string]tokenDisplay // provider name → display info
	lastWarningBand  int                     // 0=none, 80=warned at 80%, 95=warned at 95%

	// Splash screen
	showSplash       bool
	splashFrame      int    // animation frame counter (0 = start)
	version          string
	splashSessionMsg string // e.g., "Resuming session from 2h ago, 5 exchanges"

	// Operating mode
	currentMode   string // "quick", "balanced", "thorough"
	routingReason string // why current providers were selected

	// Action confirmation
	confirmPending      bool
	confirmDescription  string
	confirmToolName     string
	confirmRiskLevel    string
	confirmResponseCh   chan ConfirmResult
	confirmEditing      bool           // true when in edit sub-mode
	confirmEditContent  string         // editable content for the tool call
	confirmEditTextarea textarea.Model // textarea for editing

	// Session-level approval overrides
	sessionAllowed map[string]bool // tool patterns allowed for session (e.g., "file_write" → true)

	// Command palette (triggered by / or Ctrl+P)
	slashCommands   []slashCommand // all available commands with descriptions
	paletteOpen     bool           // true when command palette overlay is visible
	paletteFilter   string         // current filter text
	paletteMatches  []slashCommand // filtered commands
	paletteCursor   int            // currently selected item in command palette
	paletteViaCtrlP bool           // true when opened via Ctrl+P (shows files too)
	paletteFiles    []fileMatch    // fuzzy-matched files (only when opened via Ctrl+P)

	// File picker (triggered by @ in input)
	fileIdx          *fileIndex  // project file index for fuzzy search
	filePickerOpen   bool        // true when file picker overlay is visible
	filePickerFilter string      // current filter text after @
	filePickerMatches []fileMatch // fuzzy-matched files
	filePickerCursor int         // selected item in file picker
	attachedFiles    []string    // files attached via @ references

	// Tool execution status
	toolStatus    string           // e.g., "Reading main.go..." — shown in consensus panel during tool exec
	toolCalls     []ToolCallRecord // tool calls in the current turn
	concealTools  bool             // when true, tool calls shown as summary line

	// Agent team (/plan) state
	planRunning bool
	agentStages []agentStageDisplay

	// State
	showIndividual bool
	activeTab       int  // -1 = mode selector, 0 = consensus, 1..N = provider panels
	tabBarFocused   bool // true when tab bar has focus (arrow keys switch tabs)
	yoloMode        bool // auto-approve all tool actions
	modePickerOpen  bool // true when mode selection overlay is visible
	modePickerIdx   int  // cursor position in the mode picker
	modePickerItems []string
	querying       bool
	queryPhase     string // current phase: "dispatching", "thinking", "synthesizing", "executing", "verifying"
	spinner        spinner.Model

	// Conversation viewport — scrollable chat log
	chatView viewport.Model

	// Layout
	width  int
	height int

	// Theme and styles
	theme            Theme
	styles           Styles
	themePickerOpen  bool
	themePickerCursor int
	themePickerItems []string

	// View mode — which screen is displayed
	mode viewMode

	// Settings screen state
	settingsCursor int
	confirmDelete  bool
	settingsMsg    string // transient status message shown in settings
	testingProvider string // provider name currently being tested

	// MCP state
	mcpServers        []MCPServerStatus // populated via MCPStatusMsg
	mcpCallCount      int64             // total MCP tool calls (updated from MCPClient)
	showMCPDashboard  bool
	mcpDashboardData  []MCPDashboardServer
	mcpDashboardTotal int
	mcpDashboardCalls int64
	mcpDashboardCursor int
	mcpRegistryResults []MCPRegistryResult // live registry search results for wizard browse
	mcpRegistryOffline bool               // true if registry was unreachable
	mcpSettingsCursor int
	mcpSettingsFocused bool              // true = cursor in MCP section (Tab toggles)
	mcpConfirmDelete   bool
	mcpTestingServer   string            // server name currently being tested

	// MCP Wizard state
	mcpWizardStep       mcpWizardStep
	mcpWizardData       config.MCPServerConfig
	mcpWizardEnv        map[string]string
	mcpWizardEnvSecrets map[string]bool   // tracks which env vars are secrets (for masking)
	mcpWizardEnvOrder   []string          // ordered list of known env var names (required first)
	mcpWizardEnvIdx     int               // current prompting index; >= len(order) means freeform mode
	mcpWizardEnvDescs   map[string]string // env var descriptions from registry metadata
	mcpWizardInput      textinput.Model
	mcpWizardListCursor int
	mcpWizardListItems  []string
	mcpWizardEditing    bool   // true when editing an existing server
	mcpWizardEditIndex  int    // index into config.MCP.Servers being edited
	mcpWizardSource     string // "popular" or "custom"
	mcpWizardTesting    bool
	mcpWizardTestResult string

	// Wizard state (add/edit provider)
	wizardStep       wizardStep
	wizardData       config.ProviderConfig
	wizardInput      textinput.Model
	wizardListCursor int
	wizardListItems  []string
	wizardEditing    bool   // true when editing an existing provider
	wizardEditIndex  int    // index into config.Providers being edited
	wizardAPIKey     string // API key captured during stepAPIKey

	// Smart wizard state
	wizardModelSummaries []config.ModelSummary // cached model list for current provider
	wizardCustomModel    bool                  // true when user selected "Custom model..." in model step
	wizardTesting        bool                  // true while connection test is running
	wizardTestResult     string                // result message from connection test

	// Split pane layout
	splitPaneEnabled bool // auto-enabled at ≥140 cols
	splitRatio       int  // left panel percentage (default 60)
	splitPanelIdx    int  // right panel provider index (-1 = hidden)

	// Desktop notifications
	notifyEnabled   bool // opt-in via config
	terminalFocused bool // tracked via terminal focus events

	// Auto-scroll control
	autoScrollLocked bool // when true, skip GotoBottom during streaming

	// Undo/redo stack (git-backed)
	undoStack []UndoSnapshot // snapshots available for undo
	redoStack []UndoSnapshot // snapshots available for redo

	// Session picker overlay
	showSessionPicker    bool
	sessionPickerData    []config.SessionInfo
	sessionPickerCursor  int
	sessionPickerFilter  string
	sessionPickerRenaming bool
	sessionPickerRenameInput string

	// Help overlay
	showHelp bool

	// Config reference (needed for settings CRUD)
	cfg *config.Config

	// Callbacks (set by the app layer)
	onShellContext  func(command string) // runs shell command and sends ShellContextMsg back
	onSubmit        func(prompt string)
	onClear         func()
	onPlan          func(request string)
	onModeChange    func(mode string)
	onMemory        func(args string)
	onSkill         func(subcommand, args string)
	onSessions      func(subcommand, args string)
	onSave          func()
	onExport          func(path string)
	onExportMarkdown  func()
	onShareSession    func()
	onCancelProvider  func(providerName string) // cancels a specific provider mid-query
	onUndo           func()
	onRedo           func()
	onYoloToggle    func(enabled bool)
	onConfigChanged func(*config.Config)
	onTestProvider  func(providerName string)
	onMCP              func(subcommand, args string)
	onTestMCP          func(cfg config.MCPServerConfig)
	onReconnectMCP     func(serverName string)
	onMCPDashboardRefresh func() // triggers async fetch of dashboard data
	onMCPRegistryFetch  func()                                          // triggers async registry fetch for browse step
	onMCPRegistrySelect func(result MCPRegistryResult) config.MCPServerConfig // maps registry result to config (returns it)

	// Session auto-naming
	onAutoNameSession  func(prompt string, gen int) // triggers async naming via primary model
	sessionName        string              // current session display name
	sessionNameGen     int                 // generation counter — incremented on /clear and /name to ignore stale auto-names

	// Session picker
	onSessionPickerRefresh func() // triggers async fetch of session list

	// Model listing for wizard
	modelLister func(providerType string) []config.ModelSummary
}

// Styles holds all lipgloss styles for the TUI.
type Styles struct {
	App             lipgloss.Style
	StatusBar       lipgloss.Style
	StatusHealthy   lipgloss.Style
	StatusUnhealthy lipgloss.Style
	StatusPrimary   lipgloss.Style
	PanelBorder     lipgloss.Style
	ConsensusBorder lipgloss.Style
	InputBorder     lipgloss.Style
	Title           lipgloss.Style
	Prompt          lipgloss.Style
	Dimmed          lipgloss.Style
}

func defaultStyles(t Theme) Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Padding(0, 1),
		StatusBar: lipgloss.NewStyle().
			Background(t.BgPanel).
			Foreground(t.Text).
			Padding(0, 1).
			Bold(true),
		StatusHealthy: lipgloss.NewStyle().
			Foreground(t.Success),
		StatusUnhealthy: lipgloss.NewStyle().
			Foreground(t.Error),
		StatusPrimary: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),
		PanelBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderNormal).
			Padding(0, 1),
		ConsensusBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderAccent).
			Padding(0, 1),
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocused).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary),
		Prompt: lipgloss.NewStyle().
			Foreground(t.Secondary).
			Bold(true),
		Dimmed: lipgloss.NewStyle().
			Foreground(t.TextMuted),
	}
}

// NewModel creates a new TUI model.
func NewModel(providerNames []string, primaryName string, version string) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask polycode anything... (Enter to send, Shift+Enter for newline)"
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // no grey background on active line
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    80 * time.Millisecond,
	}

	panels := make([]ProviderPanel, len(providerNames))
	for i, name := range providerNames {
		vp := viewport.New(0, 0)
		panels[i] = ProviderPanel{
			Name:      name,
			IsPrimary: name == primaryName,
			Status:    StatusIdle,
			Content:   &strings.Builder{},
			Viewport:  vp,
		}
	}

	consensusVP := viewport.New(0, 0)
	chatVP := viewport.New(0, 0)

	ti := textinput.New()
	ti.CharLimit = 256

	mcpTI := textinput.New()
	mcpTI.CharLimit = 512

	return Model{
		textarea:         ta,
		panels:           panels,
		providers:        providerNames,
		consensusContent: &strings.Builder{},
		consensusView:    consensusVP,
		chatView:         chatVP,
		showSplash:      true,
		splitRatio:      60,
		splitPanelIdx:   -1,
		version:        version,
		currentMode:    "balanced",
		showIndividual: true,
		spinner:        sp,
		history:        []Exchange{},
		inputHistIdx:   -1,
		theme:           PolycodeDefault,
		styles:          defaultStyles(PolycodeDefault),
		themePickerItems: BuiltinThemeNames(),
		mode:           viewChat,
		wizardInput:    ti,
		mcpWizardInput: mcpTI,
		slashCommands: []slashCommand{
			{"/clear", "Clear conversation and reset context", ""},
			{"/compact", "Compact conversation context (summarize)", ""},
			{"/compose", "Open external editor to compose prompt", "ctrl+e"},
			{"/conceal", "Toggle tool call concealment", "ctrl+h"},
			{"/copy", "Copy last response to clipboard", "y"},
			{"/export [path]", "Export session as JSON", ""},
			{"/help", "Show keyboard shortcuts", "?"},
			{"/memory", "View repo memory", ""},
			{"/mode <name>", "Switch mode: quick, balanced, thorough", ""},
			{"/mode quick", "Fast responses, fewer providers", ""},
			{"/mode balanced", "Default — balance speed and quality", ""},
			{"/mode thorough", "Maximum quality, all providers", ""},
			{"/name <name>", "Name the current session", ""},
			{"/plan <request>", "Run multi-model agent team", ""},
			{"/save", "Save session to disk", ""},
			{"/share", "Copy session as markdown to clipboard", ""},
			{"/sessions", "Sessions: list, show, delete, name", ""},
			{"/sessions list", "List all saved sessions", ""},
			{"/sessions show <name>", "Show a saved session", ""},
			{"/sessions delete <name>", "Delete a saved session", ""},
			{"/sessions name <name>", "Name the current session", ""},
			{"/settings", "Open provider settings", "ctrl+s"},
			{"/skill", "Skills: list, install, remove", ""},
			{"/skill list", "List installed skills", ""},
			{"/skill install <path>", "Install a skill from a directory", ""},
			{"/skill remove <name>", "Remove an installed skill", ""},
			{"/mcp", "MCP: list, status, reconnect, tools, resources, prompts, search, add, remove", ""},
			{"/mcp list", "List MCP servers and their tools", ""},
			{"/mcp status", "Show MCP server connection status", ""},
			{"/mcp reconnect [name]", "Reconnect MCP server(s)", ""},
			{"/mcp tools [server]", "List tools from MCP servers", ""},
			{"/mcp resources [server]", "List resources from MCP servers", ""},
			{"/mcp prompts [server]", "List prompts from MCP servers", ""},
			{"/mcp search <query>", "Search the MCP server registry", ""},
			{"/mcp add", "Open MCP server wizard", ""},
			{"/mcp remove <name>", "Remove an MCP server", ""},
			{"/theme", "Switch color theme", "ctrl+t"},
			{"/undo", "Undo last file change", ""},
			{"/redo", "Redo last undone change", ""},
			{"/yolo", "Toggle auto-approve mode", ""},
			{"/exit", "Quit polycode", "ctrl+c"},
		},
		modePickerItems: []string{"quick", "balanced", "thorough"},
	}
}

// SetConfig sets the config reference on the model so settings screens can
// perform CRUD operations. Also applies the persisted theme if set.
func (m *Model) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	m.cfg = cfg
	// Rebuild panels so disabled providers appear with strikethrough on launch
	m.rebuildPanelsFromConfig()
	if cfg.Theme != "" {
		t := ThemeByName(cfg.Theme)
		m.theme = t
		m.styles = defaultStyles(t)
		rebuildMarkdownRenderer(t)
		// Invalidate cached markdown renders so they re-render with new theme
		for i := range m.history {
			m.history[i].renderedResponse = ""
		}
	}
}

// SetSubmitHandler sets the callback for when the user submits a prompt.
func (m *Model) SetSubmitHandler(handler func(prompt string)) {
	m.onSubmit = handler
}

// SetClearHandler sets the callback for when the user runs /clear.
func (m *Model) SetClearHandler(handler func()) {
	m.onClear = handler
}

// SetPlanHandler sets the callback for when the user runs /plan.
func (m *Model) SetPlanHandler(handler func(request string)) {
	m.onPlan = handler
}

// SetModeChangeHandler sets the callback for /mode command.
func (m *Model) SetModeChangeHandler(handler func(mode string)) {
	m.onModeChange = handler
}

// SetYoloToggleHandler sets the callback for /yolo toggle.
func (m *Model) SetYoloToggleHandler(handler func(enabled bool)) {
	m.onYoloToggle = handler
}

// SetMemoryHandler sets the callback for /memory command.
func (m *Model) SetMemoryHandler(handler func(args string)) {
	m.onMemory = handler
}

// SetSkillHandler sets the callback for /skill command.
func (m *Model) SetSkillHandler(handler func(subcommand, args string)) {
	m.onSkill = handler
}

// SetSessionsHandler sets the callback for /sessions command.
func (m *Model) SetSessionsHandler(handler func(subcommand, args string)) {
	m.onSessions = handler
}

// SetSaveHandler sets the callback for /save command.
func (m *Model) SetSaveHandler(handler func()) {
	m.onSave = handler
}

// SetExportHandler sets the callback for /export command.
func (m *Model) SetExportHandler(handler func(path string)) {
	m.onExport = handler
}

// AppendHistory adds an exchange to the display history (used for session restore).
func (m *Model) AppendHistory(ex Exchange) {
	m.history = append(m.history, ex)
	m.rebuildChatLogCache()
}

// RestorePanelsFromLastExchange populates provider panels with content from
// the most recent exchange. Called after session restore so individual
// provider responses are visible in the tab bar, not just consensus.
func (m *Model) RestorePanelsFromLastExchange() {
	if len(m.history) == 0 {
		return
	}
	last := m.history[len(m.history)-1]

	// Restore consensus view.
	if last.ConsensusResponse != "" {
		m.consensusContent.Reset()
		m.consensusContent.WriteString(last.ConsensusResponse)
		m.consensusView.SetContent(last.ConsensusResponse)
		m.consensusActive = true
	}

	// Restore individual provider panels.
	for i := range m.panels {
		name := m.panels[i].Name
		if content, ok := last.IndividualResponse[name]; ok && content != "" {
			m.panels[i].Content.Reset()
			m.panels[i].Content.WriteString(content)
			m.panels[i].Viewport.SetContent(content)
			m.panels[i].Status = StatusDone
		}
		// Restore trace sections if available.
		if traces, ok := last.ProviderTraces[name]; ok {
			m.panels[i].TraceSections = make([]TraceSection, len(traces))
			copy(m.panels[i].TraceSections, traces)
		}
	}

	// Sync the chat view so the viewport has correct content for scrolling.
	m.syncChatViewContent()
}

// SetConfigChangeHandler sets the callback invoked when the config changes
// from the settings screen (add/edit/delete provider). The app layer uses
// this to rebuild the registry and pipeline.
func (m *Model) SetConfigChangeHandler(handler func(*config.Config)) {
	m.onConfigChanged = handler
}

// SetTestProviderHandler sets the callback invoked when the user presses
// 't' in the settings screen to test a provider connection.
func (m *Model) SetTestProviderHandler(handler func(providerName string)) {
	m.onTestProvider = handler
}

// SetMCPHandler sets the callback for /mcp command.
func (m *Model) SetMCPHandler(handler func(subcommand, args string)) {
	m.onMCP = handler
}

// SetTestMCPHandler sets the callback for testing an MCP server connection.
func (m *Model) SetTestMCPHandler(handler func(cfg config.MCPServerConfig)) {
	m.onTestMCP = handler
}

// SetReconnectMCPHandler sets the callback for reconnecting an MCP server.
func (m *Model) SetReconnectMCPHandler(handler func(serverName string)) {
	m.onReconnectMCP = handler
}

// SetMCPDashboardRefreshHandler sets the callback for refreshing dashboard data.
func (m *Model) SetMCPDashboardRefreshHandler(handler func()) {
	m.onMCPDashboardRefresh = handler
}

// SetMCPRegistryFetchHandler sets the callback for triggering registry fetch.
func (m *Model) SetMCPRegistryFetchHandler(handler func()) {
	m.onMCPRegistryFetch = handler
}

// SetMCPRegistrySelectHandler sets the callback for mapping a registry result to a config.
// The callback returns a config.MCPServerConfig that the wizard applies directly.
func (m *Model) SetMCPRegistrySelectHandler(handler func(result MCPRegistryResult) config.MCPServerConfig) {
	m.onMCPRegistrySelect = handler
}

// SetMCPWizardFromConfig populates the wizard data from a mapped MCPServerConfig
// (used when selecting a server from the registry).
func (m *Model) SetMCPWizardFromConfig(cfg config.MCPServerConfig) {
	m.mcpWizardData = cfg
	m.mcpWizardEnv = make(map[string]string)
	for k, v := range cfg.Env {
		m.mcpWizardEnv[k] = v
	}
}

// SetSplashSessionInfo sets the session info shown on the splash screen.
func (m *Model) SetSplashSessionInfo(msg string) {
	m.splashSessionMsg = msg
}

// SetNotifyEnabled enables or disables desktop notifications.
func (m *Model) SetNotifyEnabled(enabled bool) {
	m.notifyEnabled = enabled
	m.terminalFocused = true // assume focused at start
}

// SetCancelProviderHandler sets the callback for cancelling a provider mid-query.
func (m *Model) SetCancelProviderHandler(handler func(providerName string)) {
	m.onCancelProvider = handler
}

// SetUndoHandler sets the callback for /undo.
func (m *Model) SetUndoHandler(handler func()) {
	m.onUndo = handler
}

// SetRedoHandler sets the callback for /redo.
func (m *Model) SetRedoHandler(handler func()) {
	m.onRedo = handler
}

// SetExportMarkdownHandler sets the callback for /export md.
func (m *Model) SetExportMarkdownHandler(handler func()) {
	m.onExportMarkdown = handler
}

// SetShareSessionHandler sets the callback for /share.
func (m *Model) SetShareSessionHandler(handler func()) {
	m.onShareSession = handler
}

// SetSessionName sets the display session name.
func (m *Model) SetSessionName(name string) {
	m.sessionName = name
}

// SetAutoNameSessionHandler sets the callback for auto-naming sessions.
func (m *Model) SetAutoNameSessionHandler(handler func(prompt string, gen int)) {
	m.onAutoNameSession = handler
}

// SetSessionPickerRefreshHandler sets the callback for refreshing session picker data.
func (m *Model) SetSessionPickerRefreshHandler(handler func()) {
	m.onSessionPickerRefresh = handler
}

// SetModelLister sets a callback that returns available models for a
// provider type. Used by the wizard to show a model list instead of
// requiring manual text entry.
func (m *Model) SetModelLister(lister func(providerType string) []config.ModelSummary) {
	m.modelLister = lister
}

// SetShellContextHandler sets the callback for ! shell context injection.
func (m *Model) SetShellContextHandler(handler func(command string)) {
	m.onShellContext = handler
}

// InitFileIndex builds the file index for @ file references.
func (m *Model) InitFileIndex(projectRoot string) {
	m.fileIdx = newFileIndex(projectRoot)
}

// removeAttachedFile removes a file from the attached files list by index.
func (m *Model) removeAttachedFile(idx int) {
	if idx >= 0 && idx < len(m.attachedFiles) {
		m.attachedFiles = append(m.attachedFiles[:idx], m.attachedFiles[idx+1:]...)
	}
}

// splashTickMsg advances the splash animation by one frame.
type splashTickMsg struct{}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		// Splash animation: tick every 100ms for 30 frames (3s animation).
		// No auto-dismiss — user must press a key to continue.
		tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return splashTickMsg{}
		}),
	)
}
