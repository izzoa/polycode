package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/tokens"
)

// newTestModel creates a Model suitable for testing, with splash dismissed.
func newTestModel() Model {
	m := NewModel([]string{"provider1", "provider2"}, "provider1", "v1")
	m.showSplash = false
	return m
}

// updateModel is a helper that calls Update and type-asserts the result back to Model.
func updateModel(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	result, cmd := m.Update(msg)
	updated, ok := result.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", result)
	}
	return updated, cmd
}

// ---------------------------------------------------------------------------
// ProviderChunkMsg
// ---------------------------------------------------------------------------

func TestProviderChunkMsg_AccumulatesContent(t *testing.T) {
	m := newTestModel()

	// Send a streaming chunk to provider1
	m, _ = updateModel(t, m, ProviderChunkMsg{
		ProviderName: "provider1",
		Delta:        "hello ",
	})

	if m.panels[0].Content.String() != "hello " {
		t.Errorf("panel content = %q, want %q", m.panels[0].Content.String(), "hello ")
	}
	if m.panels[0].Status != StatusLoading {
		t.Errorf("panel status = %v, want StatusLoading (%v)", m.panels[0].Status, StatusLoading)
	}

	// Send a second chunk
	m, _ = updateModel(t, m, ProviderChunkMsg{
		ProviderName: "provider1",
		Delta:        "world",
	})

	if m.panels[0].Content.String() != "hello world" {
		t.Errorf("panel content = %q, want %q", m.panels[0].Content.String(), "hello world")
	}
}

func TestProviderChunkMsg_DoneSetsDoneStatus(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ProviderChunkMsg{
		ProviderName: "provider1",
		Delta:        "content",
		Done:         true,
	})

	if m.panels[0].Status != StatusDone {
		t.Errorf("panel status = %v, want StatusDone (%v)", m.panels[0].Status, StatusDone)
	}
	if m.panels[0].Content.String() != "content" {
		t.Errorf("panel content = %q, want %q", m.panels[0].Content.String(), "content")
	}
}

func TestProviderChunkMsg_ErrorSetsFailedStatus(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ProviderChunkMsg{
		ProviderName: "provider2",
		Error:        errors.New("timeout"),
	})

	if m.panels[1].Status != StatusFailed {
		t.Errorf("panel status = %v, want StatusFailed (%v)", m.panels[1].Status, StatusFailed)
	}
	if m.lastError == "" {
		t.Error("lastError should be set after provider error")
	}
}

func TestProviderChunkMsg_UnknownProviderIgnored(t *testing.T) {
	m := newTestModel()

	// Should not panic for an unknown provider name
	m, _ = updateModel(t, m, ProviderChunkMsg{
		ProviderName: "unknown",
		Delta:        "data",
	})

	// Verify existing panels are untouched
	if m.panels[0].Content.String() != "" {
		t.Error("panel 0 content should be empty")
	}
	if m.panels[1].Content.String() != "" {
		t.Error("panel 1 content should be empty")
	}
}

// ---------------------------------------------------------------------------
// ConsensusChunkMsg
// ---------------------------------------------------------------------------

func TestConsensusChunkMsg_AccumulatesContent(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ConsensusChunkMsg{Delta: "part1"})
	m, _ = updateModel(t, m, ConsensusChunkMsg{Delta: " part2"})

	if m.consensusContent.String() != "part1 part2" {
		t.Errorf("consensus content = %q, want %q", m.consensusContent.String(), "part1 part2")
	}
	if !m.consensusActive {
		t.Error("consensusActive should be true after receiving consensus chunks")
	}
}

func TestConsensusChunkMsg_DoneSetsState(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ConsensusChunkMsg{Delta: "final answer"})
	m, _ = updateModel(t, m, ConsensusChunkMsg{Done: true})

	// After Done, content is markdown-rendered (may have leading/trailing whitespace)
	got := strings.TrimSpace(m.consensusContent.String())
	if got != "final answer" {
		t.Errorf("consensus content = %q, want %q", got, "final answer")
	}
	if !m.consensusActive {
		t.Error("consensusActive should be true after Done")
	}
}

func TestConsensusChunkMsg_ErrorSetsLastError(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ConsensusChunkMsg{Error: errors.New("synthesis failed")})

	if m.lastError == "" {
		t.Error("lastError should be set after consensus error")
	}
}

// ---------------------------------------------------------------------------
// QueryStartMsg / QueryDoneMsg
// ---------------------------------------------------------------------------

