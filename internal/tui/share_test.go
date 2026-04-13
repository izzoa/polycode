package tui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// providerHeading
// ---------------------------------------------------------------------------

func TestProviderHeading_PrimaryDone(t *testing.T) {
	got := providerHeading("claude-sonnet", true, StatusDone)
	want := "## claude-sonnet (Primary)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProviderHeading_NonPrimaryDone(t *testing.T) {
	got := providerHeading("gpt-4o", false, StatusDone)
	want := "## gpt-4o"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProviderHeading_Failed(t *testing.T) {
	got := providerHeading("gemini-pro", false, StatusFailed)
	want := "## gemini-pro (Failed)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProviderHeading_TimedOut(t *testing.T) {
	got := providerHeading("gpt-4o", false, StatusTimedOut)
	want := "## gpt-4o (Timed Out)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProviderHeading_Cancelled(t *testing.T) {
	got := providerHeading("gpt-4o", false, StatusCancelled)
	want := "## gpt-4o (Cancelled)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProviderHeading_PrimaryTimedOut(t *testing.T) {
	got := providerHeading("claude-sonnet", true, StatusTimedOut)
	want := "## claude-sonnet (Primary, Timed Out)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProviderHeading_InProgress(t *testing.T) {
	got := providerHeading("gpt-4o", false, StatusLoading)
	want := "## gpt-4o (In Progress)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// buildShareMarkdown — basic cases
// ---------------------------------------------------------------------------

func TestBuildShareMarkdown_NoExchange(t *testing.T) {
	m := newTestModel()
	got := m.buildShareMarkdown()
	if got != "" {
		t.Errorf("expected empty string for no exchange, got %q", got)
	}
}

func TestBuildShareMarkdown_CompletedExchange(t *testing.T) {
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:            "How do I implement retry logic?",
			ConsensusResponse: "Here is the consensus answer.",
			IndividualResponse: map[string]string{
				"provider1": "Provider 1 says this.",
				"provider2": "Provider 2 says that.",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"provider1": StatusDone,
				"provider2": StatusDone,
			},
			ProviderOrder:   []string{"provider1", "provider2"},
			PrimaryProvider: "provider1",
		},
	}

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "## Prompt") {
		t.Error("should contain Prompt heading")
	}
	if !strings.Contains(got, "How do I implement retry logic?") {
		t.Error("should contain the user's prompt")
	}
	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should contain provider1 heading with Primary annotation")
	}
	if !strings.Contains(got, "## provider2") {
		t.Error("should contain provider2 heading")
	}
	if !strings.Contains(got, "Provider 1 says this.") {
		t.Error("should contain provider1 response")
	}
	if !strings.Contains(got, "Provider 2 says that.") {
		t.Error("should contain provider2 response")
	}
	if !strings.Contains(got, "## Consensus") {
		t.Error("should contain Consensus heading")
	}
	if !strings.Contains(got, "Here is the consensus answer.") {
		t.Error("should contain consensus response")
	}
}

func TestBuildShareMarkdown_MixedStatuses(t *testing.T) {
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:            "test prompt",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"provider1": "response1",
				"provider2": "",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"provider1": StatusDone,
				"provider2": StatusFailed,
			},
			ProviderOrder:   []string{"provider1", "provider2"},
			PrimaryProvider: "provider1",
		},
	}

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should annotate provider1 as Primary")
	}
	if !strings.Contains(got, "## provider2 (Failed)") {
		t.Error("should annotate provider2 as Failed")
	}
}

func TestBuildShareMarkdown_MidStream(t *testing.T) {
	m := newTestModel()
	m.querying = true
	m.currentPrompt = "streaming question"
	m.panels[0].Content.WriteString("partial response from provider1")
	m.panels[0].Status = StatusDone
	m.panels[1].Status = StatusLoading
	m.consensusContent.WriteString("partial consensus")

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "## Prompt") {
		t.Error("should contain Prompt heading")
	}
	if !strings.Contains(got, "streaming question") {
		t.Error("should contain the prompt")
	}
	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should show provider1 as Primary")
	}
	if !strings.Contains(got, "partial response from provider1") {
		t.Error("should contain partial provider1 response")
	}
	if !strings.Contains(got, "## provider2 (In Progress)") {
		t.Error("should annotate provider2 as In Progress")
	}
	if !strings.Contains(got, "## Consensus") {
		t.Error("should contain Consensus heading")
	}
	if !strings.Contains(got, "partial consensus") {
		t.Error("should contain partial consensus")
	}
}

