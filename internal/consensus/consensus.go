package consensus

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// Engine drives the consensus synthesis step. It takes the collected responses
// from a fan-out query and asks a primary provider to synthesize them into a
// single authoritative answer.
type Engine struct {
	primary      provider.Provider
	timeout      time.Duration
	minResponses int
	mode         SynthesisMode
}

// NewEngine creates a consensus Engine.
//   - primary is the provider used to synthesize the final answer.
//   - timeout is the maximum time allowed for the synthesis query.
//   - minResponses is the minimum number of successful responses required
//     before synthesis can proceed.
func NewEngine(primary provider.Provider, timeout time.Duration, minResponses int) *Engine {
	return &Engine{
		primary:      primary,
		timeout:      timeout,
		minResponses: minResponses,
	}
}

// SynthesisMode controls the depth of the consensus synthesis prompt.
type SynthesisMode string

const (
	// SynthesisQuick produces a concise, direct answer without structured sections.
	SynthesisQuick SynthesisMode = "quick"
	// SynthesisBalanced produces a structured synthesis with confidence, agreements,
	// minority reports, and evidence sections.
	SynthesisBalanced SynthesisMode = "balanced"
	// SynthesisThorough produces deep analysis with extended reasoning, trade-off
	// analysis, step-by-step verification, and alternative approaches.
	SynthesisThorough SynthesisMode = "thorough"
)

// BuildConsensusPrompt constructs the message slice sent to the primary
// provider for synthesis. The mode controls the depth of analysis requested.
func (e *Engine) BuildConsensusPrompt(originalPrompt string, responses map[string]string, mode SynthesisMode, history ...provider.Message) []provider.Message {
	var b strings.Builder

	b.WriteString("You are synthesizing responses from multiple AI models to produce the best possible answer.\n\n")
	fmt.Fprintf(&b, "Original question: %s\n\n", originalPrompt)
	b.WriteString("Model responses:\n")

	// Sort provider IDs for deterministic prompt ordering.
	ids := make([]string, 0, len(responses))
	for id := range responses {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b.WriteString("---\n")
		fmt.Fprintf(&b, "[Model: %s]\n", id)
		b.WriteString(responses[id])
		b.WriteString("\n---\n\n")
	}

	switch mode {
	case SynthesisQuick:
		b.WriteString("Produce a concise, direct answer combining the best insights from all models. ")
		b.WriteString("Do not use structured sections — just give the answer. Be brief and actionable.\n")

	case SynthesisThorough:
		b.WriteString("Produce a deep, thorough synthesis. Think step by step.\n\n")
		b.WriteString("## Recommendation\n")
		b.WriteString("[Your synthesized answer — combine the best insights from all models. Be comprehensive.]\n\n")
		b.WriteString("## Confidence: [high/medium/low]\n\n")
		b.WriteString("## Reasoning\n")
		b.WriteString("[Walk through your reasoning step by step. Explain WHY this is the best answer, not just WHAT it is.]\n\n")
		b.WriteString("## Agreement\n")
		b.WriteString("[Points where all or most models agree, with specifics]\n\n")
		b.WriteString("## Minority Report\n")
		b.WriteString("[Dissenting views worth considering, with the model name and reasoning. Evaluate whether the dissent has merit. Write \"None — all models agreed\" if there are no disagreements.]\n\n")
		b.WriteString("## Trade-offs & Alternatives\n")
		b.WriteString("[Alternative approaches considered by any model. Pros and cons of each. When would you choose differently?]\n\n")
		b.WriteString("## Verification\n")
		b.WriteString("[Cross-check key claims against each other. Flag any factual conflicts between models. Note anything that should be verified.]\n\n")
		b.WriteString("## Evidence\n")
		b.WriteString("[Key facts, code references, documentation, or examples cited by any model]\n")

	default: // SynthesisBalanced
		b.WriteString("Analyze all responses and produce a synthesis with this structure:\n\n")
		b.WriteString("## Recommendation\n")
		b.WriteString("[Your synthesized answer — the best response combining insights from all models]\n\n")
		b.WriteString("## Confidence: [high/medium/low]\n\n")
		b.WriteString("## Agreement\n")
		b.WriteString("[Points where all or most models agree]\n\n")
		b.WriteString("## Minority Report\n")
		b.WriteString("[Dissenting views worth considering, with the model name and reasoning. Write \"None — all models agreed\" if there are no disagreements.]\n\n")
		b.WriteString("## Evidence\n")
		b.WriteString("[Key facts, code references, or documentation cited by any model]\n")
	}

	// Prepend conversation history so the synthesis model has multi-turn context.
	// Only include messages before the current user prompt (the last user message
	// is replaced by the synthesis prompt which includes it).
	var msgs []provider.Message
	for _, m := range history {
		// Skip the last user message — it's embedded in the synthesis prompt
		msgs = append(msgs, m)
	}
	msgs = append(msgs, provider.Message{
		Role:    provider.RoleUser,
		Content: b.String(),
	})
	return msgs
}

// Synthesize sends the consensus prompt to the primary provider and returns
// the streaming response channel. The caller is responsible for consuming the
// channel.
func (e *Engine) Synthesize(
	ctx context.Context,
	originalPrompt string,
	responses map[string]string,
	opts provider.QueryOpts,
	history ...provider.Message,
) (<-chan provider.StreamChunk, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)

	messages := e.BuildConsensusPrompt(originalPrompt, responses, e.mode, history...)

	ch, err := e.primary.Query(ctx, messages, opts)
	if err != nil {
		cancel()
		return nil, err
	}

	// Wrap the channel so we can release the timeout context when the stream
	// is fully consumed.
	out := make(chan provider.StreamChunk)
	go func() {
		defer cancel()
		defer close(out)
		for chunk := range ch {
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}
