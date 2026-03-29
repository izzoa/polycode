package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// trueColorSupported caches whether the terminal supports true color.
var trueColorSupported = func() bool {
	ct := os.Getenv("COLORTERM")
	return ct == "truecolor" || ct == "24bit"
}()

// ansi256ToHex maps common ANSI 256 color numbers to hex for gradient support.
var ansi256ToHex = map[string]string{
	"214": "#ffaf00", "63": "#5f5fff", "86": "#5fd7af", "42": "#00d787",
	"196": "#ff0000", "82": "#5fff00", "252": "#d0d0d0", "241": "#626262",
	"243": "#767676", "250": "#bcbcbc", "235": "#262626", "236": "#303030",
	"238": "#444444", "240": "#585858", "237": "#3a3a3a", "245": "#8a8a8a",
	"226": "#ffff00", "39": "#00afff",
}

// resolveToHex converts a lipgloss.Color to a hex string suitable for go-colorful.
func resolveToHex(c lipgloss.Color) string {
	s := string(c)
	if strings.HasPrefix(s, "#") {
		return s
	}
	if hex, ok := ansi256ToHex[s]; ok {
		return hex
	}
	return ""
}

// renderGradientTitle renders text with a per-character color gradient
// interpolated between two theme colors using HCL blending.
// Falls back to solid fromColor on non-true-color terminals.
func renderGradientTitle(text string, from, to lipgloss.Color, bold bool) string {
	runes := []rune(text)
	if !trueColorSupported || len(runes) <= 1 {
		style := lipgloss.NewStyle().Foreground(from)
		if bold {
			style = style.Bold(true)
		}
		return style.Render(text)
	}

	fromHex := resolveToHex(from)
	toHex := resolveToHex(to)
	c1, err1 := colorful.Hex(fromHex)
	c2, err2 := colorful.Hex(toHex)
	if err1 != nil || err2 != nil {
		style := lipgloss.NewStyle().Foreground(from)
		if bold {
			style = style.Bold(true)
		}
		return style.Render(text)
	}

	var b strings.Builder
	for i, r := range runes {
		t := float64(i) / float64(len(runes)-1)
		blended := c1.BlendHcl(c2, t)
		hex := blended.Hex()
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		if bold {
			style = style.Bold(true)
		}
		b.WriteString(style.Render(string(r)))
	}
	return b.String()
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Splash screen takes over the entire view
	if m.showSplash {
		return m.renderSplash()
	}

	// Help overlay takes priority
	if m.showHelp {
		return m.applyToastOverlay(m.renderHelp())
	}

	// Session picker overlay
	if m.showSessionPicker {
		return m.applyToastOverlay(m.renderSessionPickerFull())
	}

	// MCP dashboard overlay
	if m.showMCPDashboard {
		return m.applyToastOverlay(m.renderMCPDashboard())
	}

	// Dispatch based on current view mode
	switch m.mode {
	case viewSettings:
		return m.renderSettings()
	case viewAddProvider, viewEditProvider:
		return m.renderWizard()
	case viewAddMCP, viewEditMCP:
		return m.renderMCPWizard()
	default:
		return m.renderChat()
	}
}

// renderChat renders the main chat view.
func (m Model) renderChat() string {
	var sections []string

	// Tab bar — combines app title, mode, provider tabs, and query status
	sections = append(sections, m.renderTabBar())

	// Main content area — split pane or single panel
	if m.splitPaneActive() {
		// Wide terminal: always show split pane (left = active tab, right = split panel)
		if len(m.history) > 0 || m.querying || m.consensusActive ||
			(m.activeTab > 0 && m.activeTab-1 < len(m.panels)) {
			sections = append(sections, m.renderSplitPane())
		}
	} else if m.activeTab <= 0 {
		// Narrow: consensus/chat view
		if len(m.history) > 0 || m.querying || m.consensusActive {
			sections = append(sections, m.renderChatPanel())
		}
	} else if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
		// Narrow: single provider panel
		panel := m.panels[m.activeTab-1]
		sections = append(sections, m.renderSingleProviderPanel(panel))
	}

	// Worker progress (during /plan execution)
	if m.planRunning && len(m.agentStages) > 0 {
		sections = append(sections, m.renderWorkerProgress())
	}

	// Provenance panel (toggled with 'p')
	if m.showProvenance && !m.querying && m.activeTab == 0 {
		sections = append(sections, m.renderProvenance())
	}

	// Command palette overlay (triggered by /)
	if m.paletteOpen {
		sections = append(sections, m.renderCommandPalette())
	}

	// Mode picker overlay
	if m.modePickerOpen {
		sections = append(sections, m.renderModePicker())
	}

	// Theme picker overlay
	if m.themePickerOpen {
		sections = append(sections, m.renderThemePicker())
	}

	// File picker overlay (triggered by @)
	if m.filePickerOpen {
		sections = append(sections, m.renderFilePicker())
	}

	// Live task HUD during tool execution
	if m.querying && len(m.toolCalls) > 0 {
		sections = append(sections, m.renderTaskHUD())
	}

	// Confirmation prompt (overlays input area when pending)
	if m.confirmPending {
		sections = append(sections, m.renderConfirmPrompt())
	} else if !m.modePickerOpen {
		// Input area (always visible when not confirming or picking mode)
		sections = append(sections, m.renderInput())
	}

	// Persistent status bar — rendered last (truly bottom)
	sections = append(sections, m.renderStatusBar())

	output := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Constrain output to terminal height to prevent overflow
	if m.height > 0 {
		output = lipgloss.NewStyle().MaxHeight(m.height).Render(output)
	}

	return m.applyToastOverlay(output)
}

