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
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
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

	// Create consensus pipeline with tracker
	pipeline := consensus.NewPipeline(
		healthy,
		primary,
		cfg.Consensus.Timeout,
		cfg.Consensus.MinResponses,
		tracker,
	)

	// Conversation state persists across turns
	conv := &conversationState{
		messages: []provider.Message{
			{
				Role:    provider.RoleSystem,
				Content: "You are polycode, a helpful coding assistant. Provide clear, concise answers to programming questions. When the user asks you to make changes, use the available tools (file_read, file_write, shell_exec) to interact with their codebase.",
			},
		},
	}

	// Create TUI model
	model := tui.NewModel(names, primary.ID(), version)
	model.SetConfig(cfg)

	// Create the Bubble Tea program
	program := tea.NewProgram(model, tea.WithAltScreen())

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

			// Update token tracker with fan-out usage
			if fanOutResult != nil {
				for id, usage := range fanOutResult.Usage {
					tracker.Add(id, usage)
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

			// Stream consensus output and accumulate the full response
			var fullResponse string
			var consensusUsage tokens.Usage
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
					program.Send(tui.ConsensusChunkMsg{Done: true})
					break
				}
				fullResponse += chunk.Delta
				program.Send(tui.ConsensusChunkMsg{Delta: chunk.Delta})
			}

			// Track consensus synthesis usage on the primary
			if consensusUsage.InputTokens > 0 || consensusUsage.OutputTokens > 0 {
				tracker.Add(primary.ID(), consensusUsage)
			}

			// Send updated token snapshot to TUI
			program.Send(tui.TokenUpdateMsg{Usage: tracker.Summary()})

			// Append assistant response to conversation history
			if fullResponse != "" {
				conv.append(provider.Message{
					Role:    provider.RoleAssistant,
					Content: fullResponse,
				})
			}

			program.Send(tui.QueryDoneMsg{})
		}()
	})

	// Run the TUI
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
