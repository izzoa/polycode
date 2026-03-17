package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	reader := bufio.NewReader(os.Stdin)
	cfg := config.DefaultConfig()

	// Try to create a MetadataStore for model listing
	cachePath := filepath.Join(config.ConfigDir(), "model_metadata.json")
	metadataStore, _ := tokens.NewMetadataStore("", cachePath, 24*time.Hour)

	printBanner()
	fmt.Println("=== Polycode Setup ===")
	fmt.Println()
	fmt.Println("Let's configure your first LLM provider.")
	fmt.Println()

	// Provider type selection
	fmt.Println("Provider types: anthropic, openai, google, openai_compatible")
	fmt.Print("Provider type: ")
	provType, _ := reader.ReadString('\n')
	provType = strings.TrimSpace(provType)

	// Provider name
	fmt.Print("Provider name (e.g., claude, gpt4): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	// Auth method — filtered by provider type (task 2.1)
	pt := config.ProviderType(provType)
	validAuths := config.AuthMethodsByType[pt]
	if len(validAuths) > 0 {
		var authStrs []string
		for _, a := range validAuths {
			authStrs = append(authStrs, string(a))
		}
		fmt.Printf("Auth methods for %s: %s\n", provType, strings.Join(authStrs, ", "))
		fmt.Printf("Auth method [%s]: ", authStrs[0])
	} else {
		fmt.Println("Auth methods: api_key, oauth, none")
		fmt.Print("Auth method: ")
	}
	authInput, _ := reader.ReadString('\n')
	authInput = strings.TrimSpace(authInput)
	if authInput == "" && len(validAuths) > 0 {
		authInput = string(validAuths[0])
	}

	// API key entry + connection test (tasks 2.4, 2.5)
	var apiKey string
	authMethod := config.AuthMethod(authInput)
	if authMethod == config.AuthMethodAPIKey {
		for {
			fmt.Print("API key: ")
			apiKeyInput, _ := reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKeyInput)
			if apiKey == "" {
				break // user chose to skip
			}

			// Connection test
			ok := testConnection(reader, name, provType, apiKey, authInput)
			if ok {
				break
			}
			// testConnection handles retry/skip/quit internally
			// If we get here, user chose retry — loop again
		}
	}

	// Model selection (tasks 2.2, 2.3, 2.6)
	model := selectModel(reader, provType, metadataStore)

	p := config.ProviderConfig{
		Name:    name,
		Type:    pt,
		Auth:    authMethod,
		Model:   model,
		Primary: true,
	}

	if pt == config.ProviderTypeOpenAICompatible {
		fmt.Print("Base URL (e.g., http://localhost:11434/v1): ")
		baseURL, _ := reader.ReadString('\n')
		p.BaseURL = strings.TrimSpace(baseURL)
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

// selectModel shows a numbered model list or falls back to text input.
func selectModel(reader *bufio.Reader, provType string, store *tokens.MetadataStore) string {
	var models []config.ModelSummary
	if store != nil {
		models = store.ModelsForProvider(provType)
	}

	pt := config.ProviderType(provType)
	defaultModel := config.DefaultModelByType[pt]

	// If no models available, fall back to text input (task 2.6)
	if len(models) == 0 {
		hint := ""
		if defaultModel != "" {
			hint = fmt.Sprintf(" [%s]", defaultModel)
		}
		fmt.Printf("Model%s: ", hint)
		model, _ := reader.ReadString('\n')
		model = strings.TrimSpace(model)
		if model == "" && defaultModel != "" {
			return defaultModel
		}
		return model
	}

	// Limit to top 10 models
	displayModels := models
	if len(displayModels) > 10 {
		displayModels = displayModels[:10]
	}

	fmt.Printf("\nAvailable models for %s:\n", provType)

	// Find which one is the default to mark it
	defaultIdx := -1
	for i, m := range displayModels {
		if m.Name == defaultModel {
			defaultIdx = i
			break
		}
	}

	for i, m := range displayModels {
		caps := config.FormatCapabilities(m)
		tag := ""
		if i == defaultIdx {
			tag = "  [default]"
		} else if i == 0 && defaultIdx == -1 {
			tag = "  [default]"
		}
		capsStr := ""
		if caps != "" {
			capsStr = fmt.Sprintf("  (%s)", caps)
		}
		fmt.Printf("  %d. %-35s%s%s\n", i+1, m.Name, capsStr, tag)
	}
	fmt.Println("  0. Enter model name manually")

	// Default selection is the default model or first model (task 2.3)
	defaultSelection := 1
	if defaultIdx >= 0 {
		defaultSelection = defaultIdx + 1
	}

	fmt.Printf("\nSelect model [%d]: ", defaultSelection)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "" {
		return displayModels[defaultSelection-1].Name
	}

	if choice == "0" {
		hint := ""
		if defaultModel != "" {
			hint = fmt.Sprintf(" [%s]", defaultModel)
		}
		fmt.Printf("Model name%s: ", hint)
		model, _ := reader.ReadString('\n')
		model = strings.TrimSpace(model)
		if model == "" && defaultModel != "" {
			return defaultModel
		}
		return model
	}

	idx, err := strconv.Atoi(choice)
	if err == nil && idx >= 1 && idx <= len(displayModels) {
		return displayModels[idx-1].Name
	}

	// If the input isn't a valid number, treat it as a model name
	return choice
}

// testConnection tests the provider connection and handles retry/skip/quit.
// Returns true if the test passed or the user chose to skip.
func testConnection(reader *bufio.Reader, provName, provType, apiKey, authMethod string) bool {
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
		return handleTestFailure(reader)
	}

	testProvider := registry.Primary()
	if testProvider == nil {
		fmt.Println("\u2715 Connection failed: no provider created")
		return handleTestFailure(reader)
	}

	if err := testProvider.Authenticate(); err != nil {
		fmt.Printf("\u2715 Connection failed: %v\n", err)
		return handleTestFailure(reader)
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
		return handleTestFailure(reader)
	}

	// Drain stream
	for chunk := range stream {
		if chunk.Error != nil {
			fmt.Printf("\u2715 Connection failed: %v\n", chunk.Error)
			return handleTestFailure(reader)
		}
	}

	fmt.Println("\u2713 Connected successfully!")
	return true
}

// handleTestFailure prompts for retry/skip/quit. Returns true if skip,
// false if retry (caller should loop), and exits on quit.
func handleTestFailure(reader *bufio.Reader) bool {
	fmt.Print("(r)etry credentials, (s)kip validation, (q)uit: ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(strings.ToLower(choice))
	switch choice {
	case "s":
		return true // skip — caller continues
	case "q":
		fmt.Println("Setup cancelled.")
		os.Exit(0)
	}
	// "r" or anything else — retry
	return false
}
