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
	viewChat        viewMode = iota
	viewSettings
	viewAddProvider
	viewEditProvider
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

// ProviderPanel holds the state for one provider's response panel.
type ProviderPanel struct {
	Name      string
	IsPrimary bool
	Status    ProviderStatus
	Content   strings.Builder
	Viewport  viewport.Model
}

// tokenDisplay holds pre-formatted token usage info for one provider.
type tokenDisplay struct {
	Used    string  // formatted used count, e.g. "12.4K"
	Limit   string  // formatted limit, e.g. "200K", or "" if unlimited
	Percent float64 // 0-100, 0 if unlimited
	HasData bool    // false if provider reported zero usage
}

// Exchange represents a completed prompt/response pair in history.
type Exchange struct {
	Prompt             string
	ConsensusResponse  string
	IndividualResponse map[string]string // provider name → response
}

// Model is the main Bubble Tea model for the polycode TUI.
type Model struct {
	// Input
	textarea textarea.Model

	// Provider panels
	panels    []ProviderPanel
	providers []string // provider names in order

	// Consensus panel
	consensusContent strings.Builder
	consensusView    viewport.Model
	consensusActive  bool

	// Conversation state — full multi-turn dialogue
	history       []Exchange // completed exchanges for display
	currentPrompt string     // the prompt being processed right now

	// Token usage (updated after each query)
	tokenUsage map[string]tokenDisplay // provider name → display info

	// Splash screen
	showSplash bool
	version    string

	// Action confirmation
	confirmPending     bool
	confirmDescription string
	confirmResponseCh  chan bool

	// Tool execution status
	toolStatus string // e.g., "Reading main.go..." — shown in consensus panel during tool exec

	// State
	showIndividual bool
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

	// Wizard state (add/edit provider)
	wizardStep      wizardStep
	wizardData      config.ProviderConfig
	wizardInput     textinput.Model
	wizardListCursor int
	wizardListItems []string
	wizardEditing   bool   // true when editing an existing provider
	wizardEditIndex int    // index into config.Providers being edited
	wizardAPIKey    string // API key captured during stepAPIKey

	// Help overlay
	showHelp bool

	// Config reference (needed for settings CRUD)
	cfg *config.Config

	// Callbacks (set by the app layer)
	onSubmit        func(prompt string)
	onClear         func()
	onConfigChanged func(*config.Config)
	onTestProvider  func(providerName string)
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
			Viewport:  vp,
		}
	}

	consensusVP := viewport.New(0, 0)
	chatVP := viewport.New(0, 0)

	ti := textinput.New()
	ti.CharLimit = 256

	return Model{
		textarea:       ta,
		panels:         panels,
		providers:      providerNames,
		consensusView:  consensusVP,
		chatView:       chatVP,
		showSplash:     true,
		version:        version,
		showIndividual: true,
		spinner:        sp,
		history:        []Exchange{},
		styles:         defaultStyles(),
		mode:           viewChat,
		wizardInput:    ti,
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

// AppendHistory adds an exchange to the display history (used for session restore).
func (m *Model) AppendHistory(ex Exchange) {
	m.history = append(m.history, ex)
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
