package tokens

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sampleLitellmJSON is a minimal litellm-style JSON fixture for testing.
const sampleLitellmJSON = `{
	"gpt-4o": {
		"max_input_tokens": 128000,
		"max_output_tokens": 16384,
		"supports_function_calling": true,
		"supports_vision": true,
		"supports_reasoning": false,
		"supports_response_schema": true,
		"input_cost_per_token": 0.000005,
		"output_cost_per_token": 0.000015
	},
	"openai/gpt-4o-mini": {
		"max_input_tokens": 128000,
		"max_output_tokens": 16384,
		"supports_function_calling": true,
		"supports_vision": true,
		"supports_reasoning": false,
		"supports_response_schema": true
	},
	"anthropic/claude-sonnet-4-20250514": {
		"max_input_tokens": 200000,
		"max_output_tokens": 8192,
		"supports_function_calling": true,
		"supports_vision": true,
		"supports_reasoning": true,
		"supports_response_schema": false
	},
	"gemini/gemini-2.5-pro": {
		"max_input_tokens": 1048576,
		"max_output_tokens": 65536,
		"supports_function_calling": true,
		"supports_vision": true,
		"supports_reasoning": true,
		"supports_response_schema": true
	},
	"some-unknown-model": {
		"max_input_tokens": 4096,
		"max_output_tokens": 2048,
		"supports_function_calling": false,
		"supports_vision": false,
		"supports_reasoning": false,
		"supports_response_schema": false
	}
}`

func TestParseMetadata(t *testing.T) {
	models, err := ParseMetadata([]byte(sampleLitellmJSON))
	if err != nil {
		t.Fatalf("ParseMetadata returned error: %v", err)
	}

	if len(models) != 5 {
		t.Fatalf("expected 5 models, got %d", len(models))
	}

	// Verify gpt-4o fields
	gpt4o, ok := models["gpt-4o"]
	if !ok {
		t.Fatal("expected gpt-4o in parsed models")
	}
	if gpt4o.MaxInputTokens != 128000 {
		t.Errorf("gpt-4o MaxInputTokens: expected 128000, got %d", gpt4o.MaxInputTokens)
	}
	if gpt4o.MaxOutputTokens != 16384 {
		t.Errorf("gpt-4o MaxOutputTokens: expected 16384, got %d", gpt4o.MaxOutputTokens)
	}
	if !gpt4o.SupportsFunctionCalling {
		t.Error("gpt-4o should support function calling")
	}
	if !gpt4o.SupportsVision {
		t.Error("gpt-4o should support vision")
	}
	if gpt4o.SupportsReasoning {
		t.Error("gpt-4o should not support reasoning")
	}
	if !gpt4o.SupportsResponseSchema {
		t.Error("gpt-4o should support response schema")
	}

	// Verify claude model
	claude, ok := models["anthropic/claude-sonnet-4-20250514"]
	if !ok {
		t.Fatal("expected anthropic/claude-sonnet-4-20250514 in parsed models")
	}
	if claude.MaxInputTokens != 200000 {
		t.Errorf("claude MaxInputTokens: expected 200000, got %d", claude.MaxInputTokens)
	}
	if !claude.SupportsReasoning {
		t.Error("claude should support reasoning")
	}
}

