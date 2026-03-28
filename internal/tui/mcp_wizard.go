package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
)

// MCPRegistryEnvMeta holds env var metadata for display in the wizard.
type MCPRegistryEnvMeta struct {
	Name        string
	Description string
	IsSecret    bool
	IsRequired  bool
}

// MCPRegistryResult holds a server from the MCP Registry for the wizard browse step.
type MCPRegistryResult struct {
	Name           string
	Description    string
	TransportLabel string
	PackageID      string
	EnvVars        []MCPRegistryEnvMeta // env var metadata for individual prompting
	// Full server data for config mapping (passed to app layer).
	ServerData any // *mcp.RegistryServer — stored as any to avoid import cycle
}

// MCPRegistryResultsMsg delivers registry search results to the TUI.
type MCPRegistryResultsMsg struct {
	Servers []MCPRegistryResult
	Error   error
}

// MCPServerStatus holds display info for one MCP server in the settings view.
type MCPServerStatus struct {
	Name      string
	Transport string // "stdio" or "sse"
	Status    string // "connected", "failed", "disconnected"
	ToolCount int
	Error     string // populated on failure
}

// MCPStatusMsg is sent from app.go after MCP connect attempts to update the
// settings view with server status information.
type MCPStatusMsg struct {
	Servers []MCPServerStatus
}

// MCPToolsChangedMsg notifies the TUI that an MCP server's tool list has changed.
type MCPToolsChangedMsg struct {
	ServerName string
	ToolCount  int
}

// MCPCallCountMsg updates the MCP tool call count in the TUI.
type MCPCallCountMsg struct {
	Count int64
}

// MCPTestResultMsg delivers the result of a connection test for an MCP server.
type MCPTestResultMsg struct {
	ServerName string
	Success    bool
	ToolCount  int
	Error      string
}

// mcpWizardStep represents a step in the add/edit MCP server wizard.
type mcpWizardStep int

const (
	mcpStepSource    mcpWizardStep = iota // Select: "Popular servers" / "Custom server"
	mcpStepBrowse                         // Browse curated registry
	mcpStepTransport                      // Select transport: stdio / SSE
	mcpStepName                           // Enter server name
	mcpStepCommand                        // Enter command (stdio)
	mcpStepArgs                           // Enter arguments (stdio)
	mcpStepURL                            // Enter server URL (SSE)
	mcpStepEnv                            // Add environment variables
	mcpStepReadOnly                       // Mark as read-only?
	mcpStepTest                           // Connection test (auto-triggered)
	mcpStepConfirm                        // Review and save
)

// MCPTemplateEnvVar describes an env var in a server template.
type MCPTemplateEnvVar struct {
	Name     string
	IsSecret bool
}

// MCPServerTemplate defines a pre-configured MCP server for the curated registry.
type MCPServerTemplate struct {
	Name        string
	Description string
	Command     string
	Args        []string
	EnvVars     []MCPTemplateEnvVar
	ReadOnly    bool
	Category    string
}

// PopularMCPServers is the curated registry of well-known MCP servers.
var PopularMCPServers = []MCPServerTemplate{
	{
		Name:        "filesystem",
		Description: "Read/write local files and directories",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "{PATH}"},
		Category:    "filesystem",
	},
	{
		Name:        "github",
		Description: "GitHub API — repos, issues, PRs, search",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-github"},
		EnvVars:     []MCPTemplateEnvVar{{Name: "GITHUB_TOKEN", IsSecret: true}},
		Category:    "dev-tools",
	},
	{
		Name:        "postgres",
		Description: "Query PostgreSQL databases (read-only)",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-postgres", "{CONNECTION_STRING}"},
		ReadOnly:    true,
		Category:    "database",
	},
	{
		Name:        "brave-search",
		Description: "Web search via Brave Search API",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-brave-search"},
		EnvVars:     []MCPTemplateEnvVar{{Name: "BRAVE_API_KEY", IsSecret: true}},
		ReadOnly:    true,
		Category:    "search",
	},
	{
		Name:        "memory",
		Description: "Persistent knowledge graph memory",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-memory"},
		Category:    "ai",
	},
	{
		Name:        "puppeteer",
		Description: "Browser automation and web scraping",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-puppeteer"},
		Category:    "dev-tools",
	},
	{
		Name:        "sqlite",
		Description: "Query SQLite databases",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-sqlite", "{DB_PATH}"},
		ReadOnly:    true,
		Category:    "database",
	},
	{
		Name:        "slack",
		Description: "Slack workspace integration",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-slack"},
		EnvVars:     []MCPTemplateEnvVar{{Name: "SLACK_BOT_TOKEN", IsSecret: true}, {Name: "SLACK_TEAM_ID", IsSecret: false}},
		Category:    "dev-tools",
	},
}

