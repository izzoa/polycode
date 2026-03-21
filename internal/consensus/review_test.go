package consensus

import (
	"strings"
	"testing"
)

// ---------- Task 5.1: Well-formed structured output ----------

func TestParseConsensusAnalysis_WellFormed(t *testing.T) {
	input := `## Recommendation
Use a sync.Map for concurrent session storage with periodic cleanup via a background goroutine.

## Confidence: high

## Agreement
- Both models agree that a map-based approach is best for O(1) lookups.
- Both recommend some form of expiration/cleanup.

## Minority Report
[Model: gpt4] Suggested a binary search tree for range queries.
The reasoning was that sessions might need to be iterated in order.

## Evidence
- Go standard library docs for sync.Map
- The original codebase uses a mutex-guarded map in session.go:42
`

	ca := ParseConsensusAnalysis(input)

	// Raw is always the full input.
	if ca.Raw != input {
		t.Error("Raw should equal the full input")
	}

	// Recommendation
	if !strings.Contains(ca.Recommendation, "sync.Map") {
		t.Errorf("Recommendation should mention sync.Map, got: %s", ca.Recommendation)
	}

	// Confidence
	if ca.Confidence != "high" {
		t.Errorf("Confidence should be 'high', got: %q", ca.Confidence)
	}

	// Agreements
	if len(ca.Agreements) != 2 {
		t.Fatalf("expected 2 agreements, got %d", len(ca.Agreements))
	}
	if !strings.Contains(ca.Agreements[0], "map-based") {
		t.Errorf("first agreement should mention map-based, got: %s", ca.Agreements[0])
	}
	if !strings.Contains(ca.Agreements[1], "expiration") {
		t.Errorf("second agreement should mention expiration, got: %s", ca.Agreements[1])
	}

	// Minority Reports
	if len(ca.MinorityReports) != 1 {
		t.Fatalf("expected 1 minority report, got %d", len(ca.MinorityReports))
	}
	mr := ca.MinorityReports[0]
	if mr.ProviderID != "gpt4" {
		t.Errorf("minority report provider should be 'gpt4', got: %q", mr.ProviderID)
	}
	if !strings.Contains(mr.Position, "binary search tree") {
		t.Errorf("minority report position should mention binary search tree, got: %s", mr.Position)
	}

	// Evidence
	if len(ca.Evidence) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(ca.Evidence))
	}
	if !strings.Contains(ca.Evidence[0], "sync.Map") {
		t.Errorf("first evidence should mention sync.Map, got: %s", ca.Evidence[0])
	}
	if !strings.Contains(ca.Evidence[1], "session.go") {
		t.Errorf("second evidence should mention session.go, got: %s", ca.Evidence[1])
	}
}

func TestParseConsensusAnalysis_MultipleMinorityReports(t *testing.T) {
	input := `## Recommendation
Use approach A.

## Confidence: medium

## Agreement
- All agree on X.

## Minority Report
[Model: gpt4] Prefers approach B due to performance.
[Model: gemini] Recommends approach C for simplicity.

## Evidence
- Reference 1
`

	ca := ParseConsensusAnalysis(input)

	if len(ca.MinorityReports) != 2 {
		t.Fatalf("expected 2 minority reports, got %d", len(ca.MinorityReports))
	}
	if ca.MinorityReports[0].ProviderID != "gpt4" {
		t.Errorf("first minority report provider should be 'gpt4', got: %q", ca.MinorityReports[0].ProviderID)
	}
	if ca.MinorityReports[1].ProviderID != "gemini" {
		t.Errorf("second minority report provider should be 'gemini', got: %q", ca.MinorityReports[1].ProviderID)
	}
}

// ---------- Task 5.2: Raw prose (no structured headers) ----------

func TestParseConsensusAnalysis_RawProse(t *testing.T) {
	input := "Just use a map. All models agree this is the best approach for your use case."

	ca := ParseConsensusAnalysis(input)

	if ca.Recommendation != input {
		t.Errorf("Recommendation should equal full text, got: %q", ca.Recommendation)
	}
	if ca.Raw != input {
		t.Error("Raw should equal full text")
	}
	if ca.Confidence != "" {
		t.Errorf("Confidence should be empty, got: %q", ca.Confidence)
	}
	if ca.Agreements != nil {
		t.Errorf("Agreements should be nil, got: %v", ca.Agreements)
	}
	if ca.MinorityReports != nil {
		t.Errorf("MinorityReports should be nil, got: %v", ca.MinorityReports)
	}
	if ca.Evidence != nil {
		t.Errorf("Evidence should be nil, got: %v", ca.Evidence)
	}
}

func TestParseConsensusAnalysis_MinorityReportNone(t *testing.T) {
	input := `## Recommendation
Use approach A.

## Confidence: low

## Agreement
- Everyone agrees.

## Minority Report
None — all models agreed

## Evidence
- Some reference
`

	ca := ParseConsensusAnalysis(input)

	if ca.Confidence != "low" {
		t.Errorf("Confidence should be 'low', got: %q", ca.Confidence)
	}
	if ca.MinorityReports != nil {
		t.Errorf("MinorityReports should be nil for 'None' text, got: %v", ca.MinorityReports)
	}
}

func TestParseConsensusAnalysis_MinorityReportFallback(t *testing.T) {
	input := `## Recommendation
Use approach A.

## Confidence: medium

## Agreement
- All agree.

## Minority Report
One model suggested a different approach but didn't provide clear reasoning.

## Evidence
- Ref 1
`

	ca := ParseConsensusAnalysis(input)

	if len(ca.MinorityReports) != 1 {
		t.Fatalf("expected 1 fallback minority report, got %d", len(ca.MinorityReports))
	}
	if ca.MinorityReports[0].ProviderID != "" {
		t.Errorf("fallback minority report should have empty ProviderID, got: %q", ca.MinorityReports[0].ProviderID)
	}
	if !strings.Contains(ca.MinorityReports[0].Position, "different approach") {
		t.Errorf("fallback minority report position should contain the text, got: %q", ca.MinorityReports[0].Position)
	}
}

// ---------- Task 5.4: BuildConsensusPrompt contains structured markers ----------

func TestBuildConsensusPrompt_ContainsStructuredMarkers(t *testing.T) {
	e := NewEngine(nil, 0, 0)

	responses := map[string]string{
		"claude": "Use a map.",
		"gpt4":   "Use a tree.",
	}

	msgs := e.BuildConsensusPrompt("How to store sessions?", responses, SynthesisBalanced)
	content := msgs[0].Content

	markers := []string{
		"## Recommendation",
		"## Confidence:",
		"## Agreement",
		"## Minority Report",
		"## Evidence",
	}

	for _, marker := range markers {
		if !strings.Contains(content, marker) {
			t.Errorf("prompt should contain %q", marker)
		}
	}
}

func TestBuildConsensusPrompt_ContainsSynthesisInstructions(t *testing.T) {
	e := NewEngine(nil, 0, 0)

	responses := map[string]string{
		"claude": "Response A.",
	}

	msgs := e.BuildConsensusPrompt("test question", responses, SynthesisBalanced)
	content := msgs[0].Content

	if !strings.Contains(content, "Analyze all responses and produce a synthesis") {
		t.Error("prompt should contain synthesis instruction")
	}
	if !strings.Contains(content, "synthesizing responses from multiple AI models") {
		t.Error("prompt should contain role preamble")
	}
}