func TestBuildShareMarkdown_NoConsensus(t *testing.T) {
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:             "test",
			ConsensusResponse:  "",
			IndividualResponse: map[string]string{"provider1": "answer"},
			ProviderStatuses:   map[string]ProviderStatus{"provider1": StatusDone},
			ProviderOrder:      []string{"provider1"},
			PrimaryProvider:    "provider1",
		},
	}

	got := m.buildShareMarkdown()

	if strings.Contains(got, "## Consensus") {
		t.Error("should not contain Consensus heading when consensus is empty")
	}
}

func TestBuildShareMarkdown_PanelOrderPreserved(t *testing.T) {
	m := newTestModel() // provider1, provider2
	m.history = []Exchange{
		{
			Prompt:            "test",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"provider1": "first",
				"provider2": "second",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"provider1": StatusDone,
				"provider2": StatusDone,
			},
			ProviderOrder:   []string{"provider1", "provider2"},
			PrimaryProvider: "provider1",
		},
	}

	got := m.buildShareMarkdown()

	idx1 := strings.Index(got, "provider1")
	idx2 := strings.Index(got, "provider2")
	if idx1 >= idx2 {
		t.Errorf("provider1 (idx=%d) should appear before provider2 (idx=%d)", idx1, idx2)
	}
}

// ---------------------------------------------------------------------------
// Fix 1: Idle (non-routed) providers are filtered out
// ---------------------------------------------------------------------------

func TestBuildShareMarkdown_IdleProvidersOmitted_Panels(t *testing.T) {
	m := newTestModel() // provider1 (primary), provider2
	m.querying = true
	m.currentPrompt = "quick mode question"
	// In quick mode, only one provider is routed; the other stays idle.
	m.panels[0].Status = StatusDone
	m.panels[0].Content.WriteString("routed response")
	m.panels[1].Status = StatusIdle // not routed

	got := m.buildShareFromPanels()

	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should include routed provider1")
	}
	if strings.Contains(got, "provider2") {
		t.Error("should NOT include idle provider2")
	}
}

func TestBuildShareMarkdown_IdleProvidersOmitted_History(t *testing.T) {
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:            "quick mode question",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"provider1": "routed response",
				"provider2": "",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"provider1": StatusDone,
				"provider2": StatusIdle,
			},
			ProviderOrder:   []string{"provider1", "provider2"},
			PrimaryProvider: "provider1",
		},
	}

	got := m.buildShareFromHistory()

	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should include routed provider1")
	}
	if strings.Contains(got, "provider2") {
		t.Error("should NOT include idle provider2")
	}
}

// ---------------------------------------------------------------------------
// Fix 2: Historical metadata used instead of current panels
// ---------------------------------------------------------------------------

func TestBuildShareMarkdown_ConfigDrift_ProviderRemoved(t *testing.T) {
	// Exchange was recorded with providers A, B, C.
	// Current panels only have A, B (C was removed from config).
	// /share should still include C's response from history.
	m := NewModel([]string{"providerA", "providerB"}, "providerA", "v1")
	m.showSplash = false
	m.history = []Exchange{
		{
			Prompt:            "test",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"providerA": "response A",
				"providerB": "response B",
				"providerC": "response C",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"providerA": StatusDone,
				"providerB": StatusDone,
				"providerC": StatusDone,
			},
			ProviderOrder:   []string{"providerA", "providerB", "providerC"},
			PrimaryProvider: "providerA",
		},
	}

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "providerA") {
		t.Error("should include providerA")
	}
	if !strings.Contains(got, "providerB") {
		t.Error("should include providerB")
	}
	if !strings.Contains(got, "providerC") {
		t.Error("should include providerC from historical order")
	}
	if !strings.Contains(got, "response C") {
		t.Error("should include providerC's response")
	}
}

func TestBuildShareMarkdown_ConfigDrift_PrimaryChanged(t *testing.T) {
	// Exchange was recorded when providerB was primary.
	// Current panels have providerA as primary.
	// /share should label providerB as primary (historical truth).
	m := NewModel([]string{"providerA", "providerB"}, "providerA", "v1")
	m.showSplash = false
	m.history = []Exchange{
		{
			Prompt:            "test",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"providerA": "response A",
				"providerB": "response B",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"providerA": StatusDone,
				"providerB": StatusDone,
			},
			ProviderOrder:   []string{"providerA", "providerB"},
			PrimaryProvider: "providerB",
		},
	}

	got := m.buildShareMarkdown()

	if strings.Contains(got, "## providerA (Primary)") {
		t.Error("providerA should NOT be labeled Primary (historical primary was providerB)")
	}
	if !strings.Contains(got, "## providerB (Primary)") {
		t.Error("providerB SHOULD be labeled Primary (was primary at exchange time)")
	}
}