// mcpShouldShowStep returns true if the given wizard step should be shown
// based on the current wizard state.
func (m *Model) mcpShouldShowStep(step mcpWizardStep) bool {
	switch step {
	case mcpStepBrowse:
		return m.mcpWizardSource == "popular"
	case mcpStepCommand, mcpStepArgs:
		return m.mcpWizardData.URL == "" // stdio transport
	case mcpStepURL:
		return m.mcpWizardData.URL != "" || m.mcpWizardData.Command == "" // SSE transport
	default:
		return true
	}
}

// initMCPWizardForAdd resets the MCP wizard state for adding a new server.
func (m *Model) initMCPWizardForAdd() {
	m.mode = viewAddMCP
	m.mcpWizardStep = mcpStepSource
	m.mcpWizardData = config.MCPServerConfig{}
	m.mcpWizardEnv = make(map[string]string)
	m.mcpWizardEnvSecrets = make(map[string]bool)
	m.mcpWizardEnvOrder = nil
	m.mcpWizardEnvIdx = -1
	m.mcpWizardEnvDescs = make(map[string]string)
	m.mcpWizardEditing = false
	m.mcpWizardSource = ""
	m.mcpWizardTesting = false
	m.mcpWizardTestResult = ""
	m.mcpWizardListCursor = 0
	m.mcpWizardListItems = []string{"Popular servers", "Custom server"}
	m.mcpWizardInput.Reset()
}

// initMCPWizardForEdit sets up the MCP wizard for editing an existing server.
func (m *Model) initMCPWizardForEdit(index int) {
	if m.cfg == nil || index >= len(m.cfg.MCP.Servers) {
		return
	}
	srv := m.cfg.MCP.Servers[index]
	m.mode = viewEditMCP
	m.mcpWizardStep = mcpStepTransport // skip source/browse
	m.mcpWizardData = srv
	m.mcpWizardEnv = make(map[string]string)
	m.mcpWizardEnvSecrets = make(map[string]bool)
	m.mcpWizardEnvOrder = nil
	m.mcpWizardEnvIdx = -1
	m.mcpWizardEnvDescs = make(map[string]string)
	for k, v := range srv.Env {
		m.mcpWizardEnv[k] = v
		m.mcpWizardEnvOrder = append(m.mcpWizardEnvOrder, k)
		// Infer secret-ness from $KEYRING: references.
		if strings.HasPrefix(v, "$KEYRING:") {
			m.mcpWizardEnvSecrets[k] = true
		}
	}
	if len(m.mcpWizardEnvOrder) > 0 {
		m.mcpWizardEnvIdx = 0
	}
	m.mcpWizardEditing = true
	m.mcpWizardEditIndex = index
	m.mcpWizardTesting = false
	m.mcpWizardTestResult = ""
	m.mcpWizardListCursor = 0
	if srv.URL != "" {
		m.mcpWizardListItems = []string{"stdio (subprocess)", "SSE (HTTP)"}
		m.mcpWizardListCursor = 1
	} else {
		m.mcpWizardListItems = []string{"stdio (subprocess)", "SSE (HTTP)"}
		m.mcpWizardListCursor = 0
	}
	m.mcpWizardInput.Reset()
}

// nextMCPWizardStep advances to the next applicable wizard step.
func (m *Model) nextMCPWizardStep() {
	for {
		m.mcpWizardStep++
		if m.mcpWizardStep > mcpStepConfirm {
			m.mcpWizardStep = mcpStepConfirm
			return
		}
		if m.mcpShouldShowStep(m.mcpWizardStep) {
			return
		}
	}
}

// prevMCPWizardStep goes back to the previous applicable wizard step.
func (m *Model) prevMCPWizardStep() {
	for {
		if m.mcpWizardStep == 0 {
			return
		}
		m.mcpWizardStep--
		if m.mcpShouldShowStep(m.mcpWizardStep) {
			return
		}
	}
}

