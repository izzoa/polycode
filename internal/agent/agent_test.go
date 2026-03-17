package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// mockWorkerProvider implements provider.Provider for testing.
type mockWorkerProvider struct {
	id          string
	response    string
	inputTokens int
	outputTokens int
	delay       time.Duration
	err         error
}

func (m *mockWorkerProvider) ID() string          { return m.id }
func (m *mockWorkerProvider) Authenticate() error  { return nil }
func (m *mockWorkerProvider) Validate() error      { return nil }

func (m *mockWorkerProvider) Query(ctx context.Context, messages []provider.Message, opts provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				ch <- provider.StreamChunk{Error: ctx.Err()}
				return
			}
		}
		ch <- provider.StreamChunk{Delta: m.response}
		ch <- provider.StreamChunk{
			Done:         true,
			InputTokens:  m.inputTokens,
			OutputTokens: m.outputTokens,
		}
	}()
	return ch, nil
}

// --- 7.1: Worker.Run with mock provider ---

func TestWorkerRun(t *testing.T) {
	mock := &mockWorkerProvider{
		id:           "test-provider",
		response:     "Here is a plan:\n1. Step one\n2. Step two",
		inputTokens:  50,
		outputTokens: 30,
	}

	w := &Worker{
		Role:         RolePlanner,
		ProviderName: "test-provider",
		Provider:     mock,
		SystemPrompt: RolePrompts[RolePlanner],
	}

	output, usage, err := w.Run(context.Background(), "Build a REST API")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != mock.response {
		t.Errorf("expected output %q, got %q", mock.response, output)
	}
	if usage.InputTokens != 50 {
		t.Errorf("expected 50 input tokens, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 30 {
		t.Errorf("expected 30 output tokens, got %d", usage.OutputTokens)
	}
}

func TestWorkerRunNilProvider(t *testing.T) {
	w := &Worker{
		Role:         RolePlanner,
		SystemPrompt: "test",
	}

	_, _, err := w.Run(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
	if !strings.Contains(err.Error(), "provider is nil") {
		t.Errorf("expected nil provider error, got: %v", err)
	}
}

func TestWorkerRunProviderError(t *testing.T) {
	mock := &mockWorkerProvider{
		id:  "broken",
		err: fmt.Errorf("rate limited"),
	}

	w := &Worker{
		Role:         RolePlanner,
		Provider:     mock,
		SystemPrompt: "test",
	}

	_, _, err := w.Run(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error from provider")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limited error, got: %v", err)
	}
}

// --- 7.2: TaskGraph.Run with 3 stages, verify output chains ---

func TestTaskGraphRunThreeStages(t *testing.T) {
	graph := &TaskGraph{
		JobID: "test-job-3stage",
		Stages: []Stage{
			{
				Name: "planning",
				Workers: []*Worker{
					{
						Role:         RolePlanner,
						Provider:     &mockWorkerProvider{id: "p1", response: "Plan: step1, step2", inputTokens: 10, outputTokens: 20},
						SystemPrompt: RolePrompts[RolePlanner],
					},
				},
			},
			{
				Name: "research",
				Workers: []*Worker{
					{
						Role:         RoleResearcher,
						Provider:     &mockWorkerProvider{id: "p2", response: "Research: found relevant code", inputTokens: 15, outputTokens: 25},
						SystemPrompt: RolePrompts[RoleResearcher],
					},
				},
			},
			{
				Name: "review",
				Workers: []*Worker{
					{
						Role:         RoleReviewer,
						Provider:     &mockWorkerProvider{id: "p3", response: "Review: looks good", inputTokens: 20, outputTokens: 30},
						SystemPrompt: RolePrompts[RoleReviewer],
					},
				},
			},
		},
	}

	// Use a temp dir for checkpoints.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	var completedStages []string
	result, err := graph.Run(context.Background(), "Build a REST API", func(sr StageResult) {
		completedStages = append(completedStages, sr.StageName)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Complete {
		t.Error("expected job to be complete")
	}

	if len(result.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(result.Stages))
	}

	// Verify stage names.
	expectedNames := []string{"planning", "research", "review"}
	for i, name := range expectedNames {
		if result.Stages[i].StageName != name {
			t.Errorf("stage %d: expected name %q, got %q", i, name, result.Stages[i].StageName)
		}
	}

	// Verify callback was called for each stage.
	if len(completedStages) != 3 {
		t.Fatalf("expected 3 stage callbacks, got %d", len(completedStages))
	}

	// Verify total usage.
	expectedInput := 10 + 15 + 20
	expectedOutput := 20 + 25 + 30
	if result.TotalUsage.InputTokens != expectedInput {
		t.Errorf("expected %d total input tokens, got %d", expectedInput, result.TotalUsage.InputTokens)
	}
	if result.TotalUsage.OutputTokens != expectedOutput {
		t.Errorf("expected %d total output tokens, got %d", expectedOutput, result.TotalUsage.OutputTokens)
	}

	// Verify outputs.
	if result.Stages[0].WorkerOutputs[RolePlanner] != "Plan: step1, step2" {
		t.Errorf("unexpected planner output: %q", result.Stages[0].WorkerOutputs[RolePlanner])
	}
	if result.Stages[1].WorkerOutputs[RoleResearcher] != "Research: found relevant code" {
		t.Errorf("unexpected researcher output: %q", result.Stages[1].WorkerOutputs[RoleResearcher])
	}
	if result.Stages[2].WorkerOutputs[RoleReviewer] != "Review: looks good" {
		t.Errorf("unexpected reviewer output: %q", result.Stages[2].WorkerOutputs[RoleReviewer])
	}
}

func TestTaskGraphRunParallelWorkersInStage(t *testing.T) {
	graph := &TaskGraph{
		JobID: "test-parallel",
		Stages: []Stage{
			{
				Name: "parallel-stage",
				Workers: []*Worker{
					{
						Role:         RoleResearcher,
						Provider:     &mockWorkerProvider{id: "r1", response: "research output", inputTokens: 10, outputTokens: 10},
						SystemPrompt: RolePrompts[RoleResearcher],
					},
					{
						Role:         RoleImplementer,
						Provider:     &mockWorkerProvider{id: "i1", response: "impl output", inputTokens: 10, outputTokens: 10},
						SystemPrompt: RolePrompts[RoleImplementer],
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	result, err := graph.Run(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Stages[0].WorkerOutputs) != 2 {
		t.Fatalf("expected 2 worker outputs, got %d", len(result.Stages[0].WorkerOutputs))
	}
	if result.Stages[0].WorkerOutputs[RoleResearcher] != "research output" {
		t.Errorf("unexpected researcher output: %q", result.Stages[0].WorkerOutputs[RoleResearcher])
	}
	if result.Stages[0].WorkerOutputs[RoleImplementer] != "impl output" {
		t.Errorf("unexpected implementer output: %q", result.Stages[0].WorkerOutputs[RoleImplementer])
	}
}

// --- 7.3: Budget cap stops execution ---

func TestTaskGraphBudgetCap(t *testing.T) {
	graph := &TaskGraph{
		JobID:  "test-budget",
		Budget: 50, // very low budget
		Stages: []Stage{
			{
				Name: "stage1",
				Workers: []*Worker{
					{
						Role:         RolePlanner,
						Provider:     &mockWorkerProvider{id: "p1", response: "plan", inputTokens: 30, outputTokens: 30},
						SystemPrompt: "plan",
					},
				},
			},
			{
				Name: "stage2",
				Workers: []*Worker{
					{
						Role:         RoleResearcher,
						Provider:     &mockWorkerProvider{id: "p2", response: "research", inputTokens: 30, outputTokens: 30},
						SystemPrompt: "research",
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	result, err := graph.Run(context.Background(), "test", nil)

	// Should error because budget is exceeded after stage1 (60 >= 50).
	if err == nil {
		t.Fatal("expected budget error")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded error, got: %v", err)
	}

	// Only stage1 should have run.
	if len(result.Stages) != 1 {
		t.Errorf("expected 1 completed stage, got %d", len(result.Stages))
	}
	if result.Complete {
		t.Error("job should not be marked complete when budget exceeded")
	}
}

// --- 7.4: Checkpoint save/load round-trip ---

func TestCheckpointSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cp := &JobCheckpoint{
		JobID:   "round-trip-test",
		Request: "Build a REST API",
		Stages: []StageCheckpoint{
			{
				Name:     "planning",
				Complete: true,
				Outputs: map[string]string{
					"planner": "Plan: step1, step2",
				},
			},
			{
				Name:     "research",
				Complete: true,
				Outputs: map[string]string{
					"researcher": "Found relevant files",
				},
			},
		},
		CreatedAt: time.Now().Truncate(time.Second),
	}

	// Save.
	err := SaveCheckpoint(cp.JobID, cp)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(tmpDir, "polycode", "jobs", "round-trip-test.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("checkpoint file not created")
	}

	// Load.
	loaded, err := LoadCheckpoint("round-trip-test")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.JobID != cp.JobID {
		t.Errorf("expected job ID %q, got %q", cp.JobID, loaded.JobID)
	}
	if loaded.Request != cp.Request {
		t.Errorf("expected request %q, got %q", cp.Request, loaded.Request)
	}
	if len(loaded.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(loaded.Stages))
	}
	if loaded.Stages[0].Name != "planning" {
		t.Errorf("expected first stage name %q, got %q", "planning", loaded.Stages[0].Name)
	}
	if !loaded.Stages[0].Complete {
		t.Error("expected first stage to be complete")
	}
	if loaded.Stages[0].Outputs["planner"] != "Plan: step1, step2" {
		t.Errorf("unexpected planner output: %q", loaded.Stages[0].Outputs["planner"])
	}
	if loaded.Stages[1].Outputs["researcher"] != "Found relevant files" {
		t.Errorf("unexpected researcher output: %q", loaded.Stages[1].Outputs["researcher"])
	}
}

// --- 7.5: Resume skips completed stages ---

func TestTaskGraphResumeSkipsCompletedStages(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create a checkpoint with stage1 already complete.
	cp := &JobCheckpoint{
		JobID:   "resume-test",
		Request: "Build a REST API",
		Stages: []StageCheckpoint{
			{
				Name:     "planning",
				Complete: true,
				Outputs: map[string]string{
					"planner": "Plan: step1, step2",
				},
			},
		},
		CreatedAt: time.Now(),
	}
	if err := SaveCheckpoint(cp.JobID, cp); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	// Track which stages actually ran via the mock provider.
	stage2Ran := false
	stage2Mock := &mockWorkerProvider{
		id:           "reviewer",
		response:     "Review: approved",
		inputTokens:  20,
		outputTokens: 15,
	}

	graph := &TaskGraph{
		JobID: "resume-test",
		Stages: []Stage{
			{
				Name: "planning",
				Workers: []*Worker{
					{
						Role:         RolePlanner,
						Provider:     &mockWorkerProvider{id: "planner", response: "should not run"},
						SystemPrompt: RolePrompts[RolePlanner],
					},
				},
			},
			{
				Name: "review",
				Workers: []*Worker{
					{
						Role:         RoleReviewer,
						Provider:     stage2Mock,
						SystemPrompt: RolePrompts[RoleReviewer],
					},
				},
			},
		},
	}

	var completedStages []string
	result, err := graph.Resume(ctx(), "resume-test", func(sr StageResult) {
		completedStages = append(completedStages, sr.StageName)
		if sr.StageName == "review" {
			stage2Ran = true
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Complete {
		t.Error("expected job to be complete")
	}

	// Should have 2 stages total (1 restored + 1 executed).
	if len(result.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(result.Stages))
	}

	// The first stage should have the restored output.
	if result.Stages[0].WorkerOutputs[RolePlanner] != "Plan: step1, step2" {
		t.Errorf("expected restored planner output, got: %q", result.Stages[0].WorkerOutputs[RolePlanner])
	}

	// The second stage should have actually run.
	if !stage2Ran {
		t.Error("expected stage2 (review) to run")
	}
	if result.Stages[1].WorkerOutputs[RoleReviewer] != "Review: approved" {
		t.Errorf("expected reviewer output, got: %q", result.Stages[1].WorkerOutputs[RoleReviewer])
	}

	// Only stage2 should trigger the callback (stage1 was restored).
	if len(completedStages) != 1 {
		t.Fatalf("expected 1 stage callback, got %d", len(completedStages))
	}
	if completedStages[0] != "review" {
		t.Errorf("expected callback for review stage, got %q", completedStages[0])
	}
}

func ctx() context.Context {
	return context.Background()
}

// --- 7.6: ResolveProvider returns configured provider or falls back to primary ---

// mockRegistry is a simple Registry wrapper for testing ResolveProvider.
// Since provider.Registry fields are unexported, we use a helper that
// builds a real Registry via a different approach - we test ResolveProvider
// with a custom test helper.

func TestResolveProviderConfigured(t *testing.T) {
	claude := &mockWorkerProvider{id: "claude"}
	gemini := &mockWorkerProvider{id: "gemini"}
	gpt4 := &mockWorkerProvider{id: "gpt4"}

	reg := newTestRegistry(claude, gemini, gpt4)

	roles := map[RoleType]string{
		RolePlanner:    "claude",
		RoleResearcher: "gemini",
		RoleReviewer:   "gpt4",
	}

	// Should return the configured provider.
	p := ResolveProvider(RolePlanner, reg, roles)
	if p.ID() != "claude" {
		t.Errorf("expected claude for planner, got %s", p.ID())
	}

	p = ResolveProvider(RoleResearcher, reg, roles)
	if p.ID() != "gemini" {
		t.Errorf("expected gemini for researcher, got %s", p.ID())
	}

	p = ResolveProvider(RoleReviewer, reg, roles)
	if p.ID() != "gpt4" {
		t.Errorf("expected gpt4 for reviewer, got %s", p.ID())
	}
}

func TestResolveProviderFallbackToPrimary(t *testing.T) {
	claude := &mockWorkerProvider{id: "claude"}
	gemini := &mockWorkerProvider{id: "gemini"}

	reg := newTestRegistry(claude, gemini)

	// No roles configured for implementer.
	roles := map[RoleType]string{
		RolePlanner: "claude",
	}

	p := ResolveProvider(RoleImplementer, reg, roles)
	// Should fall back to primary (first provider = claude).
	if p.ID() != "claude" {
		t.Errorf("expected primary provider claude, got %s", p.ID())
	}
}

func TestResolveProviderUnknownName(t *testing.T) {
	claude := &mockWorkerProvider{id: "claude"}

	reg := newTestRegistry(claude)

	roles := map[RoleType]string{
		RolePlanner: "nonexistent",
	}

	p := ResolveProvider(RolePlanner, reg, roles)
	// Should fall back to primary when the named provider is not found.
	if p.ID() != "claude" {
		t.Errorf("expected fallback to primary claude, got %s", p.ID())
	}
}

// newTestRegistry constructs a provider.Registry for testing.
// The first provider is treated as primary.
func newTestRegistry(providers ...provider.Provider) *provider.Registry {
	return provider.NewTestRegistry(providers[0], providers...)
}
