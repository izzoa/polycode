package tokens

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/izzoa/polycode/internal/config"
)

func TestDiscoverModels_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"llama3"},{"id":"mistral"},{"id":"codellama"}]}`))
	}))
	defer srv.Close()

	ids, err := DiscoverModels(srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 models, got %d", len(ids))
	}
	// Should be sorted
	expected := []string{"codellama", "llama3", "mistral"}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("ids[%d] = %q, want %q", i, id, expected[i])
		}
	}
}

func TestDiscoverModels_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	ids, err := DiscoverModels(srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 models, got %d", len(ids))
	}
}

func TestDiscoverModels_404FallbackToV1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"id":"llama3"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Base URL without /v1 — should retry with /v1/models after 404
	ids, err := DiscoverModels(srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "llama3" {
		t.Errorf("expected [llama3], got %v", ids)
	}
}

func TestDiscoverModels_NoFallbackWhenV1InBaseURL(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Base URL already ends with /v1 — should NOT retry
	_, err := DiscoverModels(srv.URL+"/v1", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 request (no retry), got %d", callCount)
	}
}

func TestDiscoverModels_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
	}))
	defer srv.Close()

	_, err := DiscoverModels(srv.URL, "sk-test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer sk-test-key" {
		t.Errorf("expected 'Bearer sk-test-key', got %q", gotAuth)
	}
}

func TestDiscoverModels_NoAuthWhenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	_, _ = DiscoverModels(srv.URL, "")
	if gotAuth != "" {
		t.Errorf("expected no auth header, got %q", gotAuth)
	}
}

func TestDiscoverModels_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := DiscoverModels(srv.URL, "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDiscoverModels_Unreachable(t *testing.T) {
	_, err := DiscoverModels("http://127.0.0.1:1", "")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestEnrichWithMetadata_WithMatches(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"llama3": {
				MaxInputTokens:          8192,
				SupportsFunctionCalling: true,
			},
		},
	}

	results := EnrichWithMetadata([]string{"llama3", "unknown-model"}, store)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// llama3 should have litellm data
	if results[0].Name != "llama3" {
		t.Errorf("expected 'llama3', got %q", results[0].Name)
	}
	if results[0].MaxInputTokens != 8192 {
		t.Errorf("expected MaxInputTokens 8192, got %d", results[0].MaxInputTokens)
	}
	if !results[0].SupportsFunctionCalling {
		t.Error("expected SupportsFunctionCalling to be true")
	}

	// unknown-model should have zero values
	if results[1].Name != "unknown-model" {
		t.Errorf("expected 'unknown-model', got %q", results[1].Name)
	}
	if results[1].MaxInputTokens != 0 {
		t.Errorf("expected MaxInputTokens 0, got %d", results[1].MaxInputTokens)
	}
}

func TestEnrichWithMetadata_NilStore(t *testing.T) {
	results := EnrichWithMetadata([]string{"model-a", "model-b"}, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.MaxInputTokens != 0 {
			t.Errorf("expected zero MaxInputTokens with nil store, got %d", r.MaxInputTokens)
		}
	}
}

func TestEnrichWithMetadata_CapabilitiesFormat(t *testing.T) {
	store := &MetadataStore{
		models: map[string]ModelInfo{
			"llama3": {
				MaxInputTokens:          8192,
				SupportsFunctionCalling: true,
				SupportsVision:          true,
			},
		},
	}

	results := EnrichWithMetadata([]string{"llama3"}, store)
	caps := config.FormatCapabilities(results[0])
	if caps == "" {
		t.Error("expected non-empty capabilities string")
	}
}
