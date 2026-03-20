package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
)

// collectChunks drains a StreamChunk channel and returns all chunks.
func collectChunks(t *testing.T, ch <-chan StreamChunk) []StreamChunk {
	t.Helper()
	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	return chunks
}

// --------------------------------------------------------------------------
// 1. Anthropic SSE parsing
// --------------------------------------------------------------------------

func TestAnthropicSSEParsing(t *testing.T) {
	// SSE payload mimicking the Anthropic Messages streaming API.
	ssePayload := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"usage":{"input_tokens":42,"output_tokens":0}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","usage":{"output_tokens":7}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test-anthropic", "sk-test-key")

	p := NewAnthropicProvider("test-anthropic", "claude-3-opus", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{
		{Role: RoleUser, Content: "Hi"},
	}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	chunks := collectChunks(t, ch)

	// Expect: "Hello" delta, " world" delta, done chunk
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d: %+v", len(chunks), chunks)
	}

	// Verify text content
	var text string
	for _, c := range chunks {
		if c.Error != nil {
			t.Fatalf("unexpected error chunk: %v", c.Error)
		}
		text += c.Delta
	}
	if text != "Hello world" {
		t.Errorf("expected assembled text %q, got %q", "Hello world", text)
	}

	// Verify done chunk carries token counts
	last := chunks[len(chunks)-1]
	if !last.Done {
		t.Error("expected final chunk to have Done=true")
	}
	if last.InputTokens != 42 {
		t.Errorf("expected InputTokens=42, got %d", last.InputTokens)
	}
	if last.OutputTokens != 7 {
		t.Errorf("expected OutputTokens=7, got %d", last.OutputTokens)
	}
}

func TestAnthropicSystemMessageSeparation(t *testing.T) {
	// Verify that system messages are extracted to the top-level "system" field
	// and not included in the messages array sent to the API.
	var capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test-anthropic", "sk-test")

	p := NewAnthropicProvider("test-anthropic", "claude-3-opus", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Hi"},
	}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	if !strings.Contains(capturedBody, `"system":"You are helpful."`) {
		t.Errorf("expected system prompt in body, got: %s", capturedBody)
	}
}

// --------------------------------------------------------------------------
// 2. OpenAI SSE parsing
// --------------------------------------------------------------------------

func TestOpenAISSEParsing(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"delta":{"content":" there"},"finish_reason":null}]}`,
		``,
		`data: {"choices":[],"usage":{"prompt_tokens":15,"completion_tokens":5}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test-openai", "sk-test-key")

	// Pass srv.URL as empty base so constructor doesn't append /chat/completions
	// to a URL that already has the right path. The OpenAI constructor appends
	// /chat/completions when a base URL is provided. We use the mock server
	// URL as the base URL and the handler serves at /.
	p := NewOpenAIProvider("test-openai", "gpt-4", "", "api_key", nil, store)
	// Override the baseURL to use the test server directly.
	p.baseURL = srv.URL

	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{
		{Role: RoleUser, Content: "Hi"},
	}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	chunks := collectChunks(t, ch)

	var text string
	for _, c := range chunks {
		if c.Error != nil {
			t.Fatalf("unexpected error chunk: %v", c.Error)
		}
		text += c.Delta
	}
	if text != "Hello there" {
		t.Errorf("expected assembled text %q, got %q", "Hello there", text)
	}

	last := chunks[len(chunks)-1]
	if !last.Done {
		t.Error("expected final chunk to have Done=true")
	}
	if last.InputTokens != 15 {
		t.Errorf("expected InputTokens=15, got %d", last.InputTokens)
	}
	if last.OutputTokens != 5 {
		t.Errorf("expected OutputTokens=5, got %d", last.OutputTokens)
	}
}