func TestParseMetadataInvalidJSON(t *testing.T) {
	_, err := ParseMetadata([]byte(`not valid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseMetadataToleratesUnknownFields(t *testing.T) {
	// JSON with only unknown fields — should parse without error, yielding zero-value ModelInfo
	data := []byte(`{"model-x": {"unknown_field": "hello", "cost": 0.01}}`)
	models, err := ParseMetadata(data)
	if err != nil {
		t.Fatalf("ParseMetadata returned error: %v", err)
	}
	info, ok := models["model-x"]
	if !ok {
		t.Fatal("expected model-x in parsed models")
	}
	if info.MaxInputTokens != 0 {
		t.Errorf("expected 0 MaxInputTokens for unknown-only fields, got %d", info.MaxInputTokens)
	}
}

func TestLookupExactMatch(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"gpt-4o": {MaxInputTokens: 128000},
		},
	}

	info, ok := store.Lookup("gpt-4o", "openai")
	if !ok {
		t.Fatal("expected exact match for gpt-4o")
	}
	if info.MaxInputTokens != 128000 {
		t.Errorf("expected 128000, got %d", info.MaxInputTokens)
	}
}

func TestLookupProviderPrefixed(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"openai/gpt-4o-mini": {MaxInputTokens: 128000},
		},
	}

	info, ok := store.Lookup("gpt-4o-mini", "openai")
	if !ok {
		t.Fatal("expected provider-prefixed match for gpt-4o-mini")
	}
	if info.MaxInputTokens != 128000 {
		t.Errorf("expected 128000, got %d", info.MaxInputTokens)
	}
}

func TestLookupSuffixScan(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"anthropic/claude-sonnet-4-20250514": {MaxInputTokens: 200000},
		},
	}

	// Lookup without the correct provider type — should still find via suffix scan
	info, ok := store.Lookup("claude-sonnet-4-20250514", "")
	if !ok {
		t.Fatal("expected suffix scan match for claude-sonnet-4-20250514")
	}
	if info.MaxInputTokens != 200000 {
		t.Errorf("expected 200000, got %d", info.MaxInputTokens)
	}
}

func TestLookupMiss(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"gpt-4o": {MaxInputTokens: 128000},
		},
	}

	_, ok := store.Lookup("nonexistent-model", "openai")
	if ok {
		t.Error("expected miss for nonexistent-model")
	}
}

func TestStoreLimitForModel_ConfigOverride(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"gpt-4o": {MaxInputTokens: 128000},
		},
	}

	// Config override takes precedence
	got := store.LimitForModel("gpt-4o", "openai", 50000)
	if got != 50000 {
		t.Errorf("expected config override 50000, got %d", got)
	}
}

func TestStoreLimitForModel_LitellmFallback(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"some-new-model": {MaxInputTokens: 256000},
		},
	}

	// No config override, model not in KnownLimits — should use litellm data
	got := store.LimitForModel("some-new-model", "", 0)
	if got != 256000 {
		t.Errorf("expected litellm limit 256000, got %d", got)
	}
}

func TestStoreLimitForModel_HardcodedFallback(t *testing.T) {
	store := &MetadataStore{
		models: make(map[string]ModelInfo), // empty — no litellm data
	}

	// No config override, not in litellm — should fall back to KnownLimits
	got := store.LimitForModel("gpt-4o", "openai", 0)
	if got != 128000 {
		t.Errorf("expected hardcoded limit 128000, got %d", got)
	}
}

func TestStoreLimitForModel_UnknownModel(t *testing.T) {
	store := &MetadataStore{
		models: make(map[string]ModelInfo),
	}

	// Not in litellm, not in KnownLimits, no override → 0
	got := store.LimitForModel("completely-unknown", "", 0)
	if got != 0 {
		t.Errorf("expected 0 for unknown model, got %d", got)
	}
}

func TestCapabilitiesForModel_Found(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"gpt-4o": {
				MaxInputTokens:          128000,
				MaxOutputTokens:         16384,
				SupportsFunctionCalling: true,
				SupportsVision:          true,
				SupportsReasoning:       false,
				SupportsResponseSchema:  true,
			},
		},
	}

	caps := store.CapabilitiesForModel("gpt-4o", "openai")
	if !caps.SupportsFunctionCalling {
		t.Error("expected function calling support")
	}
	if !caps.SupportsVision {
		t.Error("expected vision support")
	}
	if caps.SupportsReasoning {
		t.Error("expected no reasoning support")
	}
	if !caps.SupportsResponseSchema {
		t.Error("expected response schema support")
	}
	if caps.MaxOutputTokens != 16384 {
		t.Errorf("expected MaxOutputTokens 16384, got %d", caps.MaxOutputTokens)
	}
}

func TestCapabilitiesForModel_NotFound(t *testing.T) {
	store := &MetadataStore{
		models: make(map[string]ModelInfo),
	}

	caps := store.CapabilitiesForModel("nonexistent", "")
	if caps.MaxInputTokens != 0 || caps.SupportsFunctionCalling || caps.SupportsVision {
		t.Error("expected zero-value ModelInfo for unknown model")
	}
}

func TestFetchMetadata(t *testing.T) {
	// Set up a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleLitellmJSON))
	}))
	defer server.Close()

	data, err := FetchMetadata(server.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("FetchMetadata returned error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("returned data is not valid JSON: %v", err)
	}
	if len(parsed) != 5 {
		t.Errorf("expected 5 entries, got %d", len(parsed))
	}
}

func TestFetchMetadataHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := FetchMetadata(server.URL, 5*time.Second)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "subdir", "cache.json")

	data := []byte(sampleLitellmJSON)

	// Save
	if err := SaveCachedMetadata(cachePath, data); err != nil {
		t.Fatalf("SaveCachedMetadata error: %v", err)
	}

	// Load
	loaded, mtime, err := LoadCachedMetadata(cachePath)
	if err != nil {
		t.Fatalf("LoadCachedMetadata error: %v", err)
	}

	if string(loaded) != string(data) {
		t.Error("loaded data does not match saved data")
	}

	if time.Since(mtime) > 5*time.Second {
		t.Error("mtime seems too old")
	}

	// Verify permissions
	info, _ := os.Stat(cachePath)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected 0600 permissions, got %o", perm)
	}
}

func TestLoadCachedMetadata_Missing(t *testing.T) {
	_, _, err := LoadCachedMetadata("/nonexistent/path/cache.json")
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestNewMetadataStore_FreshCache(t *testing.T) {
	// Write a fresh cache file
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")
	if err := os.WriteFile(cachePath, []byte(sampleLitellmJSON), 0600); err != nil {
		t.Fatal(err)
	}

	// Use a bogus URL — should not be contacted because cache is fresh
	store, err := NewMetadataStore("http://bogus.invalid/not-real", cachePath, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewMetadataStore error: %v", err)
	}

	// Should have parsed the cache
	info, ok := store.Lookup("gpt-4o", "")
	if !ok {
		t.Fatal("expected gpt-4o from cached data")
	}
	if info.MaxInputTokens != 128000 {
		t.Errorf("expected 128000, got %d", info.MaxInputTokens)
	}
}

func TestNewMetadataStore_StaleCache_FetchFails(t *testing.T) {
	// Write a cache file and make it "old" by using a very short TTL
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")
	if err := os.WriteFile(cachePath, []byte(sampleLitellmJSON), 0600); err != nil {
		t.Fatal(err)
	}

	// TTL of 0 means cache is always stale; bogus URL will fail
	store, err := NewMetadataStore("http://bogus.invalid/not-real", cachePath, 0)
	if err != nil {
		t.Fatalf("NewMetadataStore error: %v", err)
	}

	// Should fall back to stale cache
	info, ok := store.Lookup("gpt-4o", "")
	if !ok {
		t.Fatal("expected gpt-4o from stale cache fallback")
	}
	if info.MaxInputTokens != 128000 {
		t.Errorf("expected 128000, got %d", info.MaxInputTokens)
	}
}

func TestNewMetadataStore_NoCache_FetchFails(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent", "cache.json")

	// No cache, bogus URL — should return store with empty models
	store, err := NewMetadataStore("http://bogus.invalid/not-real", cachePath, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewMetadataStore error: %v", err)
	}

	_, ok := store.Lookup("gpt-4o", "")
	if ok {
		t.Error("expected no data when both cache and fetch fail")
	}
}

func TestNewMetadataStore_FetchSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleLitellmJSON))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	store, err := NewMetadataStore(server.URL, cachePath, 0)
	if err != nil {
		t.Fatalf("NewMetadataStore error: %v", err)
	}

	info, ok := store.Lookup("gpt-4o", "")
	if !ok {
		t.Fatal("expected gpt-4o from fetched data")
	}
	if info.MaxInputTokens != 128000 {
		t.Errorf("expected 128000, got %d", info.MaxInputTokens)
	}

	// Verify cache was saved
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("expected cache file to be saved after successful fetch")
	}
}