func TestQueryStartMsg_SetsQueryingTrue(t *testing.T) {
	m := newTestModel()

	m, cmd := updateModel(t, m, QueryStartMsg{})

	if !m.querying {
		t.Error("querying should be true after QueryStartMsg")
	}
	if m.consensusActive {
		t.Error("consensusActive should be false after QueryStartMsg")
	}
	if m.consensusContent.String() != "" {
		t.Error("consensusContent should be reset after QueryStartMsg")
	}
	// QueryStartMsg should return a spinner tick cmd
	if cmd == nil {
		t.Error("cmd should not be nil (should return spinner tick)")
	}
}

func TestQueryDoneMsg_SetsQueryingFalseAndAppendsHistory(t *testing.T) {
	m := newTestModel()

	// Simulate a full query cycle
	m.currentPrompt = "test prompt"
	m.consensusContent.WriteString("consensus answer")
	m.panels[0].Content.WriteString("provider1 answer")
	m.panels[1].Content.WriteString("provider2 answer")
	m.querying = true

	m, _ = updateModel(t, m, QueryDoneMsg{})

	if m.querying {
		t.Error("querying should be false after QueryDoneMsg")
	}
	if len(m.history) != 1 {
		t.Fatalf("history length = %d, want 1", len(m.history))
	}

	ex := m.history[0]
	if ex.Prompt != "test prompt" {
		t.Errorf("history prompt = %q, want %q", ex.Prompt, "test prompt")
	}
	if ex.ConsensusResponse != "consensus answer" {
		t.Errorf("history consensus = %q, want %q", ex.ConsensusResponse, "consensus answer")
	}
	if ex.IndividualResponse["provider1"] != "provider1 answer" {
		t.Errorf("history provider1 = %q, want %q", ex.IndividualResponse["provider1"], "provider1 answer")
	}
	if ex.IndividualResponse["provider2"] != "provider2 answer" {
		t.Errorf("history provider2 = %q, want %q", ex.IndividualResponse["provider2"], "provider2 answer")
	}
	if m.currentPrompt != "" {
		t.Error("currentPrompt should be cleared after QueryDoneMsg")
	}
}

func TestQueryDoneMsg_MultipleExchangesAccumulate(t *testing.T) {
	m := newTestModel()

	// First exchange
	m.currentPrompt = "first"
	m.consensusContent.WriteString("answer1")
	m, _ = updateModel(t, m, QueryDoneMsg{})

	// Second exchange
	m.currentPrompt = "second"
	m.consensusContent.Reset()
	m.consensusContent.WriteString("answer2")
	m, _ = updateModel(t, m, QueryDoneMsg{})

	if len(m.history) != 2 {
		t.Fatalf("history length = %d, want 2", len(m.history))
	}
	if m.history[0].Prompt != "first" {
		t.Errorf("first prompt = %q, want %q", m.history[0].Prompt, "first")
	}
	if m.history[1].Prompt != "second" {
		t.Errorf("second prompt = %q, want %q", m.history[1].Prompt, "second")
	}
}

// ---------------------------------------------------------------------------
// TokenUpdateMsg
// ---------------------------------------------------------------------------

func TestTokenUpdateMsg_UpdatesTokenUsage(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, TokenUpdateMsg{
		Usage: []tokens.ProviderUsage{
			{ProviderID: "provider1", InputTokens: 1500, OutputTokens: 500, Limit: 100000},
			{ProviderID: "provider2", InputTokens: 2000, OutputTokens: 800, Limit: 0},
		},
	})

	if m.tokenUsage == nil {
		t.Fatal("tokenUsage should not be nil")
	}

	td1, ok := m.tokenUsage["provider1"]
	if !ok {
		t.Fatal("tokenUsage should contain provider1")
	}
	if !td1.HasData {
		t.Error("provider1 HasData should be true")
	}
	if td1.Used != tokens.FormatTokenCount(1500) {
		t.Errorf("provider1 Used = %q, want %q", td1.Used, tokens.FormatTokenCount(1500))
	}
	if td1.Limit != tokens.FormatTokenCount(100000) {
		t.Errorf("provider1 Limit = %q, want %q", td1.Limit, tokens.FormatTokenCount(100000))
	}
	if td1.Percent == 0 {
		t.Error("provider1 Percent should be nonzero with limit set")
	}

	td2, ok := m.tokenUsage["provider2"]
	if !ok {
		t.Fatal("tokenUsage should contain provider2")
	}
	if td2.Limit != "" {
		t.Errorf("provider2 Limit = %q, want empty (unlimited)", td2.Limit)
	}
	if td2.Percent != 0 {
		t.Error("provider2 Percent should be 0 for unlimited")
	}
}

