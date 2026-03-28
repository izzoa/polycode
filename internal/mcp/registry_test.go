package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func sampleRegistryJSON() string {
	return `{
		"servers": [
			{
				"server": {
					"name": "io.example/server-github",
					"description": "GitHub API integration",
					"version": "1.2.0",
					"packages": [
						{
							"registryType": "npm",
							"identifier": "@example/server-github",
							"transport": {"type": "stdio"},
							"environmentVariables": [
								{"name": "GITHUB_TOKEN", "description": "GitHub PAT", "isRequired": true, "isSecret": true}
							]
						}
					],
					"repository": {"url": "https://github.com/example/server-github"}
				}
			},
			{
				"server": {
					"name": "io.example/remote-db",
					"description": "Remote database access",
					"version": "0.5.0",
					"remotes": [
						{"type": "streamable-http", "url": "https://db.example.com/mcp"}
					]
				}
			}
		],
		"metadata": {"nextCursor": "io.example/remote-db:0.5.0", "count": 2}
	}`
}

func TestRegistrySearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v0/servers" {
			http.NotFound(w, r)
			return
		}
		if q := r.URL.Query().Get("search"); q != "github" {
			t.Errorf("expected search=github, got %q", q)
		}
		if l := r.URL.Query().Get("limit"); l != "20" {
			t.Errorf("expected limit=20, got %q", l)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleRegistryJSON()))
	}))
	defer srv.Close()

	rc := &RegistryClient{
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		cache:      make(map[string]cachedResult),
	}

	servers, cursor, err := rc.Search(t.Context(), "github", 20)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	// Verify first server (npm package).
	s := servers[0]
	if s.Name != "io.example/server-github" {
		t.Errorf("name = %q, want io.example/server-github", s.Name)
	}
	if s.Description != "GitHub API integration" {
		t.Errorf("description = %q", s.Description)
	}
	if len(s.Packages) != 1 || s.Packages[0].RegistryType != "npm" {
		t.Errorf("expected 1 npm package, got %+v", s.Packages)
	}
	if s.Packages[0].Identifier != "@example/server-github" {
		t.Errorf("identifier = %q", s.Packages[0].Identifier)
	}
	if len(s.Packages[0].EnvVars) != 1 || s.Packages[0].EnvVars[0].Name != "GITHUB_TOKEN" {
		t.Errorf("env vars = %+v", s.Packages[0].EnvVars)
	}
	if s.Repository != "https://github.com/example/server-github" {
		t.Errorf("repository = %q", s.Repository)
	}

	// Verify second server (remote only).
	s2 := servers[1]
	if len(s2.Remotes) != 1 || s2.Remotes[0].URL != "https://db.example.com/mcp" {
		t.Errorf("remotes = %+v", s2.Remotes)
	}

	if cursor != "io.example/remote-db:0.5.0" {
		t.Errorf("cursor = %q", cursor)
	}
}

func TestRegistrySearchPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")
		if cursor == "" {
			// First page — no cursor.
			page++
			json.NewEncoder(w).Encode(map[string]any{
				"servers":  []any{map[string]any{"server": map[string]any{"name": "page1/server", "description": "Page 1"}}},
				"metadata": map[string]any{"nextCursor": "page1/server:1.0", "count": 1},
			})
		} else {
			// Second page — verify exact cursor from page 1.
			if cursor != "page1/server:1.0" {
				t.Errorf("expected cursor 'page1/server:1.0', got %q", cursor)
			}
			page++
			json.NewEncoder(w).Encode(map[string]any{
				"servers":  []any{map[string]any{"server": map[string]any{"name": "page2/server", "description": "Page 2"}}},
				"metadata": map[string]any{"nextCursor": "", "count": 1},
			})
		}
	}))
	defer srv.Close()

	rc := &RegistryClient{baseURL: srv.URL, httpClient: srv.Client(), cache: make(map[string]cachedResult)}

	servers1, cursor1, err := rc.Search(t.Context(), "", 1)
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}
	if len(servers1) != 1 || servers1[0].Name != "page1/server" {
		t.Errorf("page 1: %+v", servers1)
	}
	if cursor1 == "" {
		t.Fatal("expected non-empty cursor after page 1")
	}

	servers2, _, err := rc.SearchNext(t.Context(), "", 1, cursor1)
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}
	if len(servers2) != 1 || servers2[0].Name != "page2/server" {
		t.Errorf("page 2: %+v", servers2)
	}
}

func TestRegistryCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"servers":[],"metadata":{"count":0}}`))
	}))
	defer srv.Close()

	rc := &RegistryClient{baseURL: srv.URL, httpClient: srv.Client(), cache: make(map[string]cachedResult)}

	rc.Search(t.Context(), "test", 10)
	rc.Search(t.Context(), "test", 10) // should hit cache

	if calls != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", calls)
	}
}

func TestRegistryCacheExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"servers":[],"metadata":{"count":0}}`))
	}))
	defer srv.Close()

	rc := &RegistryClient{baseURL: srv.URL, httpClient: srv.Client(), cache: make(map[string]cachedResult)}

	rc.Search(t.Context(), "test", 10)

	// Expire the cache manually.
	rc.cacheMu.Lock()
	for k, v := range rc.cache {
		v.fetchedAt = time.Now().Add(-20 * time.Minute)
		rc.cache[k] = v
	}
	rc.cacheMu.Unlock()

	rc.Search(t.Context(), "test", 10) // should make fresh request

	if calls != 2 {
		t.Errorf("expected 2 HTTP calls (cache expired), got %d", calls)
	}
}

func TestRegistryUnreachable(t *testing.T) {
	rc := &RegistryClient{
		baseURL:    "http://127.0.0.1:1", // nothing listening
		httpClient: &http.Client{Timeout: 100 * time.Millisecond},
		cache:      make(map[string]cachedResult),
	}

	_, _, err := rc.Search(t.Context(), "test", 10)
	if err == nil {
		t.Fatal("expected error for unreachable registry")
	}
}

