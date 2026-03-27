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
		// If OAuth, check for token expiry and auto-refresh.
		if p.authMethod == "oauth" && p.oauthCfg != nil && auth.IsTokenExpired(*p.oauthCfg, p.store) {
			if newToken, err := auth.TryRefresh(*p.oauthCfg, p.store); err == nil {
				p.apiKey = newToken
				_ = p.store.Set(p.name, newToken)
			}
		}
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
	Contents         []geminiContent      `json:"contents"`
	Tools            []geminiToolDef      `json:"tools,omitempty"`
	GenerationConfig *geminiGenerationCfg `json:"generationConfig,omitempty"`
}

// geminiGenerationCfg holds generation parameters for Gemini.
type geminiGenerationCfg struct {
	ThinkingConfig *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// geminiThinkingConfig controls Gemini's thinking/reasoning feature.
type geminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp   `json:"functionResponse,omitempty"`
}

// geminiFunctionCall represents a function call from the model.
type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// geminiFunctionResp represents a function response sent back to the model.
type geminiFunctionResp struct {
	Name     string                 `json:"name"`
	Response map[string]any `json:"response"`
}

// geminiToolDef is a tool definition in Gemini format.
type geminiToolDef struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

// geminiFuncDecl is a function declaration within a tool.
type geminiFuncDecl struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  any `json:"parameters,omitempty"`
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
		switch m.Role {
		case RoleAssistant:
			role = "model"
		case RoleSystem:
			role = "user"
		case RoleTool:
			// Tool results use the "user" role with functionResponse parts.
			// Find the tool call name from the message history.
			toolName := m.ToolCallID // fallback to ID
			for _, prev := range messages {
				for _, tc := range prev.ToolCalls {
					if tc.ID == m.ToolCallID {
						toolName = tc.Name
						break
					}
				}
			}
			contents = append(contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFunctionResp{
						Name: toolName,
						Response: map[string]any{
							"result": m.Content,
						},
					},
				}},
			})
			continue
		}

		// Handle assistant messages with tool calls.
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			var parts []geminiPart
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if tc.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &args)
				}
				if args == nil {
					args = map[string]any{}
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: args,
					},
				})
			}
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: parts,
			})
			continue
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	reqBody := geminiRequest{
		Contents: contents,
	}

	// Map reasoning effort to Gemini's thinking config.
	if opts.ReasoningEffort != "" {
		budgetTokens := 4096
		switch opts.ReasoningEffort {
		case ReasoningMedium:
			budgetTokens = 10000
		case ReasoningHigh:
			budgetTokens = 32000
		}
		reqBody.GenerationConfig = &geminiGenerationCfg{
			ThinkingConfig: &geminiThinkingConfig{
				ThinkingBudget: budgetTokens,
			},
		}
	}

	// Map tool definitions if provided.
	if len(opts.Tools) > 0 {
		var decls []geminiFuncDecl
		for _, t := range opts.Tools {
			decls = append(decls, geminiFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
		reqBody.Tools = []geminiToolDef{{FunctionDeclarations: decls}}
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
	req.Header.Set("User-Agent", "polycode")

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
	var toolCallCounter int

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

		// Emit text content and accumulate function calls from parts.
		var toolCalls []ToolCall
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				ch <- StreamChunk{Delta: part.Text}
			}
			if part.FunctionCall != nil {
				toolCallCounter++
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, ToolCall{
					ID:        fmt.Sprintf("gemini_call_%s_%d", part.FunctionCall.Name, toolCallCounter),
					Name:      part.FunctionCall.Name,
					Arguments: string(args),
				})
			}
		}

		if candidate.FinishReason == "STOP" || candidate.FinishReason == "FUNCTION_CALL" {
			ch <- StreamChunk{Done: true, ToolCalls: toolCalls, InputTokens: inputTokens, OutputTokens: outputTokens}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("gemini: read stream: %w", err)}
		return
	}

	// Stream ended without a STOP/FUNCTION_CALL finish reason — send Done
	// so the consumer doesn't hang waiting for a terminal chunk.
	ch <- StreamChunk{Done: true, InputTokens: inputTokens, OutputTokens: outputTokens}
}