func TestTokenUpdateMsg_InitializesNilMap(t *testing.T) {
	m := newTestModel()
	if m.tokenUsage != nil {
		t.Fatal("tokenUsage should be nil initially")
	}

	m, _ = updateModel(t, m, TokenUpdateMsg{
		Usage: []tokens.ProviderUsage{
			{ProviderID: "p1", InputTokens: 100},
		},
	})

	if m.tokenUsage == nil {
		t.Fatal("tokenUsage should be initialized after TokenUpdateMsg")
	}
}

// ---------------------------------------------------------------------------
// ModeChangedMsg
// ---------------------------------------------------------------------------

func TestModeChangedMsg_UpdatesCurrentMode(t *testing.T) {
	m := newTestModel()

	if m.currentMode != "balanced" {
		t.Fatalf("initial mode = %q, want %q", m.currentMode, "balanced")
	}

	m, _ = updateModel(t, m, ModeChangedMsg{Mode: "quick"})
	if m.currentMode != "quick" {
		t.Errorf("mode = %q, want %q", m.currentMode, "quick")
	}

	m, _ = updateModel(t, m, ModeChangedMsg{Mode: "thorough"})
	if m.currentMode != "thorough" {
		t.Errorf("mode = %q, want %q", m.currentMode, "thorough")
	}
}

// ---------------------------------------------------------------------------
// ConfirmActionMsg and confirmation key handling
// ---------------------------------------------------------------------------

func TestConfirmActionMsg_SetsPendingConfirm(t *testing.T) {
	m := newTestModel()
	ch := make(chan bool, 1)

	m, _ = updateModel(t, m, ConfirmActionMsg{
		Description: "Delete file foo.go?",
		ResponseCh:  ch,
	})

	if !m.confirmPending {
		t.Error("confirmPending should be true")
	}
	if m.confirmDescription != "Delete file foo.go?" {
		t.Errorf("confirmDescription = %q, want %q", m.confirmDescription, "Delete file foo.go?")
	}
	if m.confirmResponseCh == nil {
		t.Error("confirmResponseCh should be set")
	}
}

func TestConfirmActionMsg_YeySendsTrue(t *testing.T) {
	m := newTestModel()
	ch := make(chan bool, 1)

	m, _ = updateModel(t, m, ConfirmActionMsg{
		Description: "run command?",
		ResponseCh:  ch,
	})

	// Press 'y' to confirm
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	select {
	case val := <-ch:
		if !val {
			t.Error("expected true on channel after 'y', got false")
		}
	default:
		t.Error("expected value on channel after 'y', got nothing")
	}

	if m.confirmPending {
		t.Error("confirmPending should be false after 'y'")
	}
	if m.confirmDescription != "" {
		t.Error("confirmDescription should be cleared after 'y'")
	}
}

func TestConfirmActionMsg_NoSendsFalse(t *testing.T) {
	m := newTestModel()
	ch := make(chan bool, 1)

	m, _ = updateModel(t, m, ConfirmActionMsg{
		Description: "run command?",
		ResponseCh:  ch,
	})

	// Press 'n' to deny
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	select {
	case val := <-ch:
		if val {
			t.Error("expected false on channel after 'n', got true")
		}
	default:
		t.Error("expected value on channel after 'n', got nothing")
	}

	if m.confirmPending {
		t.Error("confirmPending should be false after 'n'")
	}
}

func TestConfirmActionMsg_EscSendsFalse(t *testing.T) {
	m := newTestModel()
	ch := make(chan bool, 1)

	m, _ = updateModel(t, m, ConfirmActionMsg{
		Description: "run command?",
		ResponseCh:  ch,
	})

	// Press Esc to deny
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	select {
	case val := <-ch:
		if val {
			t.Error("expected false on channel after Esc, got true")
		}
	default:
		t.Error("expected value on channel after Esc, got nothing")
	}

	if m.confirmPending {
		t.Error("confirmPending should be false after Esc")
	}
}

func TestConfirmActionMsg_OtherKeysSwallowed(t *testing.T) {
	m := newTestModel()
	ch := make(chan bool, 1)

	m, _ = updateModel(t, m, ConfirmActionMsg{
		Description: "run command?",
		ResponseCh:  ch,
	})

	// Press an unrelated key — should be swallowed
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// Confirm should still be pending
	if !m.confirmPending {
		t.Error("confirmPending should still be true after unrelated key")
	}

	// Channel should be empty
	select {
	case <-ch:
		t.Error("channel should be empty after unrelated key")
	default:
		// expected
	}
}

// ---------------------------------------------------------------------------
// ToolCallMsg
// ---------------------------------------------------------------------------

