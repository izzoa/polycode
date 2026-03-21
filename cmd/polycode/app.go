package main

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/action"
	"github.com/izzoa/polycode/internal/agent"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/hooks"
	"github.com/izzoa/polycode/internal/mcp"
	"github.com/izzoa/polycode/internal/memory"
	"github.com/izzoa/polycode/internal/permissions"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/routing"
	"github.com/izzoa/polycode/internal/skill"
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

	// Wire cost estimation from litellm pricing data
	if metadataStore != nil {
		tracker.SetCostFunc(func(model, providerType string, inputTokens, outputTokens int) float64 {
			return metadataStore.CostForTokens(model, providerType, inputTokens, outputTokens)
		})
	}
	for _, pc := range cfg.Providers {
		tracker.SetProviderType(pc.Name, string(pc.Type))
	}

	// Create telemetry logger
	tlog, err := telemetry.NewLogger()
	if err != nil {
		log.Printf("Warning: telemetry disabled: %v", err)
	}
	if tlog != nil {
		defer tlog.Close()
	}

	// Working directory for repo-level config (instructions, permissions)
	workDir, _ := os.Getwd()

	// Create hook manager
	hookMgr := hooks.NewHookManager(cfg.Hooks)

	// Load permission policies (repo-level overrides user-level)
	policyMgr, err := permissions.LoadPolicies(workDir)
	if err != nil {
		log.Printf("Warning: failed to load permission policies: %v", err)
		// Create an empty policy manager so Check() returns PolicyAsk for everything
		policyMgr, _ = permissions.LoadPolicies("")
	}

	// Create adaptive router
	telemetryPath := filepath.Join(config.ConfigDir(), "telemetry.jsonl")
	router := routing.NewRouter(telemetryPath)
	if err := router.LoadTelemetryStats(); err != nil {
		log.Printf("Warning: failed to load telemetry stats for router: %v", err)
	}

	// Parse initial operating mode
	currentMode := routing.ModeBalanced
	if m, ok := routing.ParseMode(cfg.DefaultMode); ok {
		currentMode = m
	}

	// Select initial providers based on mode
	routed := router.SelectProviders(currentMode, healthy, primary.ID())
	if len(routed) == 0 {
		routed = healthy // fallback
	}

	// Create repo memory store
	memDir := filepath.Join(config.ConfigDir(), "memory")
	memStore := memory.NewMemoryStore(memDir)

	// Build system prompt from instruction hierarchy + repo memory
	instructions := memory.LoadInstructions(workDir)
	memPrompt := memStore.FormatForPrompt()
	systemContent := instructions
	if memPrompt != "" {
		systemContent += "\n\n" + memPrompt
	}

	// Connect to MCP servers and discover tools
	var mcpClient *mcp.MCPClient
	if len(cfg.MCP.Servers) > 0 {
		mcpClient = mcp.NewMCPClient(cfg.MCP.Servers)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := mcpClient.Connect(ctx); err != nil {
			log.Printf("Warning: MCP connection failed: %v", err)
			mcpClient = nil
		}
		cancel()
	}

	// Load installed skills
	skillsDir := filepath.Join(config.ConfigDir(), "skills")
	skillReg := skill.NewRegistry(skillsDir)
	if err := skillReg.Load(); err != nil {
		log.Printf("Warning: failed to load skills: %v", err)
	}

	// Append skill system prompts to system content
	if skillPrompts := skillReg.SystemPrompts(); skillPrompts != "" {
		systemContent += "\n\n" + skillPrompts
	}

	// Note: no static pipeline variable — providers are selected per query
	// by the router in the submit handler. Mode/config changes update the
	// router inputs (healthy, primary, cfg) which take effect on the next query.

	// System prompt built from instruction hierarchy + repo memory
	systemPrompt := provider.Message{
		Role:    provider.RoleSystem,
		Content: systemContent,
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
		// Restore conversation messages with full tool call data
		restored := fromSessionMessages(savedSession.Messages)
		conv.mu.Lock()
		conv.messages = []provider.Message{systemPrompt}
		for _, m := range restored {
			if m.Role == provider.RoleSystem {
				continue // skip saved system prompt, we use the current one
			}
			conv.messages = append(conv.messages, m)
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

		// Rebuild tracker; provider selection happens per query via router
		tracker = tokens.NewTracker(newProviderModels, newProviderLimits)
		registry = newRegistry
		healthy = newHealthy
		primary = newPrimary
		cfg = newCfg

		// Rebuild hooks and permissions from new config
		hookMgr = hooks.NewHookManager(newCfg.Hooks)
		if newPolicyMgr, err := permissions.LoadPolicies(workDir); err == nil {
			policyMgr = newPolicyMgr
		}
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

	// Wire /mode handler — updates the mode; provider selection happens per query
	model.SetModeChangeHandler(func(mode string) {
		m, ok := routing.ParseMode(mode)
		if !ok {
			return
		}
		currentMode = m
		program.Send(tui.ModeChangedMsg{Mode: mode})
	})

	// Wire /skill handler
	model.SetSkillHandler(func(subcommand, args string) {
		go func() {
			switch subcommand {
			case "", "list":
				program.Send(tui.ConsensusChunkMsg{Delta: "\n" + skillReg.FormatList() + "\n", Done: true})
			case "install":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /skill install <path>\n", Done: true})
					return
				}
				if err := skillReg.Install(args); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nInstall failed: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: "\nSkill installed successfully.\n", Done: true})
			case "remove":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /skill remove <name>\n", Done: true})
					return
				}
				if err := skillReg.Remove(args); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nRemove failed: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: "\nSkill removed.\n", Done: true})
			default:
				program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /skill [list|install <path>|remove <name>]\n", Done: true})
			}
		}()
	})

	// Wire /sessions handler
	model.SetSessionsHandler(func(subcommand, args string) {
		go func() {
			switch subcommand {
			case "", "list":
				sessions, err := config.ListSessions()
				if err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				if len(sessions) == 0 {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo saved sessions.\n", Done: true})
					return
				}
				var sb strings.Builder
				sb.WriteString("\nSaved sessions:\n\n")
				for _, s := range sessions {
					current := ""
					if s.IsCurrent {
						current = " ← current"
					}
					fmt.Fprintf(&sb, "  %-20s  %d exchanges  %s%s\n",
						s.Name, s.Exchanges,
						s.UpdatedAt.Format("Jan 02 15:04"), current)
				}
				sb.WriteString("\nUse /name <name> to name the current session.\n")
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})
			case "name":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /name <session-name>\n", Done: true})
					return
				}
				session, _ := config.LoadSession()
				if session == nil {
					session = &config.Session{}
				}
				session.Name = args
				if err := config.SaveSession(session); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nSession named %q.\n", args), Done: true})
			case "delete":
				if args == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /sessions delete <name>\n", Done: true})
					return
				}
				if err := config.DeleteSessionByName(args); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nError: %v\n", err), Done: true})
					return
				}
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nSession %q deleted.\n", args), Done: true})
			default:
				program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /sessions [list|delete <name>]\n       /name <session-name>\n", Done: true})
			}
		}()
	})

	// Wire /memory handler
	model.SetMemoryHandler(func(args string) {
		go func() {
			if args == "" {
				// Show all memory
				content := memStore.FormatForPrompt()
				if content == "" {
					content = "No project memory stored yet."
				}
				program.Send(tui.ConsensusChunkMsg{Delta: "\n" + content + "\n", Done: true})
				return
			}
			// Save memory: /memory <name> <content> or just show one entry
			program.Send(tui.ConsensusChunkMsg{Delta: "\n" + memStore.FormatForPrompt() + "\n", Done: true})
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
			if err := config.SaveSession(session); err != nil {
				log.Printf("Warning: failed to save session: %v", err)
			}
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
			// Fire pre-query hook
			hookMgr.Run(hooks.PreQuery, hooks.HookContext{Prompt: prompt})

			// Append user message to conversation
			conv.append(provider.Message{
				Role:    provider.RoleUser,
				Content: prompt,
			})

			// Get full conversation history for this query
			messages := conv.snapshot()

			// Context auto-summarization: if the primary model is nearing
			// its context limit (~80%), compress early conversation turns
			// into a dense summary to free tokens.
			primaryUsage := tracker.Get(primary.ID())
			if primaryUsage.Limit > 0 && len(messages) > 4 {
				usagePct := float64(primaryUsage.InputTokens) / float64(primaryUsage.Limit) * 100
				if usagePct >= 80 {
					messages = summarizeConversation(messages)
					// Update conversation state with compressed version
					conv.mu.Lock()
					conv.messages = messages
					conv.mu.Unlock()
				}
			}

			// Merge built-in tools with MCP-discovered and skill-provided tools
			tools := action.AllTools()
			if mcpClient != nil {
				tools = append(tools, mcpClient.ToToolDefinitions()...)
			}
			tools = append(tools, skillReg.ToToolDefinitions()...)
			// Set reasoning effort based on mode
			var reasoningEffort provider.ReasoningEffort
			switch currentMode {
			case routing.ModeQuick:
				reasoningEffort = provider.ReasoningLow
			case routing.ModeBalanced:
				reasoningEffort = provider.ReasoningMedium
			case routing.ModeThorough:
				reasoningEffort = provider.ReasoningHigh
			}

			opts := provider.QueryOpts{
				MaxTokens:       4096,
				Tools:           tools,
				ReasoningEffort: reasoningEffort,
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Consensus.Timeout+30*time.Second)
			defer cancel()

			// Re-select providers per query (adaptive routing)
			queryProviders, routingReason := router.SelectProvidersWithReason(currentMode, healthy, primary.ID())
			if len(queryProviders) == 0 {
				queryProviders = healthy
				routingReason = "fallback: using all healthy providers"
			}

			// Tell the TUI which providers are being queried and why
			var queriedNames []string
			for _, p := range queryProviders {
				queriedNames = append(queriedNames, p.ID())
			}
			program.Send(tui.QueryStartMsg{QueriedProviders: queriedNames, RoutingReason: routingReason})

			// Map routing mode to synthesis depth
			synthesisMode := consensus.SynthesisBalanced
			switch currentMode {
			case routing.ModeQuick:
				synthesisMode = consensus.SynthesisQuick
			case routing.ModeThorough:
				synthesisMode = consensus.SynthesisThorough
			}

			queryPipeline := consensus.NewPipeline(
				queryProviders,
				primary,
				cfg.Consensus.Timeout,
				cfg.Consensus.MinResponses,
				tracker,
				synthesisMode,
			)

			// Run the fan-out + consensus pipeline with full history
			stream, fanOutResult, err := queryPipeline.Run(ctx, messages, opts)
			if err != nil {
				// Even on pipeline failure, show individual provider errors
				// so the user can see what went wrong per provider.
				if fanOutResult != nil {
					for id, provErr := range fanOutResult.Errors {
						program.Send(tui.ProviderChunkMsg{
							ProviderName: id,
							Error:        provErr,
						})
					}
				}
				hookMgr.Run(hooks.OnError, hooks.HookContext{Prompt: prompt, Error: err.Error()})
				program.Send(tui.ConsensusChunkMsg{Error: err, Done: true})
				program.Send(tui.QueryDoneMsg{})
				return
			}

			// Update token tracker and log telemetry for fan-out
			if fanOutResult != nil {
				for id, usage := range fanOutResult.Usage {
					tracker.Add(id, usage)
					if tlog != nil {
						success := true
						latencyMS := fanOutResult.Latencies[id].Milliseconds()
						tlog.Log(telemetry.Event{
							ProviderID:   id,
							EventType:    telemetry.EventProviderResponse,
							LatencyMS:    latencyMS,
							InputTokens:  usage.InputTokens,
							OutputTokens: usage.OutputTokens,
							Success:      &success,
						})
					}
				}
				for id, provErr := range fanOutResult.Errors {
					if tlog != nil {
						fail := false
						latencyMS := fanOutResult.Latencies[id].Milliseconds()
						tlog.Log(telemetry.Event{
							ProviderID: id,
							EventType:  telemetry.EventProviderResponse,
							LatencyMS:  latencyMS,
							Error:      provErr.Error(),
							Success:    &fail,
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

			// Parse and surface consensus provenance data to TUI
			if fullResponse != "" {
				analysis := consensus.ParseConsensusAnalysis(fullResponse)
				if analysis != nil {
					var minorities []string
					for _, mr := range analysis.MinorityReports {
						entry := mr.Position
						if mr.ProviderID != "" {
							entry = "[" + mr.ProviderID + "] " + entry
						}
						minorities = append(minorities, entry)
					}
					program.Send(tui.ConsensusAnalysisMsg{
						Confidence: analysis.Confidence,
						Agreements: analysis.Agreements,
						Minorities: minorities,
						Evidence:   analysis.Evidence,
					})
				}
			}

			// NOTE: Don't append fullResponse to conv yet — if tool execution
			// follows, we want to combine initial text + tool output into one
			// assistant message so future turns have full context.

			// Execute tool calls if the consensus response included them
			if len(pendingToolCalls) > 0 {
				// Build confirmation callback that consults permission
				// policies, then falls back to TUI confirmation or yolo mode.
				// The executor passes the actual tool name for each call.
				confirmFunc := action.ConfirmFunc(func(toolName, description string) bool {
					// Check permission policy for this specific tool
					policy := policyMgr.Check(toolName)
					switch policy {
					case permissions.PolicyAllow:
						program.Send(tui.ToolCallMsg{
							ToolName:    toolName,
							Description: "Policy-approved: " + description,
						})
						return true
					case permissions.PolicyDeny:
						program.Send(tui.ToolCallMsg{
							ToolName:    toolName,
							Description: "Policy-denied: " + description,
						})
						return false
					}

					// PolicyAsk — check yolo mode, then prompt user
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

					var accepted bool
					select {
					case accepted = <-responseCh:
					case <-time.After(5 * time.Minute):
						accepted = false
					case <-ctx.Done():
						accepted = false
					}

					// Log user feedback for router calibration
					if tlog != nil {
						a := accepted
						tlog.Log(telemetry.Event{
							ProviderID: primary.ID(),
							EventType:  telemetry.EventUserFeedback,
							ToolName:   toolName,
							Accepted:   &a,
						})
					}
					return accepted
				})

				executor := action.NewExecutor(confirmFunc, 120*time.Second)

				// Register external tool handlers for MCP and skill tools
				executor.SetExternalHandler(func(call provider.ToolCall) (string, error) {
					// MCP tool names are "mcp_{server}_{tool}"
					if mcpClient != nil && len(call.Name) > 4 && call.Name[:4] == "mcp_" {
						rest := call.Name[4:]
						for i, ch := range rest {
							if ch == '_' {
								serverName := rest[:i]
								toolName := rest[i+1:]
								return mcpClient.CallTool(ctx, serverName, toolName, []byte(call.Arguments))
							}
						}
					}
					// Skill tool names are "skill_{name}_{tool}"
					if len(call.Name) > 6 && call.Name[:6] == "skill_" {
						return skillReg.ExecuteTool(ctx, call.Name, call.Arguments)
					}
					return "", fmt.Errorf("unknown tool: %s", call.Name)
				})

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
				var toolLoopOK bool
				var wroteFiles bool
				for chunk := range toolOut {
					if chunk.Error != nil {
						program.Send(tui.ConsensusChunkMsg{Error: chunk.Error})
						break
					}
					if chunk.Done {
						toolLoopOK = true
						break
					}
					// Track whether any file_write was executed
					if chunk.Status && strings.Contains(chunk.Delta, "file_write") {
						wroteFiles = true
					}
					// Display all chunks, but only persist model text (not status)
					program.Send(tui.ConsensusChunkMsg{Delta: chunk.Delta})
					if !chunk.Status {
						toolResponse += chunk.Delta
					}
				}
				toolCancel()

				// Combine initial consensus text + tool follow-up
				if toolResponse != "" {
					fullResponse += "\n" + toolResponse
				}

				// Run verification only if the tool loop completed successfully
				// and files were actually written.
				if toolLoopOK && wroteFiles {
					verifyCmd := cfg.Consensus.VerifyCommand
					if verifyCmd == "" {
						verifyCmd = action.DetectVerifyCommand(workDir)
					}
					if verifyCmd != "" {
						program.Send(tui.ConsensusChunkMsg{
							Delta: fmt.Sprintf("\nRunning verification: `%s`...\n", verifyCmd),
						})
						verifyOut, verifyOK, verifyErr := action.RunVerification(
							context.Background(), verifyCmd, workDir, 2*time.Minute,
						)
						if verifyErr != nil {
							program.Send(tui.ConsensusChunkMsg{
								Delta: fmt.Sprintf("\nVerification error: %v\n", verifyErr),
							})
						} else if !verifyOK {
							display := verifyOut
							if len(display) > 1000 {
								display = display[:1000] + "\n... (truncated)"
							}
							program.Send(tui.ConsensusChunkMsg{
								Delta: fmt.Sprintf("\nVerification failed:\n```\n%s\n```\n", display),
							})
						} else {
							program.Send(tui.ConsensusChunkMsg{
								Delta: "\nVerification passed.\n",
							})
						}
					}
				}

				// Fire post-tool hook
				hookMgr.Run(hooks.PostTool, hooks.HookContext{
					Prompt:   prompt,
					Response: toolResponse,
				})

				program.Send(tui.ConsensusChunkMsg{Done: true})
			}

			// Append the complete assistant response (initial consensus + tool output)
			// as a single message so future turns have full context
			if fullResponse != "" {
				conv.append(provider.Message{
					Role:    provider.RoleAssistant,
					Content: fullResponse,
				})
			}

			// Fire post-query hook
			hookMgr.Run(hooks.PostQuery, hooks.HookContext{
				Prompt:   prompt,
				Response: fullResponse,
			})

			// Send updated token snapshot to TUI
			program.Send(tui.TokenUpdateMsg{Usage: tracker.Summary()})

			// Auto-save session to disk
			go func() {
				snapshot := conv.snapshot()
				sessionMsgs := toSessionMessages(snapshot)

				individual := make(map[string]string)
				if fanOutResult != nil {
					maps.Copy(individual, fanOutResult.Responses)
				}

				session, _ := config.LoadSession()
				if session == nil {
					session = &config.Session{}
				}
				session.Messages = sessionMsgs

				// Build consensus trace for replay
				var trace *config.ConsensusTrace
				if fanOutResult != nil {
					trace = &config.ConsensusTrace{
						RoutingMode:    string(currentMode),
						RoutingReason:  routingReason,
						SynthesisModel: primary.ID(),
					}
					for _, p := range queryProviders {
						trace.Providers = append(trace.Providers, p.ID())
					}
					if len(fanOutResult.Latencies) > 0 {
						trace.Latencies = make(map[string]int64)
						for id, d := range fanOutResult.Latencies {
							trace.Latencies[id] = d.Milliseconds()
						}
					}
					if len(fanOutResult.Usage) > 0 {
						trace.TokenUsage = make(map[string][2]int)
						for id, u := range fanOutResult.Usage {
							trace.TokenUsage[id] = [2]int{u.InputTokens, u.OutputTokens}
						}
					}
					if len(fanOutResult.Errors) > 0 {
						trace.Errors = make(map[string]string)
						for id, err := range fanOutResult.Errors {
							trace.Errors[id] = err.Error()
						}
					}
					trace.Skipped = fanOutResult.Skipped
				}

				session.Exchanges = append(session.Exchanges, config.SessionExchange{
					Prompt:            prompt,
					ConsensusResponse: fullResponse,
					Individual:        individual,
					Trace:             trace,
				})
				if err := config.SaveSession(session); err != nil {
					log.Printf("Warning: auto-save session failed: %v", err)
				}
			}()

			program.Send(tui.QueryDoneMsg{})
		}()
	})

	// Create the Bubble Tea program AFTER all handlers are wired,
	// so the model copy Bubble Tea receives has all callbacks set.
	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Run the TUI
	if _, err := program.Run(); err != nil {
		if mcpClient != nil {
			mcpClient.Close()
		}
		return fmt.Errorf("TUI error: %w", err)
	}

	// Clean up MCP connections
	if mcpClient != nil {
		mcpClient.Close()
	}

	return nil
}

// toSessionMessages converts provider messages to the serializable session
// format, preserving tool call data and tool result metadata.
func toSessionMessages(msgs []provider.Message) []config.SessionMessage {
	out := make([]config.SessionMessage, 0, len(msgs))
	for _, m := range msgs {
		sm := config.SessionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
		// Preserve tool calls on assistant messages
		for _, tc := range m.ToolCalls {
			sm.ToolCalls = append(sm.ToolCalls, config.ToolCallRecord{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		// Preserve tool result metadata on tool role messages
		if m.ToolCallID != "" {
			sm.ToolResult = &config.ToolResultRecord{
				ToolCallID: m.ToolCallID,
				Output:     m.Content,
			}
		}
		out = append(out, sm)
	}
	return out
}

// summarizeConversation compresses early conversation turns into a dense
// summary, preserving the system prompt and the most recent turns. This
// frees context tokens when the session is approaching the model's limit.
func summarizeConversation(messages []provider.Message) []provider.Message {
	if len(messages) <= 4 {
		return messages
	}

	// Keep the system prompt (first message) and the last 4 messages intact.
	// Compress everything in between into a summary.
	var systemMsg provider.Message
	startIdx := 0
	if len(messages) > 0 && messages[0].Role == provider.RoleSystem {
		systemMsg = messages[0]
		startIdx = 1
	}

	keepRecent := 4
	if len(messages)-startIdx <= keepRecent {
		return messages
	}

	cutoff := len(messages) - keepRecent
	middle := messages[startIdx:cutoff]

	// Build a compressed summary of the middle conversation turns.
	var summary strings.Builder
	summary.WriteString("[Conversation summary — earlier exchanges compressed to save context]\n\n")
	exchangeCount := 0
	for _, m := range middle {
		if m.Role == provider.RoleUser {
			exchangeCount++
			content := m.Content
			if len(content) > 100 {
				content = content[:97] + "..."
			}
			fmt.Fprintf(&summary, "- User asked: %s\n", content)
		} else if m.Role == provider.RoleAssistant {
			content := m.Content
			if len(content) > 150 {
				content = content[:147] + "..."
			}
			fmt.Fprintf(&summary, "  Response: %s\n", content)
		}
		// Skip tool messages in summary — the key context is in user/assistant turns.
	}
	fmt.Fprintf(&summary, "\n[%d earlier exchanges summarized]\n", exchangeCount)

	// Reassemble: system + summary + recent messages
	result := make([]provider.Message, 0, keepRecent+2)
	if systemMsg.Role != "" {
		result = append(result, systemMsg)
	}
	result = append(result, provider.Message{
		Role:    provider.RoleUser,
		Content: summary.String(),
	})
	result = append(result, provider.Message{
		Role:    provider.RoleAssistant,
		Content: "Understood. I have the context from our earlier conversation.",
	})
	result = append(result, messages[cutoff:]...)

	return result
}

// fromSessionMessages converts serialized session messages back to provider
// messages, restoring tool call data and tool result metadata.
func fromSessionMessages(msgs []config.SessionMessage) []provider.Message {
	out := make([]provider.Message, 0, len(msgs))
	for _, sm := range msgs {
		m := provider.Message{
			Role:    provider.Role(sm.Role),
			Content: sm.Content,
		}
		// Restore tool calls
		for _, tc := range sm.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, provider.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		// Restore tool result ID and content from ToolResult
		if sm.ToolResult != nil {
			m.ToolCallID = sm.ToolResult.ToolCallID
			// Prefer ToolResult.Output if Content is empty (older sessions)
			if m.Content == "" && sm.ToolResult.Output != "" {
				m.Content = sm.ToolResult.Output
			}
		}
		out = append(out, m)
	}
	return out
}