// applyToastOverlay composites toast notifications onto the bottom-right
// of a rendered frame without extending it beyond the terminal.
func (m Model) applyToastOverlay(output string) string {
	toasts := m.renderToasts()
	if toasts == "" {
		return output
	}
	outLines := strings.Split(output, "\n")
	toastLines := strings.Split(toasts, "\n")
	startLine := len(outLines) - len(toastLines) - 2
	if startLine < 0 {
		startLine = 0
	}
	for i, tl := range toastLines {
		idx := startLine + i
		if idx >= 0 && idx < len(outLines) {
			outLines[idx] = outLines[idx] + "  " + tl
		}
	}
	result := strings.Join(outLines, "\n")
	if m.height > 0 {
		result = lipgloss.NewStyle().MaxHeight(m.height).Render(result)
	}
	return result
}

// renderTaskHUD renders a live checklist of tool calls during execution.
func (m Model) renderTaskHUD() string {
	hudStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		PaddingLeft(2)

	doneIcon := lipgloss.NewStyle().Foreground(m.theme.Success).Render("[✓]")
	runIcon := m.spinner.View()
	durStyle := lipgloss.NewStyle().Foreground(m.theme.ScrollTrack)

	var lines []string
	// Show last 5 tool calls to avoid filling the screen
	start := 0
	if len(m.toolCalls) > 5 {
		start = len(m.toolCalls) - 5
	}
	for _, tc := range m.toolCalls[start:] {
		var icon string
		if tc.Done {
			icon = doneIcon
		} else {
			icon = "[" + runIcon + "]"
		}

		desc := tc.Description
		// Truncate long descriptions
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}

		line := icon + " " + desc
		if tc.Done && tc.Duration > 0 {
			line += " " + durStyle.Render(fmt.Sprintf("(%s)", tc.Duration.Round(time.Millisecond)))
		}
		if tc.Error != "" {
			line += " " + lipgloss.NewStyle().Foreground(m.theme.Error).Render("✕")
		}

		lines = append(lines, line)
	}

	return hudStyle.Render(strings.Join(lines, "\n"))
}

// renderWithScrollbar appends a thin scrollbar track to the right side of a
// viewport's visible output. Pass the total content line count and the
// viewport's scroll percentage so the thumb can be positioned correctly.
// The view string should be the already-clipped Viewport.View() output.
func renderWithScrollbar(view string, totalContentLines int, scrollPercent float64, viewHeight int, t Theme) string {
	if viewHeight <= 0 || totalContentLines <= viewHeight {
		return view // no scrollbar needed
	}

	lines := strings.Split(view, "\n")
	trackStyle := lipgloss.NewStyle().Foreground(t.ScrollTrack)
	thumbStyle := lipgloss.NewStyle().Foreground(t.ScrollThumb)

	thumbSize := viewHeight * viewHeight / totalContentLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbPos := int(scrollPercent * float64(viewHeight-thumbSize))
	if thumbPos < 0 {
		thumbPos = 0
	}

	var result []string
	for i := 0; i < viewHeight && i < len(lines); i++ {
		var indicator string
		if i >= thumbPos && i < thumbPos+thumbSize {
			indicator = thumbStyle.Render("┃")
		} else {
			indicator = trackStyle.Render("│")
		}
		result = append(result, lines[i]+indicator)
	}
	return strings.Join(result, "\n")
}

// addDropShadow adds a ░ character shadow to the right and bottom of a rendered block.
func addDropShadow(content string, t Theme) string {
	lines := strings.Split(content, "\n")
	shadowStyle := lipgloss.NewStyle().Foreground(t.Shadow)
	var result []string
	for _, line := range lines {
		result = append(result, line+shadowStyle.Render("░"))
	}
	if len(lines) > 0 {
		width := lipgloss.Width(lines[0])
		result = append(result, " "+shadowStyle.Render(strings.Repeat("░", width+1)))
	}
	return strings.Join(result, "\n")
}

