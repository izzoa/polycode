package provider

import (
	"context"
)

// Role represents a message role in a conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // set on assistant messages requesting tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // set on tool result messages (Role = RoleTool)
}

// StreamChunk represents a single chunk of a streaming response.
type StreamChunk struct {
	// Delta is the incremental text content.
	Delta string
	// Done indicates this is the final chunk.
	Done bool
	// Status indicates this chunk is a progress/status message (not model output).
	// Status chunks should be displayed but not persisted to conversation history.
	Status bool
	// ToolCalls contains any tool calls emitted in the final chunk.
	// Only populated when Done is true and the model requested tool use.
	ToolCalls []ToolCall
	// InputTokens is the total input tokens for this request.
	// Only populated when Done is true.
	InputTokens int
	// OutputTokens is the total output tokens for this response.
	// Only populated when Done is true.
	OutputTokens int
	// Error is set if the provider encountered an error.
	Error error
}

// ToolCall represents a tool invocation from the model.
type ToolCall struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Arguments        string `json:"arguments"`
	ThoughtSignature string `json:"thought_signature,omitempty"` // Gemini thought signature for round-tripping
}

// ToolDefinition defines a tool the model can call.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  any `json:"parameters"`
}

// ReasoningEffort controls how much "thinking" a model does before responding.
// Not all models support this — providers that don't will ignore it.
type ReasoningEffort string

const (
	ReasoningOff    ReasoningEffort = ""       // no reasoning requested (model default)
	ReasoningLow    ReasoningEffort = "low"    // minimal reasoning
	ReasoningMedium ReasoningEffort = "medium" // standard reasoning
	ReasoningHigh   ReasoningEffort = "high"   // deep reasoning
)

// QueryOpts holds options for a query.
type QueryOpts struct {
	MaxTokens        int              `json:"max_tokens,omitempty"`
	Temperature      float64          `json:"temperature,omitempty"`
	Tools            []ToolDefinition `json:"tools,omitempty"`
	ReasoningEffort  ReasoningEffort  `json:"reasoning_effort,omitempty"`
}

// Provider is the interface that all LLM provider adapters must implement.
type Provider interface {
	// ID returns the unique identifier for this provider (from config name).
	ID() string

	// Query sends messages to the provider and returns a channel of streaming chunks.
	Query(ctx context.Context, messages []Message, opts QueryOpts) (<-chan StreamChunk, error)

	// Authenticate performs any necessary authentication (API key validation, OAuth flow).
	Authenticate() error

	// Validate checks that the provider is reachable and properly configured.
	Validate() error
}

// Response represents a complete response from a single provider.
type Response struct {
	ProviderID string
	Content    string
	ToolCalls  []ToolCall
	Error      error
}
