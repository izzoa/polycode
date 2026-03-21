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

const defaultAnthropicURL = "https://api.anthropic.com/v1/messages"

// AnthropicProvider implements Provider for the Anthropic Messages API.
type AnthropicProvider struct {
	name       string
	model      string
	baseURL    string
	authMethod string
	oauthCfg   *auth.DeviceFlowConfig
	apiKey     string
	store      auth.Store
	client     *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider adapter.
// If baseURL is empty, the default Anthropic API URL is used.
func NewAnthropicProvider(name, model, baseURL, authMethod string, oauthCfg *auth.DeviceFlowConfig, store auth.Store) *AnthropicProvider {
	if baseURL == "" {
		baseURL = defaultAnthropicURL
	} else {
		baseURL = strings.TrimRight(baseURL, "/")
	}
	return &AnthropicProvider{
		name:       name,
		model:      model,
		baseURL:    baseURL,
		authMethod: authMethod,
		oauthCfg:   oauthCfg,
		store:      store,
		client:     &http.Client{},
	}
}

func (p *AnthropicProvider) ID() string {
	return p.name
}

func (p *AnthropicProvider) Authenticate() error {
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
			return fmt.Errorf("anthropic oauth: %w", err)
		}
		p.apiKey = token
		// Store under provider name for future lookups.
		_ = p.store.Set(p.name, token)
		return nil
	}

	return fmt.Errorf("anthropic authenticate: no credentials found for %q — run 'polycode auth login %s'", p.name, p.name)
}

func (p *AnthropicProvider) Validate() error {
	if p.apiKey == "" {
		return fmt.Errorf("anthropic provider %q: API key not set — call Authenticate() first", p.name)
	}
	return nil
}

// anthropicRequest is the request body for the Anthropic Messages API.
type anthropicRequest struct {
	Model     string              `json:"model"`
	Messages  []anthropicMsg      `json:"messages"`
	MaxTokens int                 `json:"max_tokens"`
	Stream    bool                `json:"stream"`
	System    string              `json:"system,omitempty"`
	Tools     []anthropicTool     `json:"tools,omitempty"`
	Thinking  *anthropicThinking  `json:"thinking,omitempty"`
}

// anthropicThinking controls Anthropic's extended thinking feature.
type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// anthropicTool is a tool definition in the Anthropic format.
type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema any `json:"input_schema"`
}

type anthropicMsg struct {
	Role    string      `json:"role"`
	Content any `json:"content"` // string or []anthropicContentBlock
}

// anthropicContentBlock is a content block in an Anthropic message.
type anthropicContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     any `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
}

// anthropicSSEEvent represents a parsed SSE event from the Anthropic stream.
type anthropicSSEEvent struct {
	Type         string                      `json:"type"`
	Index        int                         `json:"index,omitempty"`
	Delta        json.RawMessage             `json:"delta,omitempty"`
	ContentBlock *anthropicContentBlockStart  `json:"content_block,omitempty"`
	Message      *anthropicMessageMeta       `json:"message,omitempty"`
	Usage        *anthropicUsage             `json:"usage,omitempty"`
}

type anthropicMessageMeta struct {
	Usage *anthropicUsage `json:"usage,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// anthropicContentBlockStart is the content_block in a content_block_start event.
type anthropicContentBlockStart struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Text  string `json:"text,omitempty"`
}