// renderToolCallSummary renders a concealed summary of tool calls for an exchange.
func renderToolCallSummary(calls []ToolCallRecord) string {
	if len(calls) == 0 {
		return ""
	}
	names := make(map[string]int)
	var totalDuration time.Duration
	for _, c := range calls {
		names[c.ToolName]++
		totalDuration += c.Duration
	}
	var nameList []string
	for name := range names {
		nameList = append(nameList, name)
	}
	sort.Strings(nameList)
	summary := fmt.Sprintf("⚙ %d tool calls (%s)", len(calls), strings.Join(nameList, ", "))
	if totalDuration > 0 {
		summary += fmt.Sprintf(" — %s total", totalDuration.Round(time.Millisecond))
	}
	return summary
}

// renderConfirmPrompt renders the action confirmation prompt.
func (m Model) renderConfirmPrompt() string {
	warnStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderAccent).
		Padding(0, 1).
		Width(m.width - 4)

	title := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Warning).Render("Confirm Action")

	// Tool metadata line
	var metaLine string
	if m.confirmToolName != "" {
		toolStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text)
		metaLine = toolStyle.Render(m.confirmToolName)
		if m.confirmRiskLevel != "" {
			var riskStyle lipgloss.Style
			switch m.confirmRiskLevel {
			case "destructive":
				riskStyle = lipgloss.NewStyle().Foreground(m.theme.Error).Bold(true)
			case "mutating":
				riskStyle = lipgloss.NewStyle().Foreground(m.theme.Warning)
			default:
				riskStyle = lipgloss.NewStyle().Foreground(m.theme.Success)
			}
			metaLine += "  " + riskStyle.Render("["+m.confirmRiskLevel+"]")
		}
		metaLine += "\n"
	}

	// Colorize diff content in the description
	desc := m.confirmDescription
	if isDiffContent(desc) {
		desc = renderColorizedDiff(desc, m.theme)
	}

	// Edit mode: show textarea instead of description
	if m.confirmEditing {
		editTitle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Warning).Render("Edit & Execute")
		editHint := lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("  Ctrl+S: execute edited  Esc: back to confirm")
		editContent := m.confirmEditTextarea.View()
		return addDropShadow(warnStyle.Render(fmt.Sprintf("%s\n%s\n\n%s\n\n%s", editTitle, metaLine, editContent, editHint)), m.theme)
	}

	hint := lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("  y: approve  e: edit  a: allow for session  n: reject  Esc: cancel")

	return addDropShadow(warnStyle.Render(fmt.Sprintf("%s\n%s\n%s\n\n%s", title, metaLine, desc, hint)), m.theme)
}

// renderProvenance renders the consensus provenance panel.
func (m Model) renderProvenance() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderFocused).
		Padding(0, 1).
		Width(m.width - 4)

	var lines []string

	title := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Secondary).Render("Consensus Provenance")
	lines = append(lines, title, "")

	// Routing decision
	if m.routingReason != "" {
		lines = append(lines, m.styles.Dimmed.Render("Routing: "+m.routingReason), "")
	}

	// Confidence
	if m.consensusConfidence != "" {
		var confStyle lipgloss.Style
		switch m.consensusConfidence {
		case "high":
			confStyle = lipgloss.NewStyle().Foreground(m.theme.Success).Bold(true)
		case "medium":
			confStyle = lipgloss.NewStyle().Foreground(m.theme.Warning).Bold(true)
		case "low":
			confStyle = lipgloss.NewStyle().Foreground(m.theme.Error).Bold(true)
		default:
			confStyle = lipgloss.NewStyle().Foreground(m.theme.Text)
		}
		lines = append(lines, "Confidence: "+confStyle.Render(m.consensusConfidence))
	}

	// Agreements
	if len(m.consensusAgreements) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Agreement:"))
		for _, a := range m.consensusAgreements {
			lines = append(lines, "  "+a)
		}
	}

	// Minority reports
	if len(m.minorityReports) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Foreground(m.theme.Warning).Render("Minority Report:"))
		for _, mr := range m.minorityReports {
			lines = append(lines, "  "+mr)
		}
	} else {
		lines = append(lines, "", m.styles.Dimmed.Render("All models agreed"))
	}

	// Evidence
	if len(m.consensusEvidence) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Evidence:"))
		for _, e := range m.consensusEvidence {
			lines = append(lines, "  "+e)
		}
	}

	lines = append(lines, "", m.styles.Dimmed.Render("Press p to close"))

	return addDropShadow(style.Render(strings.Join(lines, "\n")), m.theme)
}

