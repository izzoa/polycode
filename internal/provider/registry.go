package provider

import (
	"fmt"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
)

// Registry holds all configured providers and provides access to them.
type Registry struct {
	providers []Provider
	primary   Provider
}

// NewRegistry creates a Registry from the application config. It instantiates
// provider adapters for each entry in config.Providers.
func NewRegistry(cfg *config.Config) (*Registry, error) {
	store := auth.NewStore()

	r := &Registry{}

	for _, pc := range cfg.Providers {
		if pc.Disabled {
			continue
		}
		p, err := newProvider(pc, store)
		if err != nil {
			return nil, fmt.Errorf("creating provider %q: %w", pc.Name, err)
		}
		r.providers = append(r.providers, p)
		if pc.Primary {
			r.primary = p
		}
	}

	if r.primary == nil {
		return nil, fmt.Errorf("no primary provider configured")
	}

	return r, nil
}

// NewRegistryWithStore creates a Registry using the given auth store. This
// allows callers to pre-populate API keys before creating the registry.
func NewRegistryWithStore(cfg *config.Config, store auth.Store) (*Registry, error) {
	r := &Registry{}

	for _, pc := range cfg.Providers {
		if pc.Disabled {
			continue
		}
		p, err := newProvider(pc, store)
		if err != nil {
			return nil, fmt.Errorf("creating provider %q: %w", pc.Name, err)
		}
		r.providers = append(r.providers, p)
		if pc.Primary {
			r.primary = p
		}
	}

	if r.primary == nil {
		return nil, fmt.Errorf("no primary provider configured")
	}

	return r, nil
}

// newProvider creates a Provider implementation from a ProviderConfig.
func newProvider(pc config.ProviderConfig, store auth.Store) (Provider, error) {
	var oauthCfg *auth.DeviceFlowConfig
	if pc.Auth == config.AuthMethodOAuth {
		if pc.OAuthClientID != "" {
			// User provided explicit OAuth config
			oauthCfg = &auth.DeviceFlowConfig{
				ClientID:      pc.OAuthClientID,
				DeviceAuthURL: pc.OAuthDeviceURL,
				TokenURL:      pc.OAuthTokenURL,
			}
		} else {
			hint := "set oauth_client_id, oauth_device_url, and oauth_token_url in config"
			if pc.Type == config.ProviderTypeAnthropic || pc.Type == config.ProviderTypeOpenAI {
				hint = "use auth: api_key instead — these providers do not support OAuth for third-party apps"
			}
			return nil, fmt.Errorf("provider %q: auth is oauth but no OAuth endpoints configured — %s", pc.Name, hint)
		}
	}

	switch pc.Type {
	case config.ProviderTypeAnthropic:
		return NewAnthropicProvider(pc.Name, pc.Model, pc.BaseURL, string(pc.Auth), oauthCfg, store), nil
	case config.ProviderTypeOpenAI:
		return NewOpenAIProvider(pc.Name, pc.Model, pc.BaseURL, string(pc.Auth), oauthCfg, store), nil
	case config.ProviderTypeGoogle:
		return NewGeminiProvider(pc.Name, pc.Model, pc.BaseURL, string(pc.Auth), oauthCfg, store), nil
	case config.ProviderTypeOpenAICompatible:
		return NewOpenAICompatProvider(pc.Name, pc.Model, pc.BaseURL, pc.Auth, store), nil
	default:
		return nil, fmt.Errorf("unknown provider type %q", pc.Type)
	}
}

// Providers returns all registered providers.
func (r *Registry) Providers() []Provider {
	return r.providers
}

// Primary returns the primary provider.
func (r *Registry) Primary() Provider {
	return r.primary
}

// Healthy returns providers that pass Validate(). Providers that fail
// validation are silently excluded.
func (r *Registry) Healthy() []Provider {
	var healthy []Provider
	for _, p := range r.providers {
		if err := p.Validate(); err == nil {
			healthy = append(healthy, p)
		}
	}
	return healthy
}
