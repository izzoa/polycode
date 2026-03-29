package hooks

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"text/template"

	"github.com/izzoa/polycode/internal/config"
)

// HookEvent identifies the lifecycle point at which a hook runs.
type HookEvent string

const (
	PreQuery  HookEvent = "pre_query"
	PostQuery HookEvent = "post_query"
	PostTool  HookEvent = "post_tool"
	OnError   HookEvent = "on_error"
)

// HookContext carries data available for template substitution in hook commands.
type HookContext struct {
	Prompt   string
	Response string
	Error    string
	ToolName string
}

// HookManager executes shell-based lifecycle hooks.
type HookManager struct {
	config config.HooksConfig
}

// NewHookManager creates a HookManager from the given hooks configuration.
func NewHookManager(cfg config.HooksConfig) *HookManager {
	return &HookManager{config: cfg}
}

// Run executes the hook command for the given event, substituting template
// variables from ctx. If no command is configured for the event, it returns
// nil. Errors from hook execution are logged but not returned — hooks must
// not block the pipeline.
func (m *HookManager) Run(event HookEvent, ctx HookContext) error {
	cmd := m.commandFor(event)
	if cmd == "" {
		return nil
	}

	rendered, err := renderTemplate(cmd, ctx)
	if err != nil {
		log.Printf("hooks: template error for %s: %v", event, err)
		return nil
	}

	if execErr := exec.Command("sh", "-c", rendered).Run(); execErr != nil {
		log.Printf("hooks: %s command failed: %v", event, execErr)
	}

	return nil
}

// commandFor returns the configured shell command for the given event.
func (m *HookManager) commandFor(event HookEvent) string {
	switch event {
	case PreQuery:
		return m.config.PreQuery
	case PostQuery:
		return m.config.PostQuery
	case PostTool:
		return m.config.PostTool
	case OnError:
		return m.config.OnError
	default:
		return ""
	}
}

// shellEscape quotes a string for safe use in shell commands.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// safeCtx wraps HookContext fields with shell-escaped versions.
type safeCtx struct {
	Prompt   string
	Response string
	Error    string
	ToolName string
}

// renderTemplate applies Go text/template substitution to the command string.
// All context fields are shell-escaped to prevent injection.
func renderTemplate(cmdTemplate string, ctx HookContext) (string, error) {
	safe := safeCtx{
		Prompt:   shellEscape(ctx.Prompt),
		Response: shellEscape(ctx.Response),
		Error:    shellEscape(ctx.Error),
		ToolName: shellEscape(ctx.ToolName),
	}
	tmpl, err := template.New("hook").Parse(cmdTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, safe); err != nil {
		return "", err
	}
	return buf.String(), nil
}