func TestBuildShareMarkdown_FallbackToCurrentPanels(t *testing.T) {
	// Old exchange without ProviderOrder/PrimaryProvider (pre-migration data).
	// Should fall back to current panel order and primary.
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:            "old exchange",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"provider1": "response 1",
				"provider2": "response 2",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"provider1": StatusDone,
				"provider2": StatusDone,
			},
			// No ProviderOrder or PrimaryProvider — simulates older data.
		},
	}

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should fall back to current panels and label provider1 as Primary")
	}
	if !strings.Contains(got, "response 1") {
		t.Error("should include provider1 response")
	}
	if !strings.Contains(got, "response 2") {
		t.Error("should include provider2 response")
	}
}

// ---------------------------------------------------------------------------
// Fix 3: Restored sessions with statuses
// ---------------------------------------------------------------------------

func TestBuildShareMarkdown_RestoredSessionWithStatuses(t *testing.T) {
	// Simulates a session restored from disk where ProviderStatuses were persisted.
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:            "restored prompt",
			ConsensusResponse: "restored consensus",
			IndividualResponse: map[string]string{
				"provider1": "good response",
				"provider2": "",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"provider1": StatusDone,
				"provider2": StatusFailed,
			},
			ProviderOrder:   []string{"provider1", "provider2"},
			PrimaryProvider: "provider1",
		},
	}

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should show provider1 as Primary/Done")
	}
	if !strings.Contains(got, "## provider2 (Failed)") {
		t.Error("should show provider2 as Failed from restored status")
	}
}

func TestBuildShareMarkdown_RestoredSessionWithoutStatuses(t *testing.T) {
	// Old session format without ProviderStatuses — providers default to Done.
	m := newTestModel()
	m.history = []Exchange{
		{
			Prompt:            "old restored prompt",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"provider1": "response",
			},
			// No ProviderStatuses — nil map.
			ProviderOrder:   []string{"provider1"},
			PrimaryProvider: "provider1",
		},
	}

	got := m.buildShareMarkdown()

	// Should not crash, and should default to showing the provider (StatusDone, no error annotation)
	if !strings.Contains(got, "## provider1 (Primary)") {
		t.Error("should show provider1 with Primary annotation, defaulting to Done")
	}
	if strings.Contains(got, "Failed") || strings.Contains(got, "Timed Out") {
		t.Error("should not show error annotations for missing statuses")
	}
}

func TestBuildShareMarkdown_OldFormatConfigDrift_HistoricalProviderIncluded(t *testing.T) {
	// Old exchange without ProviderOrder, and providerC was removed from config.
	// providerC's response should still be included via the union fallback.
	m := NewModel([]string{"providerA", "providerB"}, "providerA", "v1")
	m.showSplash = false
	m.history = []Exchange{
		{
			Prompt:            "old prompt",
			ConsensusResponse: "consensus",
			IndividualResponse: map[string]string{
				"providerA": "response A",
				"providerB": "response B",
				"providerC": "response C",
			},
			ProviderStatuses: map[string]ProviderStatus{
				"providerA": StatusDone,
				"providerB": StatusDone,
				"providerC": StatusDone,
			},
			// No ProviderOrder — triggers fallback.
		},
	}

	got := m.buildShareMarkdown()

	if !strings.Contains(got, "providerA") {
		t.Error("should include providerA from current panels")
	}
	if !strings.Contains(got, "providerB") {
		t.Error("should include providerB from current panels")
	}
	if !strings.Contains(got, "providerC") {
		t.Error("should include providerC from historical IndividualResponse (union fallback)")
	}
	if !strings.Contains(got, "response C") {
		t.Error("should include providerC's response")
	}
}

// ---------------------------------------------------------------------------
// ProviderStatusString / ParseProviderStatus round-trip
// ---------------------------------------------------------------------------

func TestProviderStatusStringRoundTrip(t *testing.T) {
	statuses := []ProviderStatus{StatusIdle, StatusLoading, StatusDone, StatusFailed, StatusTimedOut, StatusCancelled}
	for _, s := range statuses {
		str := ProviderStatusString(s)
		parsed := ParseProviderStatus(str)
		if parsed != s {
			t.Errorf("round-trip failed for %v: string=%q, parsed=%v", s, str, parsed)
		}
	}
}

func TestParseProviderStatus_UnknownDefaultsToDone(t *testing.T) {
	got := ParseProviderStatus("garbage")
	if got != StatusDone {
		t.Errorf("expected StatusDone for unknown string, got %v", got)
	}
}
