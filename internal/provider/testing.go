package provider

// NewTestRegistry creates a Registry for testing. The primary parameter
// is the primary provider; all are added to the registry's provider list.
func NewTestRegistry(primary Provider, all ...Provider) *Registry {
	return &Registry{
		providers: all,
		primary:   primary,
	}
}
