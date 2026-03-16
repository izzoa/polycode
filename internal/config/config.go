package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type ProviderType string

const (
	ProviderTypeAnthropic        ProviderType = "anthropic"
	ProviderTypeOpenAI           ProviderType = "openai"
	ProviderTypeGoogle           ProviderType = "google"
	ProviderTypeOpenAICompatible ProviderType = "openai_compatible"
)

type AuthMethod string

const (
	AuthMethodAPIKey AuthMethod = "api_key"
	AuthMethodOAuth  AuthMethod = "oauth"
	AuthMethodNone   AuthMethod = "none"
)

type ProviderConfig struct {
	Name       string       `yaml:"name"`
	Type       ProviderType `yaml:"type"`
	Auth       AuthMethod   `yaml:"auth"`
	Model      string       `yaml:"model"`
	Primary    bool         `yaml:"primary,omitempty"`
	BaseURL    string       `yaml:"base_url,omitempty"`
	MaxContext int          `yaml:"max_context,omitempty"`
}

type ConsensusConfig struct {
	Timeout      time.Duration `yaml:"-"`
	TimeoutRaw   string        `yaml:"timeout"`
	MinResponses int           `yaml:"min_responses"`
}

type TUIConfig struct {
	Theme          string `yaml:"theme"`
	ShowIndividual bool   `yaml:"show_individual"`
}

type MetadataConfig struct {
	URL         string        `yaml:"url,omitempty"`
	CacheTTLRaw string        `yaml:"cache_ttl,omitempty"`
	CacheTTL    time.Duration `yaml:"-"`
}

type Config struct {
	Providers []ProviderConfig `yaml:"providers"`
	Consensus ConsensusConfig  `yaml:"consensus"`
	TUI       TUIConfig        `yaml:"tui"`
	Metadata  MetadataConfig   `yaml:"metadata,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		Consensus: ConsensusConfig{
			Timeout:      60 * time.Second,
			TimeoutRaw:   "60s",
			MinResponses: 2,
		},
		TUI: TUIConfig{
			Theme:          "dark",
			ShowIndividual: true,
		},
		Metadata: MetadataConfig{
			CacheTTL: 24 * time.Hour,
		},
	}
}

func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "polycode")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "polycode")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s — run 'polycode init' to set up", path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Consensus.TimeoutRaw != "" {
		d, err := time.ParseDuration(cfg.Consensus.TimeoutRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid consensus.timeout %q: %w", cfg.Consensus.TimeoutRaw, err)
		}
		cfg.Consensus.Timeout = d
	}

	if cfg.Metadata.CacheTTLRaw != "" {
		d, err := time.ParseDuration(cfg.Metadata.CacheTTLRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata.cache_ttl %q: %w", cfg.Metadata.CacheTTLRaw, err)
		}
		cfg.Metadata.CacheTTL = d
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("at least one provider must be configured")
	}

	names := make(map[string]bool)
	primaryCount := 0

	for i, p := range c.Providers {
		if p.Name == "" {
			return fmt.Errorf("provider[%d]: name is required", i)
		}
		if names[p.Name] {
			return fmt.Errorf("provider[%d]: duplicate name %q", i, p.Name)
		}
		names[p.Name] = true

		switch p.Type {
		case ProviderTypeAnthropic, ProviderTypeOpenAI, ProviderTypeGoogle, ProviderTypeOpenAICompatible:
			// valid
		default:
			return fmt.Errorf("provider %q: unknown type %q (must be anthropic, openai, google, or openai_compatible)", p.Name, p.Type)
		}

		if p.Model == "" {
			return fmt.Errorf("provider %q: model is required", p.Name)
		}

		switch p.Auth {
		case AuthMethodAPIKey, AuthMethodOAuth, AuthMethodNone:
			// valid
		default:
			return fmt.Errorf("provider %q: unknown auth method %q (must be api_key, oauth, or none)", p.Name, p.Auth)
		}

		if p.Type == ProviderTypeOpenAICompatible && p.BaseURL == "" {
			return fmt.Errorf("provider %q: base_url is required for openai_compatible type", p.Name)
		}

		if p.Primary {
			primaryCount++
		}
	}

	if primaryCount == 0 {
		return fmt.Errorf("exactly one provider must be marked as primary — none found")
	}
	if primaryCount > 1 {
		return fmt.Errorf("exactly one provider must be marked as primary — found %d", primaryCount)
	}

	return nil
}

func (c *Config) PrimaryProvider() *ProviderConfig {
	for i := range c.Providers {
		if c.Providers[i].Primary {
			return &c.Providers[i]
		}
	}
	return nil
}

func (c *Config) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(ConfigPath(), data, 0600)
}
