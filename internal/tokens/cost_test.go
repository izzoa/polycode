package tokens

import "testing"

func TestCostTracking(t *testing.T) {
	models := map[string]string{"openai": "gpt-4o"}
	limits := map[string]int{"openai": 128000}

	tracker := NewTracker(models, limits)
	tracker.SetProviderType("openai", "openai")
	tracker.SetCostFunc(func(model, providerType string, input, output int) float64 {
		// Simplified pricing: $5/1M input, $15/1M output
		return float64(input)*5.0/1_000_000 + float64(output)*15.0/1_000_000
	})

	tracker.Add("openai", Usage{InputTokens: 1000, OutputTokens: 500})

	total := tracker.TotalCost()
	expected := 1000*5.0/1_000_000 + 500*15.0/1_000_000 // $0.0125
	if total < expected-0.0001 || total > expected+0.0001 {
		t.Errorf("TotalCost() = %f, want ~%f", total, expected)
	}

	pu := tracker.Get("openai")
	if pu.Cost < expected-0.0001 || pu.Cost > expected+0.0001 {
		t.Errorf("Get().Cost = %f, want ~%f", pu.Cost, expected)
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		usd  float64
		want string
	}{
		{0, ""},
		{0.005, "$0.0050"},
		{0.12, "$0.12"},
		{1.50, "$1.50"},
	}
	for _, tt := range tests {
		got := FormatCost(tt.usd)
		if got != tt.want {
			t.Errorf("FormatCost(%f) = %q, want %q", tt.usd, got, tt.want)
		}
	}
}

func TestCostTrackingWithoutCostFunc(t *testing.T) {
	models := map[string]string{"openai": "gpt-4o"}
	limits := map[string]int{"openai": 128000}

	tracker := NewTracker(models, limits)
	tracker.Add("openai", Usage{InputTokens: 1000, OutputTokens: 500})

	if tracker.TotalCost() != 0 {
		t.Errorf("TotalCost() should be 0 without cost func, got %f", tracker.TotalCost())
	}
}
