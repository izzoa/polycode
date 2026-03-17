package main

import (
	"strings"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

func TestFormatSessionMarkdown(t *testing.T) {
	session := &config.Session{
		Exchanges: []config.SessionExchange{
			{
				Prompt:            "What is Go?",
				ConsensusResponse: "Go is a statically typed, compiled programming language.",
				Individual: map[string]string{
					"claude": "Go is a language designed at Google.",
					"gpt4":   "Go (Golang) is a programming language.",
				},
			},
			{
				Prompt:            "Show me an example",
				ConsensusResponse: "Here is a Hello World example.",
			},
		},
		UpdatedAt: time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
	}

	md := FormatSessionMarkdown(session)

	// Check header
	if !strings.Contains(md, "# Polycode Session Export") {
		t.Error("markdown should contain header")
	}

	// Check exported date
	if !strings.Contains(md, "*Exported:") {
		t.Error("markdown should contain export date")
	}

	// Check Turn 1
	if !strings.Contains(md, "## Turn 1") {
		t.Error("markdown should contain Turn 1")
	}

	// Check user prompt
	if !strings.Contains(md, "**User:** What is Go?") {
		t.Error("markdown should contain user prompt")
	}

	// Check consensus response
	if !strings.Contains(md, "**Consensus:** Go is a statically typed, compiled programming language.") {
		t.Error("markdown should contain consensus response")
	}

	// Check individual responses section
	if !strings.Contains(md, "**Individual Responses:**") {
		t.Error("markdown should contain individual responses section")
	}
	if !strings.Contains(md, "*claude:*") {
		t.Error("markdown should contain claude individual response")
	}
	if !strings.Contains(md, "*gpt4:*") {
		t.Error("markdown should contain gpt4 individual response")
	}

	// Check Turn 2
	if !strings.Contains(md, "## Turn 2") {
		t.Error("markdown should contain Turn 2")
	}

	// Check separator
	if !strings.Contains(md, "---") {
		t.Error("markdown should contain separator")
	}

	// Turn 2 should not have individual responses section
	// Split by turns and check
	parts := strings.Split(md, "## Turn 2")
	if len(parts) < 2 {
		t.Fatal("expected to find Turn 2")
	}
	turn2Content := parts[1]
	if strings.Contains(turn2Content, "**Individual Responses:**") {
		t.Error("Turn 2 should not have individual responses (none were provided)")
	}
}

func TestFormatSessionMarkdownLongIndividualResponse(t *testing.T) {
	longResp := strings.Repeat("x", 300)
	session := &config.Session{
		Exchanges: []config.SessionExchange{
			{
				Prompt:            "test",
				ConsensusResponse: "consensus",
				Individual: map[string]string{
					"provider1": longResp,
				},
			},
		},
	}

	md := FormatSessionMarkdown(session)

	// The individual response should be truncated
	if strings.Contains(md, longResp) {
		t.Error("long individual response should be truncated")
	}
	if !strings.Contains(md, "...") {
		t.Error("truncated response should end with ...")
	}
}

func TestReviewHasCritical(t *testing.T) {
	tests := []struct {
		name     string
		review   string
		expected bool
	}{
		{
			name:     "no critical issues",
			review:   "The changes look good. Minor formatting suggestion on line 5.",
			expected: false,
		},
		{
			name:     "lowercase critical",
			review:   "Found a critical security vulnerability in auth.go.",
			expected: true,
		},
		{
			name:     "uppercase critical",
			review:   "CRITICAL: SQL injection risk on line 42.",
			expected: true,
		},
		{
			name:     "mixed case critical",
			review:   "Severity: Critical - memory leak detected.",
			expected: true,
		},
		{
			name:     "clean review",
			review:   "All changes look good. Well-structured code with proper error handling.",
			expected: false,
		},
		{
			name:     "warning only",
			review:   "Warning: Consider adding error handling on line 10.",
			expected: false,
		},
		{
			name:     "empty review",
			review:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReviewHasCritical(tt.review)
			if result != tt.expected {
				t.Errorf("ReviewHasCritical(%q) = %v, want %v", tt.review, result, tt.expected)
			}
		})
	}
}
