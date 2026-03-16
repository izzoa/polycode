package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/anthonyizzo/polycode/internal/auth"
	"github.com/anthonyizzo/polycode/internal/config"
)

// ConfigChangedMsg is sent when the config has been modified by the settings
// screens. The app layer handles this by rebuilding the registry and pipeline.
type ConfigChangedMsg struct {
	Config *config.Config
}

// TestResultMsg is sent when a provider connection test completes.
type TestResultMsg struct {
	ProviderName string
	Success      bool
	Duration     string
	Error        error
}

// providerTypes lists the available provider types for the wizard.
var providerTypes = []string{"anthropic", "openai", "google", "openai_compatible"}

// defaultNameForType returns a sensible default provider name for a type.
func defaultNameForType(t string) string {
	switch t {
	case "anthropic":
		return "claude"
	case "openai":
		return "gpt"
	case "google":
		return "gemini"
	case "openai_compatible":
		return "custom"
	default:
		return "provider"
	}
}

// authMethodsForType returns the valid auth methods for a provider type.
func authMethodsForType(t string) []string {
	switch t {
	case "openai_compatible":
		return []string{"api_key", "none"}
	default:
		return []string{"api_key", "oauth"}
	}
}

// modelHintForType returns placeholder hint text with popular models.
func modelHintForType(t string) string {
	switch t {
	case "anthropic":
		return "e.g. claude-sonnet-4-20250514, claude-opus-4-20250514"
	case "openai":
		return "e.g. gpt-4o, gpt-4-turbo, o3-mini"
	case "google":
		return "e.g. gemini-2.5-pro, gemini-2.5-flash"
	case "openai_compatible":
		return "e.g. mistral-large-latest, llama-3-70b"
	default:
		return "enter model name"
	}
}

// initWizardForAdd initializes the wizard state for adding a new provider.
func (m *Model) initWizardForAdd() {
	m.mode = viewAddProvider
	m.wizardStep = stepType
	m.wizardData = config.ProviderConfig{}
	m.wizardEditing = false
	m.wizardEditIndex = -1
	m.wizardListCursor = 0
	m.wizardListItems = providerTypes
	m.wizardAPIKey = ""
	m.wizardInput.Reset()
	m.wizardInput.Blur()
}

// initWizardForEdit initializes the wizard state for editing an existing
// provider. All fields are pre-filled with the selected provider's data.
func (m *Model) initWizardForEdit(index int) {
	p := m.cfg.Providers[index]
	m.mode = viewEditProvider
	m.wizardStep = stepType
	m.wizardData = p
	m.wizardEditing = true
	m.wizardEditIndex = index
	m.wizardListCursor = 0
	m.wizardListItems = providerTypes
	// Pre-select the current type in the list
	for i, t := range providerTypes {
		if t == string(p.Type) {
			m.wizardListCursor = i
			break
		}
	}
	m.wizardInput.Reset()
	m.wizardInput.Blur()
}

// nextWizardStep advances to the next applicable wizard step, skipping steps
// that do not apply (e.g., stepAPIKey when auth is not api_key, stepBaseURL
// when type is not openai_compatible).
func (m *Model) nextWizardStep() {
	for {
		m.wizardStep++
		if m.shouldShowStep(m.wizardStep) {
			break
		}
		// If we've gone past the last step, clamp to confirm
		if m.wizardStep > stepConfirm {
			m.wizardStep = stepConfirm
			break
		}
	}
	m.prepareStepUI()
}

// prevWizardStep moves to the previous applicable wizard step.
func (m *Model) prevWizardStep() {
	for {
		if m.wizardStep == stepType {
			return // can't go before the first step
		}
		m.wizardStep--
		if m.shouldShowStep(m.wizardStep) {
			break
		}
	}
	m.prepareStepUI()
}

// shouldShowStep returns whether the given step should be displayed based on
// the current wizard data.
func (m *Model) shouldShowStep(step wizardStep) bool {
	switch step {
	case stepAPIKey:
		return m.wizardData.Auth == config.AuthMethodAPIKey
	case stepBaseURL:
		return m.wizardData.Type == config.ProviderTypeOpenAICompatible
	default:
		return true
	}
}

