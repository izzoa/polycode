package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to write a YAML file to a path, creating intermediate dirs.
func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadPolicies_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	writeYAML(t, filepath.Join(repoDir, ".polycode", "permissions.yaml"), `
tools:
  file_read: allow
  file_write: ask
  shell_exec: deny
`)

	pm, err := LoadPolicies(repoDir)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}

	tests := []struct {
		tool   string
		expect Policy
	}{
		{"file_read", PolicyAllow},
		{"file_write", PolicyAsk},
		{"shell_exec", PolicyDeny},
	}

	for _, tc := range tests {
		got := pm.Check(tc.tool)
		if got != tc.expect {
			t.Errorf("Check(%q) = %q, want %q", tc.tool, got, tc.expect)
		}
	}
}

func TestLoadPolicies_GlobMatch(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	writeYAML(t, filepath.Join(repoDir, ".polycode", "permissions.yaml"), `
tools:
  mcp_filesystem_*: allow
  mcp_database_*: deny
`)

	pm, err := LoadPolicies(repoDir)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}

	tests := []struct {
		tool   string
		expect Policy
	}{
		{"mcp_filesystem_read", PolicyAllow},
		{"mcp_filesystem_write", PolicyAllow},
		{"mcp_database_query", PolicyDeny},
		{"mcp_database_delete", PolicyDeny},
	}

	for _, tc := range tests {
		got := pm.Check(tc.tool)
		if got != tc.expect {
			t.Errorf("Check(%q) = %q, want %q", tc.tool, got, tc.expect)
		}
	}
}

func TestLoadPolicies_DefaultsToAsk(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	// No permissions file at all.
	pm, err := LoadPolicies(repoDir)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}

	if got := pm.Check("unknown_tool"); got != PolicyAsk {
		t.Errorf("Check(unknown_tool) = %q, want %q", got, PolicyAsk)
	}
}

func TestLoadPolicies_RepoPrecedenceOverUser(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	// Simulate user-level config dir via XDG_CONFIG_HOME.
	userConfigDir := filepath.Join(tmpDir, "usercfg")
	t.Setenv("XDG_CONFIG_HOME", userConfigDir)

	writeYAML(t, filepath.Join(userConfigDir, "polycode", "permissions.yaml"), `
tools:
  file_read: deny
  shell_exec: allow
`)

	writeYAML(t, filepath.Join(repoDir, ".polycode", "permissions.yaml"), `
tools:
  file_read: allow
`)

	pm, err := LoadPolicies(repoDir)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}

	// Repo overrides user for file_read.
	if got := pm.Check("file_read"); got != PolicyAllow {
		t.Errorf("Check(file_read) = %q, want %q (repo override)", got, PolicyAllow)
	}

	// User-level shell_exec policy still applies (not overridden by repo).
	if got := pm.Check("shell_exec"); got != PolicyAllow {
		t.Errorf("Check(shell_exec) = %q, want %q (user fallback)", got, PolicyAllow)
	}
}

func TestLoadPolicies_UserOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	userConfigDir := filepath.Join(tmpDir, "usercfg")
	t.Setenv("XDG_CONFIG_HOME", userConfigDir)

	writeYAML(t, filepath.Join(userConfigDir, "polycode", "permissions.yaml"), `
tools:
  file_write: deny
`)

	pm, err := LoadPolicies(repoDir)
	if err != nil {
		t.Fatalf("LoadPolicies: %v", err)
	}

	if got := pm.Check("file_write"); got != PolicyDeny {
		t.Errorf("Check(file_write) = %q, want %q", got, PolicyDeny)
	}
}
