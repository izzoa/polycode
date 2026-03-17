package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
)

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		if strings.Contains(err.Error(), "config file not found") {
			fmt.Println("No configuration found. Let's set up polycode!")
			fmt.Println()
			if err := runSetupWizard(); err != nil {
				return nil, err
			}
			return config.Load()
		}
		return nil, err
	}
	return cfg, nil
}

func runSetupWizard() error {
	cfg := config.DefaultConfig()

	// Try to create a MetadataStore for model listing
	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, _ := tokens.NewMetadataStore("", cachePath, 24*time.Hour)

	printBanner()
	fmt.Println("=== Polycode Setup ===")
	fmt.Println()
	fmt.Println("Let's configure your first LLM provider.")
	fmt.Println()

	// Provider type — selectable list
	var provType string
	err := huh.NewSelect[string]().
		Title("Provider type").
		Options(
			huh.NewOption("anthropic", "anthropic"),
			huh.NewOption("openai", "openai"),
			huh.NewOption("google", "google"),
			huh.NewOption("openai_compatible", "openai_compatible"),
		).
		Value(&provType).
		Run()
	if err != nil {
		return fmt.Errorf("setup cancelled: %w", err)
	}

	// Provider name — text input
	var name string
	err = huh.NewInput().
		Title("Provider name").
		Placeholder("e.g., claude, gpt4").
		Value(&name).
		Run()
	if err != nil {
		return fmt.Errorf("setup cancelled: %w", err)
	}

	// Auth method — selectable list filtered by provider type
	pt := config.ProviderType(provType)
	validAuths := config.AuthMethodsByType[pt]
	var authInput string
	if len(validAuths) > 0 {
		authOpts := make([]huh.Option[string], len(validAuths))
		for i, a := range validAuths {
			authOpts[i] = huh.NewOption(string(a), string(a))
		}
		err = huh.NewSelect[string]().
			Title("Auth method").
			Options(authOpts...).
			Value(&authInput).
			Run()
		if err != nil {
			return fmt.Errorf("setup cancelled: %w", err)
		}
	}

	// API key — password input + connection test
	var apiKey string
	authMethod := config.AuthMethod(authInput)
	if authMethod == config.AuthMethodAPIKey {
		for {
			apiKey = "" // reset for retry
			err = huh.NewInput().
				Title("API key").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Run()
			if err != nil {
				return fmt.Errorf("setup cancelled: %w", err)
			}
			if apiKey == "" {
				break // user chose to skip
			}

			// Connection test
			ok := testConnection(name, provType, apiKey, authInput)
			if ok {
				break
			}
			// testConnection handles retry/skip/quit internally
			// If we get here, user chose retry — loop again
		}
	}

	// Model selection — filterable list from litellm or text input fallback
	model := selectModel(provType, metadataStore)

	p := config.ProviderConfig{
		Name:    name,
		Type:    pt,
		Auth:    authMethod,
		Model:   model,
		Primary: true,
	}

	// Base URL — text input (only for openai_compatible)
	if pt == config.ProviderTypeOpenAICompatible {
		var baseURL string
		err = huh.NewInput().
			Title("Base URL").
			Placeholder("e.g., http://localhost:11434/v1").
			Value(&baseURL).
			Run()
		if err != nil {
			return fmt.Errorf("setup cancelled: %w", err)
		}
		p.BaseURL = baseURL
	}

	cfg.Providers = append(cfg.Providers, p)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Store API key if provided
	if apiKey != "" {
		store := auth.NewStore()
		_ = store.Set(name, apiKey)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nConfiguration saved to %s\n", config.ConfigPath())
	fmt.Println("You can add more providers by editing the config file.")
	fmt.Println("Run 'polycode' to start the TUI.")
	return nil
}

// selectModel shows a filterable model list from litellm or falls back to text input.
func selectModel(provType string, store *tokens.MetadataStore) string {
	var models []config.ModelSummary
	if store != nil {
		models = store.ModelsForProvider(provType)
	}

	pt := config.ProviderType(provType)
	defaultModel := config.DefaultModelByType[pt]

	// Fallback to text input when no models available
	if len(models) == 0 {
		var model string
		huh.NewInput().
			Title("Model").
			Placeholder(modelHintForType(provType)).
			Value(&model).
			Run() //nolint:errcheck
		if model == "" && defaultModel != "" {
			return defaultModel
		}
		return model
	}

	// Build selectable options from litellm models
	const customSentinel = "__custom__"
	opts := make([]huh.Option[string], 0, len(models)+1)

	for _, m := range models {
		label := m.Name
		caps := config.FormatCapabilities(m)
		if caps != "" {
			label += "  (" + caps + ")"
		}
		opts = append(opts, huh.NewOption(label, m.Name))
	}
	opts = append(opts, huh.NewOption("Custom model...", customSentinel))

	// Pre-select the default model
	model := defaultModel
	if model == "" && len(models) > 0 {
		model = models[0].Name
	}

	huh.NewSelect[string]().
		Title("Model").
		Options(opts...).
		Value(&model).
		Filtering(true).
		Height(15).
		Run() //nolint:errcheck

	if model == customSentinel {
		var custom string
		huh.NewInput().
			Title("Model name").
			Placeholder(modelHintForType(provType)).
			Value(&custom).
			Run() //nolint:errcheck
		if custom == "" && defaultModel != "" {
			return defaultModel
		}
		return custom
	}

	return model
}

// modelHintForType returns placeholder text for manual model entry.
func modelHintForType(t string) string {
	pt := config.ProviderType(t)
	if defaultModel, ok := config.DefaultModelByType[pt]; ok {
		return "e.g. " + defaultModel
	}
	switch t {
	case "anthropic":
		return "e.g. claude-sonnet-4-20250514, claude-opus-4-20250514"
	case "openai":
		return "e.g. gpt-4o, gpt-4-turbo, o3-mini"
	case "google":
		return "e.g. gemini-2.5-pro, gemini-2.5-flash"
	case "openai_compatible":
		return "e.g. mistral-large-latest, llama-3-70b"
	}
	return "model name"
}

// testConnection tests the provider connection and handles retry/skip/quit.
// Returns true if the test passed or the user chose to skip.
func testConnection(provName, provType, apiKey, authMethod string) bool {
	fmt.Printf("Testing connection to %s...\n", provType)

	// Build a temporary config with just this provider
	tmpCfg := &config.Config{
		Providers: []config.ProviderConfig{{
			Name:    provName,
			Type:    config.ProviderType(provType),
			Auth:    config.AuthMethod(authMethod),
			Model:   config.DefaultModelByType[config.ProviderType(provType)],
			Primary: true,
		}},
	}

	// If no default model, use a placeholder — we just need to test auth
	if tmpCfg.Providers[0].Model == "" {
		tmpCfg.Providers[0].Model = "test-model"
	}

	// Create an in-memory auth store with the API key
	memStore := auth.NewMemStore()
	memStore.Set(provName, apiKey)

	registry, err := provider.NewRegistryWithStore(tmpCfg, memStore)
	if err != nil {
		fmt.Printf("\u2715 Connection failed: %v\n", err)
		return handleTestFailure()
	}

	testProvider := registry.Primary()
	if testProvider == nil {
		fmt.Println("\u2715 Connection failed: no provider created")
		return handleTestFailure()
	}

	if err := testProvider.Authenticate(); err != nil {
		fmt.Printf("\u2715 Connection failed: %v\n", err)
		return handleTestFailure()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "Say hello in one word"},
	}
	opts := provider.QueryOpts{MaxTokens: 16}

	stream, err := testProvider.Query(ctx, msgs, opts)
	if err != nil {
		fmt.Printf("\u2715 Connection failed: %v\n", err)
		return handleTestFailure()
	}

	// Drain stream
	for chunk := range stream {
		if chunk.Error != nil {
			fmt.Printf("\u2715 Connection failed: %v\n", chunk.Error)
			return handleTestFailure()
		}
	}

	fmt.Println("\u2713 Connected successfully!")
	return true
}

// handleTestFailure prompts for retry/skip/quit using a selectable list.
// Returns true if skip, false if retry (caller should loop), and exits on quit.
func handleTestFailure() bool {
	var choice string
	huh.NewSelect[string]().
		Title("Connection failed").
		Options(
			huh.NewOption("Retry credentials", "retry"),
			huh.NewOption("Skip validation", "skip"),
			huh.NewOption("Quit setup", "quit"),
		).
		Value(&choice).
		Run() //nolint:errcheck

	switch choice {
	case "skip":
		return true // skip — caller continues
	case "quit":
		fmt.Println("Setup cancelled.")
		os.Exit(0)
	}
	// "retry" or default — retry
	return false
}
