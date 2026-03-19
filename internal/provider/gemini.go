package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/izzoa/polycode/internal/auth"
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiProvider implements Provider for the Google Gemini API.
type GeminiProvider struct {
	name       string
	model      string
	baseURL    string
	authMethod string
	oauthCfg   *auth.DeviceFlowConfig
	apiKey     string
	store      auth.Store
	client     *http.Client
}

// NewGeminiProvider creates a new Gemini provider adapter.
// If baseURL is empty, the default Gemini API URL is used.
func NewGeminiProvider(name, model, baseURL, authMethod string, oauthCfg *auth.DeviceFlowConfig, store auth.Store) *GeminiProvider {
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	} else {
		baseURL = strings.TrimRight(baseURL, "/")
	}
	return &GeminiProvider{
		name:       name,
		model:      model,
		baseURL:    baseURL,
		authMethod: authMethod,
		oauthCfg:   oauthCfg,
		store:      store,
		client:     &http.Client{},
	}
}

func (p *GeminiProvider) ID() string {
	return p.name
}

func (p *GeminiProvider) Authenticate() error {
	// Try loading existing token/key from store first.
	key, err := p.store.Get(p.name)
	if err == nil && key != "" {
		p.apiKey = key
		return nil
	}

	// If OAuth, run the device flow to get a token.
	if p.authMethod == "oauth" && p.oauthCfg != nil {
		token, err := auth.RunDeviceFlow(*p.oauthCfg, p.store)
		if err != nil {
			return fmt.Errorf("gemini oauth: %w", err)
		}
		p.apiKey = token
		_ = p.store.Set(p.name, token)
		return nil
	}

	return fmt.Errorf("gemini authenticate: no credentials found for %q — run 'polycode auth login %s'", p.name, p.name)
}

func (p *GeminiProvider) Validate() error {
	if p.apiKey == "" {
		return fmt.Errorf("gemini provider %q: API key not set — call Authenticate() first", p.name)
	}
	return nil
}

// geminiRequest is the request body for the Gemini streaming API.
type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

// geminiSSEResponse represents one SSE data payload from Gemini.
type geminiSSEResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

func (p *GeminiProvider) Query(ctx context.Context, messages []Message, opts QueryOpts) (<-chan StreamChunk, error) {
	var contents []geminiContent
	for _, m := range messages {
		role := string(m.Role)
		// Gemini uses "model" instead of "assistant", and "user" for both
		// user and system messages (system instructions are handled
		// separately in the full API, but we fold them into user here).
		switch m.Role {
		case RoleAssistant:
			role = "model"
		case RoleSystem, RoleTool:
			role = "user"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	reqBody := geminiRequest{
		Contents: contents,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	var reqURL string
	if p.authMethod == "oauth" {
		// OAuth uses Bearer header, no key in URL
		reqURL = fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse", p.baseURL, p.model)
	} else {
		// API key goes in the URL query parameter
		reqURL = fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse&key=%s", p.baseURL, p.model, p.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.authMethod == "oauth" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini: API returned status %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamChunk, 64)
	go p.readSSE(ctx, resp.Body, ch)
	return ch, nil
}

func (p *GeminiProvider) readSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	var inputTokens, outputTokens int

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var resp geminiSSEResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("gemini: parse SSE event: %w", err)}
			return
		}

		// Capture usage metadata (Gemini includes it in the final chunk).
		if resp.UsageMetadata != nil {
			inputTokens = resp.UsageMetadata.PromptTokenCount
			outputTokens = resp.UsageMetadata.CandidatesTokenCount
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		candidate := resp.Candidates[0]

		if len(candidate.Content.Parts) > 0 && candidate.Content.Parts[0].Text != "" {
			ch <- StreamChunk{Delta: candidate.Content.Parts[0].Text}
		}

		if candidate.FinishReason == "STOP" {
			ch <- StreamChunk{Done: true, InputTokens: inputTokens, OutputTokens: outputTokens}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("gemini: read stream: %w", err)}
	}
}
