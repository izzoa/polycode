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
	cfgVal := config.DefaultConfig()
	cfg := &cfgVal

	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, _ := tokens.NewMetadataStore("", cachePath, 24*time.Hour)

	printBanner()
	fmt.Println("=== Polycode Setup ===")
	fmt.Println()
	fmt.Println("Let's configure your first LLM provider.")
	fmt.Println()

	// Add the first provider (always primary)
	if err := addProviderWizard(cfg, metadataStore, true); err != nil {
		return err
	}

	// Ask to add more providers
	for {
		var addMore bool
		err := huh.NewConfirm().
			Title("Add another provider?").
			Description("More providers improve consensus quality").
			Value(&addMore).
			Run()
		if err != nil || !addMore {
			break
		}
		fmt.Println()
		if err := addProviderWizard(cfg, metadataStore, false); err != nil {
			return err
		}
	}

	// Adjust min_responses based on provider count
	if len(cfg.Providers) == 1 {
		cfg.Consensus.MinResponses = 1
	} else if cfg.Consensus.MinResponses < 2 {
		cfg.Consensus.MinResponses = 2
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nConfiguration saved to %s\n", config.ConfigPath())
	fmt.Printf("Configured %d provider(s). Run 'polycode' to start the TUI.\n", len(cfg.Providers))
	return nil
}

// runConfigShow prints the current config in a readable format.
func runConfigShow() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("Config file: %s\n\n", config.ConfigPath())
	fmt.Printf("Providers (%d):\n", len(cfg.Providers))
	for _, p := range cfg.Providers {
		primary := ""
		if p.Primary {
			primary = " ★"
		}
		fmt.Printf("  %-15s  type=%-20s model=%-30s auth=%s%s\n",
			p.Name, p.Type, p.Model, p.Auth, primary)
		if p.BaseURL != "" {
			fmt.Printf("  %-15s  base_url=%s\n", "", p.BaseURL)
		}
	}

	fmt.Printf("\nConsensus:\n")
	fmt.Printf("  timeout:        %s\n", cfg.Consensus.Timeout)
	fmt.Printf("  min_responses:  %d\n", cfg.Consensus.MinResponses)

	fmt.Printf("\nMode: %s\n", cfg.DefaultMode)
	return nil
}

// runConfigEdit runs an interactive editor over the existing config.
func runConfigEdit() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, _ := tokens.NewMetadataStore("", cachePath, 24*time.Hour)

	fmt.Println("=== Polycode Config Editor ===")
	fmt.Println()

	for {
		// Build menu options
		opts := []huh.Option[string]{}
		for _, p := range cfg.Providers {
			label := fmt.Sprintf("Edit %s (%s, %s)", p.Name, p.Type, p.Model)
			if p.Primary {
				label += " ★"
			}
			opts = append(opts, huh.NewOption(label, "edit:"+p.Name))
		}
		opts = append(opts,
			huh.NewOption("Add new provider", "add"),
			huh.NewOption("Remove a provider", "remove"),
			huh.NewOption("Done — save and exit", "done"),
		)

		var choice string
		err := huh.NewSelect[string]().
			Title("What would you like to do?").
			Options(opts...).
			Value(&choice).
			Run()
		if err != nil {
			break // Ctrl+C
		}

		switch {
		case choice == "done":
			if err := cfg.Validate(); err != nil {
				fmt.Printf("Validation error: %v\n", err)
				continue
			}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("\nConfiguration saved to %s\n", config.ConfigPath())
			return nil

		case choice == "add":
			isPrimary := len(cfg.Providers) == 0
			if err := addProviderWizard(cfg, metadataStore, isPrimary); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if len(cfg.Providers) >= 2 && cfg.Consensus.MinResponses < 2 {
				cfg.Consensus.MinResponses = 2
			}

		case choice == "remove":
			if len(cfg.Providers) == 0 {
				fmt.Println("No providers to remove.")
				continue
			}
			removeOpts := make([]huh.Option[string], len(cfg.Providers))
			for i, p := range cfg.Providers {
				removeOpts[i] = huh.NewOption(p.Name, p.Name)
			}
			var removeName string
			huh.NewSelect[string]().
				Title("Remove which provider?").
				Options(removeOpts...).
				Value(&removeName).
				Run() //nolint:errcheck
			if removeName != "" {
				newProviders := make([]config.ProviderConfig, 0, len(cfg.Providers))
				for _, p := range cfg.Providers {
					if p.Name != removeName {
						newProviders = append(newProviders, p)
					}
				}
				cfg.Providers = newProviders
				// Remove credentials
				store := auth.NewStore()
				_ = store.Delete(removeName)
				fmt.Printf("✓ Removed %s\n", removeName)
				// If we removed the primary, set first remaining as primary
				hasPrimary := false
				for _, p := range cfg.Providers {
					if p.Primary {
						hasPrimary = true
					}
				}
				if !hasPrimary && len(cfg.Providers) > 0 {
					cfg.Providers[0].Primary = true
					fmt.Printf("  %s is now the primary provider\n", cfg.Providers[0].Name)
				}
			}

		case strings.HasPrefix(choice, "edit:"):
			provName := strings.TrimPrefix(choice, "edit:")
			for i, p := range cfg.Providers {
				if p.Name != provName {
					continue
				}
				// Let user edit model or set as primary
				editOpts := []huh.Option[string]{
					huh.NewOption("Rename (current: "+p.Name+")", "rename"),
					huh.NewOption("Change model (current: "+p.Model+")", "model"),
					huh.NewOption("Set as primary", "primary"),
					huh.NewOption("Change API key", "apikey"),
					huh.NewOption("Back", "back"),
				}
				var editChoice string
				huh.NewSelect[string]().
					Title("Edit "+p.Name).
					Options(editOpts...).
					Value(&editChoice).
					Run() //nolint:errcheck

				switch editChoice {
				case "rename":
					var newName string
					huh.NewInput().
						Title("New name for " + p.Name).
						Value(&newName).
						Run() //nolint:errcheck
					if newName != "" && newName != p.Name {
						// Migrate credentials to new name
						store := auth.NewStore()
						if key, err := store.Get(p.Name); err == nil && key != "" {
							_ = store.Set(newName, key)
							_ = store.Delete(p.Name)
						}
						cfg.Providers[i].Name = newName
						fmt.Printf("✓ Renamed to %s\n", newName)
					}
				case "model":
					model := selectModel(string(p.Type), p.BaseURL, "", metadataStore)
					if model != "" {
						cfg.Providers[i].Model = model
						fmt.Printf("✓ Model changed to %s\n", model)
					}
				case "primary":
					for j := range cfg.Providers {
						cfg.Providers[j].Primary = false
					}
					cfg.Providers[i].Primary = true
					fmt.Printf("✓ %s is now the primary provider\n", p.Name)
				case "apikey":
					var newKey string
					huh.NewInput().
						Title("New API key for " + p.Name).
						EchoMode(huh.EchoModePassword).
						Value(&newKey).
						Run() //nolint:errcheck
					if newKey != "" {
						store := auth.NewStore()
						_ = store.Set(p.Name, newKey)
						fmt.Printf("✓ API key updated for %s\n", p.Name)
					}
				}
				break
			}
		}
		fmt.Println()
	}

	return nil
}

