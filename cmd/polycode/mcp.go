package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/izzoa/polycode/internal/auth"
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

func runMCPSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	rc := mcp.NewRegistryClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	servers, _, err := rc.Search(ctx, query, 20)
	if err != nil {
		return fmt.Errorf("registry search failed: %w", err)
	}

	if len(servers) == 0 {
		fmt.Printf("No servers found for '%s'.\n", query)
		return nil
	}

	fmt.Printf("MCP Registry — %d results for '%s'\n\n", len(servers), query)
	fmt.Printf("  %-28s %-16s %-30s %s\n", "NAME", "TRANSPORT", "PACKAGE", "DESCRIPTION")
	fmt.Printf("  %-28s %-16s %-30s %s\n", "----", "---------", "-------", "-----------")
	for _, s := range servers {
		fmt.Printf("  %-28s %-16s %-30s %s\n",
			truncate(s.Name, 26),
			truncate(s.TransportLabel(), 14),
			truncate(s.PackageIdentifier(), 28),
			truncate(s.Description, 40),
		)
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func runMCPBrowse(cmd *cobra.Command, args []string) error {
	// Search query.
	var query string
	err := huh.NewInput().
		Title("Search MCP Registry").
		Placeholder("e.g., github, database, filesystem").
		Value(&query).
		Run()
	if err != nil || query == "" {
		return nil // cancelled
	}

	rc := mcp.NewRegistryClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	servers, _, err := rc.Search(ctx, query, 20)
	if err != nil {
		return fmt.Errorf("registry search failed: %w", err)
	}

	if len(servers) == 0 {
		fmt.Printf("No servers found for '%s'.\n", query)
		return nil
	}

	// Build selection list.
	opts := make([]huh.Option[int], len(servers))
	for i, s := range servers {
		desc := s.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		label := fmt.Sprintf("%-25s  %s", s.Name, desc)
		opts[i] = huh.NewOption(label, i)
	}

	var selected int
	err = huh.NewSelect[int]().
		Title(fmt.Sprintf("Select server (%d results)", len(servers))).
		Options(opts...).
		Value(&selected).
		Run()
	if err != nil {
		return nil // cancelled
	}

	// Map to config.
	srv := servers[selected]
	cfg, envMeta := mcp.ToMCPServerConfig(srv)

	// Show what will be added.
	// Preflight: load config and check duplicates before prompting user.
	appCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	for _, s := range appCfg.MCP.Servers {
		if s.Name == cfg.Name {
			return fmt.Errorf("MCP server %q already exists — use a different name or remove the existing one first", cfg.Name)
		}
	}

	fmt.Printf("\nServer: %s\n", srv.Name)
	fmt.Printf("  Description: %s\n", srv.Description)
	if cfg.Command != "" {
		cmdStr := cfg.Command
		if len(cfg.Args) > 0 {
			cmdStr += " " + strings.Join(cfg.Args, " ")
		}
		fmt.Printf("  Command: %s\n", cmdStr)
	}
	if cfg.URL != "" {
		fmt.Printf("  URL: %s\n", cfg.URL)
	}
	if len(cfg.Env) > 0 {
		envKeys := make([]string, 0, len(cfg.Env))
		for k := range cfg.Env {
			envKeys = append(envKeys, k)
		}
		fmt.Printf("  Env vars needed: %s\n", strings.Join(envKeys, ", "))
	}

	// Confirm.
	var confirm bool
	err = huh.NewConfirm().
		Title("Add this server?").
		Value(&confirm).
		Run()
	if err != nil || !confirm {
		return nil
	}

	// Build secret lookup from metadata.
	secretVars := make(map[string]bool)
	for _, m := range envMeta {
		if m.IsSecret {
			secretVars[m.Name] = true
		}
	}

	// Prompt for required env var values.
	if len(cfg.Env) > 0 {
		store := auth.NewStore()
		for k := range cfg.Env {
			var val string
			input := huh.NewInput().
				Title(fmt.Sprintf("Value for %s", k)).
				Value(&val)
			if secretVars[k] {
				input = input.EchoMode(huh.EchoModePassword)
			}
			err = input.Run()
			if err != nil {
				return nil
			}
			if secretVars[k] && val != "" {
				keyringKey := fmt.Sprintf("mcp_%s_%s", cfg.Name, k)
				if err := store.Set(keyringKey, val); err != nil {
					return fmt.Errorf("failed to store secret %s: %w", k, err)
				}
				cfg.Env[k] = "$KEYRING:" + keyringKey
			} else {
				cfg.Env[k] = val
			}
		}
	}

	appCfg.MCP.Servers = append(appCfg.MCP.Servers, cfg)
	if err := appCfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if err := appCfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nMCP server '%s' added. %d server(s) configured.\n", cfg.Name, len(appCfg.MCP.Servers))
	return nil
}