// renderMCPWizard renders the MCP server add/edit wizard.
func (m Model) renderMCPWizard() string {
	var sections []string

	action := "Add"
	if m.mcpWizardEditing {
		action = "Edit"
	}
	title := m.styles.Title.Render(fmt.Sprintf("MCP Wizard — %s Server", action))
	sections = append(sections, title)
	sections = append(sections, "")

	// Step indicator
	totalSteps := 0
	currentStep := 0
	for s := mcpStepSource; s <= mcpStepConfirm; s++ {
		if m.mcpShouldShowStep(s) {
			totalSteps++
			if s < m.mcpWizardStep {
				currentStep++
			} else if s == m.mcpWizardStep {
				currentStep++
			}
		}
	}
	stepIndicator := m.styles.Dimmed.Render(fmt.Sprintf("Step %d of %d", currentStep, totalSteps))
	sections = append(sections, stepIndicator)
	sections = append(sections, "")

	switch m.mcpWizardStep {
	case mcpStepSource:
		sections = append(sections, "Select source:")
		sections = append(sections, "")
		sections = append(sections, m.renderMCPList()...)

	case mcpStepBrowse:
		if m.mcpRegistryOffline {
			sections = append(sections, m.styles.Dimmed.Render("(offline — showing built-in servers)"))
		} else {
			sections = append(sections, "Select a server from the MCP Registry:")
		}
		sections = append(sections, "")
		if len(m.mcpRegistryResults) > 0 {
			for i, r := range m.mcpRegistryResults {
				cursor := "    "
				if i == m.mcpWizardListCursor {
					cursor = m.styles.Prompt.Render("  > ")
				}
				desc := r.Description
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}
				sections = append(sections, fmt.Sprintf("%s%-22s %-10s %s", cursor, r.Name, m.styles.Dimmed.Render(r.TransportLabel), desc))
			}
		} else {
			sections = append(sections, m.spinner.View()+" Loading from registry...")
		}

	case mcpStepTransport:
		sections = append(sections, "Select transport:")
		sections = append(sections, "")
		m.mcpWizardListItems = []string{"stdio (subprocess)", "SSE (HTTP)"}
		sections = append(sections, m.renderMCPList()...)

	case mcpStepName:
		sections = append(sections, "Server name:")
		sections = append(sections, "")
		m.mcpWizardInput.Placeholder = "e.g., filesystem"
		sections = append(sections, m.mcpWizardInput.View())

	case mcpStepCommand:
		sections = append(sections, "Command to run:")
		sections = append(sections, "")
		m.mcpWizardInput.Placeholder = "e.g., npx"
		sections = append(sections, m.mcpWizardInput.View())

	case mcpStepArgs:
		sections = append(sections, "Arguments (space-separated):")
		sections = append(sections, "")
		m.mcpWizardInput.Placeholder = "e.g., -y @modelcontextprotocol/server-filesystem /path"
		sections = append(sections, m.mcpWizardInput.View())

	case mcpStepURL:
		sections = append(sections, "Server URL:")
		sections = append(sections, "")
		m.mcpWizardInput.Placeholder = "e.g., http://localhost:3000/mcp"
		sections = append(sections, m.mcpWizardInput.View())

	case mcpStepEnv:
		inKnownVarMode := m.mcpWizardEnvIdx >= 0 && m.mcpWizardEnvIdx < len(m.mcpWizardEnvOrder)

		if inKnownVarMode {
			// Individual prompting mode — prompt one known var at a time.
			currentVar := m.mcpWizardEnvOrder[m.mcpWizardEnvIdx]
			sections = append(sections, fmt.Sprintf("Environment variable %d of %d", m.mcpWizardEnvIdx+1, len(m.mcpWizardEnvOrder)))
			sections = append(sections, "")

			// Show already-entered vars above.
			for i := 0; i < m.mcpWizardEnvIdx; i++ {
				k := m.mcpWizardEnvOrder[i]
				v := m.mcpWizardEnv[k]
				if m.mcpWizardEnvSecrets[k] && v != "" {
					sections = append(sections, fmt.Sprintf("  %s = •••••••• (configured)", k))
				} else if v != "" {
					sections = append(sections, fmt.Sprintf("  %s = %s", k, v))
				} else {
					sections = append(sections, fmt.Sprintf("  %s = (skipped)", k))
				}
			}
			if m.mcpWizardEnvIdx > 0 {
				sections = append(sections, "")
			}

			// Current var prompt.
			label := currentVar
			if m.mcpWizardEnvSecrets[currentVar] {
				label += " (secret)"
			}
			if desc, ok := m.mcpWizardEnvDescs[currentVar]; ok && desc != "" {
				label += " — " + desc
			}
			sections = append(sections, label+":")
			sections = append(sections, m.mcpWizardInput.View())
		} else {
			// Freeform mode — show all entered vars + freeform input.
			sections = append(sections, "Environment variables:")
			sections = append(sections, "")
			for _, k := range m.mcpWizardEnvOrder {
				v := m.mcpWizardEnv[k]
				if m.mcpWizardEnvSecrets[k] && v != "" {
					sections = append(sections, fmt.Sprintf("  %s = •••••••• (configured)", k))
				} else if v != "" {
					sections = append(sections, fmt.Sprintf("  %s = %s", k, v))
				}
			}
			// Also show any extra freeform vars.
			knownSet := make(map[string]bool)
			for _, k := range m.mcpWizardEnvOrder {
				knownSet[k] = true
			}
			for k, v := range m.mcpWizardEnv {
				if !knownSet[k] && v != "" {
					sections = append(sections, fmt.Sprintf("  %s = %s", k, v))
				}
			}
			sections = append(sections, "")
			m.mcpWizardInput.Placeholder = "KEY=value (or Enter to continue)"
			sections = append(sections, m.mcpWizardInput.View())
		}

	case mcpStepReadOnly:
		sections = append(sections, "Mark as read-only? (tools skip confirmation)")
		sections = append(sections, "")
		items := []string{"No", "Yes"}
		for i, item := range items {
			cursor := "  "
			if i == m.mcpWizardListCursor {
				cursor = m.styles.Prompt.Render("> ")
			}
			sections = append(sections, cursor+item)
		}
		m.mcpWizardListItems = items

	case mcpStepTest:
		sections = append(sections, "Connection Test")
		sections = append(sections, "")
		if m.mcpWizardTesting {
			sections = append(sections, m.spinner.View()+" Testing connection to "+m.mcpWizardData.Name+"...")
		} else if m.mcpWizardTestResult != "" {
			sections = append(sections, m.mcpWizardTestResult)
			sections = append(sections, "")
			sections = append(sections, m.styles.Dimmed.Render("Enter:continue  Esc:back"))
		} else {
			// Auto-trigger test when entering this step.
			sections = append(sections, "Preparing test...")
		}

	case mcpStepConfirm:
		sections = append(sections, m.renderMCPSummary()...)
	}

	// Spacer
	contentLines := len(sections)
	remaining := m.height - contentLines - 4
	if remaining > 0 {
		sections = append(sections, strings.Repeat("\n", remaining))
	}

	// Hints
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	switch m.mcpWizardStep {
	case mcpStepSource, mcpStepBrowse, mcpStepTransport, mcpStepReadOnly:
		sections = append(sections, hintStyle.Render("j/k:navigate  Enter:select  Esc:cancel"))
	case mcpStepTest:
		if m.mcpWizardTesting {
			sections = append(sections, hintStyle.Render("Testing...  Esc:skip"))
		} else {
			sections = append(sections, hintStyle.Render("Enter:continue  Esc:back"))
		}
	case mcpStepConfirm:
		sections = append(sections, hintStyle.Render("Enter:save  Esc:cancel"))
	default:
		sections = append(sections, hintStyle.Render("Enter:next  Esc:back"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.styles.App.Width(m.width).Render(content)
}

// renderMCPList renders a simple selectable list.
func (m Model) renderMCPList() []string {
	var lines []string
	for i, item := range m.mcpWizardListItems {
		cursor := "  "
		if i == m.mcpWizardListCursor {
			cursor = m.styles.Prompt.Render("> ")
		}
		lines = append(lines, cursor+item)
	}
	return lines
}


// renderMCPSummary renders the confirmation/review step.
func (m Model) renderMCPSummary() []string {
	var lines []string
	lines = append(lines, "Review:")
	lines = append(lines, "")

	d := m.mcpWizardData
	lines = append(lines, fmt.Sprintf("  Name:       %s", d.Name))
	if d.URL != "" {
		lines = append(lines, fmt.Sprintf("  Transport:  SSE"))
		lines = append(lines, fmt.Sprintf("  URL:        %s", d.URL))
	} else {
		lines = append(lines, fmt.Sprintf("  Transport:  stdio"))
		cmd := d.Command
		if len(d.Args) > 0 {
			cmd += " " + strings.Join(d.Args, " ")
		}
		lines = append(lines, fmt.Sprintf("  Command:    %s", cmd))
	}
	if len(m.mcpWizardEnv) > 0 {
		envKeys := make([]string, 0, len(m.mcpWizardEnv))
		for k := range m.mcpWizardEnv {
			envKeys = append(envKeys, k)
		}
		lines = append(lines, fmt.Sprintf("  Env:        %s", strings.Join(envKeys, ", ")))
	}
	ro := "no"
	if d.ReadOnly {
		ro = "yes"
	}
	lines = append(lines, fmt.Sprintf("  Read-only:  %s", ro))

	if m.mcpWizardTestResult != "" {
		lines = append(lines, fmt.Sprintf("  Test:       %s", m.mcpWizardTestResult))
	}

	return lines
}

// updateMCPWizard handles key events in the MCP wizard.
func (m Model) updateMCPWizard(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.mcpWizardStep == mcpStepSource || (m.mcpWizardEditing && m.mcpWizardStep == mcpStepTransport) {
			m.mode = viewSettings
			m.textarea.Focus()
			return m, nil
		}
		// Env step: navigate backward through individual vars.
		if m.mcpWizardStep == mcpStepEnv && len(m.mcpWizardEnvOrder) > 0 {
			if m.mcpWizardEnvIdx > 0 {
				// Go back to previous known var.
				m.mcpWizardEnvIdx--
				m.mcpWizardInput.Reset()
				currentVar := m.mcpWizardEnvOrder[m.mcpWizardEnvIdx]
				m.mcpWizardInput.SetValue(m.mcpWizardEnv[currentVar])
				if m.mcpWizardEnvSecrets[currentVar] {
					m.mcpWizardInput.EchoMode = textinput.EchoPassword
				} else {
					m.mcpWizardInput.EchoMode = textinput.EchoNormal
				}
				return m, nil
			} else if m.mcpWizardEnvIdx >= len(m.mcpWizardEnvOrder) {
				// In freeform mode — go back to last known var.
				m.mcpWizardEnvIdx = len(m.mcpWizardEnvOrder) - 1
				m.mcpWizardInput.Reset()
				currentVar := m.mcpWizardEnvOrder[m.mcpWizardEnvIdx]
				m.mcpWizardInput.SetValue(m.mcpWizardEnv[currentVar])
				if m.mcpWizardEnvSecrets[currentVar] {
					m.mcpWizardInput.EchoMode = textinput.EchoPassword
				} else {
					m.mcpWizardInput.EchoMode = textinput.EchoNormal
				}
				return m, nil
			}
			// idx == 0: fall through to prevMCPWizardStep
		}
		m.prevMCPWizardStep()
		m.prepareMCPInput()
		return m, nil
	}

	switch m.mcpWizardStep {
	case mcpStepSource:
		return m.updateMCPWizardList(msg, func(idx int) {
			if idx == 0 {
				m.mcpWizardSource = "popular"
				m.mcpWizardListCursor = 0
				m.mcpRegistryResults = nil
				// Trigger async registry fetch.
				if m.onMCPRegistryFetch != nil {
					m.onMCPRegistryFetch()
				}
			} else {
				m.mcpWizardSource = "custom"
			}
		})

	case mcpStepBrowse:
		return m.updateMCPWizardBrowse(msg)

	case mcpStepTransport:
		return m.updateMCPWizardList(msg, func(idx int) {
			if idx == 1 {
				// SSE — clear command fields
				m.mcpWizardData.Command = ""
				m.mcpWizardData.Args = nil
			} else {
				// stdio — clear URL
				m.mcpWizardData.URL = ""
			}
		})

	case mcpStepName, mcpStepCommand, mcpStepArgs, mcpStepURL, mcpStepEnv:
		return m.updateMCPWizardInput(msg)

	case mcpStepReadOnly:
		return m.updateMCPWizardList(msg, func(idx int) {
			m.mcpWizardData.ReadOnly = idx == 1
		})

	case mcpStepTest:
		if key == "enter" {
			// Advance past test step regardless of result
			m.nextMCPWizardStep()
			return m, nil
		}
		// If not yet testing and no result, auto-trigger
		if !m.mcpWizardTesting && m.mcpWizardTestResult == "" {
			m.mcpWizardTesting = true
			if m.onTestMCP != nil {
				m.onTestMCP(m.stagedMCPConfig())
			}
			return m, m.spinner.Tick
		}

	case mcpStepConfirm:
		if key == "enter" {
			return m.saveMCPWizard()
		}
	}

	return m, nil
}

// updateMCPWizardList handles j/k/enter in list steps.
func (m Model) updateMCPWizardList(msg tea.KeyMsg, onSelect func(int)) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		if m.mcpWizardListCursor < len(m.mcpWizardListItems)-1 {
			m.mcpWizardListCursor++
		}
	case "k", "up":
		if m.mcpWizardListCursor > 0 {
			m.mcpWizardListCursor--
		}
	case "enter":
		if onSelect != nil {
			onSelect(m.mcpWizardListCursor)
		}
		m.mcpWizardListCursor = 0
		m.nextMCPWizardStep()
		m.prepareMCPInput()
		if cmd := m.maybeAutoTriggerTest(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

// updateMCPWizardBrowse handles navigation in the registry browser.
func (m Model) updateMCPWizardBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	count := len(m.mcpRegistryResults)
	if count == 0 {
		return m, nil // still loading
	}
	switch key {
	case "j", "down":
		if m.mcpWizardListCursor < count-1 {
			m.mcpWizardListCursor++
		}
	case "k", "up":
		if m.mcpWizardListCursor > 0 {
			m.mcpWizardListCursor--
		}
	case "enter":
		if m.mcpWizardListCursor < count {
			selected := m.mcpRegistryResults[m.mcpWizardListCursor]
			// Check if this is a hardcoded fallback (ServerData is int index).
			if idx, ok := selected.ServerData.(int); ok && idx < len(PopularMCPServers) {
				tmpl := PopularMCPServers[idx]
				m.mcpWizardData.Name = tmpl.Name
				m.mcpWizardData.Command = tmpl.Command
				m.mcpWizardData.Args = make([]string, len(tmpl.Args))
				copy(m.mcpWizardData.Args, tmpl.Args)
				m.mcpWizardData.ReadOnly = tmpl.ReadOnly
				// Clear all env state before applying new selection.
				m.mcpWizardEnv = make(map[string]string)
				m.mcpWizardEnvSecrets = make(map[string]bool)
				m.mcpWizardEnvDescs = make(map[string]string)
				m.mcpWizardEnvOrder = nil
				m.mcpWizardEnvIdx = -1
				for _, ev := range tmpl.EnvVars {
					m.mcpWizardEnv[ev.Name] = ""
					m.mcpWizardEnvOrder = append(m.mcpWizardEnvOrder, ev.Name)
					if ev.IsSecret {
						m.mcpWizardEnvSecrets[ev.Name] = true
					}
				}
				if len(m.mcpWizardEnvOrder) > 0 {
					m.mcpWizardEnvIdx = 0
				}
			} else if m.onMCPRegistrySelect != nil {
				// Live registry — map to config and apply directly on this model.
				cfg := m.onMCPRegistrySelect(selected)
				m.mcpWizardData = cfg
				m.mcpWizardEnv = make(map[string]string)
				m.mcpWizardEnvSecrets = make(map[string]bool)
				m.mcpWizardEnvOrder = nil
				m.mcpWizardEnvIdx = -1
				m.mcpWizardEnvDescs = make(map[string]string)
				for k, v := range cfg.Env {
					m.mcpWizardEnv[k] = v
				}
				// Build env var order and metadata from registry result.
				for _, ev := range selected.EnvVars {
					m.mcpWizardEnvOrder = append(m.mcpWizardEnvOrder, ev.Name)
					m.mcpWizardEnvDescs[ev.Name] = ev.Description
					if ev.IsSecret {
						m.mcpWizardEnvSecrets[ev.Name] = true
					}
				}
				if len(m.mcpWizardEnvOrder) > 0 {
					m.mcpWizardEnvIdx = 0
				}
			}
		}
		m.mcpWizardListCursor = 0
		m.nextMCPWizardStep()
		m.prepareMCPInput()
		if cmd := m.maybeAutoTriggerTest(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

// updateMCPWizardInput handles text input steps.
func (m Model) updateMCPWizardInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if key == "enter" {
		val := strings.TrimSpace(m.mcpWizardInput.Value())

		switch m.mcpWizardStep {
		case mcpStepName:
			if val == "" {
				return m, nil // require name
			}
			m.mcpWizardData.Name = val
		case mcpStepCommand:
			if val == "" {
				return m, nil // require command
			}
			m.mcpWizardData.Command = val
		case mcpStepArgs:
			if val != "" {
				m.mcpWizardData.Args = strings.Fields(val)
			}
		case mcpStepURL:
			if val == "" {
				return m, nil // require URL
			}
			m.mcpWizardData.URL = val
		case mcpStepEnv:
			inKnownVarMode := m.mcpWizardEnvIdx >= 0 && m.mcpWizardEnvIdx < len(m.mcpWizardEnvOrder)
			if inKnownVarMode {
				// Individual var mode — save value and advance to next var.
				currentVar := m.mcpWizardEnvOrder[m.mcpWizardEnvIdx]
				m.mcpWizardEnv[currentVar] = val
				m.mcpWizardEnvIdx++
				m.mcpWizardInput.Reset()
				// Set EchoMode for next var.
				if m.mcpWizardEnvIdx < len(m.mcpWizardEnvOrder) {
					nextVar := m.mcpWizardEnvOrder[m.mcpWizardEnvIdx]
					if m.mcpWizardEnvSecrets[nextVar] {
						m.mcpWizardInput.EchoMode = textinput.EchoPassword
					} else {
						m.mcpWizardInput.EchoMode = textinput.EchoNormal
					}
				} else {
					m.mcpWizardInput.EchoMode = textinput.EchoNormal
				}
				return m, nil // stay on env step
			}
			// Freeform mode.
			if val != "" {
				if parts := strings.SplitN(val, "=", 2); len(parts) == 2 {
					m.mcpWizardEnv[parts[0]] = parts[1]
					m.mcpWizardInput.Reset()
					return m, nil // stay on env step for more entries
				}
			}
			// Empty enter = move to next step
		}

		m.mcpWizardInput.Reset()
		m.mcpWizardListCursor = 0
		m.nextMCPWizardStep()
		m.prepareMCPInput()
		if cmd := m.maybeAutoTriggerTest(); cmd != nil {
			return m, cmd
		}
		return m, nil
	}

	// Forward to text input
	var cmd tea.Cmd
	m.mcpWizardInput, cmd = m.mcpWizardInput.Update(msg)
	return m, cmd
}

// stagedMCPConfig returns a copy of mcpWizardData with mcpWizardEnv merged
// into the Env field, so the test validates the full staged config including
// unsaved env edits.
func (m *Model) stagedMCPConfig() config.MCPServerConfig {
	cfg := m.mcpWizardData
	if len(m.mcpWizardEnv) > 0 {
		cfg.Env = make(map[string]string, len(m.mcpWizardEnv))
		for k, v := range m.mcpWizardEnv {
			cfg.Env[k] = v
		}
	}
	return cfg
}

// maybeAutoTriggerTest checks if we've landed on mcpStepTest and auto-fires
// the connection test. Returns a spinner tick command if test was triggered.
func (m *Model) maybeAutoTriggerTest() tea.Cmd {
	if m.mcpWizardStep == mcpStepTest && !m.mcpWizardTesting && m.mcpWizardTestResult == "" {
		m.mcpWizardTesting = true
		m.mcpWizardTestResult = ""
		if m.onTestMCP != nil {
			m.onTestMCP(m.stagedMCPConfig())
		}
		return m.spinner.Tick
	}
	return nil
}

// prepareMCPInput pre-fills the text input for the current wizard step.
func (m *Model) prepareMCPInput() {
	m.mcpWizardInput.Focus()
	m.mcpWizardInput.EchoMode = textinput.EchoNormal // default
	switch m.mcpWizardStep {
	case mcpStepName:
		m.mcpWizardInput.SetValue(m.mcpWizardData.Name)
	case mcpStepCommand:
		m.mcpWizardInput.SetValue(m.mcpWizardData.Command)
	case mcpStepArgs:
		m.mcpWizardInput.SetValue(strings.Join(m.mcpWizardData.Args, " "))
	case mcpStepURL:
		m.mcpWizardInput.SetValue(m.mcpWizardData.URL)
	case mcpStepEnv:
		// Set EchoMode for the current known var (if in individual mode).
		if m.mcpWizardEnvIdx >= 0 && m.mcpWizardEnvIdx < len(m.mcpWizardEnvOrder) {
			currentVar := m.mcpWizardEnvOrder[m.mcpWizardEnvIdx]
			m.mcpWizardInput.SetValue(m.mcpWizardEnv[currentVar])
			if m.mcpWizardEnvSecrets[currentVar] {
				m.mcpWizardInput.EchoMode = textinput.EchoPassword
			}
		} else {
			m.mcpWizardInput.SetValue("")
		}
	default:
		m.mcpWizardInput.SetValue("")
	}
}

// saveMCPWizard persists the MCP server configuration.
func (m Model) saveMCPWizard() (Model, tea.Cmd) {
	if m.cfg == nil {
		return m, nil
	}

	// Apply env vars to config — store secrets in keyring.
	if len(m.mcpWizardEnv) > 0 {
		m.mcpWizardData.Env = make(map[string]string)
		store := auth.NewStore()
		for k, v := range m.mcpWizardEnv {
			if m.mcpWizardEnvSecrets[k] && v != "" {
				keyringKey := fmt.Sprintf("mcp_%s_%s", m.mcpWizardData.Name, k)
				if err := store.Set(keyringKey, v); err != nil {
					m.settingsMsg = m.styles.StatusUnhealthy.Render(
						fmt.Sprintf("Failed to store secret %s: %v", k, err))
					m.mode = viewSettings
					m.textarea.Focus()
					return m, nil
				}
				m.mcpWizardData.Env[k] = "$KEYRING:" + keyringKey
			} else {
				m.mcpWizardData.Env[k] = v
			}
		}
	}

	if m.mcpWizardEditing {
		if m.mcpWizardEditIndex < len(m.cfg.MCP.Servers) {
			m.cfg.MCP.Servers[m.mcpWizardEditIndex] = m.mcpWizardData
		}
	} else {
		// Check for duplicate names before adding.
		for _, s := range m.cfg.MCP.Servers {
			if s.Name == m.mcpWizardData.Name {
				m.settingsMsg = m.styles.StatusUnhealthy.Render(
					fmt.Sprintf("MCP server '%s' already exists", m.mcpWizardData.Name))
				m.mode = viewSettings
				m.textarea.Focus()
				return m, nil
			}
		}
		m.cfg.MCP.Servers = append(m.cfg.MCP.Servers, m.mcpWizardData)
	}

	if err := m.cfg.Validate(); err != nil {
		m.settingsMsg = m.styles.StatusUnhealthy.Render("Invalid config: " + err.Error())
		m.mode = viewSettings
		m.textarea.Focus()
		return m, nil
	}

	if err := m.cfg.Save(); err != nil {
		m.settingsMsg = m.styles.StatusUnhealthy.Render("Failed to save: " + err.Error())
	} else {
		m.settingsMsg = m.styles.StatusHealthy.Render(
			fmt.Sprintf("MCP server '%s' saved — connecting...", m.mcpWizardData.Name))
	}

	if m.onConfigChanged != nil {
		m.onConfigChanged(m.cfg)
	}

	m.mode = viewSettings
	m.textarea.Focus()
	return m, nil
}

// deleteSelectedMCPServer removes the currently selected MCP server.
func (m Model) deleteSelectedMCPServer() (Model, tea.Cmd) {
	if m.cfg == nil || m.mcpSettingsCursor >= len(m.cfg.MCP.Servers) {
		return m, nil
	}

	name := m.cfg.MCP.Servers[m.mcpSettingsCursor].Name
	m.cfg.MCP.Servers = append(
		m.cfg.MCP.Servers[:m.mcpSettingsCursor],
		m.cfg.MCP.Servers[m.mcpSettingsCursor+1:]...,
	)

	if err := m.cfg.Save(); err != nil {
		m.settingsMsg = m.styles.StatusUnhealthy.Render("Failed to save: " + err.Error())
	} else {
		m.settingsMsg = m.styles.StatusHealthy.Render(
			fmt.Sprintf("Removed MCP server '%s'", name))
	}

	// Adjust cursor
	if m.mcpSettingsCursor >= len(m.cfg.MCP.Servers) && m.mcpSettingsCursor > 0 {
		m.mcpSettingsCursor--
	}

	if m.onConfigChanged != nil {
		m.onConfigChanged(m.cfg)
	}

	return m, nil
}
