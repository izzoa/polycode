package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/izzoa/polycode/internal/tokens"
)

// MergeFunc combines the outputs of parallel workers within a stage
// into a single string that becomes the input for the next stage.
type MergeFunc func(outputs map[RoleType]string) string

// Stage represents a group of workers that execute in parallel.
// Stages execute sequentially within a TaskGraph.
type Stage struct {
	Name    string
	Workers []*Worker
	Merge   MergeFunc // nil = default merge
}

// StageResult captures the output of a completed stage.
type StageResult struct {
	StageName     string
	WorkerOutputs map[RoleType]string
	Usage         tokens.Usage
}

// JobResult captures the full result of a task graph execution.
type JobResult struct {
	JobID      string
	Request    string
	Stages     []StageResult
	TotalUsage tokens.Usage
	Complete   bool
	Error      string
}

// TaskGraph is an ordered list of stages that execute sequentially.
// Within each stage, workers run in parallel.
type TaskGraph struct {
	JobID  string
	Stages []Stage
	Budget int // max total tokens (input+output), 0 = unlimited
}

// DefaultMerge concatenates worker outputs with role headers.
func DefaultMerge(outputs map[RoleType]string) string {
	// Sort roles for deterministic output.
	roles := make([]RoleType, 0, len(outputs))
	for r := range outputs {
		roles = append(roles, r)
	}
	sort.Slice(roles, func(i, j int) bool {
		return string(roles[i]) < string(roles[j])
	})

	var buf strings.Builder
	for _, r := range roles {
		fmt.Fprintf(&buf, "[Role: %s]\n%s\n---\n", r, outputs[r])
	}
	return buf.String()
}

// Run executes the task graph: stages run sequentially, workers within
// each stage run in parallel. The merged output of each stage becomes
// the input for the next stage. The onStageComplete callback is called
// after each stage finishes.
func (g *TaskGraph) Run(ctx context.Context, input string, onStageComplete func(StageResult)) (*JobResult, error) {
	result := &JobResult{
		JobID:   g.JobID,
		Request: input,
	}

	currentInput := input

	for _, stage := range g.Stages {
		sr, err := g.runStage(ctx, stage, currentInput)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}

		result.Stages = append(result.Stages, sr)
		result.TotalUsage.InputTokens += sr.Usage.InputTokens
		result.TotalUsage.OutputTokens += sr.Usage.OutputTokens

		// Check budget.
		if g.Budget > 0 {
			total := result.TotalUsage.InputTokens + result.TotalUsage.OutputTokens
			if total >= g.Budget {
				result.Error = fmt.Sprintf("budget exceeded: %d >= %d", total, g.Budget)
				return result, fmt.Errorf("%s", result.Error)
			}
		}

		// Save checkpoint after stage.
		checkpoint := jobResultToCheckpoint(result)
		if err := SaveCheckpoint(g.JobID, checkpoint); err != nil {
			// Checkpoint save failure is non-fatal; log but continue.
			_ = err
		}

		if onStageComplete != nil {
			onStageComplete(sr)
		}

		// Merge outputs for the next stage.
		merge := stage.Merge
		if merge == nil {
			merge = DefaultMerge
		}
		currentInput = merge(sr.WorkerOutputs)
	}

	result.Complete = true

	// Save final checkpoint.
	checkpoint := jobResultToCheckpoint(result)
	_ = SaveCheckpoint(g.JobID, checkpoint)

	return result, nil
}

// Resume loads a checkpoint and continues execution from the next
// incomplete stage.
func (g *TaskGraph) Resume(ctx context.Context, jobID string, onStageComplete func(StageResult)) (*JobResult, error) {
	cp, err := LoadCheckpoint(jobID)
	if err != nil {
		return nil, fmt.Errorf("loading checkpoint: %w", err)
	}

	g.JobID = cp.JobID

	result := &JobResult{
		JobID:   cp.JobID,
		Request: cp.Request,
	}

	// Determine where to resume from.
	var currentInput string
	startIdx := 0

	for i, scp := range cp.Stages {
		if !scp.Complete {
			break
		}

		// Reconstruct the stage result from the checkpoint.
		sr := StageResult{
			StageName:     scp.Name,
			WorkerOutputs: make(map[RoleType]string),
		}
		for role, output := range scp.Outputs {
			sr.WorkerOutputs[RoleType(role)] = output
		}

		result.Stages = append(result.Stages, sr)
		startIdx = i + 1

		// Merge outputs to reconstruct input for the next stage.
		merge := g.Stages[i].Merge
		if merge == nil {
			merge = DefaultMerge
		}
		currentInput = merge(sr.WorkerOutputs)
	}

	if startIdx == 0 {
		currentInput = cp.Request
	}

	// Execute remaining stages.
	for i := startIdx; i < len(g.Stages); i++ {
		stage := g.Stages[i]

		sr, err := g.runStage(ctx, stage, currentInput)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}

		result.Stages = append(result.Stages, sr)
		result.TotalUsage.InputTokens += sr.Usage.InputTokens
		result.TotalUsage.OutputTokens += sr.Usage.OutputTokens

		// Check budget.
		if g.Budget > 0 {
			total := result.TotalUsage.InputTokens + result.TotalUsage.OutputTokens
			if total >= g.Budget {
				result.Error = fmt.Sprintf("budget exceeded: %d >= %d", total, g.Budget)
				return result, fmt.Errorf("%s", result.Error)
			}
		}

		// Save checkpoint.
		checkpoint := jobResultToCheckpoint(result)
		_ = SaveCheckpoint(g.JobID, checkpoint)

		if onStageComplete != nil {
			onStageComplete(sr)
		}

		merge := stage.Merge
		if merge == nil {
			merge = DefaultMerge
		}
		currentInput = merge(sr.WorkerOutputs)
	}

	result.Complete = true
	checkpoint := jobResultToCheckpoint(result)
	_ = SaveCheckpoint(g.JobID, checkpoint)

	return result, nil
}

// runStage executes all workers in a stage concurrently and collects results.
func (g *TaskGraph) runStage(ctx context.Context, stage Stage, input string) (StageResult, error) {
	sr := StageResult{
		StageName:     stage.Name,
		WorkerOutputs: make(map[RoleType]string),
	}

	type workerResult struct {
		role   RoleType
		output string
		usage  tokens.Usage
		err    error
	}

	var wg sync.WaitGroup
	results := make(chan workerResult, len(stage.Workers))

	for _, w := range stage.Workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			output, usage, err := w.Run(ctx, input)
			results <- workerResult{
				role:   w.Role,
				output: output,
				usage:  usage,
				err:    err,
			}
		}(w)
	}

	// Close results channel when all workers are done.
	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for wr := range results {
		if wr.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stage %q worker %s: %w", stage.Name, wr.role, wr.err)
		}
		sr.WorkerOutputs[wr.role] = wr.output
		sr.Usage.InputTokens += wr.usage.InputTokens
		sr.Usage.OutputTokens += wr.usage.OutputTokens
	}

	if firstErr != nil {
		return sr, firstErr
	}

	return sr, nil
}
