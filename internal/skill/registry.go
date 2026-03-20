package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// Skill is a loaded, ready-to-use skill instance.
type Skill struct {
	Manifest     Manifest
	Dir          string // absolute path to skill directory
	SystemPrompt string // contents of system_prompt.md (may be empty)
}

// Registry manages installed skills.
type Registry struct {
	skillsDir string
	skills    map[string]*Skill
}

// NewRegistry creates a Registry that loads skills from the given directory.
func NewRegistry(skillsDir string) *Registry {
	return &Registry{
		skillsDir: skillsDir,
		skills:    make(map[string]*Skill),
	}
}

// Load discovers and loads all skills from the skills directory.
// Invalid skills are skipped with a warning logged to stderr.
func (r *Registry) Load() error {
	r.skills = make(map[string]*Skill)

	entries, err := os.ReadDir(r.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no skills directory yet
		}
		return fmt.Errorf("reading skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(r.skillsDir, entry.Name())
		manifestPath := filepath.Join(skillDir, "skill.yaml")

		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping skill %q: %v\n", entry.Name(), err)
			continue
		}

		r.skills[manifest.Name] = &Skill{
			Manifest:     *manifest,
			Dir:          skillDir,
			SystemPrompt: LoadSystemPrompt(skillDir),
		}
	}

	return nil
}

// List returns all loaded skills, sorted by name.
func (r *Registry) List() []*Skill {
	out := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest.Name < out[j].Manifest.Name
	})
	return out
}

// Get returns a skill by name, or nil if not found.
func (r *Registry) Get(name string) *Skill {
	return r.skills[name]
}

// Install copies a skill from sourcePath into the skills directory.
// The skill is validated before installation.
func (r *Registry) Install(sourcePath string) error {
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	manifestPath := filepath.Join(absSource, "skill.yaml")
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("invalid skill: %w", err)
	}

	// Check for name conflicts
	if existing := r.skills[manifest.Name]; existing != nil {
		return fmt.Errorf("skill %q is already installed at %s", manifest.Name, existing.Dir)
	}

	// Create skills directory if needed
	if err := os.MkdirAll(r.skillsDir, 0755); err != nil {
		return fmt.Errorf("creating skills dir: %w", err)
	}

	// Copy skill directory
	destDir := filepath.Join(r.skillsDir, manifest.Name)
	if err := copyDir(absSource, destDir); err != nil {
		return fmt.Errorf("copying skill: %w", err)
	}

	// Load the installed skill
	r.skills[manifest.Name] = &Skill{
		Manifest:     *manifest,
		Dir:          destDir,
		SystemPrompt: LoadSystemPrompt(destDir),
	}

	return nil
}

// Remove uninstalls a skill by name.
func (r *Registry) Remove(name string) error {
	skill := r.skills[name]
	if skill == nil {
		return fmt.Errorf("skill %q is not installed", name)
	}

	if err := os.RemoveAll(skill.Dir); err != nil {
		return fmt.Errorf("removing skill directory: %w", err)
	}

	delete(r.skills, name)
	return nil
}

// ToToolDefinitions returns provider.ToolDefinition objects for all tools
// provided by all loaded skills. Tool names are prefixed with "skill_{name}_"
// to avoid collisions.
func (r *Registry) ToToolDefinitions() []provider.ToolDefinition {
	var defs []provider.ToolDefinition
	for _, skill := range r.skills {
		for _, tool := range skill.Manifest.Tools {
			params := tool.Parameters
			if params == nil {
				params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			defs = append(defs, provider.ToolDefinition{
				Name:        fmt.Sprintf("skill_%s_%s", skill.Manifest.Name, tool.Name),
				Description: tool.Description,
				Parameters:  params,
			})
		}
	}
	return defs
}

// SystemPrompts returns the concatenated system prompts from all loaded skills.
func (r *Registry) SystemPrompts() string {
	var parts []string
	for _, skill := range r.List() {
		if skill.SystemPrompt != "" {
			parts = append(parts, skill.SystemPrompt)
		}
	}
	return strings.Join(parts, "\n\n")
}

// ExecuteTool runs a skill tool's handler script and returns the output.
// The tool name should be in the format "skill_{skillname}_{toolname}".
func (r *Registry) ExecuteTool(ctx context.Context, fullName string, arguments string) (string, error) {
	// Parse "skill_{skillname}_{toolname}"
	if !strings.HasPrefix(fullName, "skill_") {
		return "", fmt.Errorf("not a skill tool: %s", fullName)
	}
	rest := fullName[6:] // after "skill_"

	// Find the skill name and tool name
	var skillName, toolName string
	for name := range r.skills {
		prefix := name + "_"
		if strings.HasPrefix(rest, prefix) {
			skillName = name
			toolName = rest[len(prefix):]
			break
		}
	}
	if skillName == "" {
		return "", fmt.Errorf("unknown skill tool: %s", fullName)
	}

	skill := r.skills[skillName]
	var toolManifest *ToolManifest
	for i := range skill.Manifest.Tools {
		if skill.Manifest.Tools[i].Name == toolName {
			toolManifest = &skill.Manifest.Tools[i]
			break
		}
	}
	if toolManifest == nil {
		return "", fmt.Errorf("tool %q not found in skill %q", toolName, skillName)
	}

	// Execute the handler script with arguments as stdin
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", toolManifest.Handler)
	cmd.Dir = skill.Dir
	cmd.Stdin = strings.NewReader(arguments)

	// Pass arguments as environment variable too
	cmd.Env = append(os.Environ(), "POLYCODE_TOOL_ARGS="+arguments)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("tool handler failed: %w", err)
	}

	return string(out), nil
}

// SlashCommands returns the slash commands registered by all skills.
func (r *Registry) SlashCommands() []string {
	var cmds []string
	for _, skill := range r.List() {
		if skill.Manifest.Command != "" {
			cmds = append(cmds, "/"+skill.Manifest.Command)
		}
	}
	return cmds
}

// HandleCommand dispatches a slash command to the appropriate skill.
// Returns the skill's system prompt context and true if handled, or empty and false.
func (r *Registry) HandleCommand(command string) (string, bool) {
	// Strip leading /
	cmd := strings.TrimPrefix(command, "/")
	cmd = strings.Fields(cmd)[0] // just the command name

	for _, skill := range r.skills {
		if skill.Manifest.Command == cmd {
			return skill.SystemPrompt, true
		}
	}
	return "", false
}

// FormatList returns a human-readable listing of installed skills.
func (r *Registry) FormatList() string {
	skills := r.List()
	if len(skills) == 0 {
		return "No skills installed."
	}

	var b strings.Builder
	b.WriteString("Installed skills:\n\n")
	for _, s := range skills {
		b.WriteString(fmt.Sprintf("  %s", s.Manifest.Name))
		if s.Manifest.Version != "" {
			b.WriteString(fmt.Sprintf(" v%s", s.Manifest.Version))
		}
		b.WriteString(fmt.Sprintf(" — %s\n", s.Manifest.Description))
		if s.Manifest.Command != "" {
			b.WriteString(fmt.Sprintf("    command: /%s\n", s.Manifest.Command))
		}
		if len(s.Manifest.Tools) > 0 {
			b.WriteString(fmt.Sprintf("    tools: %d\n", len(s.Manifest.Tools)))
		}
	}
	return b.String()
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// Ensure json is available for potential future use
var _ = json.Marshal
