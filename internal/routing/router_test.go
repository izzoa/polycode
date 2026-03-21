package routing

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/izzoa/polycode/internal/provider"
)

// --- 6.6: ParseMode validates mode names ---

func TestParseMode_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  Mode
	}{
		{"quick", ModeQuick},
		{"balanced", ModeBalanced},
		{"thorough", ModeThorough},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseMode(tt.input)
			if !ok {
				t.Fatalf("ParseMode(%q) returned ok=false, want true", tt.input)
			}
			if got != tt.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseMode_Invalid(t *testing.T) {
	invalid := []string{"", "fast", "slow", "Quick", "BALANCED", "unknown"}
	for _, s := range invalid {
		t.Run(s, func(t *testing.T) {
			_, ok := ParseMode(s)
			if ok {
				t.Fatalf("ParseMode(%q) returned ok=true, want false", s)
			}
		})
	}
}

// --- 6.1: LoadTelemetryStats with sample JSONL ---

func TestLoadTelemetryStats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")

	trueVal := true
	falseVal := false
	_ = falseVal

	// Write sample JSONL data.
	lines := []string{
		fmt.Sprintf(`{"event_type":"provider_response","provider_id":"claude","latency_ms":200,"success":%v}`, trueVal),
		fmt.Sprintf(`{"event_type":"provider_response","provider_id":"claude","latency_ms":300,"success":%v}`, trueVal),
		fmt.Sprintf(`{"event_type":"provider_response","provider_id":"claude","latency_ms":0,"success":%v}`, falseVal),
		fmt.Sprintf(`{"event_type":"provider_response","provider_id":"gemini","latency_ms":150,"success":%v}`, trueVal),
		`{"event_type":"query_start","provider_id":"claude"}`,
	}

	var data []byte
	for _, line := range lines {
		data = append(data, []byte(line+"\n")...)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	r := NewRouter(path)
	if err := r.LoadTelemetryStats(); err != nil {
		t.Fatalf("LoadTelemetryStats: %v", err)
	}

	// Check claude stats: 2 successful (200+300 avg=250), 1 error, 3 total, error rate 1/3.
	claude, ok := r.stats["claude"]
	if !ok {
		t.Fatal("missing stats for claude")
	}
	if claude.TotalSuccessful != 2 {
		t.Errorf("claude TotalSuccessful = %d, want 2", claude.TotalSuccessful)
	}
	if math.Abs(claude.AvgLatencyMS-250.0) > 0.01 {
		t.Errorf("claude AvgLatencyMS = %f, want 250.0", claude.AvgLatencyMS)
	}
	expectedErrorRate := 1.0 / 3.0
	if math.Abs(claude.ErrorRate-expectedErrorRate) > 0.01 {
		t.Errorf("claude ErrorRate = %f, want %f", claude.ErrorRate, expectedErrorRate)
	}

	// Check gemini stats: 1 successful at 150ms, 0 errors.
	gemini, ok := r.stats["gemini"]
	if !ok {
		t.Fatal("missing stats for gemini")
	}
	if gemini.TotalSuccessful != 1 {
		t.Errorf("gemini TotalSuccessful = %d, want 1", gemini.TotalSuccessful)
	}
	if math.Abs(gemini.AvgLatencyMS-150.0) > 0.01 {
		t.Errorf("gemini AvgLatencyMS = %f, want 150.0", gemini.AvgLatencyMS)
	}
	if gemini.ErrorRate != 0.0 {
		t.Errorf("gemini ErrorRate = %f, want 0.0", gemini.ErrorRate)
	}
}

func TestLoadTelemetryStats_NoFile(t *testing.T) {
	r := NewRouter("/nonexistent/path/telemetry.jsonl")
	if err := r.LoadTelemetryStats(); err != nil {
		t.Fatalf("LoadTelemetryStats on missing file should not error, got: %v", err)
	}
	if len(r.stats) != 0 {
		t.Errorf("expected empty stats, got %d entries", len(r.stats))
	}
}

// --- 6.2: ScoreProvider with known inputs ---

func TestScoreProvider(t *testing.T) {
	r := NewRouter("")

	tests := []struct {
		name  string
		stats ProviderStats
		want  float64
	}{
		{
			name: "zero history gets neutral score",
			stats: ProviderStats{
				ProviderID:      "new",
				AvgLatencyMS:    0,
				ErrorRate:       0,
				TotalSuccessful: 0,
			},
			want: 1.0,
		},
		{
			name: "known provider",
			stats: ProviderStats{
				ProviderID:      "claude",
				AvgLatencyMS:    200,
				ErrorRate:       0.1,
				TotalSuccessful: 10,
			},
			// (1/200) * (1-0.1) * log(11) = 0.005 * 0.9 * 2.3979... = 0.01079...
			want: (1.0 / 200.0) * 0.9 * math.Log(11),
		},
		{
			name: "perfect provider",
			stats: ProviderStats{
				ProviderID:      "fast",
				AvgLatencyMS:    100,
				ErrorRate:       0,
				TotalSuccessful: 100,
			},
			// (1/100) * 1.0 * log(101)
			want: (1.0 / 100.0) * 1.0 * math.Log(101),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.ScoreProvider(tt.stats)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("ScoreProvider(%+v) = %f, want %f", tt.stats, got, tt.want)
			}
		})
	}
}

// --- 6.3: SelectProviders returns correct subsets for each mode ---

// mockProvider is a minimal provider.Provider implementation for testing.
type mockProvider struct {
	id string
}

func (m *mockProvider) ID() string { return m.id }
func (m *mockProvider) Query(_ context.Context, _ []provider.Message, _ provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (m *mockProvider) Authenticate() error { return nil }
func (m *mockProvider) Validate() error     { return nil }

// All modes now query all providers — the mode controls synthesis depth, not provider selection.

func TestSelectProviders_AllModesReturnAllProviders(t *testing.T) {
	r := NewRouter("")
	_ = r.LoadTelemetryStats()

	providers := []provider.Provider{
		&mockProvider{id: "claude"},
		&mockProvider{id: "gemini"},
		&mockProvider{id: "gpt"},
	}

	for _, mode := range []Mode{ModeQuick, ModeBalanced, ModeThorough} {
		result := r.SelectProviders(mode, providers, "claude")
		if len(result) != 3 {
			t.Errorf("%s mode: got %d providers, want 3", mode, len(result))
		}
	}
}

func TestSelectProviders_ReasonIncludesMode(t *testing.T) {
	r := NewRouter("")
	_ = r.LoadTelemetryStats()

	providers := []provider.Provider{
		&mockProvider{id: "claude"},
		&mockProvider{id: "gemini"},
	}

	_, reason := r.SelectProvidersWithReason(ModeQuick, providers, "claude")
	if reason == "" {
		t.Error("expected non-empty routing reason")
	}
}
