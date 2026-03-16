package tokens

import (
	"sync"
	"testing"
)

func TestTrackerAddAndGet(t *testing.T) {
	tracker := NewTracker(
		map[string]string{"a": "gpt-4o"},
		map[string]int{"a": 128000},
	)

	tracker.Add("a", Usage{InputTokens: 500, OutputTokens: 100})
	tracker.Add("a", Usage{InputTokens: 800, OutputTokens: 200})

	got := tracker.Get("a")
	if got.InputTokens != 1300 {
		t.Errorf("expected 1300 input tokens, got %d", got.InputTokens)
	}
	if got.OutputTokens != 300 {
		t.Errorf("expected 300 output tokens, got %d", got.OutputTokens)
	}
	if got.Limit != 128000 {
		t.Errorf("expected limit 128000, got %d", got.Limit)
	}
}

func TestTrackerGetUnknownProvider(t *testing.T) {
	tracker := NewTracker(nil, nil)
	got := tracker.Get("nonexistent")
	if got.InputTokens != 0 {
		t.Errorf("expected 0 for unknown provider, got %d", got.InputTokens)
	}
}

func TestTrackerSummary(t *testing.T) {
	tracker := NewTracker(
		map[string]string{"a": "m1", "b": "m2"},
		map[string]int{"a": 100, "b": 200},
	)
	tracker.Add("a", Usage{InputTokens: 10, OutputTokens: 5})
	tracker.Add("b", Usage{InputTokens: 20, OutputTokens: 10})

	summary := tracker.Summary()
	if len(summary) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(summary))
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	tracker := NewTracker(
		map[string]string{"a": "model"},
		map[string]int{"a": 0},
	)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.Add("a", Usage{InputTokens: 1, OutputTokens: 1})
			_ = tracker.Get("a")
			_ = tracker.Summary()
		}()
	}
	wg.Wait()

	got := tracker.Get("a")
	if got.InputTokens != 100 {
		t.Errorf("expected 100 input tokens after concurrent adds, got %d", got.InputTokens)
	}
}

func TestWouldExceedLimit(t *testing.T) {
	tracker := NewTracker(
		map[string]string{"limited": "m", "unlimited": "m"},
		map[string]int{"limited": 100, "unlimited": 0},
	)

	// Under limit
	tracker.Add("limited", Usage{InputTokens: 50})
	if tracker.WouldExceedLimit("limited") {
		t.Error("should not exceed at 50/100")
	}

	// At limit
	tracker.Add("limited", Usage{InputTokens: 50})
	if !tracker.WouldExceedLimit("limited") {
		t.Error("should exceed at 100/100")
	}

	// Unlimited never exceeds
	tracker.Add("unlimited", Usage{InputTokens: 999999})
	if tracker.WouldExceedLimit("unlimited") {
		t.Error("unlimited provider should never exceed")
	}
}

func TestProviderUsagePercent(t *testing.T) {
	pu := ProviderUsage{InputTokens: 80000, Limit: 100000}
	if pu.Percent() != 80.0 {
		t.Errorf("expected 80.0%%, got %.1f%%", pu.Percent())
	}

	unlimited := ProviderUsage{InputTokens: 1000, Limit: 0}
	if unlimited.Percent() != 0 {
		t.Errorf("expected 0%% for unlimited, got %.1f%%", unlimited.Percent())
	}
}

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1234, "1.2K"},
		{12400, "12.4K"},
		{128000, "128.0K"},
		{200000, "200.0K"},
		{1000000, "1.0M"},
		{1048576, "1.0M"},
		{2097152, "2.1M"},
	}

	for _, tt := range tests {
		got := FormatTokenCount(tt.n)
		if got != tt.want {
			t.Errorf("FormatTokenCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestLimitForModel(t *testing.T) {
	// Known model, no override
	if got := LimitForModel("gpt-4o", 0); got != 128000 {
		t.Errorf("expected 128000 for gpt-4o, got %d", got)
	}

	// Known model, with override
	if got := LimitForModel("gpt-4o", 50000); got != 50000 {
		t.Errorf("expected override 50000, got %d", got)
	}

	// Unknown model, no override → unlimited
	if got := LimitForModel("custom-model-xyz", 0); got != 0 {
		t.Errorf("expected 0 for unknown model, got %d", got)
	}

	// Unknown model, with override
	if got := LimitForModel("custom-model-xyz", 32000); got != 32000 {
		t.Errorf("expected override 32000, got %d", got)
	}
}