// renderWorkerProgress renders the agent team progress panel during /plan.
func (m Model) renderWorkerProgress() string {
	style := m.styles.ConsensusBorder.Width(m.width - 4)

	var lines []string
	title := m.styles.Title.Render("Agent Team")
	lines = append(lines, title)

	for _, stage := range m.agentStages {
		for _, w := range stage.Workers {
			var icon string
			switch w.Status {
			case "complete":
				icon = m.styles.StatusHealthy.Render("✓")
			case "running":
				icon = m.spinner.View()
			default:
				icon = m.styles.Dimmed.Render("○")
			}

			line := fmt.Sprintf("  %s %s (%s)", icon, w.Role, w.Provider)
			if w.Summary != "" {
				summary := w.Summary
				if len(summary) > 60 {
					summary = summary[:57] + "..."
				}
				line += " — " + m.styles.Dimmed.Render(summary)
			}
			lines = append(lines, line)
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

// renderMCPDashboard renders the MCP dashboard overlay.
func (m Model) renderMCPDashboard() string {
	var sections []string

	title := m.styles.Title.Render("MCP Dashboard")
	sections = append(sections, title)
	sections = append(sections, "")

	if len(m.mcpDashboardData) == 0 {
		sections = append(sections, m.styles.Dimmed.Render("  No MCP servers configured."))
	} else {
		// Table header.
		header := fmt.Sprintf("  %-22s %-10s %-16s %-6s %-5s",
			"SERVER", "TRANSPORT", "STATUS", "TOOLS", "R/O")
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Text).
			Background(m.theme.BgPanel)
		sections = append(sections, headerStyle.Width(m.width-4).Render(header))

		// Server rows.
		for i, ds := range m.mcpDashboardData {
			cursor := "  "
			if i == m.mcpDashboardCursor {
				cursor = m.styles.Prompt.Render("> ")
			}

			var statusStyle lipgloss.Style
			statusText := ds.Status
			switch ds.Status {
			case "connected":
				statusStyle = lipgloss.NewStyle().Foreground(m.theme.Success)
				statusText = "✓ connected"
			case "failed":
				statusStyle = lipgloss.NewStyle().Foreground(m.theme.Error)
				statusText = "✗ failed"
			default:
				statusStyle = lipgloss.NewStyle().Foreground(m.theme.TextMuted)
			}

			toolCount := "—"
			if ds.ToolCount > 0 {
				toolCount = fmt.Sprintf("%d", ds.ToolCount)
			}
			ro := "no"
			if ds.ReadOnly {
				ro = "yes"
			}

			transport := ds.Transport
			if transport == "" {
				transport = "stdio"
			}

			row := fmt.Sprintf("%s%-22s %-10s %s %-6s %-5s",
				cursor,
				ds.Name,
				transport,
				statusStyle.Render(fmt.Sprintf("%-16s", statusText)),
				toolCount,
				ro,
			)

			rowStyle := lipgloss.NewStyle()
			if i == m.mcpDashboardCursor {
				rowStyle = rowStyle.
					Background(m.theme.BgSelected).
					Foreground(m.theme.Text)
			}
			sections = append(sections, rowStyle.Width(m.width-4).Render(row))

			// Error detail for failed servers.
			if ds.Error != "" {
				errLine := "                              └ " + ds.Error
				sections = append(sections, m.styles.StatusUnhealthy.Render(errLine))
			}
		}

		// Per-server tools.
		sections = append(sections, "")
		sections = append(sections, m.styles.Dimmed.Render("  Tools:"))
		for _, ds := range m.mcpDashboardData {
			if len(ds.Tools) == 0 {
				continue
			}
			toolList := strings.Join(ds.Tools, ", ")
			if len(toolList) > m.width-30 && m.width > 40 {
				toolList = toolList[:m.width-33] + "..."
			}
			sections = append(sections, fmt.Sprintf("    %s: %s", ds.Name, m.styles.Dimmed.Render(toolList)))
		}

		// Aggregate stats.
		sections = append(sections, "")
		connectedCount := 0
		for _, ds := range m.mcpDashboardData {
			if ds.Status == "connected" {
				connectedCount++
			}
		}
		stats := fmt.Sprintf("  %d servers | %d tools | %d calls this session",
			connectedCount, m.mcpDashboardTotal, m.mcpDashboardCalls)
		sections = append(sections, m.styles.Dimmed.Render(stats))
	}

	// Spacer.
	contentLines := len(sections)
	remaining := m.height - contentLines - 4
	if remaining > 0 {
		sections = append(sections, strings.Repeat("\n", remaining))
	}

	// Action hints.
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextMuted)
	sections = append(sections, hintStyle.Render("j/k:navigate  r:reconnect  t:test  /:command  m/Esc:close"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.styles.App.Width(m.width).Render(content)
}

// renderHelp renders the help overlay showing all keyboard shortcuts.
func (m Model) renderHelp() string {
	var sections []string

	title := m.styles.Title.Render("Keyboard Shortcuts")
	sections = append(sections, title)
	sections = append(sections, "")

	labelStyle := lipgloss.NewStyle().Bold(true).Width(16)
	valueStyle := lipgloss.NewStyle().Foreground(m.theme.Text)

	helpItems := []struct{ key, desc string }{
		{"Ctrl+C", "Quit"},
		{"Ctrl+E", "Open external editor to compose prompt"},
		{"Ctrl+G", "Toggle auto-scroll lock"},
		{"Ctrl+H", "Toggle tool call concealment"},
		{"Ctrl+P", "Open command palette (search commands and files)"},
		{"Ctrl+T", "Switch color theme"},
		{"Ctrl+S", "Open settings"},
		{"/sessions", "Open session browser"},
		{"@", "Attach a file to the prompt (fuzzy search)"},
		{"!", "Run shell command and inject output as context"},
		{"/settings", "Open settings (type in input)"},
		{"/clear", "Clear conversation and reset context"},
		{"/save", "Save session to disk"},
		{"/export [path]", "Export session as shareable artifact"},
		{"/plan <request>", "Run multi-model agent team pipeline"},
		{"/mode <name>", "Switch mode: quick, balanced, thorough, yolo"},
		{"/yolo", "Toggle yolo mode (auto-approve all tool actions)"},
		{"/memory", "View repo memory"},
		{"/skill [list|install|remove]", "Manage installed skills"},
		{"/help", "Toggle this help overlay"},
		{"/exit", "Quit polycode"},
		{"Tab", "Accept slash completion"},
		{"↑ (input empty)", "Focus tab bar for ←/→ navigation + shortcuts"},
		{"↓ / Enter / Esc", "Return focus to input"},
		{"m (tab bar)", "Toggle MCP dashboard"},
		{"c (tab bar)", "Cancel selected provider during query"},
		{"p (tab bar)", "Toggle consensus provenance panel"},
		{"? (tab bar)", "Toggle this help overlay"},
		{"Enter", "Submit prompt"},
		{"PgUp / PgDn", "Scroll chat history"},
		{"Ctrl+U / Ctrl+D", "Half-page scroll"},
		{"j / k", "Scroll one line (input empty)"},
		{"d / u", "Half-page scroll (input empty)"},
		{"g / G", "Jump to top / bottom (input empty)"},
		{"t", "Toggle trace expansion on last exchange (input empty)"},
		{"y", "Copy last response to clipboard (input empty)"},
		{"Home / End", "Jump to top / bottom of chat"},
		{"", ""},
		{"", "Settings Screen"},
		{"j / Down", "Move cursor down"},
		{"k / Up", "Move cursor up"},
		{"a", "Add new provider"},
		{"e", "Edit selected provider"},
		{"d", "Delete selected provider"},
		{"t", "Test selected provider connection"},
		{"Esc", "Return to chat / cancel"},
	}

	for _, item := range helpItems {
		if item.key == "" && item.desc == "" {
			sections = append(sections, "")
			continue
		}
		if item.key == "" {
			sections = append(sections, m.styles.Title.Render(item.desc))
			continue
		}
		sections = append(sections, labelStyle.Render(item.key)+"  "+valueStyle.Render(item.desc))
	}

	sections = append(sections, "")
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextMuted)
	sections = append(sections, hintStyle.Render("Press ? or Esc to close"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Center vertically
	contentLines := strings.Count(content, "\n") + 1
	topPad := (m.height - contentLines) / 2
	if topPad < 0 {
		topPad = 0
	}

	return lipgloss.NewStyle().
		Width(m.width).
		PaddingTop(topPad).
		PaddingLeft(4).
		Render(content)
}


func (m Model) renderChatPanel() string {
	style := m.styles.ConsensusBorder.Width(m.width - 4)
	content := renderWithScrollbar(m.chatView.View(), m.chatView.TotalLineCount(), m.chatView.ScrollPercent(), m.chatView.Height, m.theme)
	return style.Render(content)
}

// renderTabBar renders the combined status + tab bar.
func (m Model) renderTabBar() string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Tertiary).
		Background(m.theme.BgSelected).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextBright).
		Background(m.theme.BgPanel).
		Padding(0, 1)

	// When tab bar is focused, use a brighter active style with underline
	if m.tabBarFocused {
		activeStyle = activeStyle.
			Foreground(m.theme.Info).
			Background(m.theme.BgFocused).
			Underline(true)
	}

	// App title + mode + focus indicator — gradient when true color supported
	var header []string
	if m.tabBarFocused {
		header = append(header, "▸ "+renderGradientTitle("polycode", m.theme.Primary, m.theme.Secondary, true))
	} else {
		header = append(header, renderGradientTitle("polycode", m.theme.Primary, m.theme.Secondary, true))
	}
	if m.currentMode != "" {
		modeLabel := m.currentMode
		if m.yoloMode {
			modeLabel += "|yolo"
		}
		if m.tabBarFocused && m.activeTab == -1 {
			// Mode selector is highlighted
			header = append(header, activeStyle.Render(modeLabel))
		} else {
			modeStyle := lipgloss.NewStyle().Foreground(m.theme.Secondary)
			header = append(header, m.styles.Dimmed.Render("["))
			header = append(header, modeStyle.Render(modeLabel))
			header = append(header, m.styles.Dimmed.Render("] "))
		}
	} else {
		header = append(header, " ")
	}

	// Consensus tab
	consensusLabel := "Consensus"
	if m.querying {
		// Phase-aware label
		switch m.queryPhase {
		case "dispatching":
			consensusLabel = m.spinner.View() + " Dispatching..."
		case "thinking":
			consensusLabel = m.spinner.View() + " Thinking..."
		case "synthesizing":
			consensusLabel = m.spinner.View() + " Synthesizing..."
		case "executing":
			consensusLabel = m.spinner.View() + " Executing tools..."
		case "verifying":
			consensusLabel = m.spinner.View() + " Verifying..."
		default:
			consensusLabel = m.spinner.View() + " Querying"
		}
	}
	if m.activeTab == 0 {
		header = append(header, activeStyle.Render(consensusLabel))
	} else {
		header = append(header, inactiveStyle.Render(consensusLabel))
	}

	// Provider tabs with status + token usage
	for i, panel := range m.panels {
		// Check if this provider is disabled in config
		isDisabled := false
		if m.cfg != nil {
			for _, pc := range m.cfg.Providers {
				if pc.Name == panel.Name && pc.Disabled {
					isDisabled = true
					break
				}
			}
		}

		var icon string
		if isDisabled {
			icon = "×"
		} else {
			switch panel.Status {
			case StatusIdle:
				icon = "○"
			case StatusLoading:
				icon = m.spinner.View()
			case StatusDone:
				icon = "✓"
			case StatusFailed:
				icon = "✕"
			case StatusTimedOut:
				icon = "⏱"
			case StatusCancelled:
				icon = "⊘"
			}
		}

		label := icon + " " + panel.Name
		if panel.IsPrimary {
			label += "★"
		}

		// Compact token usage + cost
		if td, ok := m.tokenUsage[panel.Name]; ok && td.HasData {
			label += " " + td.Used
			if td.Cost != "" {
				label += " " + td.Cost
			}
		}

		if isDisabled {
			// Strikethrough + dimmed for disabled providers
			disabledStyle := lipgloss.NewStyle().
				Foreground(m.theme.TextMuted).
				Background(m.theme.BgPanel).
				Strikethrough(true).
				Padding(0, 1)
			header = append(header, disabledStyle.Render(label))
		} else if m.activeTab == i+1 {
			header = append(header, activeStyle.Render(label))
		} else {
			header = append(header, inactiveStyle.Render(label))
		}
	}

	// MCP connection indicator + call count — selectable tab when tab bar focused
	if len(m.mcpServers) > 0 {
		connected := 0
		total := len(m.mcpServers)
		for _, s := range m.mcpServers {
			if s.Status == "connected" {
				connected++
			}
		}
		mcpLabel := fmt.Sprintf("MCP: %d/%d", connected, total)
		if connected == total {
			mcpLabel += " ✓"
		} else {
			mcpLabel += " ⚠"
		}
		if m.mcpCallCount > 0 {
			mcpLabel += fmt.Sprintf(" %d calls", m.mcpCallCount)
		}

		mcpTabIdx := len(m.panels) + 1
		if m.tabBarFocused && m.activeTab == mcpTabIdx {
			header = append(header, activeStyle.Render(mcpLabel))
		} else {
			// Color by health status when not selected.
			var mcpStyle lipgloss.Style
			if connected == total {
				mcpStyle = lipgloss.NewStyle().Foreground(m.theme.Success)
			} else {
				mcpStyle = lipgloss.NewStyle().Foreground(m.theme.Primary)
			}
			header = append(header, "  "+mcpStyle.Render(mcpLabel))
		}
	}

	// Scroll lock indicator
	if m.autoScrollLocked {
		lockStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)
		header = append(header, "  "+lockStyle.Render("[scroll locked]"))
	}

	bar := strings.Join(header, "")
	return m.styles.StatusBar.Width(m.width).Render(bar)
}

