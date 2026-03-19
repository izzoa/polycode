package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/action"
	"github.com/izzoa/polycode/internal/agent"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/telemetry"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/izzoa/polycode/internal/tui"
)

// conversationState maintains the full multi-turn dialogue context.
type conversationState struct {
	mu       sync.Mutex
	messages []provider.Message
}

func (c *conversationState) append(msgs ...provider.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msgs...)
}

func (c *conversationState) snapshot() []provider.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]provider.Message, len(c.messages))
	copy(out, c.messages)
	return out
}

func startTUI(cfg *config.Config) error {
	// Create provider registry
	registry, err := provider.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("creating provider registry: %w", err)
	}

	// Authenticate all providers
	for _, p := range registry.Providers() {
		if err := p.Authenticate(); err != nil {
			fmt.Printf("Warning: failed to authenticate %s: %v\n", p.ID(), err)
		}
	}

	// Get healthy providers
	healthy := registry.Healthy()
	if len(healthy) == 0 {
		return fmt.Errorf("no healthy providers available — run 'polycode auth login <provider>' to authenticate")
	}

	primary := registry.Primary()
	if err := primary.Validate(); err != nil {
		return fmt.Errorf("primary provider %s is not healthy: %w", primary.ID(), err)
	}

	// Create metadata store for litellm model metadata
	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, err := tokens.NewMetadataStore(
		cfg.Metadata.URL,
		cachePath,
		cfg.Metadata.CacheTTL,
	)
	if err != nil {
		log.Printf("Warning: failed to initialize metadata store: %v", err)
	}

	// Build provider name list and resolve token limits
	var names []string
	providerModels := make(map[string]string)
	providerLimits := make(map[string]int)
	for _, pc := range cfg.Providers {
		providerModels[pc.Name] = pc.Model
		if metadataStore != nil {
			providerLimits[pc.Name] = metadataStore.LimitForModel(pc.Model, string(pc.Type), pc.MaxContext)
		} else {
			providerLimits[pc.Name] = tokens.LimitForModel(pc.Model, pc.MaxContext)
		}
	}
	for _, p := range healthy {
		names = append(names, p.ID())
	}

	// Create token tracker
	tracker := tokens.NewTracker(providerModels, providerLimits)

	// Create telemetry logger
	tlog, err := telemetry.NewLogger()
	if err != nil {
		log.Printf("Warning: telemetry disabled: %v", err)
	}
	if tlog != nil {
		defer tlog.Close()
	}

	// Create consensus pipeline with tracker
	pipeline := consensus.NewPipeline(
		healthy,
		primary,
		cfg.Consensus.Timeout,
		cfg.Consensus.MinResponses,
		tracker,
	)

	// System prompt used at the start of every conversation.
	systemPrompt := provider.Message{
		Role:    provider.RoleSystem,
		Content: "You are polycode, a helpful coding assistant. Provide clear, concise answers to programming questions. When the user asks you to make changes, use the available tools (file_read, file_write, shell_exec) to interact with their codebase.",
	}

	// Conversation state persists across turns
	conv := &conversationState{
		messages: []provider.Message{systemPrompt},
	}

	// Create TUI model
	model := tui.NewModel(names, primary.ID(), version)

	// Task 4.1/4.2: Pass model listing closure to the TUI model for wizard use
	model.SetModelLister(func(providerType string) []config.ModelSummary {
		if metadataStore == nil {
			return nil
		}
		return metadataStore.ModelsForProvider(providerType)
	})

	// Auto-resume: load saved session if one exists
	if savedSession, err := config.LoadSession(); err == nil && savedSession != nil && len(savedSession.Messages) > 0 {
		// Restore conversation messages
		conv.mu.Lock()
		conv.messages = []provider.Message{systemPrompt}
		for _, m := range savedSession.Messages {
			if m.Role == "system" {
				continue // skip saved system prompt, we use the current one
			}
			conv.messages = append(conv.messages, provider.Message{
				Role:    provider.Role(m.Role),
				Content: m.Content,
			})
		}
		conv.mu.Unlock()

		// Restore display history
		for _, ex := range savedSession.Exchanges {
			model.AppendHistory(tui.Exchange{
				Prompt:             ex.Prompt,
				ConsensusResponse:  ex.ConsensusResponse,
				IndividualResponse: ex.Individual,
			})
		}
	}
	model.SetConfig(cfg)

	// Declare program early so handler closures can capture it.
	// It's set after NewProgram but before Run(), so it's always
	// non-nil by the time any handler goroutine executes.
	var program *tea.Program

	// Set up config change handler that rebuilds registry + pipeline
	model.SetConfigChangeHandler(func(newCfg *config.Config) {
		newRegistry, err := provider.NewRegistry(newCfg)
		if err != nil {
			log.Printf("Warning: failed to rebuild registry after config change: %v", err)
			return
		}

		// Authenticate all providers
		for _, p := range newRegistry.Providers() {
			if authErr := p.Authenticate(); authErr != nil {
				log.Printf("Warning: failed to authenticate %s: %v", p.ID(), authErr)
			}
		}

		newHealthy := newRegistry.Healthy()
		if len(newHealthy) == 0 {
			log.Printf("Warning: no healthy providers after config change")
			return
		}

		newPrimary := newRegistry.Primary()

		// Update tracker models and limits
		newProviderModels := make(map[string]string)
		newProviderLimits := make(map[string]int)
		for _, pc := range newCfg.Providers {
			newProviderModels[pc.Name] = pc.Model
			if metadataStore != nil {
				newProviderLimits[pc.Name] = metadataStore.LimitForModel(pc.Model, string(pc.Type), pc.MaxContext)
			} else {
				newProviderLimits[pc.Name] = tokens.LimitForModel(pc.Model, pc.MaxContext)
			}
		}

		// Rebuild tracker and pipeline
		tracker = tokens.NewTracker(newProviderModels, newProviderLimits)
		pipeline = consensus.NewPipeline(
			newHealthy,
			newPrimary,
			newCfg.Consensus.Timeout,
			newCfg.Consensus.MinResponses,
			tracker,
		)
		registry = newRegistry
		healthy = newHealthy
		primary = newPrimary
		cfg = newCfg
	})

	// Set up test provider handler
	model.SetTestProviderHandler(func(providerName string) {
		go func() {
			start := time.Now()
			var testProvider provider.Provider
			for _, p := range registry.Providers() {
				if p.ID() == providerName {
					testProvider = p
					break
				}
			}
			if testProvider == nil {
				program.Send(tui.TestResultMsg{
					ProviderName: providerName,
					Success:      false,
					Error:        fmt.Errorf("provider not found in registry"),
				})
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			msgs := []provider.Message{
				{Role: provider.RoleUser, Content: "Say hello"},
			}
			opts := provider.QueryOpts{MaxTokens: 16}

			stream, err := testProvider.Query(ctx, msgs, opts)
			if err != nil {
				program.Send(tui.TestResultMsg{
					ProviderName: providerName,
					Success:      false,
					Error:        err,
					Duration:     time.Since(start).Truncate(time.Millisecond).String(),
				})
				return
			}

			// Drain stream
			for chunk := range stream {
				if chunk.Error != nil {
					program.Send(tui.TestResultMsg{
						ProviderName: providerName,
						Success:      false,
						Error:        chunk.Error,
						Duration:     time.Since(start).Truncate(time.Millisecond).String(),
					})
					return
				}
			}

			program.Send(tui.TestResultMsg{
				ProviderName: providerName,
				Success:      true,
				Duration:     time.Since(start).Truncate(time.Millisecond).String(),
			})
		}()
	})

	// Set up /plan handler for agent team pipeline
	model.SetPlanHandler(func(request string) {
		go func() {
			program.Send(tui.QueryStartMsg{})

			// Resolve providers for each role
			roleProviders := map[agent.RoleType]string{
				agent.RolePlanner:     cfg.Roles.Planner,
				agent.RoleResearcher:  cfg.Roles.Researcher,
				agent.RoleImplementer: cfg.Roles.Implementer,
				agent.RoleTester:      cfg.Roles.Tester,
				agent.RoleReviewer:    cfg.Roles.Reviewer,
			}

			resolveProvider := func(role agent.RoleType) provider.Provider {
				name := roleProviders[role]
				if name != "" {
					for _, p := range registry.Providers() {
						if p.ID() == name {
							return p
						}
					}
				}
				return primary
			}

			// Build default pipeline: planner → researcher → reviewer
			graph := &agent.TaskGraph{
				JobID: fmt.Sprintf("plan_%d", time.Now().Unix()),
				Stages: []agent.Stage{
					{
						Name: "Planning",
						Workers: []*agent.Worker{{
							Role:         agent.RolePlanner,
							ProviderName: resolveProvider(agent.RolePlanner).ID(),
							Provider:     resolveProvider(agent.RolePlanner),
							SystemPrompt: agent.RolePrompts[agent.RolePlanner],
							MaxTokens:    4096,
						}},
					},
					{
						Name: "Research",
						Workers: []*agent.Worker{{
							Role:         agent.RoleResearcher,
							ProviderName: resolveProvider(agent.RoleResearcher).ID(),
							Provider:     resolveProvider(agent.RoleResearcher),
							SystemPrompt: agent.RolePrompts[agent.RoleResearcher],
							MaxTokens:    4096,
						}},
					},
					{
						Name: "Review",
						Workers: []*agent.Worker{{
							Role:         agent.RoleReviewer,
							ProviderName: resolveProvider(agent.RoleReviewer).ID(),
							Provider:     resolveProvider(agent.RoleReviewer),
							SystemPrompt: agent.RolePrompts[agent.RoleReviewer],
							MaxTokens:    4096,
						}},
					},
				},
			}

			ctx := context.Background()

			result, err := graph.Run(ctx, request, func(sr agent.StageResult) {
				// Update TUI with stage completion
				for role, output := range sr.WorkerOutputs {
					summary := output
					if len(summary) > 100 {
						summary = summary[:97] + "..."
					}
					program.Send(tui.WorkerProgressMsg{
						StageName:    sr.StageName,
						Role:         string(role),
						ProviderName: "", // could resolve but not critical
						Status:       "complete",
						Summary:      summary,
					})
				}
			})

			if err != nil {
				program.Send(tui.PlanDoneMsg{Error: err})
			} else if result != nil && len(result.Stages) > 0 {
				// Use the last stage's output as the final answer
				lastStage := result.Stages[len(result.Stages)-1]
				var finalOutput string
				for _, output := range lastStage.WorkerOutputs {
					finalOutput = output
				}
				program.Send(tui.PlanDoneMsg{FinalOutput: finalOutput})

				// Append to conversation
				conv.append(provider.Message{
					Role:    provider.RoleUser,
					Content: "/plan " + request,
				})
				conv.append(provider.Message{
					Role:    provider.RoleAssistant,
					Content: finalOutput,
				})
			} else {
				program.Send(tui.PlanDoneMsg{Error: fmt.Errorf("plan produced no output")})
			}

			program.Send(tui.QueryDoneMsg{})
		}()
	})

	// Track yolo mode for auto-approve
	yoloEnabled := false
	model.SetYoloToggleHandler(func(enabled bool) {
		yoloEnabled = enabled
	})

	// Set up clear handler to reset conversation state and delete saved session
	model.SetClearHandler(func() {
		conv.mu.Lock()
		conv.messages = []provider.Message{systemPrompt}
		conv.mu.Unlock()
		_ = config.ClearSession()
	})

	model.SetSaveHandler(func() {
		session, _ := config.LoadSession()
		if session != nil {
			_ = config.SaveSession(session)
		}
		program.Send(tui.ConsensusChunkMsg{Delta: "\n*Session saved.*\n", Done: true})
	})

	model.SetExportHandler(func(path string) {
		go func() {
			session, _ := config.LoadSession()
			if session == nil {
				program.Send(tui.ConsensusChunkMsg{Delta: "\n*No session to export.*\n", Done: true})
				return
			}
			if path == "" {
				path = "polycode-export.json"
			}
			if err := config.ExportSession(session, path); err != nil {
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\n*Export failed: %v*\n", err), Done: true})
				return
			}
			program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\n*Session exported to %s*\n", path), Done: true})
		}()
	})

	// Set up the submit handler that bridges TUI → pipeline
	model.SetSubmitHandler(func(prompt string) {
		go func() {
			program.Send(tui.QueryStartMsg{})

			// Append user message to conversation
			conv.append(provider.Message{
				Role:    provider.RoleUser,
				Content: prompt,
			})

			// Get full conversation history for this query
			messages := conv.snapshot()

			tools := action.AllTools()
			opts := provider.QueryOpts{
				MaxTokens: 4096,
				Tools:     tools,
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Consensus.Timeout+30*time.Second)
			defer cancel()

			// Run the fan-out + consensus pipeline with full history
			stream, fanOutResult, err := pipeline.Run(ctx, messages, opts)
			if err != nil {
				program.Send(tui.ConsensusChunkMsg{Error: err, Done: true})
				program.Send(tui.QueryDoneMsg{})
				return
			}

			// Update token tracker and log telemetry for fan-out
			if fanOutResult != nil {
				for id, usage := range fanOutResult.Usage {
					tracker.Add(id, usage)
					if tlog != nil {
						tlog.Log(telemetry.Event{
							ProviderID:   id,
							EventType:    telemetry.EventProviderResponse,
							InputTokens:  usage.InputTokens,
							OutputTokens: usage.OutputTokens,
						})
					}
				}
				for id, provErr := range fanOutResult.Errors {
					if tlog != nil {
						tlog.Log(telemetry.Event{
							ProviderID: id,
							EventType:  telemetry.EventProviderResponse,
							Error:      provErr.Error(),
						})
					}
				}

				// Send individual provider results to TUI
				for id, content := range fanOutResult.Responses {
					program.Send(tui.ProviderChunkMsg{
						ProviderName: id,
						Delta:        content,
						Done:         true,
					})
				}
				for id, err := range fanOutResult.Errors {
					program.Send(tui.ProviderChunkMsg{
						ProviderName: id,
						Error:        err,
					})
				}

				// Notify TUI about skipped providers
				for _, id := range fanOutResult.Skipped {
					program.Send(tui.ProviderChunkMsg{
						ProviderName: id,
						Error:        fmt.Errorf("skipped: context limit exceeded"),
					})
				}
			}

			// Stream consensus output, accumulate response, detect tool calls
			var fullResponse string
			var consensusUsage tokens.Usage
			var pendingToolCalls []provider.ToolCall
			for chunk := range stream {
				if chunk.Error != nil {
					program.Send(tui.ConsensusChunkMsg{Error: chunk.Error})
					break
				}
				if chunk.Done {
					consensusUsage = tokens.Usage{
						InputTokens:  chunk.InputTokens,
						OutputTokens: chunk.OutputTokens,
					}
					pendingToolCalls = chunk.ToolCalls
					if len(pendingToolCalls) == 0 {
						program.Send(tui.ConsensusChunkMsg{Done: true})
					}
					break
				}
				fullResponse += chunk.Delta
				program.Send(tui.ConsensusChunkMsg{Delta: chunk.Delta})
			}

			// Track consensus synthesis usage on the primary
			if consensusUsage.InputTokens > 0 || consensusUsage.OutputTokens > 0 {
				tracker.Add(primary.ID(), consensusUsage)
			}

			// Append the assistant's text response to conversation
			if fullResponse != "" {
				conv.append(provider.Message{
					Role:    provider.RoleAssistant,
					Content: fullResponse,
				})
			}

			// Execute tool calls if the consensus response included them
			if len(pendingToolCalls) > 0 {
				// Build confirmation callback that bridges to TUI
				// In yolo mode, auto-approve all tool actions
				confirmFunc := action.ConfirmFunc(func(description string) bool {
					if yoloEnabled {
						program.Send(tui.ToolCallMsg{
							ToolName:    "yolo",
							Description: "Auto-approved: " + description,
						})
						return true
					}
					responseCh := make(chan bool, 1)
					program.Send(tui.ConfirmActionMsg{
						Description: description,
						ResponseCh:  responseCh,
					})
					select {
					case response := <-responseCh:
						return response
					case <-time.After(5 * time.Minute):
						return false
					case <-ctx.Done():
						return false
					}
				})

				executor := action.NewExecutor(confirmFunc, 120*time.Second)
				toolLoop := action.NewToolLoop(executor, primary)

				// Build synthesis-context messages for the tool loop:
				// system prompt + user prompt + consensus response with tool calls
				toolMsgs := []provider.Message{
					systemPrompt,
					{Role: provider.RoleUser, Content: prompt},
				}
				// Include the consensus text + tool calls as the assistant turn
				if fullResponse != "" || len(pendingToolCalls) > 0 {
					toolMsgs = append(toolMsgs, provider.Message{
						Role:      provider.RoleAssistant,
						Content:   fullResponse,
						ToolCalls: pendingToolCalls,
					})
				}

				// Separate timeout for tool loop
				toolCtx, toolCancel := context.WithTimeout(context.Background(), 5*time.Minute)

				// Stream tool loop output live to TUI
				toolOut := make(chan provider.StreamChunk, 16)
				go func() {
					defer toolCancel()
					if err := toolLoop.Run(toolCtx, toolMsgs, pendingToolCalls, opts, toolOut); err != nil {
						toolOut <- provider.StreamChunk{Error: err}
					}
					close(toolOut)
				}()

				var toolResponse string
				for chunk := range toolOut {
					if chunk.Error != nil {
						program.Send(tui.ConsensusChunkMsg{Error: chunk.Error})
						break
					}
					if chunk.Done {
						break
					}
					// Display all chunks, but only persist model text (not status)
					program.Send(tui.ConsensusChunkMsg{Delta: chunk.Delta})
					if !chunk.Status {
						toolResponse += chunk.Delta
					}
				}
				toolCancel()

				// Append the tool loop's final response
				if toolResponse != "" {
					fullResponse += "\n" + toolResponse
					conv.append(provider.Message{
						Role:    provider.RoleAssistant,
						Content: toolResponse,
					})
				}

				program.Send(tui.ConsensusChunkMsg{Done: true})
			}

			// Send updated token snapshot to TUI
			program.Send(tui.TokenUpdateMsg{Usage: tracker.Summary()})

			// Auto-save session to disk
			go func() {
				snapshot := conv.snapshot()
				var sessionMsgs []config.SessionMessage
				for _, m := range snapshot {
					sessionMsgs = append(sessionMsgs, config.SessionMessage{
						Role:    string(m.Role),
						Content: m.Content,
					})
				}

				individual := make(map[string]string)
				if fanOutResult != nil {
					for id, content := range fanOutResult.Responses {
						individual[id] = content
					}
				}

				session, _ := config.LoadSession()
				if session == nil {
					session = &config.Session{}
				}
				session.Messages = sessionMsgs
				session.Exchanges = append(session.Exchanges, config.SessionExchange{
					Prompt:            prompt,
					ConsensusResponse: fullResponse,
					Individual:        individual,
				})
				_ = config.SaveSession(session)
			}()

			program.Send(tui.QueryDoneMsg{})
		}()
	})

	// Create the Bubble Tea program AFTER all handlers are wired,
	// so the model copy Bubble Tea receives has all callbacks set.
	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Run the TUI
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