// prepareStepUI sets up the textinput or list items for the current step.
func (m *Model) prepareStepUI() {
	m.wizardInput.Reset()
	m.wizardInput.Blur()
	m.wizardListCursor = 0
	m.wizardListItems = nil

	switch m.wizardStep {
	case stepType:
		m.wizardListItems = providerTypes
		for i, t := range providerTypes {
			if t == string(m.wizardData.Type) {
				m.wizardListCursor = i
				break
			}
		}
	case stepName:
		m.wizardInput.Placeholder = "provider name"
		if m.wizardData.Name != "" {
			m.wizardInput.SetValue(m.wizardData.Name)
		} else {
			m.wizardInput.SetValue(defaultNameForType(string(m.wizardData.Type)))
		}
		m.wizardInput.Focus()
	case stepAuth:
		m.wizardListItems = authMethodsForType(string(m.wizardData.Type))
		for i, a := range m.wizardListItems {
			if a == string(m.wizardData.Auth) {
				m.wizardListCursor = i
				break
			}
		}
	case stepAPIKey:
		m.wizardInput.Placeholder = "enter API key"
		m.wizardInput.EchoMode = textinput.EchoPassword
		m.wizardInput.EchoCharacter = '*'
		m.wizardInput.Focus()
	case stepModel:
		m.wizardInput.Placeholder = modelHintForType(string(m.wizardData.Type))
		if m.wizardData.Model != "" {
			m.wizardInput.SetValue(m.wizardData.Model)
		}
		m.wizardInput.EchoMode = textinput.EchoNormal
		m.wizardInput.Focus()
	case stepBaseURL:
		m.wizardInput.Placeholder = "https://api.example.com/v1"
		if m.wizardData.BaseURL != "" {
			m.wizardInput.SetValue(m.wizardData.BaseURL)
		}
		m.wizardInput.EchoMode = textinput.EchoNormal
		m.wizardInput.Focus()
	case stepPrimary:
		m.wizardListItems = []string{"yes", "no"}
		if m.wizardData.Primary {
			m.wizardListCursor = 0
		} else {
			m.wizardListCursor = 1
		}
	case stepConfirm:
		// no input needed — summary view
	}
}

// renderWizard renders the current wizard step.
func (m Model) renderWizard() string {
	var sections []string

	modeLabel := "Add Provider"
	if m.wizardEditing {
		modeLabel = "Edit Provider"
	}
	title := m.styles.Title.Render(fmt.Sprintf("Settings — %s", modeLabel))
	sections = append(sections, title)
	sections = append(sections, "")

	// Step indicator
	stepNames := []string{"Type", "Name", "Auth", "API Key", "Model", "Base URL", "Primary", "Confirm"}
	stepNum := int(m.wizardStep) + 1
	totalSteps := 0
	for s := stepType; s <= stepConfirm; s++ {
		if m.shouldShowStep(s) {
			totalSteps++
		}
	}
	visibleStep := 0
	for s := stepType; s <= m.wizardStep; s++ {
		if m.shouldShowStep(s) {
			visibleStep++
		}
	}
	stepIndicator := m.styles.Dimmed.Render(
		fmt.Sprintf("Step %d/%d: %s", visibleStep, totalSteps, stepNames[stepNum-1]))
	sections = append(sections, stepIndicator)
	sections = append(sections, "")

	switch m.wizardStep {
	case stepType:
		sections = append(sections, "Select provider type:")
		sections = append(sections, "")
		sections = append(sections, m.renderWizardList()...)

	case stepName:
		sections = append(sections, "Enter a name for this provider:")
		sections = append(sections, "")
		sections = append(sections, m.wizardInput.View())

	case stepAuth:
		sections = append(sections, "Select authentication method:")
		sections = append(sections, "")
		sections = append(sections, m.renderWizardList()...)

	case stepAPIKey:
		sections = append(sections, "Enter your API key:")
		sections = append(sections, "")
		sections = append(sections, m.wizardInput.View())

	case stepModel:
		sections = append(sections, "Enter the model name:")
		sections = append(sections, "")
		sections = append(sections, m.wizardInput.View())

	case stepBaseURL:
		sections = append(sections, "Enter the base URL for this provider:")
		sections = append(sections, "")
		sections = append(sections, m.wizardInput.View())

	case stepPrimary:
		sections = append(sections, "Set as primary provider?")
		sections = append(sections, "")
		sections = append(sections, m.renderWizardList()...)

	case stepConfirm:
		sections = append(sections, m.renderWizardSummary()...)
	}

	sections = append(sections, "")

	// Navigation hints
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if m.wizardStep == stepConfirm {
		sections = append(sections, hintStyle.Render("Enter:save  Esc:cancel"))
	} else {
		sections = append(sections, hintStyle.Render("Enter:next  Esc:cancel"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.styles.App.Width(m.width).Render(content)
}

// renderWizardList renders the list selector for the current wizard step.
func (m Model) renderWizardList() []string {
	var lines []string
	for i, item := range m.wizardListItems {
		cursor := "  "
		if i == m.wizardListCursor {
			cursor = m.styles.Prompt.Render("> ")
		}
		lines = append(lines, cursor+item)
	}
	return lines
}

// renderWizardSummary renders the confirmation screen with all wizard data.
func (m Model) renderWizardSummary() []string {
	var lines []string

	labelStyle := lipgloss.NewStyle().Bold(true).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	lines = append(lines, "Review your provider configuration:")
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Type:")+"  "+valueStyle.Render(string(m.wizardData.Type)))
	lines = append(lines, labelStyle.Render("Name:")+"  "+valueStyle.Render(m.wizardData.Name))
	lines = append(lines, labelStyle.Render("Auth:")+"  "+valueStyle.Render(string(m.wizardData.Auth)))
	if m.wizardData.Auth == config.AuthMethodAPIKey {
		lines = append(lines, labelStyle.Render("API Key:")+"  "+valueStyle.Render("****"))
	}
	lines = append(lines, labelStyle.Render("Model:")+"  "+valueStyle.Render(m.wizardData.Model))
	if m.wizardData.Type == config.ProviderTypeOpenAICompatible {
		lines = append(lines, labelStyle.Render("Base URL:")+"  "+valueStyle.Render(m.wizardData.BaseURL))
	}
	primary := "no"
	if m.wizardData.Primary {
		primary = "yes"
	}
	lines = append(lines, labelStyle.Render("Primary:")+"  "+valueStyle.Render(primary))

	return lines
}

// updateWizard handles key events for the wizard (add/edit provider).
func (m Model) updateWizard(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Cancel wizard, return to settings
		m.mode = viewSettings
		return m, nil
	}

	// Step-specific key handling
	switch m.wizardStep {
	case stepType, stepAuth, stepPrimary:
		return m.updateWizardList(msg)
	case stepName, stepAPIKey, stepModel, stepBaseURL:
		return m.updateWizardInput(msg)
	case stepConfirm:
		return m.updateWizardConfirm(msg)
	}

	return m, nil
}

// updateWizardList handles navigation in list-selection wizard steps.
func (m Model) updateWizardList(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		if m.wizardListCursor < len(m.wizardListItems)-1 {
			m.wizardListCursor++
		}
	case "k", "up":
		if m.wizardListCursor > 0 {
			m.wizardListCursor--
		}
	case "enter":
		selected := m.wizardListItems[m.wizardListCursor]

		switch m.wizardStep {
		case stepType:
			m.wizardData.Type = config.ProviderType(selected)
		case stepAuth:
			m.wizardData.Auth = config.AuthMethod(selected)
		case stepPrimary:
			m.wizardData.Primary = selected == "yes"
		}

		m.nextWizardStep()
	}
	return m, nil
}

// updateWizardInput handles text input in wizard steps.
func (m Model) updateWizardInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	if key == "enter" {
		value := strings.TrimSpace(m.wizardInput.Value())
		if value == "" && m.wizardStep != stepAPIKey {
			// Don't allow empty values for required fields
			// (API key can be empty if user wants to set it later)
			return m, nil
		}

		switch m.wizardStep {
		case stepName:
			m.wizardData.Name = value
		case stepAPIKey:
			m.wizardAPIKey = m.wizardInput.Value()
		case stepModel:
			m.wizardData.Model = value
		case stepBaseURL:
			m.wizardData.BaseURL = value
		}

		m.nextWizardStep()
		return m, nil
	}

	// Pass key to textinput
	var cmd tea.Cmd
	m.wizardInput, cmd = m.wizardInput.Update(msg)
	return m, cmd
}

