package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
)

// OpenAICompatProvider implements Provider for OpenAI-compatible APIs with a
// configurable base URL. It reuses the OpenAI SSE parsing logic.
type OpenAICompatProvider struct {
	name    string
	model   string
	baseURL string
	apiKey  string
	noAuth  bool
	store   auth.Store
	client  *http.Client
}

// NewOpenAICompatProvider creates a new OpenAI-compatible provider adapter.
// The auth method determines whether an Authorization header is sent.
func NewOpenAICompatProvider(name, model, baseURL string, authMethod config.AuthMethod, store auth.Store) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		name:    name,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		noAuth:  authMethod == config.AuthMethodNone,
		store:   store,
		client:  &http.Client{},
	}
}

func (p *OpenAICompatProvider) ID() string {
	return p.name
}

func (p *OpenAICompatProvider) Authenticate() error {
	if p.noAuth {
		return nil
	}
	key, err := p.store.Get(p.name)
	if err != nil {
		return fmt.Errorf("openai-compat authenticate: %w", err)
	}
	p.apiKey = key
	return nil
}

func (p *OpenAICompatProvider) Validate() error {
	if p.noAuth {
		return nil
	}
	if p.apiKey == "" {
		return fmt.Errorf("openai-compat provider %q: API key not set — call Authenticate() first", p.name)
	}
	return nil
}

func (p *OpenAICompatProvider) Query(ctx context.Context, messages []Message, opts QueryOpts) (<-chan StreamChunk, error) {
	var msgs []openaiMsg
	for _, m := range messages {
		msgs = append(msgs, openaiMsg{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := openaiRequest{
		Model:    p.model,
		Messages: msgs,
		Stream:   true,
	}

	for _, t := range opts.Tools {
		reqBody.Tools = append(reqBody.Tools, openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai-compat: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai-compat: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if !p.noAuth {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai-compat: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai-compat: API returned status %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamChunk, 64)
	go readOpenAISSE(ctx, resp.Body, ch)
	return ch, nil
}
