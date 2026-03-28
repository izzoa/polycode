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
type ProviderPanel struct {
	Name      string
	IsPrimary bool
	Status    ProviderStatus
	Content   *strings.Builder
	Viewport  viewport.Model

	// Phase-ordered trace sections for structured provider activity.
	TraceSections []TraceSection
	currentPhase  string // phase of the most recently appended section
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

// Exchange represents a completed prompt/response pair in history.
type Exchange struct {
	Prompt             string
	ConsensusResponse  string
	IndividualResponse map[string]string            // provider name → response
	ProviderTraces     map[string][]TraceSection     // provider name → ordered trace sections
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
	lastError string // cleared on next successful query or /clear

	// Token usage (updated after each query)
	tokenUsage map[string]tokenDisplay // provider name → display info

	// Splash screen
	showSplash bool
	version    string

	// Operating mode
	currentMode   string // "quick", "balanced", "thorough"
	routingReason string // why current providers were selected

	// Action confirmation
	confirmPending     bool
	confirmDescription string
	confirmResponseCh  chan bool

	// Command palette (triggered by /)
	slashCommands  []slashCommand // all available commands with descriptions
	paletteOpen    bool           // true when command palette overlay is visible
	paletteFilter  string         // current filter text
	paletteMatches []slashCommand // filtered commands

	// Tool execution status
	toolStatus string // e.g., "Reading main.go..." — shown in consensus panel during tool exec

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
	spinner        spinner.Model

	// Conversation viewport — scrollable chat log
	chatView viewport.Model

	// Layout
	width  int
	height int

	// Styles
	styles Styles

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

	// Help overlay
	showHelp bool

	// Config reference (needed for settings CRUD)
	cfg *config.Config

	// Callbacks (set by the app layer)
	onSubmit        func(prompt string)
	onClear         func()
	onPlan          func(request string)
	onModeChange    func(mode string)
	onMemory        func(args string)
	onSkill         func(subcommand, args string)
	onSessions      func(subcommand, args string)
	onSave          func()
	onExport        func(path string)
	onYoloToggle    func(enabled bool)
	onConfigChanged func(*config.Config)
	onTestProvider  func(providerName string)
	onMCP              func(subcommand, args string)
	onTestMCP          func(cfg config.MCPServerConfig)
	onReconnectMCP     func(serverName string)
	onMCPDashboardRefresh func() // triggers async fetch of dashboard data
	onMCPRegistryFetch  func()                                          // triggers async registry fetch for browse step
	onMCPRegistrySelect func(result MCPRegistryResult) config.MCPServerConfig // maps registry result to config (returns it)

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

func defaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Padding(0, 1),
		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1).
			Bold(true),
		StatusHealthy: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),
		StatusUnhealthy: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),
		StatusPrimary: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true),
		PanelBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		ConsensusBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(0, 1),
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")),
		Prompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("63")).
			Bold(true),
		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
	}
}

// NewModel creates a new TUI model.
func NewModel(providerNames []string, primaryName string, version string) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask polycode anything... (Enter to send, Shift+Enter for newline)"
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // no grey background on active line
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot

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
		showSplash:     true,
		version:        version,
		currentMode:    "balanced",
		showIndividual: true,
		spinner:        sp,
		history:        []Exchange{},
		inputHistIdx:   -1,
		styles:         defaultStyles(),
		mode:           viewChat,
		wizardInput:    ti,
		mcpWizardInput: mcpTI,
		slashCommands: []slashCommand{
			{"/clear", "Clear conversation and reset context", ""},
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
			{"/yolo", "Toggle auto-approve mode", ""},
			{"/exit", "Quit polycode", "ctrl+c"},
		},
		modePickerItems: []string{"quick", "balanced", "thorough"},
	}
}

// SetConfig sets the config reference on the model so settings screens can
// perform CRUD operations.
func (m *Model) SetConfig(cfg *config.Config) {
	m.cfg = cfg
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

// SetModelLister sets a callback that returns available models for a
// provider type. Used by the wizard to show a model list instead of
// requiring manual text entry.
func (m *Model) SetModelLister(lister func(providerType string) []config.ModelSummary) {
	m.modelLister = lister
}

// splashDoneMsg is sent when the splash timeout expires.
type splashDoneMsg struct{}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
			return splashDoneMsg{}
		}),
	)
}