// updateWizardConfirm handles the confirm step of the wizard.
func (m Model) updateWizardConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	if key == "enter" {
		return m.saveWizard()
	}

	return m, nil
}

// saveWizard persists the wizard's data to the config, stores the API key,
// and sends a ConfigChangedMsg.
func (m Model) saveWizard() (Model, tea.Cmd) {
	if m.cfg == nil {
		m.mode = viewSettings
		return m, nil
	}

	// If set as primary, un-mark others
	if m.wizardData.Primary {
		for i := range m.cfg.Providers {
			m.cfg.Providers[i].Primary = false
		}
	}

	if m.wizardEditing {
		// Update existing provider
		m.cfg.Providers[m.wizardEditIndex] = m.wizardData
	} else {
		// Append new provider
		m.cfg.Providers = append(m.cfg.Providers, m.wizardData)
	}

	// Store API key if one was entered during stepAPIKey
	if m.wizardData.Auth == config.AuthMethodAPIKey && m.wizardAPIKey != "" {
		store := auth.NewStore()
		_ = store.Set(m.wizardData.Name, m.wizardAPIKey)
		m.wizardAPIKey = ""
	}

	// Save config to disk
	_ = m.cfg.Save()

	// Return to settings
	m.mode = viewSettings
	m.settingsMsg = m.styles.StatusHealthy.Render("Provider saved successfully")

	// Rebuild panels from config
	m.rebuildPanelsFromConfig()

	// Notify app layer
	if m.onConfigChanged != nil {
		m.onConfigChanged(m.cfg)
	}

	return m, func() tea.Msg {
		return ConfigChangedMsg{Config: m.cfg}
	}
}

// rebuildPanelsFromConfig rebuilds the TUI panels list from the current config.
func (m *Model) rebuildPanelsFromConfig() {
	if m.cfg == nil {
		return
	}
	var names []string
	for _, p := range m.cfg.Providers {
		names = append(names, p.Name)
	}
	m.providers = names

	// Keep existing panel state for panels that still exist; create new ones.
	oldPanels := make(map[string]ProviderPanel)
	for _, p := range m.panels {
		oldPanels[p.Name] = p
	}

	panels := make([]ProviderPanel, len(m.cfg.Providers))
	for i, pc := range m.cfg.Providers {
		if old, ok := oldPanels[pc.Name]; ok {
			old.IsPrimary = pc.Primary
			panels[i] = old
		} else {
			panels[i] = ProviderPanel{
				Name:      pc.Name,
				IsPrimary: pc.Primary,
				Status:    StatusIdle,
			}
		}
	}
	m.panels = panels
}
