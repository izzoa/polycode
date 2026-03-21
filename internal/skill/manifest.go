// Package skill implements a local plugin system for polycode.
// Skills are directories containing a skill.yaml manifest, optional system
// prompt, and optional tool definitions with handler scripts.
package skill

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest describes a skill's metadata and capabilities.
type Manifest struct {
	Name        string           `yaml:"name"`
	Version     string           `yaml:"version,omitempty"`
	Description string           `yaml:"description"`
	Command     string           `yaml:"command,omitempty"` // slash command name (without /)
	Tools       []ToolManifest   `yaml:"tools,omitempty"`
	Enabled     bool             `yaml:"enabled,omitempty"`
}

// ToolManifest describes a tool provided by a skill.
type ToolManifest struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Parameters  map[string]any `yaml:"parameters,omitempty"` // JSON Schema object
	Handler     string                 `yaml:"handler"`             // shell command to execute
}

// LoadManifest reads and parses a skill.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("skill manifest missing required field: name")
	}

	return &m, nil
}

// LoadSystemPrompt reads the system_prompt.md file from a skill directory,
// returning empty string if the file does not exist.
func LoadSystemPrompt(skillDir string) string {
	data, err := os.ReadFile(filepath.Join(skillDir, "system_prompt.md"))
	if err != nil {
		return ""
	}
	return string(data)
}
