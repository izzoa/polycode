package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/mcp"
	"github.com/izzoa/polycode/internal/tui"
)

// wireMCPHandlers sets up MCP-related handlers: the /mcp command,
// MCP test/reconnect, dashboard refresh, and registry browse/select.
func wireMCPHandlers(
	model *tui.Model,
	programRef **tea.Program,
	state *atomic.Pointer[appState],
	mcpH *mcpHolder,
) {
	model.SetMCPHandler(func(subcommand, args string) {
		go func() {
			program := *programRef
			s := state.Load()
			mc := mcpH.get()
			switch subcommand {
			case "", "list":
				if mc == nil {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo MCP servers configured.\n", Done: true})
					return
				}
				statuses := mc.Status()
				tools := mc.Tools()
				var sb strings.Builder
				connected := 0
				for _, s := range statuses {
					if s.Connected {
						connected++
					}
				}
				sb.WriteString(fmt.Sprintf("\nMCP Servers (%d connected)\n\n", connected))
				for _, s := range statuses {
					status := "disconnected"
					if s.Connected {
						status = "connected"
					} else if s.Error != nil {
						status = "failed"
					}
					sb.WriteString(fmt.Sprintf("  %s (%s), %d tools\n", s.Name, status, s.ToolCount))
					// List tools for this server
					for _, t := range tools {
						if t.ServerName == s.Name {
							ro := ""
							if t.ReadOnly {
								ro = " [read-only]"
							}
							sb.WriteString(fmt.Sprintf("    • mcp_%s_%s — %s%s\n", s.Name, t.Name, t.Description, ro))
						}
					}
				}
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})

			case "status":
				if mc == nil {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo MCP servers configured.\n", Done: true})
					return
				}
				statuses := mc.Status()
				var sb strings.Builder
				sb.WriteString("\nMCP Server Status\n\n")
				for _, s := range statuses {
					status := "disconnected"
					if s.Connected {
						status = "✓ connected"
					} else if s.Error != nil {
						status = "✗ " + s.Error.Error()
					}
					sb.WriteString(fmt.Sprintf("  %-20s %s (%d tools)\n", s.Name, status, s.ToolCount))
				}
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})

			case "reconnect":
				if mc == nil {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo MCP client.\n", Done: true})
					return
				}
				serverName := strings.TrimSpace(args)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if serverName != "" {
					err := mc.Reconnect(ctx, serverName)
					if err != nil {
						program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nReconnect %s failed: %v\n", serverName, err), Done: true})
					} else {
						program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nReconnected to %s.\n", serverName), Done: true})
					}
				} else {
					// Reconnect all — report per-server results
					var sb strings.Builder
					sb.WriteString("\nReconnecting all MCP servers...\n")
					for _, s := range mc.Status() {
						err := mc.Reconnect(ctx, s.Name)
						if err != nil {
							sb.WriteString(fmt.Sprintf("  ✗ %s: %v\n", s.Name, err))
						} else {
							sb.WriteString(fmt.Sprintf("  ✓ %s\n", s.Name))
						}
					}
					program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})
				}
				sendMCPStatus(program, mc)

			case "tools":
				if mc == nil {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo MCP servers configured.\n", Done: true})
					return
				}
				tools := mc.Tools()
				serverFilter := strings.TrimSpace(args)
				var sb strings.Builder
				sb.WriteString("\nMCP Tools\n\n")
				for _, t := range tools {
					if serverFilter != "" && t.ServerName != serverFilter {
						continue
					}
					ro := ""
					if t.ReadOnly {
						ro = " [read-only]"
					}
					sb.WriteString(fmt.Sprintf("  mcp_%s_%s — %s%s\n", t.ServerName, t.Name, t.Description, ro))
				}
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})

			case "resources":
				if mc == nil {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo MCP servers configured.\n", Done: true})
					return
				}
				resources := mc.Resources()
				serverFilter := strings.TrimSpace(args)
				var sb strings.Builder
				sb.WriteString("\nMCP Resources\n\n")
				if len(resources) == 0 {
					sb.WriteString("  No resources available.\n")
				}
				for _, r := range resources {
					if serverFilter != "" && r.ServerName != serverFilter {
						continue
					}
					sb.WriteString(fmt.Sprintf("  [%s] %s — %s", r.ServerName, r.Name, r.URI))
					if r.Description != "" {
						sb.WriteString(fmt.Sprintf("  (%s)", r.Description))
					}
					sb.WriteString("\n")
				}
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})

			case "prompts":
				if mc == nil {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nNo MCP servers configured.\n", Done: true})
					return
				}
				prompts := mc.Prompts()
				serverFilter := strings.TrimSpace(args)
				var sb strings.Builder
				sb.WriteString("\nMCP Prompts\n\n")
				if len(prompts) == 0 {
					sb.WriteString("  No prompts available.\n")
				}
				for _, p := range prompts {
					if serverFilter != "" && p.ServerName != serverFilter {
						continue
					}
					sb.WriteString(fmt.Sprintf("  [%s] %s — %s\n", p.ServerName, p.Name, p.Description))
					for _, a := range p.Arguments {
						req := ""
						if a.Required {
							req = " (required)"
						}
						sb.WriteString(fmt.Sprintf("    • %s%s — %s\n", a.Name, req, a.Description))
					}
				}
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})

			case "remove":
				serverName := strings.TrimSpace(args)
				if serverName == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /mcp remove <name>\n", Done: true})
					return
				}
				// Build a new servers slice without mutating the current config in place.
				oldServers := s.cfg.MCP.Servers
				var newServers []config.MCPServerConfig
				found := false
				for _, srv := range oldServers {
					if srv.Name == serverName {
						found = true
						continue
					}
					newServers = append(newServers, srv)
				}
				if !found {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nMCP server '%s' not found.\n", serverName), Done: true})
					return
				}
				// Shallow-copy the config, update the servers slice, and save.
				newCfg := *s.cfg
				newCfg.MCP.Servers = newServers
				if err := newCfg.Save(); err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nFailed to save config: %v\n", err), Done: true})
					return
				}
				// Update the atomic state with the new config.
				state.Store(&appState{
					tracker:   s.tracker,
					registry:  s.registry,
					healthy:   s.healthy,
					primary:   s.primary,
					cfg:       &newCfg,
					hookMgr:   s.hookMgr,
					policyMgr: s.policyMgr,
				})
				// Apply at runtime — reconfigure with new server list
				if mc != nil {
					rctx, rcancel := context.WithTimeout(context.Background(), 15*time.Second)
					if rErr := mc.Reconfigure(rctx, newServers); rErr != nil {
						log.Printf("Warning: MCP reconfigure after remove: %v", rErr)
					}
					rcancel()
					sendMCPStatus(program, mc)
				}
				program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nRemoved and disconnected MCP server '%s'.\n", serverName), Done: true})

			case "search":
				query := strings.TrimSpace(args)
				if query == "" {
					program.Send(tui.ConsensusChunkMsg{Delta: "\nUsage: /mcp search <query>\n", Done: true})
					return
				}
				rc := mcp.NewRegistryClient()
				sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer scancel()
				servers, _, err := rc.Search(sctx, query, 20)
				if err != nil {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nRegistry search failed: %v\n", err), Done: true})
					return
				}
				if len(servers) == 0 {
					program.Send(tui.ConsensusChunkMsg{Delta: fmt.Sprintf("\nNo servers found for '%s'.\n", query), Done: true})
					return
				}
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("\nMCP Registry — %d results for '%s'\n\n", len(servers), query))
				for _, s := range servers {
					name := s.Name
					if len(name) > 28 {
						name = name[:25] + "..."
					}
					desc := s.Description
					if len(desc) > 45 {
						desc = desc[:42] + "..."
					}
					transport := s.TransportLabel()
					if len(transport) > 12 {
						transport = transport[:12]
					}
					sb.WriteString(fmt.Sprintf("  %-28s  %-12s  %s\n", name, transport, desc))
				}
				sb.WriteString("\nUse /mcp add or polycode mcp browse to install a server.\n")
				program.Send(tui.ConsensusChunkMsg{Delta: sb.String(), Done: true})

			default:
				program.Send(tui.ConsensusChunkMsg{
					Delta: "\nUsage: /mcp [list|status|reconnect|tools|resources|prompts|search|add|remove]\n",
					Done:  true,
				})
			}
		}()
	})

	// Set up MCP test handler — uses standalone TestConnection with staged config
	model.SetTestMCPHandler(func(mcpCfg config.MCPServerConfig) {
		go func() {
			program := *programRef
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			toolCount, err := mcp.TestConnection(ctx, mcpCfg)
			if err != nil {
				program.Send(tui.MCPTestResultMsg{
					ServerName: mcpCfg.Name,
					Success:    false,
					Error:      err.Error(),
				})
				return
			}
			program.Send(tui.MCPTestResultMsg{
				ServerName: mcpCfg.Name,
				Success:    true,
				ToolCount:  toolCount,
			})
		}()
	})

	model.SetReconnectMCPHandler(func(serverName string) {
		go func() {
			program := *programRef
			mc := mcpH.get()
			if mc == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = mc.Reconnect(ctx, serverName)
			sendMCPStatus(program, mc)
		}()
	})

	// Set up MCP dashboard refresh handler.
	model.SetMCPDashboardRefreshHandler(func() {
		go sendMCPDashboardData(*programRef, mcpH, state.Load().cfg)
	})

	// Set up MCP registry handlers for the wizard browse step.
	registryClient := mcp.NewRegistryClient()

	model.SetMCPRegistryFetchHandler(func() {
		go func() {
			program := *programRef
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			servers, _, err := registryClient.Search(ctx, "", 50)
			if err != nil {
				program.Send(tui.MCPRegistryResultsMsg{Error: err})
				return
			}
			var results []tui.MCPRegistryResult
			for i := range servers {
				r := tui.MCPRegistryResult{
					Name:           servers[i].Name,
					Description:    servers[i].Description,
					TransportLabel: servers[i].TransportLabel(),
					PackageID:      servers[i].PackageIdentifier(),
					ServerData:     &servers[i],
				}
				// Attach env var metadata for individual prompting.
				if len(servers[i].Packages) > 0 {
					for _, ev := range servers[i].Packages[0].EnvVars {
						r.EnvVars = append(r.EnvVars, tui.MCPRegistryEnvMeta{
							Name:        ev.Name,
							Description: ev.Description,
							IsSecret:    ev.IsSecret,
							IsRequired:  ev.IsRequired,
						})
					}
				}
				results = append(results, r)
			}
			program.Send(tui.MCPRegistryResultsMsg{Servers: results})
		}()
	})

	model.SetMCPRegistrySelectHandler(func(result tui.MCPRegistryResult) config.MCPServerConfig {
		if srv, ok := result.ServerData.(*mcp.RegistryServer); ok {
			cfg, _ := mcp.ToMCPServerConfig(*srv)
			return cfg
		}
		return config.MCPServerConfig{Name: result.Name}
	})
}