// renderSingleProviderPanel renders one provider's response as a full panel.
func (m Model) renderSingleProviderPanel(panel ProviderPanel) string {
	content := panel.Viewport.View()
	if content == "" {
		switch panel.Status {
		case StatusLoading:
			content = m.spinner.View() + " Waiting for response..."
		case StatusIdle:
			content = m.styles.Dimmed.Render("No response yet")
		case StatusDone:
			if len(panel.TraceSections) == 0 {
				content = m.styles.Dimmed.Render("Model responded with tool calls only (no text output).\nTool execution is handled by the consensus orchestrator.")
			}
		case StatusFailed:
			content = m.styles.StatusUnhealthy.Render("Provider failed")
		}
	}

	// Append a spinner when the panel has content but is still loading
	if panel.Status == StatusLoading && content != "" {
		content += "\n" + m.spinner.View()
	}

	content = renderWithScrollbar(content, panel.Viewport.TotalLineCount(), panel.Viewport.ScrollPercent(), panel.Viewport.Height, m.theme)
	style := m.styles.ConsensusBorder.Width(m.width - 4)
	return style.Render(content)
}

func (m Model) renderModePicker() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Tertiary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Info).Background(m.theme.BgSelected)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.TextBright)
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
	yoloOnStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Primary)

	var lines []string
	lines = append(lines, titleStyle.Render("Select Mode"))
	lines = append(lines, "")

	descriptions := map[string]string{
		"quick":    "All providers, concise direct answer",
		"balanced": "All providers, structured synthesis",
		"thorough": "All providers, deep reasoning + trade-offs",
	}

	// Mode items (0..2) + yolo toggle (3)
	totalItems := len(m.modePickerItems) + 1 // +1 for yolo toggle

	for i, item := range m.modePickerItems {
		cursor := "  "
		style := normalStyle
		if i == m.modePickerIdx {
			cursor = "▸ "
			style = selectedStyle
		}
		current := ""
		if item == m.currentMode {
			current = " (current)"
		}
		line := cursor + style.Render(item+current)
		if desc, ok := descriptions[item]; ok {
			line += "  " + hintStyle.Render(desc)
		}
		lines = append(lines, line)
	}

	// Yolo toggle as last item
	lines = append(lines, "")
	yoloIdx := totalItems - 1
	cursor := "  "
	style := normalStyle
	if m.modePickerIdx == yoloIdx {
		cursor = "▸ "
		style = selectedStyle
	}
	checkbox := "[ ]"
	if m.yoloMode {
		checkbox = yoloOnStyle.Render("[✓]")
	}
	lines = append(lines, cursor+checkbox+" "+style.Render("yolo")+"  "+hintStyle.Render("Auto-approve all tool actions"))

	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("  ↑/↓ navigate  Enter select  Esc cancel"))

	border := m.styles.InputBorder.Width(m.width - 4)
	return addDropShadow(border.Render(strings.Join(lines, "\n")), m.theme)
}

