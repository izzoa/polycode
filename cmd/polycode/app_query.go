package main

import (
	"context"
	"fmt"
	"log"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/action"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/hooks"
	"github.com/izzoa/polycode/internal/permissions"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/routing"
	"github.com/izzoa/polycode/internal/skill"
	"github.com/izzoa/polycode/internal/telemetry"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/izzoa/polycode/internal/tui"
)

// wireQueryHandler sets up the submit handler that bridges TUI → pipeline.
// This is the core query execution flow: fan-out, consensus, tool loop.
func wireQueryHandler(
	model *tui.Model,
	programRef **tea.Program,
	state *atomic.Pointer[appState],
	conv *conversationState,
	mcpH *mcpHolder,
	router *routing.Router,
	getMode func() routing.Mode,
	skillReg *skill.Registry,
	metadataStore *tokens.MetadataStore,
	tlog *telemetry.Logger,
	systemPrompt provider.Message,
	workDir string,
	yoloEnabled *atomic.Bool,
	createUndoSnapshot func(string),
	cancelFuncs *cancelTracker,
) {
	model.SetSubmitHandler(func(prompt string) {
		go func() {
			program := *programRef
			s := state.Load()
			// Snapshot mode at query start for consistent use throughout.
			queryMode := getMode()

			// Fire pre-query hook
			s.hookMgr.Run(hooks.PreQuery, hooks.HookContext{Prompt: prompt})

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
			primaryUsage := s.tracker.Get(s.primary.ID())
			if primaryUsage.Limit > 0 && len(messages) > 4 {
				usagePct := float64(primaryUsage.LastInputTokens) / float64(primaryUsage.Limit) * 100
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
			if mc := mcpH.get(); mc != nil {
				tools = append(tools, mc.ToToolDefinitions()...)
			}
			tools = append(tools, skillReg.ToToolDefinitions()...)
			// Set reasoning effort based on mode
			var reasoningEffort provider.ReasoningEffort
			switch queryMode {
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

			// No timeout — providers run until they complete naturally.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Re-select providers per query (adaptive routing)
			queryProviders, routingReason := router.SelectProvidersWithReason(queryMode, s.healthy, s.primary.ID())
			if len(queryProviders) == 0 {
				queryProviders = s.healthy
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
			switch queryMode {
			case routing.ModeQuick:
				synthesisMode = consensus.SynthesisQuick
			case routing.ModeThorough:
				synthesisMode = consensus.SynthesisThorough
			}

			queryPipeline := consensus.NewPipeline(
				queryProviders,
				s.primary,
				s.cfg.Consensus.Timeout,
				s.cfg.Consensus.MinResponses,
				s.tracker,
				synthesisMode,
			)

			// Allow read-only built-in tools + MCP + skill tools during fan-out
			// so providers can inspect the codebase and use external tools.
			// Write/exec built-in tools remain synthesis-only.
			// Only send tools to providers whose model supports structured tool
			// calling according to litellm metadata.
			toolCapable := make(map[string]bool)
			for _, pc := range s.cfg.Providers {
				if metadataStore != nil {
					_, found := metadataStore.Lookup(pc.Model, string(pc.Type))
					if found {
						// Model is in litellm — trust its capability flag.
						toolCapable[pc.Name] = metadataStore.SupportsToolCalling(pc.Model, string(pc.Type))
					} else {
						// Model not in litellm — fall back to type-based default.
						switch pc.Type {
						case "anthropic", "openai", "google", "openai_compatible":
							toolCapable[pc.Name] = true
						}
					}
				} else {
					// No metadata store — fall back to type-based default.
					switch pc.Type {
					case "anthropic", "openai", "google", "openai_compatible":
						toolCapable[pc.Name] = true
					}
				}
			}

			// Fan-out only gets read-only built-in tools (file_read).
			// Include built-in read-only tools plus read-only MCP tools in fan-out.
			fanOutTools := action.ReadOnlyTools()
			if mc := mcpH.get(); mc != nil {
				fanOutTools = append(fanOutTools, mc.ReadOnlyToolDefinitions()...)
			}

			// Build allowed tool name set for fan-out safety filtering.
			allowedFanOutTools := make(map[string]bool, len(fanOutTools))
			for _, t := range fanOutTools {
				allowedFanOutTools[t.Name] = true
			}

			fanOutExecutor := action.NewExecutor(nil, 30*time.Second)
			// Register MCP handler for read-only MCP tools in fan-out.
			if mc := mcpH.get(); mc != nil {
				fanOutExecutor.SetExternalHandler(func(call provider.ToolCall) (string, error) {
					if len(call.Name) > 4 && call.Name[:4] == "mcp_" {
						if serverName, toolName, ok := mc.ResolveToolCall(call.Name); ok {
							return mc.CallTool(ctx, serverName, toolName, []byte(call.Arguments))
						}
					}
					return "", fmt.Errorf("unknown tool: %s", call.Name)
				})
			}
			queryPipeline.SetFanOutTools(
				fanOutTools,
				func(call provider.ToolCall) (string, error) {
					// Reject tools that weren't offered — providers can hallucinate
					// tool names like shell_exec even when only file_read was given.
					if !allowedFanOutTools[call.Name] {
						return "", fmt.Errorf("tool %q not available during fan-out", call.Name)
					}
					result := fanOutExecutor.Execute(call)
					if result.Error != nil {
						return result.Output, result.Error
					}
					return result.Output, nil
				},
				toolCapable,
			)

			// Accumulate fan-out text per provider from the live (untruncated) stream.
			// The chunk callback fires from concurrent goroutines, so this needs a mutex.
			var fanoutMu sync.Mutex
			fanoutLiveText := make(map[string]string)

			// Stream individual provider output to TUI tabs in real-time.
			queryPipeline.SetChunkCallback(func(providerID string, chunk provider.StreamChunk) {
				if chunk.Error != nil {
					fanoutMu.Lock()
					fanoutLiveText[providerID] += "[ERROR: " + chunk.Error.Error() + "]"
					fanoutMu.Unlock()
					program.Send(tui.ProviderTraceMsg{
						ProviderName: providerID,
						Phase:        tui.PhaseFanout,
						Error:        chunk.Error,
					})
				} else if chunk.Done {
					// Non-primary providers are done after fan-out.
					// The primary stays loading — synthesis etc. follow.
					if providerID != s.primary.ID() {
						program.Send(tui.ProviderTraceMsg{
							ProviderName: providerID,
							Phase:        tui.PhaseFanout,
							Done:         true,
						})
					}
				} else if chunk.Delta != "" {
					fanoutMu.Lock()
					fanoutLiveText[providerID] += chunk.Delta
					fanoutMu.Unlock()
					program.Send(tui.ProviderTraceMsg{
						ProviderName: providerID,
						Phase:        tui.PhaseFanout,
						Delta:        chunk.Delta,
					})
				}
			})

			// Store cancel funcs from fan-out so providers can be cancelled mid-query.
			// The pipeline populates these during fan-out goroutine launch.
			queryPipeline.SetCancelCallback(func(cancels map[string]context.CancelFunc) {
				cancelFuncs.Lock()
				cancelFuncs.m = cancels
				cancelFuncs.Unlock()
			})

			// Run the fan-out + consensus pipeline with full history
			stream, fanOutResult, err := queryPipeline.Run(ctx, messages, opts)
			if err != nil {
				// Even on pipeline failure, show individual provider errors
				// so the user can see what went wrong per provider.
				if fanOutResult != nil {
					for id, provErr := range fanOutResult.Errors {
						program.Send(tui.ProviderTraceMsg{
							ProviderName: id,
							Phase:        tui.PhaseFanout,
							Error:        provErr,
						})
					}
				}
				// Reconcile primary provider tab — mark it failed so it
				// doesn't remain stuck in loading after a pipeline failure.
				program.Send(tui.ProviderTraceMsg{
					ProviderName: s.primary.ID(),
					Phase:        tui.PhaseFanout,
					Error:        err,
				})
				s.hookMgr.Run(hooks.OnError, hooks.HookContext{Prompt: prompt, Error: err.Error()})
				program.Send(tui.ConsensusChunkMsg{Error: err, Done: true})
				program.Send(tui.QueryDoneMsg{})
				if mc := mcpH.get(); mc != nil {
					program.Send(tui.MCPCallCountMsg{Count: mc.CallCount()})
				}
				return
			}

			// Update token tracker and log telemetry for fan-out
			if fanOutResult != nil {
				for id, usage := range fanOutResult.Usage {
					s.tracker.Add(id, usage)
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

				// Individual provider results already streamed via chunk callback.
				// Only notify about skipped providers.
				for _, id := range fanOutResult.Skipped {
					program.Send(tui.ProviderTraceMsg{
						ProviderName: id,
						Phase:        tui.PhaseFanout,
						Error:        fmt.Errorf("skipped: context limit exceeded"),
					})
				}
			}

			// Accumulate provider traces for persistence alongside the exchange.
			providerTraces := make(map[string][]config.ProviderTraceSection)
			appendTrace := func(providerID string, phase, delta string) {
				sections := providerTraces[providerID]
				if len(sections) == 0 || sections[len(sections)-1].Phase != phase {
					sections = append(sections, config.ProviderTraceSection{Phase: phase})
				}
				sections[len(sections)-1].Content += delta
				providerTraces[providerID] = sections
			}

			// Capture fan-out traces from the live (untruncated) stream.
			// fanoutLiveText was populated by the concurrent chunk callback
			// and is safe to read now — FanOut has returned.
			for id, text := range fanoutLiveText {
				if text != "" {
					appendTrace(id, "fanout", text)
				}
			}
			// Also record skipped providers (not covered by the chunk callback).
			if fanOutResult != nil {
				for _, id := range fanOutResult.Skipped {
					appendTrace(id, "fanout", "[skipped: context limit exceeded]")
				}
			}

			// Detect primary-only direct-response path: if only the primary
			// responded successfully, the pipeline returns the primary's fan-out
			// text as a "synthesis" stream. We must not duplicate it as synthesis.
			primaryOnlyDirect := fanOutResult != nil &&
				len(fanOutResult.Responses) == 1 &&
				fanOutResult.Responses[s.primary.ID()] != ""

			// Stream consensus output, accumulate response, detect tool calls.
			// Mirror synthesis chunks into the primary provider tab.
			var fullResponse string
			var consensusUsage tokens.Usage
			var pendingToolCalls []provider.ToolCall
			primaryID := s.primary.ID()
			for chunk := range stream {
				if chunk.Error != nil {
					program.Send(tui.ConsensusChunkMsg{Error: chunk.Error})
					if !primaryOnlyDirect {
						program.Send(tui.ProviderTraceMsg{
							ProviderName: primaryID,
							Phase:        tui.PhaseSynthesis,
							Error:        chunk.Error,
						})
						appendTrace(primaryID, "synthesis", "[ERROR: "+chunk.Error.Error()+"]")
					}
					break
				}
				// Process Delta before Done — a chunk can carry both (e.g. direct-response path).
				if chunk.Delta != "" {
					fullResponse += chunk.Delta
					program.Send(tui.ConsensusChunkMsg{Delta: chunk.Delta})
					// Only mirror as synthesis when actual synthesis occurred.
					// In primary-only mode the stream just echoes the fan-out text.
					if !primaryOnlyDirect {
						program.Send(tui.ProviderTraceMsg{
							ProviderName: primaryID,
							Phase:        tui.PhaseSynthesis,
							Delta:        chunk.Delta,
						})
						appendTrace(primaryID, "synthesis", chunk.Delta)
					}
				}
				if chunk.Done {
					consensusUsage = tokens.Usage{
						InputTokens:  chunk.InputTokens,
						OutputTokens: chunk.OutputTokens,
					}
					pendingToolCalls = chunk.ToolCalls
					if len(pendingToolCalls) == 0 {
						program.Send(tui.ConsensusChunkMsg{Done: true})
						// Mark primary done — in direct mode, fan-out was the only phase
						program.Send(tui.ProviderTraceMsg{
							ProviderName: primaryID,
							Phase:        tui.PhaseSynthesis,
							Done:         true,
						})
					}
					break
				}
			}

			// Track consensus synthesis usage on the primary
			if consensusUsage.InputTokens > 0 || consensusUsage.OutputTokens > 0 {
				s.tracker.Add(s.primary.ID(), consensusUsage)
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
			// follows, we want to preserve the structured tool call/result
			// messages in the conversation state for future turns.
			var toolResponse string
			var toolLoopMsgs []provider.Message
			var consensusText string // consensus text before tool output (for structured conv state)

			// Execute tool calls if the consensus response included them
			if len(pendingToolCalls) > 0 {
				// Build confirmation callback that consults permission
				// policies, then falls back to TUI confirmation or yolo mode.
				// The executor passes the actual tool name for each call.
				confirmFunc := action.ConfirmFunc(func(toolName, description string) (bool, *string) {
					now := time.Now()
					// Snapshot before mutating tools for undo support
					mutating := toolName == "file_write" || toolName == "file_edit" ||
						toolName == "file_delete" || toolName == "file_rename" || toolName == "shell_exec"

					// Check permission policy for this specific tool
					policy := s.policyMgr.Check(toolName)
					switch policy {
					case permissions.PolicyAllow:
						if mutating {
							createUndoSnapshot(description)
						}
						program.Send(tui.ToolCallMsg{
							ToolName:    toolName,
							Description: "Policy-approved: " + description,
							StartTime:   now,
						})
						return true, nil
					case permissions.PolicyDeny:
						program.Send(tui.ToolCallMsg{
							ToolName:    toolName,
							Description: "Policy-denied: " + description,
							StartTime:   now,
						})
						return false, nil
					}

					// PolicyAsk — check yolo mode, then prompt user
					if yoloEnabled.Load() {
						if mutating {
							createUndoSnapshot(description)
						}
						program.Send(tui.ToolCallMsg{
							ToolName:    toolName,
							Description: "Auto-approved: " + description,
							StartTime:   now,
						})
						return true, nil
					}
					// Determine risk level for the approval dialog
					riskLevel := "mutating"
					if toolName == "file_delete" || toolName == "shell_exec" {
						riskLevel = "destructive"
					}

					// EditableContent is intentionally left empty until per-tool
					// raw argument extraction is implemented. Passing the formatted
					// description would corrupt files on edit+submit.
					editableContent := ""
					responseCh := make(chan tui.ConfirmResult, 1)
					program.Send(tui.ConfirmActionMsg{
						Description:     description,
						ResponseCh:      responseCh,
						ToolName:        toolName,
						RiskLevel:       riskLevel,
						EditableContent: editableContent,
					})

					var result tui.ConfirmResult
					select {
					case result = <-responseCh:
					case <-time.After(5 * time.Minute):
						result = tui.ConfirmResult{Approved: false}
					case <-ctx.Done():
						result = tui.ConfirmResult{Approved: false}
					}

					// Emit ToolCallMsg after interactive approval so tracking works
					if result.Approved {
						if mutating {
							createUndoSnapshot(description)
						}
						program.Send(tui.ToolCallMsg{
							ToolName:    toolName,
							Description: "Approved: " + description,
							StartTime:   time.Now(),
						})
					}

					// Log user feedback for router calibration
					if tlog != nil {
						a := result.Approved
						tlog.Log(telemetry.Event{
							ProviderID: s.primary.ID(),
							EventType:  telemetry.EventUserFeedback,
							ToolName:   toolName,
							Accepted:   &a,
						})
					}
					return result.Approved, result.EditedContent
				})

				executor := action.NewExecutor(confirmFunc, 120*time.Second)

				// Register external tool handlers for MCP and skill tools
				executor.SetExternalHandler(func(call provider.ToolCall) (string, error) {
					// MCP tool names are "mcp_{server}_{tool}" — resolved via lookup map
					if mc := mcpH.get(); mc != nil && len(call.Name) > 4 && call.Name[:4] == "mcp_" {
						if serverName, toolName, ok := mc.ResolveToolCall(call.Name); ok {
							policy := s.policyMgr.Check(call.Name)
							if policy == permissions.PolicyDeny {
								return "", fmt.Errorf("MCP tool %q denied by policy", call.Name)
							}
							if policy != permissions.PolicyAllow && !mc.IsServerReadOnly(serverName) {
								desc := fmt.Sprintf("MCP tool %s → %s.%s", call.Name, serverName, toolName)
								approved, _ := confirmFunc(call.Name, desc)
								if !approved {
									return "", fmt.Errorf("MCP tool %q denied by user", call.Name)
								}
							}
							return mc.CallTool(ctx, serverName, toolName, []byte(call.Arguments))
						}
					}
					// Skill tool names are "skill_{name}_{tool}"
					if len(call.Name) > 6 && call.Name[:6] == "skill_" {
						return skillReg.ExecuteTool(ctx, call.Name, call.Arguments)
					}
					return "", fmt.Errorf("unknown tool: %s", call.Name)
				})

				toolLoop := action.NewToolLoop(executor, s.primary)
				toolLoop.SetToolDoneCallback(func(toolName string, duration time.Duration, errStr string) {
					program.Send(tui.ToolCallDoneMsg{
						ToolName: toolName,
						Duration: duration,
						Error:    errStr,
					})
				})

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

				// No timeout — tool loop runs until the model stops issuing tool calls.
				toolCtx, toolCancel := context.WithCancel(context.Background())

				// Stream tool loop output live to TUI
				toolOut := make(chan provider.StreamChunk, 16)
				go func() {
					defer toolCancel()
					loopMsgs, err := toolLoop.RunWithMessages(toolCtx, toolMsgs, pendingToolCalls, opts, toolOut)
					toolLoopMsgs = loopMsgs // writes to outer var; safe because consumer drains toolOut first
					if err != nil {
						toolOut <- provider.StreamChunk{Error: err}
					}
					close(toolOut)
				}()

				var toolLoopOK bool
				var wroteFiles bool
				for chunk := range toolOut {
					if chunk.Error != nil {
						program.Send(tui.ConsensusChunkMsg{Error: chunk.Error})
						program.Send(tui.ProviderTraceMsg{
							ProviderName: primaryID,
							Phase:        tui.PhaseTool,
							Error:        chunk.Error,
						})
						appendTrace(primaryID, "tool", "[ERROR: "+chunk.Error.Error()+"]")
						continue // drain remaining chunks so goroutine can finish
					}
					if chunk.Done {
						toolLoopOK = true
						continue // drain remaining chunks so goroutine can finish
					}
					// Track whether any file_write was executed
					if chunk.Status && strings.Contains(chunk.Delta, "file_write") {
						wroteFiles = true
					}
					// Display all chunks, but only persist model text (not status)
					program.Send(tui.ConsensusChunkMsg{Delta: chunk.Delta, Status: chunk.Status})
					// Mirror all tool output (status + model text) into primary tab
					program.Send(tui.ProviderTraceMsg{
						ProviderName: primaryID,
						Phase:        tui.PhaseTool,
						Delta:        chunk.Delta,
					})
					appendTrace(primaryID, "tool", chunk.Delta)
					if !chunk.Status {
						toolResponse += chunk.Delta
					}
				}
				// Channel is closed — goroutine has finished, toolLoopMsgs is safe to read.
				toolCancel()

				// Save consensus-only text before combining (needed for structured conv state).
				consensusText = fullResponse

				// Combine initial consensus text + tool follow-up for display/history.
				if toolResponse != "" {
					fullResponse += "\n" + toolResponse
				}

				// Run verification only if the tool loop completed successfully
				// and files were actually written.
				if toolLoopOK && wroteFiles {
					verifyCmd := s.cfg.Consensus.VerifyCommand
					if verifyCmd == "" {
						verifyCmd = action.DetectVerifyCommand(workDir)
					}
					if verifyCmd != "" {
						verifyStartMsg := fmt.Sprintf("\nRunning verification: `%s`...\n", verifyCmd)
						program.Send(tui.ConsensusChunkMsg{Delta: verifyStartMsg})
						program.Send(tui.ProviderTraceMsg{
							ProviderName: primaryID,
							Phase:        tui.PhaseVerify,
							Delta:        verifyStartMsg,
						})
						appendTrace(primaryID, "verify", verifyStartMsg)
						verifyOut, verifyOK, verifyErr := action.RunVerification(
							context.Background(), verifyCmd, workDir, 2*time.Minute,
						)
						if verifyErr != nil {
							verifyErrMsg := fmt.Sprintf("\nVerification error: %v\n", verifyErr)
							program.Send(tui.ConsensusChunkMsg{Delta: verifyErrMsg})
							program.Send(tui.ProviderTraceMsg{
								ProviderName: primaryID,
								Phase:        tui.PhaseVerify,
								Delta:        verifyErrMsg,
							})
							appendTrace(primaryID, "verify", verifyErrMsg)
						} else if !verifyOK {
							display := verifyOut
							if len(display) > 1000 {
								display = display[:1000] + "\n... (truncated)"
							}
							verifyFailMsg := fmt.Sprintf("\nVerification failed:\n```\n%s\n```\n", display)
							program.Send(tui.ConsensusChunkMsg{Delta: verifyFailMsg})
							program.Send(tui.ProviderTraceMsg{
								ProviderName: primaryID,
								Phase:        tui.PhaseVerify,
								Delta:        verifyFailMsg,
							})
							appendTrace(primaryID, "verify", verifyFailMsg)
						} else {
							verifyPassMsg := "\nVerification passed.\n"
							program.Send(tui.ConsensusChunkMsg{Delta: verifyPassMsg})
							program.Send(tui.ProviderTraceMsg{
								ProviderName: primaryID,
								Phase:        tui.PhaseVerify,
								Delta:        verifyPassMsg,
							})
							appendTrace(primaryID, "verify", verifyPassMsg)
						}
					}
				}

				// Fire post-tool hook
				s.hookMgr.Run(hooks.PostTool, hooks.HookContext{
					Prompt:   prompt,
					Response: toolResponse,
				})

				// Mark primary provider done only if the tool loop succeeded.
				// If it failed, the error trace already set StatusFailed.
				if toolLoopOK {
					program.Send(tui.ProviderTraceMsg{
						ProviderName: primaryID,
						Phase:        tui.PhaseTool,
						Done:         true,
					})
				}
				program.Send(tui.ConsensusChunkMsg{Done: true})
			}

			// Append conversation messages so future turns have full context.
			// When tool calls occurred, preserve the structured message sequence
			// (assistant+tool_calls → tool results → follow-up assistant) so
			// providers with native tool support get proper conversation history.
			if len(pendingToolCalls) > 0 && len(toolLoopMsgs) > 0 {
				// Initial assistant message with tool calls
				conv.append(provider.Message{
					Role:      provider.RoleAssistant,
					Content:   consensusText,
					ToolCalls: pendingToolCalls,
				})
				// Tool results and follow-up messages from the tool loop
				conv.append(toolLoopMsgs...)
			} else if fullResponse != "" {
				conv.append(provider.Message{
					Role:    provider.RoleAssistant,
					Content: fullResponse,
				})
			}

			// Fire post-query hook
			s.hookMgr.Run(hooks.PostQuery, hooks.HookContext{
				Prompt:   prompt,
				Response: fullResponse,
			})

			// Send updated token snapshot to TUI
			program.Send(tui.TokenUpdateMsg{Usage: s.tracker.Summary()})

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
						RoutingMode:    string(queryMode),
						RoutingReason:  routingReason,
						SynthesisModel: s.primary.ID(),
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
					ProviderTraces:    providerTraces,
					Trace:             trace,
				})
				if err := config.SaveSession(session); err != nil {
					log.Printf("Warning: auto-save session failed: %v", err)
				}
			}()

			program.Send(tui.QueryDoneMsg{})
			if mc := mcpH.get(); mc != nil {
				program.Send(tui.MCPCallCountMsg{Count: mc.CallCount()})
			}
		}()
	})
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