// sendMCPStatus builds and sends an MCPStatusMsg to the TUI from the current
// MCPClient state. Safe to call from goroutines.
func sendMCPStatus(program *tea.Program, client *mcp.MCPClient) {
	if client == nil {
		return
	}
	statuses := client.Status()
	var servers []tui.MCPServerStatus
	for _, s := range statuses {
		status := "disconnected"
		errMsg := ""
		if s.Connected {
			status = "connected"
		} else if s.Error != nil {
			status = "failed"
			errMsg = s.Error.Error()
		}
		servers = append(servers, tui.MCPServerStatus{
			Name:      s.Name,
			Status:    status,
			ToolCount: s.ToolCount,
			Error:     errMsg,
		})
	}
	program.Send(tui.MCPStatusMsg{Servers: servers})
}

// sendMCPDashboardData builds and sends full dashboard data to the TUI.
func sendMCPDashboardData(program *tea.Program, mcpH *mcpHolder, cfg *config.Config) {
	mc := mcpH.get()
	if mc == nil {
		program.Send(tui.MCPDashboardDataMsg{})
		return
	}

	statuses := mc.Status()
	tools := mc.Tools()
	resources := mc.Resources()
	prompts := mc.Prompts()

	// Count tools/resources/prompts per server.
	toolsByServer := make(map[string][]string)
	for _, t := range tools {
		prefixed := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
		toolsByServer[t.ServerName] = append(toolsByServer[t.ServerName], prefixed)
	}
	resByServer := make(map[string]int)
	for _, r := range resources {
		resByServer[r.ServerName]++
	}
	promptsByServer := make(map[string]int)
	for _, p := range prompts {
		promptsByServer[p.ServerName]++
	}

	// Build transport lookup from config.
	transportByName := make(map[string]string)
	if cfg != nil {
		for _, sc := range cfg.MCP.Servers {
			if sc.URL != "" {
				transportByName[sc.Name] = "sse"
			} else {
				transportByName[sc.Name] = "stdio"
			}
		}
	}

	var dashServers []tui.MCPDashboardServer
	for _, s := range statuses {
		transport := transportByName[s.Name]
		if transport == "" {
			transport = "stdio"
		}
		ds := tui.MCPDashboardServer{
			Name:          s.Name,
			Transport:     transport,
			Status:        "disconnected",
			ToolCount:     s.ToolCount,
			ReadOnly:      mc.IsServerReadOnly(s.Name),
			Tools:         toolsByServer[s.Name],
			ResourceCount: resByServer[s.Name],
			PromptCount:   promptsByServer[s.Name],
		}
		if s.Connected {
			ds.Status = "connected"
		} else if s.Error != nil {
			ds.Status = "failed"
			ds.Error = s.Error.Error()
		}
		dashServers = append(dashServers, ds)
	}

	program.Send(tui.MCPDashboardDataMsg{
		Servers:    dashServers,
		TotalTools: len(tools),
		TotalCalls: mc.CallCount(),
	})
}

// wireMCPNotifications attaches the tool-refresh notification handler to an
// MCPClient. Must be called after `program` is available.
func wireMCPNotifications(program *tea.Program, client *mcp.MCPClient) {
	client.SetNotificationHandler(func(serverName, method string, _ json.RawMessage) {
		if method == "notifications/tools/list_changed" {
			toolCount := 0
			for _, t := range client.Tools() {
				if t.ServerName == serverName {
					toolCount++
				}
			}
			program.Send(tui.MCPToolsChangedMsg{
				ServerName: serverName,
				ToolCount:  toolCount,
			})
			sendMCPStatus(program, client)
		}
	})
}
