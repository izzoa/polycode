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
	ProviderID       string
	Model            string
	InputTokens      int // accumulated across all turns (for display)
	OutputTokens     int // accumulated across all turns (for display)
	LastInputTokens  int // most recent request's input tokens (for context % calculation)
	Limit            int     // 0 means unlimited
	Cost             float64 // accumulated estimated cost in USD (0 if pricing unavailable)
}

// Percent returns the context window usage as a percentage of the limit,
// based on the most recent request's input tokens (not accumulated total).
// Returns 0 if the limit is unlimited (0).
func (u ProviderUsage) Percent() float64 {
	if u.Limit == 0 {
		return 0
	}
	return float64(u.LastInputTokens) / float64(u.Limit) * 100
}

// CostFunc computes the estimated USD cost for given token counts.
// model is the model name, providerType is the provider type string.
type CostFunc func(model, providerType string, inputTokens, outputTokens int) float64

// TokenTracker accumulates token usage per provider across a session.
type TokenTracker struct {
	mu            sync.RWMutex
	usage         map[string]*ProviderUsage // provider ID → usage
	models        map[string]string         // provider ID → model name
	providerTypes map[string]string         // provider ID → provider type
	costFn        CostFunc                  // optional cost calculator
}

// NewTracker creates a TokenTracker. Pass provider ID → model name
// and provider ID → resolved context limit.
func NewTracker(providerModels map[string]string, providerLimits map[string]int) *TokenTracker {
	t := &TokenTracker{
		usage:         make(map[string]*ProviderUsage, len(providerModels)),
		models:        providerModels,
		providerTypes: make(map[string]string),
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

// Reset clears all accumulated token usage and cost for every provider,
// preserving the provider list, models, limits, and cost function.
func (t *TokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, u := range t.usage {
		t.usage[id] = &ProviderUsage{
			ProviderID: u.ProviderID,
			Model:      u.Model,
			Limit:      u.Limit,
		}
	}
}

// SetCostFunc sets an optional cost estimation function.
func (t *TokenTracker) SetCostFunc(fn CostFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.costFn = fn
}

// SetProviderType records the provider type for a provider ID.
func (t *TokenTracker) SetProviderType(providerID, providerType string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.providerTypes[providerID] = providerType
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
	pu.LastInputTokens = u.InputTokens // track most recent for context %
	if t.costFn != nil {
		model := t.models[providerID]
		ptype := t.providerTypes[providerID]
		pu.Cost += t.costFn(model, ptype, u.InputTokens, u.OutputTokens)
	}
}

// TotalCost returns the accumulated cost across all providers.
func (t *TokenTracker) TotalCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var total float64
	for _, pu := range t.usage {
		total += pu.Cost
	}
	return total
}

// FormatCost formats a USD amount for display.
func FormatCost(usd float64) string {
	if usd == 0 {
		return ""
	}
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
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
	return pu.LastInputTokens >= pu.Limit
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
