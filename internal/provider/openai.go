package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/izzoa/polycode/internal/auth"
)

const defaultOpenAIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements Provider for the OpenAI Chat Completions API.
type OpenAIProvider struct {
	name       string
	model      string
	baseURL    string
	authMethod string
	oauthCfg   *auth.DeviceFlowConfig
	apiKey     string
	store      auth.Store
	client     *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider adapter.
// If baseURL is empty, the default OpenAI API URL is used.
func NewOpenAIProvider(name, model, baseURL, authMethod string, oauthCfg *auth.DeviceFlowConfig, store auth.Store) *OpenAIProvider {
	if baseURL == "" {
		baseURL = defaultOpenAIURL
	} else {
		baseURL = strings.TrimRight(baseURL, "/") + "/chat/completions"
	}
	return &OpenAIProvider{
		name:       name,
		model:      model,
		baseURL:    baseURL,
		authMethod: authMethod,
		oauthCfg:   oauthCfg,
		store:      store,
		client:     &http.Client{},
	}
}

func (p *OpenAIProvider) ID() string {
	return p.name
}

func (p *OpenAIProvider) Authenticate() error {
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
			return fmt.Errorf("openai oauth: %w", err)
		}
		p.apiKey = token
		_ = p.store.Set(p.name, token)
		return nil
	}

	return fmt.Errorf("openai authenticate: no credentials found for %q — run 'polycode auth login %s'", p.name, p.name)
}

func (p *OpenAIProvider) Validate() error {
	if p.apiKey == "" {
		return fmt.Errorf("openai provider %q: API key not set — call Authenticate() first", p.name)
	}
	return nil
}

// openaiRequest is the request body for the OpenAI Chat Completions API.
type openaiRequest struct {
	Model         string              `json:"model"`
	Messages      []openaiMsg         `json:"messages"`
	Stream        bool                `json:"stream"`
	StreamOptions *openaiStreamOpts   `json:"stream_options,omitempty"`
	Tools         []openaiTool        `json:"tools,omitempty"`
}

type openaiStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiTool struct {
	Type     string          `json:"type"`
	Function openaiFunction  `json:"function"`
}

type openaiFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// openaiSSEChunk represents a chunk in the OpenAI streaming response.
type openaiSSEChunk struct {
	Choices []openaiChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openaiChoice struct {
	Delta        openaiDelta     `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type openaiDelta struct {
	Content   string              `json:"content,omitempty"`
	ToolCalls []openaiToolCallDelta `json:"tool_calls,omitempty"`
}

type openaiToolCallDelta struct {
	Index    int                   `json:"index"`
	ID       string                `json:"id,omitempty"`
	Type     string                `json:"type,omitempty"`
	Function openaiToolCallFunc    `json:"function,omitempty"`
}

type openaiToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func (p *OpenAIProvider) Query(ctx context.Context, messages []Message, opts QueryOpts) (<-chan StreamChunk, error) {
	var msgs []openaiMsg
	for _, m := range messages {
		msgs = append(msgs, openaiMsg{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := openaiRequest{
		Model:         p.model,
		Messages:      msgs,
		Stream:        true,
		StreamOptions: &openaiStreamOpts{IncludeUsage: true},
	}

	// Map tool definitions if provided.
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
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai: API returned status %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamChunk, 64)
	go readOpenAISSE(ctx, resp.Body, ch)
	return ch, nil
}

// readOpenAISSE reads and parses SSE events from an OpenAI-compatible streaming
// response. It is shared between OpenAIProvider and OpenAICompatProvider.
func readOpenAISSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	// Accumulate tool calls across multiple SSE chunks. OpenAI streams
	// tool call arguments in fragments, so we buffer by index and emit
	// the assembled calls in the final Done chunk.
	type toolCallBuf struct {
		id   string
		name string
		args strings.Builder
	}
	var toolBufs []toolCallBuf
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

		// OpenAI signals end of stream with [DONE].
		if data == "[DONE]" {
			var calls []ToolCall
			for _, tb := range toolBufs {
				calls = append(calls, ToolCall{
					ID:        tb.id,
					Name:      tb.name,
					Arguments: tb.args.String(),
				})
			}
			ch <- StreamChunk{Done: true, ToolCalls: calls, InputTokens: inputTokens, OutputTokens: outputTokens}
			return
		}

		var chunk openaiSSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("openai: parse SSE chunk: %w", err)}
			return
		}

		// Capture usage from the final chunk (when stream_options.include_usage is set).
		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Emit text content.
		if choice.Delta.Content != "" {
			ch <- StreamChunk{Delta: choice.Delta.Content}
		}

		// Accumulate tool call fragments.
		for _, tc := range choice.Delta.ToolCalls {
			// Grow the buffer slice if needed.
			for tc.Index >= len(toolBufs) {
				toolBufs = append(toolBufs, toolCallBuf{})
			}
			if tc.ID != "" {
				toolBufs[tc.Index].id = tc.ID
			}
			if tc.Function.Name != "" {
				toolBufs[tc.Index].name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolBufs[tc.Index].args.WriteString(tc.Function.Arguments)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("openai: read stream: %w", err)}
	}
}
