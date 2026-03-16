package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/anthonyizzo/polycode/internal/auth"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiProvider implements Provider for the Google Gemini API.
type GeminiProvider struct {
	name   string
	model  string
	apiKey string
	store  auth.Store
	client *http.Client
}

// NewGeminiProvider creates a new Gemini provider adapter.
func NewGeminiProvider(name, model string, store auth.Store) *GeminiProvider {
	return &GeminiProvider{
		name:   name,
		model:  model,
		store:  store,
		client: &http.Client{},
	}
}

func (p *GeminiProvider) ID() string {
	return p.name
}

func (p *GeminiProvider) Authenticate() error {
	key, err := p.store.Get(p.name)
	if err != nil {
		return fmt.Errorf("gemini authenticate: %w", err)
	}
	p.apiKey = key
	return nil
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
		case RoleSystem:
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

	url := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse&key=%s", geminiBaseURL, p.model, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