// runAddProvider adds a provider to an existing config (polycode provider add).
func runAddProvider() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, _ := tokens.NewMetadataStore("", cachePath, 24*time.Hour)

	fmt.Println("=== Add Provider ===")
	fmt.Println()

	isPrimary := len(cfg.Providers) == 0
	if err := addProviderWizard(cfg, metadataStore, isPrimary); err != nil {
		return err
	}

	// If this is the second provider, bump min_responses if it was 1
	if len(cfg.Providers) >= 2 && cfg.Consensus.MinResponses < 2 {
		cfg.Consensus.MinResponses = 2
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nProvider added. %d provider(s) configured.\n", len(cfg.Providers))
	return nil
}

// addProviderWizard runs the interactive provider setup flow and appends the
// result to cfg.Providers. If forcePrimary is true, the provider is set as
// primary without asking.
func addProviderWizard(cfg *config.Config, metadataStore *tokens.MetadataStore, forcePrimary bool) error {
	// Provider type
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

	// Provider name
	var name string
	err = huh.NewInput().
		Title("Provider name").
		Placeholder("e.g., claude, gpt4").
		Value(&name).
		Run()
	if err != nil {
		return fmt.Errorf("setup cancelled: %w", err)
	}

	pt := config.ProviderType(provType)

	// Base URL (openai_compatible only, collected early for model discovery)
	var baseURL string
	if pt == config.ProviderTypeOpenAICompatible {
		err = huh.NewInput().
			Title("Base URL").
			Placeholder("e.g., http://localhost:11434/v1").
			Value(&baseURL).
			Run()
		if err != nil {
			return fmt.Errorf("setup cancelled: %w", err)
		}
	}

	// Auth method
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

	// API key + connection test
	var apiKey string
	authMethod := config.AuthMethod(authInput)
	hasBaseURL := pt == config.ProviderTypeOpenAICompatible
	if authMethod == config.AuthMethodAPIKey {
		for {
			apiKey = ""
			err = huh.NewInput().
				Title("API key").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Run()
			if err != nil {
				return fmt.Errorf("setup cancelled: %w", err)
			}
			if apiKey == "" {
				break
			}

			ok := testConnection(name, provType, apiKey, authInput, baseURL)
			if ok {
				break
			}
			action := handleTestFailure(hasBaseURL)
			if action == "skip" {
				break
			}
			if action == "quit" {
				fmt.Println("Setup cancelled.")
				os.Exit(0)
			}
			if action == "edit_url" {
				huh.NewInput().
					Title("Base URL").
					Placeholder("e.g., http://localhost:11434/v1").
					Value(&baseURL).
					Run() //nolint:errcheck
			}
		}
	}

	// Model selection
	model := selectModel(provType, baseURL, apiKey, metadataStore)

	// Primary — ask if not forced and other providers exist
	primary := forcePrimary
	if !forcePrimary && len(cfg.Providers) > 0 {
		err = huh.NewConfirm().
			Title("Set as primary provider?").
			Description("The primary model synthesizes consensus responses").
			Value(&primary).
			Run()
		if err != nil {
			return fmt.Errorf("setup cancelled: %w", err)
		}
	}

	// If setting this as primary, unset any existing primary
	if primary {
		for i := range cfg.Providers {
			cfg.Providers[i].Primary = false
		}
	}

	cfg.Providers = append(cfg.Providers, config.ProviderConfig{
		Name:    name,
		Type:    pt,
		Auth:    authMethod,
		Model:   model,
		Primary: primary,
		BaseURL: baseURL,
	})

	// Store API key
	if apiKey != "" {
		store := auth.NewStore()
		_ = store.Set(name, apiKey)
	}

	fmt.Printf("✓ Added %s (%s)\n", name, provType)
	return nil
}

