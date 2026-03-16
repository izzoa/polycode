package tokens

import (
	"fmt"
	"sync"
)

// Usage holds token counts for a single API call.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ProviderUsage holds the accumulated usage and limit for one provider.
type ProviderUsage struct {
	ProviderID   string
	Model        string
	InputTokens  int
	OutputTokens int
	Limit        int // 0 means unlimited
}

// Percent returns the input token usage as a percentage of the limit.
// Returns 0 if the limit is unlimited (0).
func (u ProviderUsage) Percent() float64 {
	if u.Limit == 0 {
		return 0
	}
	return float64(u.InputTokens) / float64(u.Limit) * 100
}

// TokenTracker accumulates token usage per provider across a session.
type TokenTracker struct {
	mu     sync.RWMutex
	usage  map[string]*ProviderUsage // provider ID → usage
	models map[string]string         // provider ID → model name
}

// NewTracker creates a TokenTracker. Pass provider ID → model name
// and provider ID → resolved context limit.
func NewTracker(providerModels map[string]string, providerLimits map[string]int) *TokenTracker {
	t := &TokenTracker{
		usage:  make(map[string]*ProviderUsage, len(providerModels)),
		models: providerModels,
	}
	for id, model := range providerModels {
		limit := providerLimits[id]
		t.usage[id] = &ProviderUsage{
			ProviderID: id,
			Model:      model,
			Limit:      limit,
		}
	}
	return t
}

// Add records token usage for a provider from a single API call.
func (t *TokenTracker) Add(providerID string, u Usage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pu, ok := t.usage[providerID]
	if !ok {
		pu = &ProviderUsage{ProviderID: providerID}
		t.usage[providerID] = pu
	}
	pu.InputTokens += u.InputTokens
	pu.OutputTokens += u.OutputTokens
}

// Get returns the accumulated usage for a single provider.
func (t *TokenTracker) Get(providerID string) ProviderUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if pu, ok := t.usage[providerID]; ok {
		return *pu
	}
	return ProviderUsage{ProviderID: providerID}
}

// Summary returns a snapshot of all providers' usage.
func (t *TokenTracker) Summary() []ProviderUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]ProviderUsage, 0, len(t.usage))
	for _, pu := range t.usage {
		out = append(out, *pu)
	}
	return out
}

// WouldExceedLimit returns true if the provider's current input tokens
// are at or above its context limit. Returns false if limit is 0 (unlimited).
func (t *TokenTracker) WouldExceedLimit(providerID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	pu, ok := t.usage[providerID]
	if !ok {
		return false
	}
	if pu.Limit == 0 {
		return false
	}
	return pu.InputTokens >= pu.Limit
}

// FormatTokenCount formats a token count for display.
// e.g., 1234 → "1.2K", 1234567 → "1.2M", 500 → "500"
func FormatTokenCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
