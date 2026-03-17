package agent

import (
	"github.com/izzoa/polycode/internal/provider"
)

// ResolveProvider looks up the provider for a given role. It checks the
// roles map (role -> provider name), finds the matching provider in the
// registry by ID, and falls back to the registry's primary provider if
// no match is found.
func ResolveProvider(role RoleType, registry *provider.Registry, roles map[RoleType]string) provider.Provider {
	name, ok := roles[role]
	if ok && name != "" {
		for _, p := range registry.Providers() {
			if p.ID() == name {
				return p
			}
		}
	}

	// Fall back to primary provider.
	return registry.Primary()
}
