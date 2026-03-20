package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
	"github.com/spf13/cobra"
)

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	registry, err := provider.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("creating provider registry: %w", err)
	}

	for _, p := range registry.Providers() {
		if err := p.Authenticate(); err != nil {
			log.Printf("Warning: failed to authenticate %s: %v", p.ID(), err)
		}
	}

	healthy := registry.Healthy()
	if len(healthy) == 0 {
		return fmt.Errorf("no healthy providers available")
	}

	primary := registry.Primary()
	if err := primary.Validate(); err != nil {
		return fmt.Errorf("primary provider not healthy: %w", err)
	}

	// Build token tracker
	providerModels := make(map[string]string)
	providerLimits := make(map[string]int)
	for _, pc := range cfg.Providers {
		providerModels[pc.Name] = pc.Model
		providerLimits[pc.Name] = tokens.LimitForModel(pc.Model, pc.MaxContext)
	}
	tracker := tokens.NewTracker(providerModels, providerLimits)

	pipeline := consensus.NewPipeline(healthy, primary, cfg.Consensus.Timeout, cfg.Consensus.MinResponses, tracker)

	mux := http.NewServeMux()

	// POST /prompt
	mux.HandleFunc("/prompt", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		messages := []provider.Message{
			{Role: provider.RoleUser, Content: req.Prompt},
		}
		opts := provider.QueryOpts{MaxTokens: 4096}

		ctx, cancel := context.WithTimeout(r.Context(), cfg.Consensus.Timeout+30*time.Second)
		defer cancel()

		stream, _, err := pipeline.Run(ctx, messages, opts)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		var response strings.Builder
		for chunk := range stream {
			if chunk.Error != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": chunk.Error.Error()})
				return
			}
			response.WriteString(chunk.Delta)
		}

		writeJSON(w, http.StatusOK, map[string]string{"response": response.String()})
	})

	// POST /review
	mux.HandleFunc("/review", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Diff string `json:"diff"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		prompt := fmt.Sprintf("Review the following code changes. For each issue found, specify severity (critical/warning/info), file location, and description.\n\n```diff\n%s\n```", req.Diff)

		messages := []provider.Message{
			{Role: provider.RoleUser, Content: prompt},
		}
		opts := provider.QueryOpts{MaxTokens: 4096}

		ctx, cancel := context.WithTimeout(r.Context(), cfg.Consensus.Timeout+30*time.Second)
		defer cancel()

		stream, _, err := pipeline.Run(ctx, messages, opts)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		var response strings.Builder
		for chunk := range stream {
			if chunk.Error != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": chunk.Error.Error()})
				return
			}
			response.WriteString(chunk.Delta)
		}

		writeJSON(w, http.StatusOK, map[string]string{"review": response.String()})
	})

	// GET /status
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}

		providerStatus := make([]map[string]interface{}, 0)
		for _, p := range registry.Providers() {
			status := "healthy"
			if err := p.Validate(); err != nil {
				status = "unhealthy"
			}
			isPrimary := p.ID() == primary.ID()
			usage := tracker.Get(p.ID())

			providerStatus = append(providerStatus, map[string]interface{}{
				"id":            p.ID(),
				"primary":       isPrimary,
				"status":        status,
				"input_tokens":  usage.InputTokens,
				"output_tokens": usage.OutputTokens,
				"limit":         usage.Limit,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version":   version,
			"mode":      cfg.DefaultMode,
			"providers": providerStatus,
		})
	})

	// CORS middleware
	handler := corsMiddleware(mux)

	// Bind to loopback only — the editor bridge should not be exposed on the
	// local network. Use --addr 0.0.0.0 to override if needed.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down editor bridge...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// Use repo-level config path for logging
	configPath := filepath.Join(".", ".polycode", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		configPath = config.ConfigPath()
	}

	fmt.Printf("Polycode editor bridge listening on http://%s\n", addr)
	fmt.Printf("Config: %s\n", configPath)
	fmt.Printf("Endpoints: POST /prompt, POST /review, GET /status\n")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow requests from localhost origins (editor extensions).
		origin := r.Header.Get("Origin")
		if isLocalhostOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isLocalhostOrigin returns true if the origin is from localhost or 127.0.0.1.
// It parses the URL to check the exact hostname, preventing spoofed origins
// like "http://localhost.evil.com".
func isLocalhostOrigin(origin string) bool {
	if strings.HasPrefix(origin, "vscode-webview://") {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
