package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/spf13/cobra"
)

func runReview(cmd *cobra.Command, args []string) error {
	prNumber, _ := cmd.Flags().GetInt("pr")
	postComment, _ := cmd.Flags().GetBool("comment")

	// 4.8: Check gh availability before any PR operation.
	if prNumber > 0 {
		if _, err := exec.LookPath("gh"); err != nil {
			return fmt.Errorf("GitHub CLI (gh) is required for PR operations — install from https://cli.github.com")
		}
	}

	// 4.3: Diff acquisition.
	var diffContent string
	var err error

	if prNumber > 0 {
		// Get diff from a GitHub PR.
		out, execErr := exec.Command("gh", "pr", "diff", strconv.Itoa(prNumber)).CombinedOutput()
		if execErr != nil {
			return fmt.Errorf("failed to get PR diff: %w\n%s", execErr, string(out))
		}
		diffContent = string(out)
	} else {
		// Try staged changes first.
		diffArgs := []string{"diff", "--cached"}
		diffArgs = append(diffArgs, args...)
		out, execErr := exec.Command("git", diffArgs...).CombinedOutput()
		if execErr != nil {
			return fmt.Errorf("failed to run git diff --cached: %w\n%s", execErr, string(out))
		}
		diffContent = strings.TrimSpace(string(out))

		// Fall back to unstaged changes if nothing is staged.
		if diffContent == "" {
			diffArgs = []string{"diff"}
			diffArgs = append(diffArgs, args...)
			out, execErr = exec.Command("git", diffArgs...).CombinedOutput()
			if execErr != nil {
				return fmt.Errorf("failed to run git diff: %w\n%s", execErr, string(out))
			}
			diffContent = strings.TrimSpace(string(out))
		}
	}

	if strings.TrimSpace(diffContent) == "" {
		fmt.Println("No changes to review.")
		return nil
	}

	// 4.4: Construct review prompt.
	reviewPrompt := fmt.Sprintf(`Review the following code changes. For each issue found, specify:
- Severity: critical, warning, or info
- File and line location
- Description of the issue and suggested fix

If the changes look good, say so.

`+"```diff\n%s\n```", diffContent)

	// 4.5: Run the fan-out + consensus pipeline headlessly.
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	registry, err := provider.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("creating provider registry: %w", err)
	}

	// Authenticate all providers.
	for _, p := range registry.Providers() {
		if authErr := p.Authenticate(); authErr != nil {
			fmt.Printf("Warning: failed to authenticate %s: %v\n", p.ID(), authErr)
		}
	}

	healthy := registry.Healthy()
	if len(healthy) == 0 {
		return fmt.Errorf("no healthy providers available — run 'polycode auth login <provider>' to authenticate")
	}

	primary := registry.Primary()
	if err := primary.Validate(); err != nil {
		return fmt.Errorf("primary provider %s is not healthy: %w", primary.ID(), err)
	}

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

	// 4.6: Print the review output.
	fmt.Println(reviewOutput.String())

	// 4.7: Post as PR comment if requested.
	if postComment && prNumber > 0 {
		out, execErr := exec.Command("gh", "pr", "comment", strconv.Itoa(prNumber), "--body", reviewOutput.String()).CombinedOutput()
		if execErr != nil {
			return fmt.Errorf("failed to post PR comment: %w\n%s", execErr, string(out))
		}
		fmt.Println("Review posted as PR comment.")
	}

	return nil
}