// selectModel shows a filterable model list from litellm or endpoint discovery,
// falling back to text input when neither is available.
func selectModel(provType, baseURL, apiKey string, store *tokens.MetadataStore) string {
	var models []config.ModelSummary

	// For openai_compatible, try discovering models from the endpoint first
	if provType == "openai_compatible" && baseURL != "" {
		fmt.Print("Discovering models...")
		ids, err := tokens.DiscoverModels(baseURL, apiKey)
		if err != nil {
			fmt.Printf(" could not discover models: %v\n", err)
		} else {
			fmt.Printf(" found %d models\n", len(ids))
			models = tokens.EnrichWithMetadata(ids, store)
		}
	}

	// Fall back to litellm if no discovered models
	if len(models) == 0 && store != nil {
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

	// Build selectable options
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

// testConnection tests the provider connection. Returns true on success.
// For openai_compatible, uses /models as a lightweight connectivity check
// since the chat endpoint requires a valid model name we don't have yet.
func testConnection(provName, provType, apiKey, authMethod, baseURL string) bool {
	fmt.Printf("Testing connection to %s...\n", provType)

	// For openai_compatible, just verify the /models endpoint is reachable.
	if provType == "openai_compatible" && baseURL != "" {
		_, err := tokens.DiscoverModels(baseURL, apiKey)
		if err != nil {
			fmt.Printf("\u2715 Connection failed: %v\n", err)
			return false
		}
		fmt.Println("\u2713 Connected successfully!")
		return true
	}

	// For standard providers, do a full chat test
	tmpCfg := &config.Config{
		Providers: []config.ProviderConfig{{
			Name:    provName,
			Type:    config.ProviderType(provType),
			Auth:    config.AuthMethod(authMethod),
			Model:   config.DefaultModelByType[config.ProviderType(provType)],
			BaseURL: baseURL,
			Primary: true,
		}},
	}

	if tmpCfg.Providers[0].Model == "" {
		tmpCfg.Providers[0].Model = "test-model"
	}

	memStore := auth.NewMemStore()
	memStore.Set(provName, apiKey)

	registry, err := provider.NewRegistryWithStore(tmpCfg, memStore)
	if err != nil {
		fmt.Printf("\u2715 Connection failed: %v\n", err)
		return false
	}

	testProvider := registry.Primary()
	if testProvider == nil {
		fmt.Println("\u2715 Connection failed: no provider created")
		return false
	}

	if err := testProvider.Authenticate(); err != nil {
		fmt.Printf("\u2715 Connection failed: %v\n", err)
		return false
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
		return false
	}

	for chunk := range stream {
		if chunk.Error != nil {
			fmt.Printf("\u2715 Connection failed: %v\n", chunk.Error)
			return false
		}
	}

	fmt.Println("\u2713 Connected successfully!")
	return true
}

// handleTestFailure prompts for retry/skip/quit (and optionally edit URL).
func handleTestFailure(showEditURL bool) string {
	opts := []huh.Option[string]{
		huh.NewOption("Retry credentials", "retry"),
	}
	if showEditURL {
		opts = append(opts, huh.NewOption("Edit base URL", "edit_url"))
	}
	opts = append(opts,
		huh.NewOption("Skip validation", "skip"),
		huh.NewOption("Quit setup", "quit"),
	)

	var choice string
	huh.NewSelect[string]().
		Title("Connection failed").
		Options(opts...).
		Value(&choice).
		Run() //nolint:errcheck

	return choice
}