func (p *AnthropicProvider) Query(ctx context.Context, messages []Message, opts QueryOpts) (<-chan StreamChunk, error) {
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// Separate system messages from conversation messages.
	var systemPrompt string
	var convMsgs []anthropicMsg
	for _, m := range messages {
		if m.Role == RoleSystem {
			if systemPrompt != "" {
				systemPrompt += "\n"
			}
			systemPrompt += m.Content
			continue
		}

		// Handle tool result messages with proper content blocks.
		if m.Role == RoleTool && m.ToolCallID != "" {
			convMsgs = append(convMsgs, anthropicMsg{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
			continue
		}

		// Handle assistant messages with tool calls.
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			var blocks []anthropicContentBlock
			if m.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var input any
				if tc.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &input)
				}
				if input == nil {
					input = map[string]any{}
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			convMsgs = append(convMsgs, anthropicMsg{
				Role:    "assistant",
				Content: blocks,
			})
			continue
		}

		role := string(m.Role)
		if m.Role == RoleTool {
			role = "user"
		}
		convMsgs = append(convMsgs, anthropicMsg{
			Role:    role,
			Content: m.Content,
		})
	}

	reqBody := anthropicRequest{
		Model:     p.model,
		Messages:  convMsgs,
		MaxTokens: maxTokens,
		Stream:    true,
		System:    systemPrompt,
	}

	// Map reasoning effort to Anthropic's thinking feature.
	if opts.ReasoningEffort != "" {
		budgetTokens := 4096 // default for low
		switch opts.ReasoningEffort {
		case ReasoningMedium:
			budgetTokens = 10000
		case ReasoningHigh:
			budgetTokens = 32000
		}
		reqBody.Thinking = &anthropicThinking{
			Type:         "enabled",
			BudgetTokens: budgetTokens,
		}
		// Anthropic requires max_tokens to be larger than budget_tokens
		if reqBody.MaxTokens < budgetTokens+1024 {
			reqBody.MaxTokens = budgetTokens + 1024
		}
	}

	// Map tool definitions if provided.
	for _, t := range opts.Tools {
		reqBody.Tools = append(reqBody.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic: API returned status %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamChunk, 64)
	go p.readSSE(ctx, resp.Body, ch)
	return ch, nil
}

func (p *AnthropicProvider) readSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	var inputTokens, outputTokens int

	// Track tool_use blocks being streamed: index → {id, name, args}
	type toolUseBuf struct {
		id   string
		name string
		args strings.Builder
	}
	var toolBufs []toolUseBuf

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

		var evt anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("anthropic: parse SSE event: %w", err)}
			return
		}

		switch evt.Type {
		case "message_start":
			// message_start carries input token count
			if evt.Message != nil && evt.Message.Usage != nil {
				inputTokens = evt.Message.Usage.InputTokens
			}
		case "content_block_start":
			// A new content block is starting. If it's a tool_use block,
			// record its ID and name for argument accumulation.
			if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
				for evt.Index >= len(toolBufs) {
					toolBufs = append(toolBufs, toolUseBuf{})
				}
				toolBufs[evt.Index] = toolUseBuf{
					id:   evt.ContentBlock.ID,
					name: evt.ContentBlock.Name,
				}
			}
		case "content_block_delta":
			var delta anthropicDelta
			if err := json.Unmarshal(evt.Delta, &delta); err != nil {
				ch <- StreamChunk{Error: fmt.Errorf("anthropic: parse delta: %w", err)}
				return
			}
			if delta.Type == "text_delta" && delta.Text != "" {
				ch <- StreamChunk{Delta: delta.Text}
			}
			if delta.Type == "input_json_delta" && delta.PartialJSON != "" {
				if evt.Index < len(toolBufs) {
					toolBufs[evt.Index].args.WriteString(delta.PartialJSON)
				}
			}
		case "message_delta":
			// message_delta carries output token count
			if evt.Usage != nil {
				outputTokens = evt.Usage.OutputTokens
			}
		case "message_stop":
			// Assemble accumulated tool calls.
			var calls []ToolCall
			for i, tb := range toolBufs {
				if tb.name == "" {
					continue
				}
				id := tb.id
				if id == "" {
					id = fmt.Sprintf("anthropic_call_%d", i)
				}
				calls = append(calls, ToolCall{
					ID:        id,
					Name:      tb.name,
					Arguments: tb.args.String(),
				})
			}
			ch <- StreamChunk{Done: true, ToolCalls: calls, InputTokens: inputTokens, OutputTokens: outputTokens}
			return
		case "error":
			ch <- StreamChunk{Error: fmt.Errorf("anthropic: stream error: %s", string(evt.Delta))}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("anthropic: read stream: %w", err)}
	}
}
