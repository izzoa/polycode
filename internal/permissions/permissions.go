package permissions

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/izzoa/polycode/internal/config"
	"gopkg.in/yaml.v3"
)

// Policy represents a tool-approval policy.
type Policy string

const (
	PolicyAllow Policy = "allow"
	PolicyAsk   Policy = "ask"
	PolicyDeny  Policy = "deny"
)

// PolicyManager evaluates per-tool permission policies.
type PolicyManager struct {
	policies map[string]Policy // tool name or glob pattern -> policy
}

// permissionsFile mirrors the YAML structure on disk.
type permissionsFile struct {
	Tools map[string]string `yaml:"tools"`
}

// LoadPolicies builds a PolicyManager by reading permissions YAML files.
// It checks the repo-level file (.polycode/permissions.yaml in workDir) first,
// then the user-level file (~/.config/polycode/permissions.yaml). Repo-level
// policies take precedence over user-level policies.
func LoadPolicies(workDir string) (*PolicyManager, error) {
	merged := make(map[string]Policy)

	// 1. Load user-level policies (lower precedence).
	userPath := filepath.Join(config.ConfigDir(), "permissions.yaml")
	if err := loadInto(userPath, merged); err != nil {
		return nil, err
	}

	// 2. Load repo-level policies (higher precedence — overwrites user).
	repoPath := filepath.Join(workDir, ".polycode", "permissions.yaml")
	if err := loadInto(repoPath, merged); err != nil {
		return nil, err
	}

	return &PolicyManager{policies: merged}, nil
}

// loadInto reads a permissions YAML file and merges entries into dst.
// If the file does not exist, it is silently skipped.
func loadInto(path string, dst map[string]Policy) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var pf permissionsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return err
	}

	for tool, policyStr := range pf.Tools {
		switch Policy(policyStr) {
		case PolicyAllow, PolicyAsk, PolicyDeny:
			dst[tool] = Policy(policyStr)
		}
	}
	return nil
}

// Check returns the policy for the given tool name.
// Evaluation order:
//  1. Exact match in the policies map.
//  2. Glob/pattern match (entries containing "*").
//  3. Default: PolicyAsk.
func (p *PolicyManager) Check(toolName string) Policy {
	// Exact match.
	if pol, ok := p.policies[toolName]; ok {
		return pol
	}

	// Glob pattern match — iterate patterns that contain a wildcard.
	for pattern, pol := range p.policies {
		if !strings.Contains(pattern, "*") {
			continue
		}
		if matched, _ := path.Match(pattern, toolName); matched {
			return pol
		}
	}

	// Default.
	return PolicyAsk
}