// renderThemePicker renders the theme selection overlay.
func (m Model) renderThemePicker() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Tertiary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Info).Background(m.theme.BgSelected)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.TextBright)
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)

	var lines []string
	lines = append(lines, titleStyle.Render("Select Theme"))
	lines = append(lines, "")

	for i, name := range m.themePickerItems {
		cursor := "  "
		style := normalStyle
		if i == m.themePickerCursor {
			cursor = "▸ "
			style = selectedStyle
		}
		current := ""
		if name == m.theme.Name {
			current = " (current)"
		}
		lines = append(lines, cursor+style.Render(name+current))
	}

	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("  ↑/↓ navigate  Enter select  Esc cancel"))

	border := m.styles.InputBorder.Width(m.width - 4)
	return addDropShadow(border.Render(strings.Join(lines, "\n")), m.theme)
}

// renderCommandPalette renders the slash command palette overlay.
func (m Model) renderCommandPalette() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Tertiary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Info).Background(m.theme.BgSelected)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.TextBright)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
	shortcutStyle := lipgloss.NewStyle().Foreground(m.theme.Secondary)
	filterStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
	categoryStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.TextMuted)
	extStyle := lipgloss.NewStyle().Foreground(m.theme.Secondary)

	var lines []string

	if m.paletteViaCtrlP {
		lines = append(lines, titleStyle.Render("Command Palette")+" "+lipgloss.NewStyle().Foreground(m.theme.BorderNormal).Render(strings.Repeat("─", 40)))
	} else {
		lines = append(lines, titleStyle.Render("Commands")+" "+lipgloss.NewStyle().Foreground(m.theme.BorderNormal).Render(strings.Repeat("/", 50)))
	}
	lines = append(lines, "")

	// Filter input
	filterDisplay := m.paletteFilter
	if filterDisplay == "" {
		if m.paletteViaCtrlP {
			filterDisplay = hintStyle.Render("Search commands and files...")
		} else {
			filterDisplay = hintStyle.Render("Type to filter")
		}
	} else {
		filterDisplay = filterStyle.Render(filterDisplay)
	}
	lines = append(lines, "  > "+filterDisplay)
	lines = append(lines, "")

	itemIdx := 0 // global index across all categories for cursor tracking
	hasContent := false

	// Commands section
	if len(m.paletteMatches) > 0 {
		if m.paletteViaCtrlP && len(m.paletteFiles) > 0 {
			lines = append(lines, "  "+categoryStyle.Render("Commands"))
		}
		hasContent = true
		for _, cmd := range m.paletteMatches {
			style := normalStyle
			cursor := "  "
			if itemIdx == m.paletteCursor {
				style = selectedStyle
				cursor = "> "
			}

			line := cursor + style.Render(cmd.Name)

			// Pad name to align descriptions
			padding := 22 - len(cmd.Name)
			if padding < 2 {
				padding = 2
			}
			line += strings.Repeat(" ", padding)
			line += descStyle.Render(cmd.Description)

			if cmd.Shortcut != "" {
				line += "  " + shortcutStyle.Render(cmd.Shortcut)
			}

			lines = append(lines, line)
			itemIdx++
		}
	}

	// Files section (only in Ctrl+P mode)
	if m.paletteViaCtrlP && len(m.paletteFiles) > 0 {
		if hasContent {
			lines = append(lines, "")
		}
		lines = append(lines, "  "+categoryStyle.Render("Files"))
		for _, fm := range m.paletteFiles {
			style := normalStyle
			cursor := "  "
			if itemIdx == m.paletteCursor {
				style = selectedStyle
				cursor = "> "
			}

			ext := ""
			if fm.Ext != "" {
				ext = "  " + extStyle.Render("["+fm.Ext+"]")
			}
			lines = append(lines, cursor+style.Render(fm.Path)+ext)
			itemIdx++
		}
	}

	if !hasContent && len(m.paletteFiles) == 0 {
		lines = append(lines, "  "+descStyle.Render("No matching commands"))
	}

	lines = append(lines, "")
	if m.paletteViaCtrlP {
		lines = append(lines, "  "+hintStyle.Render("↑↓ navigate  Enter select  Esc close  type to filter"))
	} else {
		lines = append(lines, "  "+hintStyle.Render("↑↓ navigate  Tab accept  Enter submit  type to filter"))
	}

	border := m.styles.InputBorder.Width(m.width - 4)
	return addDropShadow(border.Render(strings.Join(lines, "\n")), m.theme)
}