func TestToMCPServerConfig_NPM(t *testing.T) {
	srv := RegistryServer{
		Name: "io.example/server-github",
		Packages: []RegistryPackage{{
			RegistryType: "npm",
			Identifier:   "@example/server-github",
			Transport:    "stdio",
		}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if cfg.Name != "server-github" {
		t.Errorf("name = %q, want server-github", cfg.Name)
	}
	if cfg.Command != "npx" {
		t.Errorf("command = %q, want npx", cfg.Command)
	}
	if len(cfg.Args) != 2 || cfg.Args[0] != "-y" || cfg.Args[1] != "@example/server-github" {
		t.Errorf("args = %v", cfg.Args)
	}
}

func TestToMCPServerConfig_Pip(t *testing.T) {
	srv := RegistryServer{
		Name:     "io.example/mcp-server-sqlite",
		Packages: []RegistryPackage{{RegistryType: "pip", Identifier: "mcp-server-sqlite", Transport: "stdio"}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if cfg.Command != "uvx" || len(cfg.Args) != 1 || cfg.Args[0] != "mcp-server-sqlite" {
		t.Errorf("pip mapping: command=%q args=%v", cfg.Command, cfg.Args)
	}
}

func TestToMCPServerConfig_PyPI(t *testing.T) {
	srv := RegistryServer{
		Name:     "io.example/mcp-server-sqlite",
		Packages: []RegistryPackage{{RegistryType: "pypi", Identifier: "mcp-server-sqlite", Transport: "stdio"}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if cfg.Command != "uvx" || len(cfg.Args) != 1 || cfg.Args[0] != "mcp-server-sqlite" {
		t.Errorf("pypi mapping: command=%q args=%v", cfg.Command, cfg.Args)
	}
}

func TestToMCPServerConfig_UnknownPackageFallsBackToRemote(t *testing.T) {
	srv := RegistryServer{
		Name:     "io.example/unknown-pkg",
		Packages: []RegistryPackage{{RegistryType: "cargo", Identifier: "some-crate", Transport: "stdio"}},
		Remotes:  []RegistryRemote{{Type: "streamable-http", URL: "https://fallback.example.com/mcp"}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if cfg.URL != "https://fallback.example.com/mcp" {
		t.Errorf("expected fallback to remote URL, got command=%q url=%q", cfg.Command, cfg.URL)
	}
}

func TestToMCPServerConfig_OCI(t *testing.T) {
	srv := RegistryServer{
		Name:     "io.example/docker-server",
		Packages: []RegistryPackage{{RegistryType: "oci", Identifier: "docker.io/org/server:1.0", Transport: "stdio"}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if cfg.Command != "docker" {
		t.Errorf("command = %q, want docker", cfg.Command)
	}
	if len(cfg.Args) != 4 || cfg.Args[3] != "docker.io/org/server:1.0" {
		t.Errorf("args = %v", cfg.Args)
	}
}

func TestToMCPServerConfig_Remote(t *testing.T) {
	srv := RegistryServer{
		Name:    "io.example/remote-server",
		Remotes: []RegistryRemote{{Type: "streamable-http", URL: "https://example.com/mcp"}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if cfg.URL != "https://example.com/mcp" {
		t.Errorf("url = %q", cfg.URL)
	}
	if cfg.Command != "" {
		t.Errorf("command should be empty for remote, got %q", cfg.Command)
	}
}

func TestToMCPServerConfig_EnvVarMeta(t *testing.T) {
	srv := RegistryServer{
		Name: "io.example/server-with-secrets",
		Packages: []RegistryPackage{{
			RegistryType: "npm",
			Identifier:   "@example/server",
			Transport:    "stdio",
			EnvVars: []RegistryEnvVar{
				{Name: "API_KEY", IsRequired: true, IsSecret: true, Description: "API key"},
				{Name: "BASE_URL", IsRequired: false, IsSecret: false, Description: "Base URL"},
			},
		}},
	}
	_, meta := ToMCPServerConfig(srv)
	if len(meta) != 2 {
		t.Fatalf("expected 2 env var meta entries, got %d", len(meta))
	}
	if !meta[0].IsSecret || meta[0].Name != "API_KEY" {
		t.Errorf("meta[0]: expected API_KEY secret, got %+v", meta[0])
	}
	if meta[1].IsSecret || meta[1].Name != "BASE_URL" {
		t.Errorf("meta[1]: expected BASE_URL non-secret, got %+v", meta[1])
	}
	if !meta[0].IsRequired {
		t.Error("API_KEY should be required")
	}
}

func TestToMCPServerConfig_EnvVars(t *testing.T) {
	srv := RegistryServer{
		Name: "io.example/server-with-env",
		Packages: []RegistryPackage{{
			RegistryType: "npm",
			Identifier:   "@example/server",
			Transport:    "stdio",
			EnvVars: []RegistryEnvVar{
				{Name: "API_KEY", IsRequired: true, IsSecret: true},
				{Name: "BASE_URL", IsRequired: false},
			},
		}},
	}
	cfg, _ := ToMCPServerConfig(srv)
	if len(cfg.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(cfg.Env))
	}
	if _, ok := cfg.Env["API_KEY"]; !ok {
		t.Error("missing API_KEY in env")
	}
	if _, ok := cfg.Env["BASE_URL"]; !ok {
		t.Error("missing BASE_URL in env")
	}
	// Values should be empty (user fills them).
	if cfg.Env["API_KEY"] != "" {
		t.Errorf("API_KEY should be empty, got %q", cfg.Env["API_KEY"])
	}
}

func TestTransportLabel(t *testing.T) {
	npm := RegistryServer{Packages: []RegistryPackage{{RegistryType: "npm", Transport: "stdio"}}}
	if npm.TransportLabel() != "npm/stdio" {
		t.Errorf("npm label = %q", npm.TransportLabel())
	}

	remote := RegistryServer{Remotes: []RegistryRemote{{Type: "streamable-http", URL: "https://x"}}}
	if remote.TransportLabel() != "streamable-http" {
		t.Errorf("remote label = %q, want streamable-http", remote.TransportLabel())
	}
}

func TestDeriveServerName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"io.example/server-github", "server-github"},
		{"ai.smithery/mcp-tool", "mcp-tool"},
		{"simple-name", "simple-name"},
	}
	for _, tt := range tests {
		got := deriveServerName(tt.input)
		if got != tt.want {
			t.Errorf("deriveServerName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
