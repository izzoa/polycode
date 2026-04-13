package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/agent"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/hooks"
	"github.com/izzoa/polycode/internal/mcp"
	"github.com/izzoa/polycode/internal/memory"
	"github.com/izzoa/polycode/internal/permissions"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/routing"
	"github.com/izzoa/polycode/internal/skill"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/izzoa/polycode/internal/tui"
)

// cancelTracker tracks per-provider cancel functions for active queries.
type cancelTracker struct {
	sync.Mutex
	m map[string]context.CancelFunc
}

// wireConfigChangeHandler sets up the handler that rebuilds the provider
// registry and pipeline when the user modifies the config via the settings UI.
func wireConfigChangeHandler(
	model *tui.Model,
	programRef **tea.Program,
	state *atomic.Pointer[appState],
	mcpH *mcpHolder,
	metadataStore *tokens.MetadataStore,
	workDir string,
) {
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
		newTracker := tokens.NewTracker(newProviderModels, newProviderLimits)
		// Re-wire cost tracking (was missing — caused cost display to break after config changes)
		if metadataStore != nil {
			newTracker.SetCostFunc(func(model, providerType string, inputTokens, outputTokens int) float64 {
				return metadataStore.CostForTokens(model, providerType, inputTokens, outputTokens)
			})
		}
		for _, pc := range newCfg.Providers {
			newTracker.SetProviderType(pc.Name, string(pc.Type))
		}

		// Rebuild hooks and permissions from new config
		newHookMgr := hooks.NewHookManager(newCfg.Hooks)
		newPolicyMgr := state.Load().policyMgr // keep old as fallback
		if pm, err := permissions.LoadPolicies(workDir); err == nil {
			newPolicyMgr = pm
		}

		state.Store(&appState{
			tracker:   newTracker,
			registry:  newRegistry,
			healthy:   newHealthy,
			primary:   newPrimary,
			cfg:       newCfg,
			hookMgr:   newHookMgr,
			policyMgr: newPolicyMgr,
		})

		program := *programRef

		// Reconfigure MCP client if servers changed
		if client := mcpH.get(); client != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := client.Reconfigure(ctx, newCfg.MCP.Servers); err != nil {
					log.Printf("Warning: MCP reconfigure: %v", err)
				}
				sendMCPStatus(program, client)
			}()
		} else if len(newCfg.MCP.Servers) > 0 {
			go func() {
				newMCP := mcp.NewMCPClient(newCfg.MCP.Servers, newCfg.MCP.Debug)
				wireMCPNotifications(program, newMCP)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := newMCP.Connect(ctx); err != nil {
					log.Printf("Warning: MCP: %v", err)
				}
				cancel()
				if len(newMCP.Tools()) > 0 || len(newMCP.Resources()) > 0 {
					mcpH.set(newMCP)
				} else {
					newMCP.Close()
				}
				sendMCPStatus(program, mcpH.get())
			}()
		}
	})
}