func TestToolCallMsg_SetsToolStatus(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ToolCallMsg{
		ToolName:    "file_read",
		Description: "Reading main.go",
	})

	if m.toolStatus != "Reading main.go" {
		t.Errorf("toolStatus = %q, want %q", m.toolStatus, "Reading main.go")
	}
}

// ---------------------------------------------------------------------------
// ConsensusAnalysisMsg
// ---------------------------------------------------------------------------

func TestConsensusAnalysisMsg_SetsProvenance(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, ConsensusAnalysisMsg{
		Confidence: "high",
		Agreements: []string{"point1", "point2"},
		Minorities: []string{"dissent1"},
		Evidence:   []string{"evidence1"},
	})

	if m.consensusConfidence != "high" {
		t.Errorf("confidence = %q, want %q", m.consensusConfidence, "high")
	}
	if len(m.consensusAgreements) != 2 {
		t.Errorf("agreements len = %d, want 2", len(m.consensusAgreements))
	}
	if len(m.minorityReports) != 1 {
		t.Errorf("minorities len = %d, want 1", len(m.minorityReports))
	}
	if len(m.consensusEvidence) != 1 {
		t.Errorf("evidence len = %d, want 1", len(m.consensusEvidence))
	}
}

// ---------------------------------------------------------------------------
// WindowSizeMsg
// ---------------------------------------------------------------------------

func TestWindowSizeMsg_UpdatesDimensions(t *testing.T) {
	m := newTestModel()

	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}

// ---------------------------------------------------------------------------
// SplashDoneMsg
// ---------------------------------------------------------------------------

func TestSplashDoneMsg_DismissesSplash(t *testing.T) {
	m := NewModel([]string{"provider1"}, "provider1", "v1")
	if !m.showSplash {
		t.Fatal("showSplash should be true initially")
	}

	m, _ = updateModel(t, m, splashDoneMsg{})

	if m.showSplash {
		t.Error("showSplash should be false after splashDoneMsg")
	}
}

// ---------------------------------------------------------------------------
// Integration: full query lifecycle
// ---------------------------------------------------------------------------

func TestFullQueryLifecycle(t *testing.T) {
	m := newTestModel()

	// 1. Query starts
	m, _ = updateModel(t, m, QueryStartMsg{})
	if !m.querying {
		t.Fatal("should be querying after QueryStartMsg")
	}

	m.currentPrompt = "explain Go interfaces"

	// 2. Provider chunks stream in
	m, _ = updateModel(t, m, ProviderChunkMsg{ProviderName: "provider1", Delta: "Interfaces in Go "})
	m, _ = updateModel(t, m, ProviderChunkMsg{ProviderName: "provider2", Delta: "Go interfaces are "})
	m, _ = updateModel(t, m, ProviderChunkMsg{ProviderName: "provider1", Delta: "define behavior.", Done: true})
	m, _ = updateModel(t, m, ProviderChunkMsg{ProviderName: "provider2", Delta: "contracts.", Done: true})

	if m.panels[0].Status != StatusDone {
		t.Error("provider1 should be done")
	}
	if m.panels[1].Status != StatusDone {
		t.Error("provider2 should be done")
	}

	// 3. Consensus synthesis streams in
	m, _ = updateModel(t, m, ConsensusChunkMsg{Delta: "Go interfaces define "})
	m, _ = updateModel(t, m, ConsensusChunkMsg{Delta: "behavioral contracts."})
	m, _ = updateModel(t, m, ConsensusChunkMsg{Done: true})

	if !m.consensusActive {
		t.Error("consensus should be active")
	}

	// 4. Token update
	m, _ = updateModel(t, m, TokenUpdateMsg{
		Usage: []tokens.ProviderUsage{
			{ProviderID: "provider1", InputTokens: 500, OutputTokens: 200, Limit: 100000},
		},
	})

	if _, ok := m.tokenUsage["provider1"]; !ok {
		t.Error("token usage should be recorded for provider1")
	}

	// 5. Query done — adds to history
	m, _ = updateModel(t, m, QueryDoneMsg{})

	if m.querying {
		t.Error("should not be querying after QueryDoneMsg")
	}
	if len(m.history) != 1 {
		t.Fatalf("history len = %d, want 1", len(m.history))
	}
	if m.history[0].Prompt != "explain Go interfaces" {
		t.Errorf("history prompt = %q, want %q", m.history[0].Prompt, "explain Go interfaces")
	}
	if m.history[0].ConsensusResponse != "Go interfaces define behavioral contracts." {
		t.Errorf("history consensus = %q, want %q",
			m.history[0].ConsensusResponse, "Go interfaces define behavioral contracts.")
	}
}
