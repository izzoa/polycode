package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/skill"
	"github.com/spf13/cobra"
)

var version = "dev"

func init() {
	// When installed via `go install`, the version is embedded in build info.
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "polycode",
		Short: "Multi-LLM consensus coding assistant",
		Long:  "Polycode queries multiple LLMs in parallel and synthesizes a consensus response via a designated primary model.",
		RunE:  runTUI,
	}

	rootCmd.Version = version

	// Auth subcommands
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage provider authentication",
	}

	authLoginCmd := &cobra.Command{
		Use:   "login [provider]",
		Short: "Authenticate with a provider",
		Args:  cobra.ExactArgs(1),
		RunE:  runAuthLogin,
	}

	authLogoutCmd := &cobra.Command{
		Use:   "logout [provider]",
		Short: "Remove credentials for a provider (omit name to pick from list)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAuthLogout,
	}

	authStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status for all providers",
		RunE:  runAuthStatus,
	}

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)

	// Provider subcommands
	providerCmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage providers",
	}

	providerAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new LLM provider to your config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddProvider()
		},
	}

	providerCmd.AddCommand(providerAddCmd)
	rootCmd.AddCommand(providerCmd)

	// Config subcommands
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit polycode configuration",
	}

	configEditCmd := &cobra.Command{
		Use:   "edit",
		Short: "Interactively edit providers and settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEdit()
		},
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Print current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow()
		},
	}

	configPathCmd := &cobra.Command{
		Use:   "path",
		Short: "Print config file location",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.ConfigPath())
		},
	}

	configCmd.AddCommand(configEditCmd, configShowCmd, configPathCmd)
	rootCmd.AddCommand(configCmd)

	// Init command
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize polycode configuration",
		RunE:  runInit,
	}
	rootCmd.AddCommand(initCmd)

	// Review command
	reviewCmd := &cobra.Command{
		Use:   "review [flags] [-- files...]",
		Short: "Review code changes using multi-model consensus",
		RunE:  runReview,
	}
	reviewCmd.Flags().Int("pr", 0, "GitHub PR number to review")
	reviewCmd.Flags().Bool("comment", false, "Post review as PR comment (requires --pr)")
	rootCmd.AddCommand(reviewCmd)

	// Serve command (editor bridge)
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server for editor integration",
		RunE:  runServe,
	}
	serveCmd.Flags().Int("port", 9876, "Port to listen on")
	rootCmd.AddCommand(serveCmd)

	// CI command
	ciCmd := &cobra.Command{
		Use:   "ci",
		Short: "Run automated PR review in CI environments",
		RunE:  runCI,
	}
	ciCmd.Flags().Int("pr", 0, "GitHub PR number to review")
	rootCmd.AddCommand(ciCmd)

	// Export command
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export current session as a shareable artifact",
		RunE:  runExport,
	}
	exportCmd.Flags().String("format", "md", "Output format: md or json")
	exportCmd.Flags().String("output", "", "Output file path (default: stdout)")
	rootCmd.AddCommand(exportCmd)

	// Import command
	importCmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a previously exported session",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}
	rootCmd.AddCommand(importCmd)

	// Skill subcommands
	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage installed skills/plugins",
	}

	skillListCmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := skill.NewRegistry(filepath.Join(config.ConfigDir(), "skills"))
			if err := reg.Load(); err != nil {
				return err
			}
			fmt.Print(reg.FormatList())
			return nil
		},
	}

	skillInstallCmd := &cobra.Command{
		Use:   "install <path>",
		Short: "Install a skill from a local directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := skill.NewRegistry(filepath.Join(config.ConfigDir(), "skills"))
			if err := reg.Load(); err != nil {
				return err
			}
			if err := reg.Install(args[0]); err != nil {
				return err
			}
			fmt.Println("Skill installed successfully.")
			return nil
		},
	}

	skillRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := skill.NewRegistry(filepath.Join(config.ConfigDir(), "skills"))
			if err := reg.Load(); err != nil {
				return err
			}
			if err := reg.Remove(args[0]); err != nil {
				return err
			}
			fmt.Println("Skill removed.")
			return nil
		},
	}

	skillCmd.AddCommand(skillListCmd, skillInstallCmd, skillRemoveCmd)
	rootCmd.AddCommand(skillCmd)

	// Session subcommands
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage conversation sessions",
	}

	sessionListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all saved sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := config.ListSessions()
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				fmt.Println("No saved sessions.")
				return nil
			}
			fmt.Println("Saved sessions:")
			for _, s := range sessions {
				current := ""
				if s.IsCurrent {
					current = " (current)"
				}
				fmt.Printf("  %-20s  %d exchanges  updated %s%s\n",
					s.Name, s.Exchanges,
					s.UpdatedAt.Format("2006-01-02 15:04"), current)
			}
			return nil
		},
	}

	sessionDeleteCmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.DeleteSessionByName(args[0]); err != nil {
				return err
			}
			fmt.Printf("Session %q deleted.\n", args[0])
			return nil
		},
	}

	sessionShowCmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show session details (default: current session)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var s *config.Session
			var err error
			if len(args) > 0 {
				s, _, err = config.LoadSessionByName(args[0])
			} else {
				s, err = config.LoadSession()
			}
			if err != nil {
				return err
			}
			if s == nil {
				fmt.Println("No session found.")
				return nil
			}
			name := s.Name
			if name == "" {
				name = "(unnamed)"
			}
			fmt.Printf("Session: %s\n", name)
			fmt.Printf("Updated: %s\n", s.UpdatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Exchanges: %d\n\n", len(s.Exchanges))
			for i, ex := range s.Exchanges {
				fmt.Printf("--- Exchange %d ---\n", i+1)
				prompt := ex.Prompt
				if len(prompt) > 80 {
					prompt = prompt[:77] + "..."
				}
				fmt.Printf("Prompt: %s\n", prompt)
				resp := ex.ConsensusResponse
				if len(resp) > 200 {
					resp = resp[:197] + "..."
				}
				fmt.Printf("Response: %s\n\n", resp)
			}
			return nil
		},
	}

	sessionCmd.AddCommand(sessionListCmd, sessionDeleteCmd, sessionShowCmd)
	rootCmd.AddCommand(sessionCmd)

	// MCP subcommands
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP (Model Context Protocol) servers",
	}
	mcpListCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE:  runMCPList,
	}
	mcpAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new MCP server interactively",
		RunE:  runMCPAdd,
	}
	mcpRemoveCmd := &cobra.Command{
		Use:   "remove [name]",
		Short: "Remove an MCP server",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMCPRemove,
	}
	mcpTestCmd := &cobra.Command{
		Use:   "test [name]",
		Short: "Test an MCP server connection",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMCPTest,
	}
	mcpCmd.AddCommand(mcpListCmd, mcpAddCmd, mcpRemoveCmd, mcpTestCmd)
	rootCmd.AddCommand(mcpCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return startTUI(cfg)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	providerName := args[0]
	var found bool
	for _, p := range cfg.Providers {
		if p.Name == providerName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("provider %q not found in config", providerName)
	}

	fmt.Printf("Authenticating with %s...\n", providerName)

	// Create the registry so provider adapters handle the auth flow
	// (API key lookup, OAuth device flow, etc.)
	registry, err := provider.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("creating provider registry: %w", err)
	}

	// Find and authenticate the requested provider
	for _, p := range registry.Providers() {
		if p.ID() == providerName {
			if err := p.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			fmt.Println("Authentication successful.")
			return nil
		}
	}

	return fmt.Errorf("provider %q not found in registry", providerName)
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	var providerName string
	if len(args) > 0 {
		providerName = args[0]
	} else {
		// No arg — load config and let user pick
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if len(cfg.Providers) == 0 {
			fmt.Println("No providers configured.")
			return nil
		}
		opts := make([]huh.Option[string], len(cfg.Providers))
		for i, p := range cfg.Providers {
			opts[i] = huh.NewOption(p.Name, p.Name)
		}
		err = huh.NewSelect[string]().
			Title("Remove credentials for which provider?").
			Options(opts...).
			Value(&providerName).
			Run()
		if err != nil {
			return nil // cancelled
		}
	}

	fmt.Printf("Removing credentials for %s...\n", providerName)
	store := auth.NewStore()
	if err := store.Delete(providerName); err != nil {
		// Treat "not found" as success — credential was already gone
		if strings.Contains(err.Error(), "not found") {
			fmt.Println("No credentials found (already removed).")
			return nil
		}
		return fmt.Errorf("removing credentials: %w", err)
	}
	fmt.Println("Credentials removed.")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fmt.Println("Provider Authentication Status:")
	fmt.Println()
	for _, p := range cfg.Providers {
		primary := ""
		if p.Primary {
			primary = " [PRIMARY]"
		}
		fmt.Printf("  %-20s %-20s auth: %-10s%s\n", p.Name, string(p.Type), string(p.Auth), primary)
	}
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	return runSetupWizard()
}