// wireGeneralHandlers sets up non-query, non-MCP, non-session handler closures:
// test provider, plan, skill, mode change, memory, undo, redo, yolo,
// shell context, cancel, clear, save, export, share.
func wireGeneralHandlers(
	model *tui.Model,
	programRef **tea.Program,
	state *atomic.Pointer[appState],
	conv *conversationState,
	mcpH *mcpHolder,
	setMode func(routing.Mode),
	skillReg *skill.Registry,
	memStore *memory.MemoryStore,
	workDir string,
	systemPrompt provider.Message,
	yoloEnabled *atomic.Bool,
	isGitRepo bool,
	cancelFuncs *cancelTracker,
) {
	// Set up test provider handler
	model.SetTestProviderHandler(func(providerName string) {
		go func() {
			program := *programRef
			s := state.Load()
			start := time.Now()
			var testProvider provider.Provider
			for _, p := range s.registry.Providers() {
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
			program := *programRef
			s := state.Load()
			program.Send(tui.QueryStartMsg{})

			// Resolve providers for each role
			roleProviders := map[agent.RoleType]string{
				agent.RolePlanner:     s.cfg.Roles.Planner,
				agent.RoleResearcher:  s.cfg.Roles.Researcher,
				agent.RoleImplementer: s.cfg.Roles.Implementer,
				agent.RoleTester:      s.cfg.Roles.Tester,
				agent.RoleReviewer:    s.cfg.Roles.Reviewer,
			}

			resolveProvider := func(role agent.RoleType) provider.Provider {
				name := roleProviders[role]
				if name != "" {
					for _, p := range s.registry.Providers() {
						if p.ID() == name {
							return p
						}
					}
				}
				return s.primary
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
			if mc := mcpH.get(); mc != nil {
				program.Send(tui.MCPCallCountMsg{Count: mc.CallCount()})
			}
		}()
	})

	// Wire /mode handler — updates the mode; provider selection happens per query
	model.SetModeChangeHandler(func(mode string) {
		m, ok := routing.ParseMode(mode)
		if !ok {
			return
		}
		setMode(m)
		(*programRef).Send(tui.ModeChangedMsg{Mode: mode})
	})

	// Wire /skill handler
	model.SetSkillHandler(func(subcommand, args string) {
		go func() {
			program := *programRef
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

	// Wire /memory handler
	model.SetMemoryHandler(func(args string) {
		go func() {
			program := *programRef
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

	model.SetUndoHandler(func() {
		go func() {
			program := *programRef
			if !isGitRepo {
				program.Send(tui.UndoAppliedMsg{Error: fmt.Errorf("not a git repository")})
				return
			}
			// Restore files to state before the last mutating tool call.
			// Uses git checkout to restore all tracked files from the snapshot commit.
			// First: check if there's a polycode undo tag to restore from.
			cmd := exec.Command("git", "diff", "--quiet")
			cmd.Dir = workDir
			if cmd.Run() == nil {
				// Working tree is clean — nothing to undo
				program.Send(tui.UndoAppliedMsg{Error: fmt.Errorf("working tree is clean, nothing to undo")})
				return
			}
			// Revert all tracked file changes
			cmd = exec.Command("git", "checkout", "--", ".")
			cmd.Dir = workDir
			if err := cmd.Run(); err != nil {
				program.Send(tui.UndoAppliedMsg{Error: fmt.Errorf("git checkout: %w", err)})
				return
			}
			program.Send(tui.UndoAppliedMsg{Description: "file changes reverted"})
		}()
	})

	model.SetRedoHandler(func() {
		go func() {
			program := *programRef
			program.Send(tui.UndoAppliedMsg{
				Error:  fmt.Errorf("redo not yet supported"),
				IsRedo: true,
			})
		}()
	})

	model.SetYoloToggleHandler(func(enabled bool) {
		yoloEnabled.Store(enabled)
	})

	// Set up shell context injection handler (! prefix)
	model.SetShellContextHandler(func(command string) {
		go func() {
			program := *programRef
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			cmd.Dir = workDir
			out, err := cmd.CombinedOutput()
			// Cap output to 50KB to avoid blowing up context
			const maxOutput = 50 * 1024
			output := string(out)
			if len(output) > maxOutput {
				output = output[:maxOutput] + "\n... (output truncated at 50KB)"
			}
			program.Send(tui.ShellContextMsg{
				Command: command,
				Output:  output,
				Error:   err,
			})
		}()
	})

	// Per-provider cancel support
	model.SetCancelProviderHandler(func(providerName string) {
		cancelFuncs.Lock()
		defer cancelFuncs.Unlock()
		if fn, ok := cancelFuncs.m[providerName]; ok {
			fn()
			delete(cancelFuncs.m, providerName)
		}
	})

	// Set up clear handler to reset conversation state and delete saved session
	model.SetClearHandler(func() {
		conv.mu.Lock()
		conv.messages = []provider.Message{systemPrompt}
		conv.mu.Unlock()
		// Reset token tracker for all providers so context checks start fresh
		state.Load().tracker.Reset()
		// NOTE: Do NOT call program.Send() here — this runs synchronously inside
		// Update(), so Send() would deadlock the Bubble Tea event loop.
		// Token display is cleared by the TUI's /clear handler directly.
		_ = config.ClearSession()
	})

	model.SetSaveHandler(func() {
		program := *programRef
		session, _ := config.LoadSession()
		if session != nil {
			if err := config.SaveSession(session); err != nil {
				log.Printf("Warning: failed to save session: %v", err)
			}
		}
		program.Send(tui.ConsensusChunkMsg{Delta: "\n*Session saved.*\n", Done: true})
	})

	model.SetExportMarkdownHandler(func() {
		go func() {
			program := *programRef
			session, _ := config.LoadSession()
			if session == nil {
				program.Send(tui.ConsensusChunkMsg{Delta: "\n*No session to export.*\n", Done: true})
				return
			}
			md := config.ExportSessionMarkdown(session)
			path := "polycode-export.md"
			if err := os.WriteFile(path, []byte(md), 0600); err != nil {
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\n*Export failed: %v*\n", err), Done: true})
				return
			}
			program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\n*Session exported to %s*\n", path), Done: true})
		}()
	})

	model.SetExportHandler(func(path string) {
		go func() {
			program := *programRef
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
}