// renderFilePicker renders the file picker overlay above the input.
func (m Model) renderFilePicker() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Tertiary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Info).Background(m.theme.BgSelected)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.TextBright)
	pathStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
	extStyle := lipgloss.NewStyle().Foreground(m.theme.Secondary)
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
	filterStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)

	var lines []string
	lines = append(lines, titleStyle.Render("Files")+" "+lipgloss.NewStyle().Foreground(m.theme.BorderNormal).Render(strings.Repeat("─", 50)))
	lines = append(lines, "")

	// Filter display
	filterDisplay := m.filePickerFilter
	if filterDisplay == "" {
		filterDisplay = hintStyle.Render("Type to filter")
	} else {
		filterDisplay = filterStyle.Render(filterDisplay)
	}
	lines = append(lines, "  @ "+filterDisplay)
	lines = append(lines, "")

	// File list
	if len(m.filePickerMatches) == 0 {
		lines = append(lines, "  "+pathStyle.Render("No matching files"))
	} else {
		for i, fm := range m.filePickerMatches {
			style := normalStyle
			cursor := "  "
			if i == m.filePickerCursor {
				style = selectedStyle
				cursor = "> "
			}

			ext := ""
			if fm.Ext != "" {
				ext = "  " + extStyle.Render("["+fm.Ext+"]")
			}

			lines = append(lines, cursor+style.Render(fm.Path)+ext)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "  "+hintStyle.Render("↑↓ navigate  Tab/Enter accept  Esc cancel"))

	border := m.styles.InputBorder.Width(m.width - 4)
	return addDropShadow(border.Render(strings.Join(lines, "\n")), m.theme)
}

func (m Model) renderInput() string {
	label := m.styles.Prompt.Render("❯ ")
	input := m.textarea.View()

	var parts []string

	// Transient status message
	if m.chatStatusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(m.theme.Success).
			Italic(true)
		parts = append(parts, statusStyle.Render(m.chatStatusMsg))
	}

	// Attachment pills
	if len(m.attachedFiles) > 0 {
		pillStyle := lipgloss.NewStyle().
			Foreground(m.theme.Tertiary).
			Background(m.theme.BgSelected).
			Padding(0, 1)
		var pills []string
		for _, f := range m.attachedFiles {
			pills = append(pills, pillStyle.Render("@"+f+" ×"))
		}
		parts = append(parts, strings.Join(pills, " "))
	}

	parts = append(parts, fmt.Sprintf("%s\n%s", label, input))

	style := m.styles.InputBorder.Width(m.width - 4)
	return style.Render(strings.Join(parts, "\n"))
}

