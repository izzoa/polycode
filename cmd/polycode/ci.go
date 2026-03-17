package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/spf13/cobra"
)

func runCI(cmd *cobra.Command, args []string) error {
	prNumber, _ := cmd.Flags().GetInt("pr")
	if prNumber <= 0 {
		return fmt.Errorf("--pr flag is required and must be a positive integer")
	}

	// Check gh availability.
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI (gh) is required for CI mode — install from https://cli.github.com")
	}

	// Load config: try repo-level .polycode/config.yaml first, fall back to user config.
	cfg, err := loadCIConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up auth store and inject environment variable keys.
	store := auth.NewStore()
	injectEnvKeys(cfg, store)

	// Create provider registry with the pre-populated store.
	registry, err := provider.NewRegistryWithStore(cfg, store)
	if err != nil {
		return fmt.Errorf("creating provider registry: %w", err)
	}

	// Authenticate all providers.
	for _, p := range registry.Providers() {
		if authErr := p.Authenticate(); authErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to authenticate %s: %v\n", p.ID(), authErr)
		}
	}

	healthy := registry.Healthy()
	if len(healthy) == 0 {
		return fmt.Errorf("no healthy providers available — set POLYCODE_<PROVIDER>_KEY environment variables or configure auth")
	}

	primary := registry.Primary()
	if err := primary.Validate(); err != nil {
		return fmt.Errorf("primary provider %s is not healthy: %w", primary.ID(), err)
	}

	// Get PR diff.
	out, execErr := exec.Command("gh", "pr", "diff", strconv.Itoa(prNumber)).CombinedOutput()
	if execErr != nil {
		return fmt.Errorf("failed to get PR diff: %w\n%s", execErr, string(out))
	}
	diffContent := string(out)

	if strings.TrimSpace(diffContent) == "" {
		fmt.Println("No changes to review.")
		return nil
	}

	// Build review prompt.
	reviewPrompt := fmt.Sprintf(`Review the following code changes. For each issue found, specify:
- Severity: critical, warning, or info
- File and line location
- Description of the issue and suggested fix

If the changes look good, say so.

`+"```diff\n%s\n```", diffContent)

	// Build token tracker.
	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, err := tokens.NewMetadataStore(
		cfg.Metadata.URL,
		cachePath,
		cfg.Metadata.CacheTTL,
	)
	if err != nil {
		log.Printf("Warning: failed to initialize metadata store: %v", err)
	}

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

	tracker := tokens.NewTracker(providerModels, providerLimits)

	pipeline := consensus.NewPipeline(
		healthy,
		primary,
		cfg.Consensus.Timeout,
		cfg.Consensus.MinResponses,
		tracker,
	)

	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: reviewPrompt,
		},
	}

	opts := provider.QueryOpts{
		MaxTokens: 4096,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Consensus.Timeout+30*time.Second)
	defer cancel()

	stream, _, err := pipeline.Run(ctx, messages, opts)
	if err != nil {
		return fmt.Errorf("consensus pipeline failed: %w", err)
	}

	// Drain the stream and collect the full response.
	var reviewOutput strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			return fmt.Errorf("error during consensus: %w", chunk.Error)
		}
		reviewOutput.WriteString(chunk.Delta)
	}

	review := reviewOutput.String()

	// Post review as PR comment.
	commentOut, execErr := exec.Command("gh", "pr", "comment", strconv.Itoa(prNumber), "--body", review).CombinedOutput()
	if execErr != nil {
		return fmt.Errorf("failed to post PR comment: %w\n%s", execErr, string(commentOut))
	}
	fmt.Println("Review posted as PR comment.")

	// Check for critical issues and exit accordingly.
	if ReviewHasCritical(review) {
		fmt.Println("Critical issues found in review.")
		os.Exit(1)
	}

	fmt.Println("No critical issues found.")
	return nil
}

// loadCIConfig loads config from .polycode/config.yaml in the current directory
// first, falling back to the normal user config.
func loadCIConfig() (*config.Config, error) {
	repoConfigPath := filepath.Join(".polycode", "config.yaml")
	if _, err := os.Stat(repoConfigPath); err == nil {
		return config.LoadFrom(repoConfigPath)
	}
	return config.Load()
}

// injectEnvKeys checks for POLYCODE_<PROVIDER>_KEY environment variables
// and stores them in the auth store for each configured provider.
func injectEnvKeys(cfg *config.Config, store auth.Store) {
	for _, p := range cfg.Providers {
		envKey := "POLYCODE_" + strings.ToUpper(strings.ReplaceAll(p.Name, "-", "_")) + "_KEY"
		if val := os.Getenv(envKey); val != "" {
			_ = store.Set(p.Name, val)
		}
	}
}

// ReviewHasCritical returns true if the review text mentions critical issues.
func ReviewHasCritical(review string) bool {
	lower := strings.ToLower(review)
	return strings.Contains(lower, "critical")
}
