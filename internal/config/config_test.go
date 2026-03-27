package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateMinimalValid(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "test", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "gpt-4o", Primary: true},
		},
		Consensus: ConsensusConfig{MinResponses: 1},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestValidateNoProviders(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for no providers")
	}
}

func TestValidateNoPrimary(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "a", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for no primary")
	}
}

func TestValidateMultiplePrimaries(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "a", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m", Primary: true},
			{Name: "b", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m", Primary: true},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for multiple primaries")
	}
}

func TestValidateDuplicateNames(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "same", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m", Primary: true},
			{Name: "same", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
}

func TestValidateUnknownType(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "bad", Type: "unknown", Auth: AuthMethodAPIKey, Model: "m", Primary: true},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestValidateOpenAICompatMissingBaseURL(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "compat", Type: ProviderTypeOpenAICompatible, Auth: AuthMethodNone, Model: "m", Primary: true},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
}

func TestValidateOpenAICompatWithBaseURL(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "compat", Type: ProviderTypeOpenAICompatible, Auth: AuthMethodNone, Model: "m", Primary: true, BaseURL: "http://localhost:11434/v1"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

// --- MCP validation tests ---

func validBaseConfig() Config {
	return Config{
		Providers: []ProviderConfig{
			{Name: "p", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m", Primary: true},
		},
	}
}

func TestValidateMCPEmptyName(t *testing.T) {
	cfg := validBaseConfig()
	cfg.MCP.Servers = []MCPServerConfig{{Name: "", Command: "npx"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty MCP server name")
	}
}

func TestValidateMCPDuplicateNames(t *testing.T) {
	cfg := validBaseConfig()
	cfg.MCP.Servers = []MCPServerConfig{
		{Name: "fs", Command: "npx"},
		{Name: "fs", Command: "node"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate MCP server names")
	}
}

func TestValidateMCPNoCommandOrURL(t *testing.T) {
	cfg := validBaseConfig()
	cfg.MCP.Servers = []MCPServerConfig{{Name: "broken"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for MCP server with no command or URL")
	}
}

func TestValidateMCPNegativeTimeout(t *testing.T) {
	cfg := validBaseConfig()
	cfg.MCP.Servers = []MCPServerConfig{{Name: "fs", Command: "npx", Timeout: -1}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative MCP timeout")
	}
}

func TestValidateMCPValidConfig(t *testing.T) {
	cfg := validBaseConfig()
	cfg.MCP.Servers = []MCPServerConfig{
		{Name: "fs", Command: "npx", Args: []string{"-y", "server-fs"}},
		{Name: "remote", URL: "http://localhost:3000/mcp"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid MCP config, got: %v", err)
	}
}

func TestValidateMCPProviderNameCollision(t *testing.T) {
	// Same name for provider and MCP server should pass (warning only, not error).
	cfg := validBaseConfig()
	cfg.Providers[0].Name = "github"
	cfg.MCP.Servers = []MCPServerConfig{{Name: "github", Command: "npx"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation to pass with warning, got error: %v", err)
	}
}

func TestPrimaryProvider(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{
			{Name: "a", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "m"},
			{Name: "b", Type: ProviderTypeAnthropic, Auth: AuthMethodAPIKey, Model: "m", Primary: true},
		},
	}
	p := cfg.PrimaryProvider()
	if p == nil {
		t.Fatal("expected non-nil primary")
	}
	if p.Name != "b" {
		t.Fatalf("expected primary 'b', got '%s'", p.Name)
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := DefaultConfig()
	cfg.Providers = []ProviderConfig{
		{Name: "test", Type: ProviderTypeOpenAI, Auth: AuthMethodAPIKey, Model: "gpt-4o", Primary: true},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	path := filepath.Join(tmpDir, "polycode", "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded.Providers) != 1 || loaded.Providers[0].Name != "test" {
		t.Fatalf("loaded config doesn't match: %+v", loaded)
	}
}
