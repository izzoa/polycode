package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/provider"
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
		Short: "Remove credentials for a provider",
		Args:  cobra.ExactArgs(1),
		RunE:  runAuthLogout,
	}

	authStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status for all providers",
		RunE:  runAuthStatus,
	}

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)

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
	providerName := args[0]
	fmt.Printf("Removing credentials for %s...\n", providerName)
	store := auth.NewStore()
	if err := store.Delete(providerName); err != nil {
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