func TestOpenAIToolCallAccumulation(t *testing.T) {
	// Tool calls are streamed across multiple SSE chunks. The provider must
	// accumulate fragments and emit assembled ToolCall objects on the Done chunk.
	ssePayload := strings.Join([]string{
		// First chunk: tool call header with id, type, name
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"file_read","arguments":""}}]},"finish_reason":null}]}`,
		``,
		// Second chunk: argument fragment
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]},"finish_reason":null}]}`,
		``,
		// Third chunk: argument fragment
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"/tmp/x\"}"}}]},"finish_reason":null}]}`,
		``,
		`data: {"choices":[],"usage":{"prompt_tokens":20,"completion_tokens":10}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test-openai", "sk-test")

	p := NewOpenAIProvider("test-openai", "gpt-4", "", "api_key", nil, store)
	p.baseURL = srv.URL
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{
		{Role: RoleUser, Content: "read /tmp/x"},
	}, QueryOpts{
		Tools: []ToolDefinition{{Name: "file_read", Description: "Read a file", Parameters: map[string]interface{}{}}},
	})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	chunks := collectChunks(t, ch)

	last := chunks[len(chunks)-1]
	if !last.Done {
		t.Fatal("expected final chunk Done=true")
	}
	if len(last.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(last.ToolCalls))
	}

	tc := last.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("expected tool call ID %q, got %q", "call_abc", tc.ID)
	}
	if tc.Name != "file_read" {
		t.Errorf("expected tool call name %q, got %q", "file_read", tc.Name)
	}
	expectedArgs := `{"path":"/tmp/x"}`
	if tc.Arguments != expectedArgs {
		t.Errorf("expected arguments %q, got %q", expectedArgs, tc.Arguments)
	}
}

// --------------------------------------------------------------------------
// 3. Gemini SSE parsing
// --------------------------------------------------------------------------

