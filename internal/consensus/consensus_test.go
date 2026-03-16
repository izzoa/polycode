package consensus

import (
	"strings"
	"testing"
)

func TestBuildConsensusPrompt(t *testing.T) {
	e := NewEngine(nil, 0, 0)

	responses := map[string]string{
		"claude": "Use a map for O(1) lookups.",
		"gpt4":   "A binary search tree would work well.",
	}

	msgs := e.BuildConsensusPrompt("How should I store user sessions?", responses)

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	content := msgs[0].Content

	if !strings.Contains(content, "How should I store user sessions?") {
		t.Error("prompt should contain original question")
	}
	if !strings.Contains(content, "[Model: claude]") {
		t.Error("prompt should contain claude's response label")
	}
	if !strings.Contains(content, "[Model: gpt4]") {
		t.Error("prompt should contain gpt4's response label")
	}
	if !strings.Contains(content, "Use a map for O(1) lookups.") {
		t.Error("prompt should contain claude's response")
	}
	if !strings.Contains(content, "binary search tree") {
		t.Error("prompt should contain gpt4's response")
	}
	if !strings.Contains(content, "Analyze all responses") {
		t.Error("prompt should contain synthesis instructions")
	}
}

func TestBuildConsensusPromptDeterministicOrder(t *testing.T) {
	e := NewEngine(nil, 0, 0)

	responses := map[string]string{
		"zeta":  "Response Z",
		"alpha": "Response A",
		"beta":  "Response B",
	}

	msgs := e.BuildConsensusPrompt("test", responses)
	content := msgs[0].Content

	// Alpha should appear before beta, beta before zeta
	alphaIdx := strings.Index(content, "[Model: alpha]")
	betaIdx := strings.Index(content, "[Model: beta]")
	zetaIdx := strings.Index(content, "[Model: zeta]")

	if alphaIdx >= betaIdx || betaIdx >= zetaIdx {
		t.Error("responses should be sorted alphabetically by provider ID")
	}
}

func TestTruncateResponsesUnderBudget(t *testing.T) {
	responses := map[string]string{
		"a": "short",
		"b": "also short",
	}

	result := TruncateResponses(responses, 1000)

	if result["a"] != "short" || result["b"] != "also short" {
		t.Error("responses under budget should not be modified")
	}
}

func TestTruncateResponsesOverBudget(t *testing.T) {
	responses := map[string]string{
		"a": strings.Repeat("x", 500),
		"b": strings.Repeat("y", 500),
	}

	result := TruncateResponses(responses, 200)

	totalLen := 0
	for _, v := range result {
		totalLen += len(v)
	}

	if totalLen > 200 {
		t.Errorf("total length %d exceeds budget 200", totalLen)
	}

	// At least one should be truncated
	truncated := false
	for _, v := range result {
		if strings.HasSuffix(v, "[truncated]") {
			truncated = true
		}
	}
	if !truncated {
		t.Error("expected at least one response to be truncated")
	}
}

func TestTruncateResponsesZeroBudget(t *testing.T) {
	responses := map[string]string{"a": "hello"}
	result := TruncateResponses(responses, 0)
	if result["a"] != "hello" {
		t.Error("zero budget should return responses unchanged")
	}
}
