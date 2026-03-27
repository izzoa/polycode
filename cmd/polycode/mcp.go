package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/mcp"
)

func runMCPList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.MCP.Servers) == 0 {
		fmt.Println("No MCP servers configured.")
		fmt.Println("  Add one with: polycode mcp add")
		return nil
	}

	fmt.Printf("MCP Servers (%d configured)\n\n", len(cfg.MCP.Servers))
	for _, s := range cfg.MCP.Servers {
		transport := "stdio"
		target := s.Command
		if len(s.Args) > 0 {
			// Quote args containing spaces for display clarity.
			quoted := make([]string, len(s.Args))
			for i, a := range s.Args {
				if strings.ContainsAny(a, " \t\"") {
					quoted[i] = fmt.Sprintf("%q", a)
				} else {
					quoted[i] = a
				}
			}
			target += " " + strings.Join(quoted, " ")
		}
		if s.URL != "" {
			transport = "sse"
			target = s.URL
		}
		ro := ""
		if s.ReadOnly {
			ro = " [read-only]"
		}
		fmt.Printf("  %-20s  %-6s  %s%s\n", s.Name, transport, target, ro)
	}
	return nil
}

func runMCPAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Transport selection
	var transport string
	err = huh.NewSelect[string]().
		Title("Transport type").
		Options(
			huh.NewOption("stdio (subprocess)", "stdio"),
			huh.NewOption("SSE (HTTP)", "sse"),
		).
		Value(&transport).
		Run()
	if err != nil {
		return nil // cancelled
	}

	// Server name
	var name string
	err = huh.NewInput().
		Title("Server name").
		Placeholder("e.g., filesystem").
		Value(&name).
		Run()
	if err != nil || name == "" {
		return nil
	}

	// Check for duplicates
	for _, s := range cfg.MCP.Servers {
		if s.Name == name {
			return fmt.Errorf("MCP server %q already exists", name)
		}
	}

	server := config.MCPServerConfig{Name: name}

	if transport == "stdio" {
		var command string
		err = huh.NewInput().
			Title("Command").
			Placeholder("e.g., npx").
			Value(&command).
			Run()
		if err != nil || command == "" {
			return nil
		}
		server.Command = command

		var argsStr string
		err = huh.NewInput().
			Title("Arguments (space-separated)").
			Placeholder("e.g., -y @modelcontextprotocol/server-filesystem /path").
			Value(&argsStr).
			Run()
		if err != nil {
			return nil
		}
		if argsStr != "" {
			server.Args = strings.Fields(argsStr)
		}
	} else {
		var url string
		err = huh.NewInput().
			Title("Server URL").
			Placeholder("e.g., http://localhost:3000/mcp").
			Value(&url).
			Run()
		if err != nil || url == "" {
			return nil
		}
		server.URL = url
	}

	// Read-only
	var readOnly bool
	err = huh.NewConfirm().
		Title("Read-only? (skip confirmation for this server's tools)").
		Value(&readOnly).
		Run()
	if err != nil {
		return nil
	}
	server.ReadOnly = readOnly

	// Add and save
	cfg.MCP.Servers = append(cfg.MCP.Servers, server)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nMCP server '%s' added. %d server(s) configured.\n", name, len(cfg.MCP.Servers))
	return nil
}

func runMCPRemove(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		if len(cfg.MCP.Servers) == 0 {
			fmt.Println("No MCP servers configured.")
			return nil
		}
		opts := make([]huh.Option[string], len(cfg.MCP.Servers))
		for i, s := range cfg.MCP.Servers {
			opts[i] = huh.NewOption(s.Name, s.Name)
		}
		err = huh.NewSelect[string]().
			Title("Remove which MCP server?").
			Options(opts...).
			Value(&name).
			Run()
		if err != nil {
			return nil // cancelled
		}
	}

	found := false
	for i, s := range cfg.MCP.Servers {
		if s.Name == name {
			cfg.MCP.Servers = append(cfg.MCP.Servers[:i], cfg.MCP.Servers[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("MCP server %q not found", name)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("MCP server '%s' removed. %d server(s) remaining.\n", name, len(cfg.MCP.Servers))
	return nil
}

func runMCPTest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		if len(cfg.MCP.Servers) == 0 {
			fmt.Println("No MCP servers configured.")
			return nil
		}
		opts := make([]huh.Option[string], len(cfg.MCP.Servers))
		for i, s := range cfg.MCP.Servers {
			opts[i] = huh.NewOption(s.Name, s.Name)
		}
		err = huh.NewSelect[string]().
			Title("Test which MCP server?").
			Options(opts...).
			Value(&name).
			Run()
		if err != nil {
			return nil // cancelled
		}
	}

	// Find the server config
	var serverCfg *config.MCPServerConfig
	for i := range cfg.MCP.Servers {
		if cfg.MCP.Servers[i].Name == name {
			serverCfg = &cfg.MCP.Servers[i]
			break
		}
	}
	if serverCfg == nil {
		return fmt.Errorf("MCP server %q not found", name)
	}

	fmt.Printf("Testing connection to '%s'...\n", name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	toolCount, err := mcp.TestConnection(ctx, *serverCfg)
	if err != nil {
		fmt.Printf("  ✗ Connection failed: %v\n", err)
		return nil // don't return error — test failure is informational
	}

	fmt.Printf("  ✓ Connected successfully (%d tools discovered)\n", toolCount)
	return nil
}