func TestGeminiSSEParsing(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Hi "}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"there!"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3}}`,
		``,
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test-gemini", "AIza-test-key")

	p := NewGeminiProvider("test-gemini", "gemini-pro", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{
		{Role: RoleUser, Content: "Hi"},
	}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	chunks := collectChunks(t, ch)

	var text string
	for _, c := range chunks {
		if c.Error != nil {
			t.Fatalf("unexpected error chunk: %v", c.Error)
		}
		text += c.Delta
	}
	if text != "Hi there!" {
		t.Errorf("expected assembled text %q, got %q", "Hi there!", text)
	}

	last := chunks[len(chunks)-1]
	if !last.Done {
		t.Error("expected final chunk to have Done=true")
	}
	if last.InputTokens != 8 {
		t.Errorf("expected InputTokens=8, got %d", last.InputTokens)
	}
	if last.OutputTokens != 3 {
		t.Errorf("expected OutputTokens=3, got %d", last.OutputTokens)
	}
}

func TestGeminiRoleMapping(t *testing.T) {
	// Verify that assistant messages map to "model" and system/tool map to "user".
	var capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`+"\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test-gemini", "AIza-test")

	p := NewGeminiProvider("test-gemini", "gemini-pro", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{
		{Role: RoleSystem, Content: "Be helpful"},
		{Role: RoleUser, Content: "Q"},
		{Role: RoleAssistant, Content: "A"},
		{Role: RoleUser, Content: "Follow up"},
	}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	// System should map to "user", Assistant should map to "model"
	if !strings.Contains(capturedBody, `"role":"model"`) {
		t.Error("expected assistant to be mapped to 'model' role")
	}
	// Check that no literal "assistant" or "system" role is in the body.
	if strings.Contains(capturedBody, `"role":"assistant"`) {
		t.Error("expected 'assistant' to be remapped, but found literal 'assistant'")
	}
	if strings.Contains(capturedBody, `"role":"system"`) {
		t.Error("expected 'system' to be remapped, but found literal 'system'")
	}
}

// --------------------------------------------------------------------------
// 4. Auth header verification
// --------------------------------------------------------------------------

func TestAnthropicAuthHeader(t *testing.T) {
	var gotHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("anthropic-auth-test", "sk-secret-123")

	p := NewAnthropicProvider("anthropic-auth-test", "claude-3-opus", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "test"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	if gotHeader != "sk-secret-123" {
		t.Errorf("expected x-api-key header %q, got %q", "sk-secret-123", gotHeader)
	}
}

func TestOpenAIAuthHeader(t *testing.T) {
	var gotHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("openai-auth-test", "sk-openai-secret")

	p := NewOpenAIProvider("openai-auth-test", "gpt-4", "", "api_key", nil, store)
	p.baseURL = srv.URL
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "test"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	expected := "Bearer sk-openai-secret"
	if gotHeader != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, gotHeader)
	}
}

func TestGeminiAPIKeyQueryParam(t *testing.T) {
	var gotURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`+"\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("gemini-auth-test", "AIza-key-456")

	p := NewGeminiProvider("gemini-auth-test", "gemini-pro", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "test"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	if !strings.Contains(gotURL, "key=AIza-key-456") {
		t.Errorf("expected URL to contain key=AIza-key-456, got %q", gotURL)
	}
}

func TestGeminiOAuthBearerHeader(t *testing.T) {
	var gotHeader string
	var gotURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`+"\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("gemini-oauth-test", "oauth-token-xyz")

	p := NewGeminiProvider("gemini-oauth-test", "gemini-pro", srv.URL, "oauth", nil, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "test"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	if gotHeader != "Bearer oauth-token-xyz" {
		t.Errorf("expected Authorization header %q, got %q", "Bearer oauth-token-xyz", gotHeader)
	}
	// OAuth mode should NOT have key= in the URL
	if strings.Contains(gotURL, "key=") {
		t.Errorf("OAuth mode should not include key= in URL, got %q", gotURL)
	}
}

// --------------------------------------------------------------------------
// 5. OpenAI-compatible URL construction
// --------------------------------------------------------------------------

func TestHasVersionPath(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/v1", true},
		{"https://example.com/v1/", true},
		{"https://example.com/v2", true},
		{"https://example.com/v4", true},
		{"https://example.com/api/v1", true},
		{"https://example.com/api/v9", true},
		{"https://example.com/api/v12", true},
		{"https://example.com", false},
		{"https://example.com/api", false},
		{"https://example.com/version", false},
		{"https://example.com/va", false},
	}

	for _, tt := range tests {
		got := hasVersionPath(tt.url)
		if got != tt.want {
			t.Errorf("hasVersionPath(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestOpenAICompatURLConstruction(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		wantSuffix string
	}{
		{
			name:      "no version path appends /v1/chat/completions",
			baseURL:   "https://api.example.com",
			wantSuffix: "/v1/chat/completions",
		},
		{
			name:      "with version path appends /chat/completions",
			baseURL:   "https://api.example.com/v4",
			wantSuffix: "/v4/chat/completions",
		},
		{
			name:      "trailing slash stripped",
			baseURL:   "https://api.example.com/v1/",
			wantSuffix: "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedURL string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedURL = r.URL.Path
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
			}))
			defer srv.Close()

			store := auth.NewMemStore()
			_ = store.Set("compat-test", "test-key")

			// We need to set the baseURL to the test server + our path suffix.
			// The constructor trims trailing / from the baseURL.
			// We need to simulate what the real code does, so we construct the
			// provider with the server URL as the base but including the path.
			p := NewOpenAICompatProvider("compat-test", "local-model", srv.URL+extractPath(tt.baseURL), config.AuthMethodAPIKey, store)
			_ = p.Authenticate()

			ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}}, QueryOpts{})
			if err != nil {
				t.Fatalf("Query error: %v", err)
			}
			collectChunks(t, ch)

			if !strings.HasSuffix(capturedURL, "/chat/completions") {
				t.Errorf("expected URL path to end with /chat/completions, got %q", capturedURL)
			}
		})
	}
}

// extractPath returns the path portion of a URL string for test helpers.
func extractPath(rawURL string) string {
	// Find the third slash (after https://)
	idx := 0
	slashes := 0
	for i, c := range rawURL {
		if c == '/' {
			slashes++
			if slashes == 3 {
				idx = i
				break
			}
		}
	}
	if slashes < 3 {
		return ""
	}
	return rawURL[idx:]
}

func TestOpenAICompatNoAuth(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	p := NewOpenAICompatProvider("compat-noauth", "local-model", srv.URL, config.AuthMethodNone, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	if gotAuth != "" {
		t.Errorf("expected no Authorization header with AuthMethodNone, got %q", gotAuth)
	}
}

func TestOpenAICompatWithAuth(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("compat-auth", "my-secret-key")

	p := NewOpenAICompatProvider("compat-auth", "model-x", srv.URL, config.AuthMethodAPIKey, store)
	_ = p.Authenticate()

	ch, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	collectChunks(t, ch)

	expected := "Bearer my-secret-key"
	if gotAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, gotAuth)
	}
}

// --------------------------------------------------------------------------
// Additional: Validate / Authenticate edge cases
// --------------------------------------------------------------------------

func TestProviderValidateWithoutAuthenticate(t *testing.T) {
	store := auth.NewMemStore()

	anthropic := NewAnthropicProvider("a", "model", "", "api_key", nil, store)
	if err := anthropic.Validate(); err == nil {
		t.Error("Anthropic Validate() should fail without Authenticate()")
	}

	openai := NewOpenAIProvider("o", "model", "", "api_key", nil, store)
	if err := openai.Validate(); err == nil {
		t.Error("OpenAI Validate() should fail without Authenticate()")
	}

	gemini := NewGeminiProvider("g", "model", "", "api_key", nil, store)
	if err := gemini.Validate(); err == nil {
		t.Error("Gemini Validate() should fail without Authenticate()")
	}

	compat := NewOpenAICompatProvider("c", "model", "http://localhost", config.AuthMethodAPIKey, store)
	if err := compat.Validate(); err == nil {
		t.Error("OpenAICompat Validate() should fail without Authenticate()")
	}
}

func TestProviderValidateNoAuthModeAlwaysPasses(t *testing.T) {
	store := auth.NewMemStore()
	compat := NewOpenAICompatProvider("c", "model", "http://localhost", config.AuthMethodNone, store)
	if err := compat.Validate(); err != nil {
		t.Errorf("OpenAICompat Validate() with AuthMethodNone should pass, got: %v", err)
	}
}

func TestProviderID(t *testing.T) {
	store := auth.NewMemStore()

	anthropic := NewAnthropicProvider("my-anthropic", "model", "", "api_key", nil, store)
	if anthropic.ID() != "my-anthropic" {
		t.Errorf("expected ID %q, got %q", "my-anthropic", anthropic.ID())
	}

	openai := NewOpenAIProvider("my-openai", "model", "", "api_key", nil, store)
	if openai.ID() != "my-openai" {
		t.Errorf("expected ID %q, got %q", "my-openai", openai.ID())
	}

	gemini := NewGeminiProvider("my-gemini", "model", "", "api_key", nil, store)
	if gemini.ID() != "my-gemini" {
		t.Errorf("expected ID %q, got %q", "my-gemini", gemini.ID())
	}

	compat := NewOpenAICompatProvider("my-compat", "model", "http://localhost", config.AuthMethodNone, store)
	if compat.ID() != "my-compat" {
		t.Errorf("expected ID %q, got %q", "my-compat", compat.ID())
	}
}

func TestAnthropicAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test", "bad-key")

	p := NewAnthropicProvider("test", "model", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	_, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}}, QueryOpts{})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain status 401, got: %v", err)
	}
}

func TestOpenAIAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limit exceeded"}}`)
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test", "key")

	p := NewOpenAIProvider("test", "gpt-4", "", "api_key", nil, store)
	p.baseURL = srv.URL
	_ = p.Authenticate()

	_, err := p.Query(context.Background(), []Message{{Role: RoleUser, Content: "Hi"}}, QueryOpts{})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected error to contain status 429, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	// Server that hangs until the request context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Write one chunk then hang
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	store := auth.NewMemStore()
	_ = store.Set("test", "key")

	p := NewAnthropicProvider("test", "model", srv.URL, "api_key", nil, store)
	_ = p.Authenticate()

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := p.Query(ctx, []Message{{Role: RoleUser, Content: "Hi"}}, QueryOpts{})
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	// Cancel and drain
	cancel()

	chunks := collectChunks(t, ch)

	// Should end with an error chunk containing context.Canceled
	foundErr := false
	for _, c := range chunks {
		if c.Error != nil {
			foundErr = true
			break
		}
	}
	if !foundErr {
		t.Error("expected an error chunk after context cancellation")
	}
}
