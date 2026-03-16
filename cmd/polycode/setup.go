package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/izzoa/polycode/internal/config"
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

	fmt.Println("=== Polycode Setup ===")
	fmt.Println()
	fmt.Println("Let's configure your first LLM provider.")
	fmt.Println()

	fmt.Println("Provider types: anthropic, openai, google, openai_compatible")
	fmt.Print("Provider type: ")
	provType, _ := reader.ReadString('\n')
	provType = strings.TrimSpace(provType)

	fmt.Print("Provider name (e.g., claude, gpt4): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Print("Model (e.g., claude-sonnet-4-20250514, gpt-4o): ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)

	fmt.Println("Auth methods: api_key, oauth, none")
	fmt.Print("Auth method: ")
	auth, _ := reader.ReadString('\n')
	auth = strings.TrimSpace(auth)

	p := config.ProviderConfig{
		Name:    name,
		Type:    config.ProviderType(provType),
		Auth:    config.AuthMethod(auth),
		Model:   model,
		Primary: true,
	}

	if config.ProviderType(provType) == config.ProviderTypeOpenAICompatible {
		fmt.Print("Base URL (e.g., http://localhost:11434/v1): ")
		baseURL, _ := reader.ReadString('\n')
		p.BaseURL = strings.TrimSpace(baseURL)
	}

	cfg.Providers = append(cfg.Providers, p)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nConfiguration saved to %s\n", config.ConfigPath())
	fmt.Println("You can add more providers by editing the config file.")
	fmt.Println("Run 'polycode' to start the TUI.")
	return nil
}
