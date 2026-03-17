// Package mcp implements a client for the Model Context Protocol.
// It supports connecting to MCP servers via stdio (subprocess) transport,
// discovering tools, and invoking them on behalf of the primary model.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/provider"
)

// MCPTool represents a tool discovered from an MCP server.
type MCPTool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	ServerName  string // which server provides this tool
}

// MCPClient manages connections to one or more MCP servers.
type MCPClient struct {
	configs []config.MCPServerConfig
	servers map[string]*serverConn
	tools   []MCPTool
	mu      sync.Mutex
}

// serverConn holds the state for a single MCP server connection.
type serverConn struct {
	config  config.MCPServerConfig
	process *exec.Cmd      // nil for SSE
	stdin   io.WriteCloser // nil for SSE
	stdout  io.ReadCloser  // nil for SSE
	scanner *bufio.Scanner
	nextID  atomic.Int64
	mu      sync.Mutex // serialises request/response pairs
}

// jsonrpcRequest is a JSON-RPC 2.0 request.
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse is a JSON-RPC 2.0 response.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolsListResult is the result payload from tools/list.
type toolsListResult struct {
	Tools []struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	} `json:"tools"`
}

// toolCallResult is the result payload from tools/call.
type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// NewMCPClient creates a new MCP client from the given server configurations.
// No connections are made until Connect is called.
func NewMCPClient(configs []config.MCPServerConfig) *MCPClient {
	return &MCPClient{
		configs: configs,
		servers: make(map[string]*serverConn),
	}
}

// Connect starts all configured MCP servers and discovers their tools.
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cfg := range c.configs {
		if cfg.URL != "" {
			// SSE transport is not implemented in v1; skip with a warning.
			continue
		}

		conn, err := c.connectStdio(ctx, cfg)
		if err != nil {
			// Clean up any servers we already started.
			for _, s := range c.servers {
				s.close()
			}
			c.servers = make(map[string]*serverConn)
			return fmt.Errorf("connecting to MCP server %q: %w", cfg.Name, err)
		}
		c.servers[cfg.Name] = conn
	}

	// Discover tools from all connected servers.
	for name, conn := range c.servers {
		tools, err := c.discoverTools(ctx, name, conn)
		if err != nil {
			return fmt.Errorf("discovering tools from %q: %w", name, err)
		}
		c.tools = append(c.tools, tools...)
	}

	return nil
}

// connectStdio spawns a subprocess and performs the MCP initialize handshake.
func (c *MCPClient) connectStdio(ctx context.Context, cfg config.MCPServerConfig) (*serverConn, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("starting process: %w", err)
	}

	conn := &serverConn{
		config:  cfg,
		process: cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
	}

	// Send initialize request.
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "polycode",
			"version": "1.0.0",
		},
	}

	_, err = conn.sendRequest("initialize", initParams)
	if err != nil {
		conn.close()
		return nil, fmt.Errorf("initialize handshake: %w", err)
	}

	// Send initialized notification (no response expected, but we send it as
	// a notification — id omitted). For simplicity we send it as a request
	// that we don't wait a response for. MCP spec says this is a notification.
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, _ := json.Marshal(notification)
	data = append(data, '\n')
	conn.mu.Lock()
	_, err = conn.stdin.Write(data)
	conn.mu.Unlock()
	if err != nil {
		conn.close()
		return nil, fmt.Errorf("sending initialized notification: %w", err)
	}

	return conn, nil
}

// discoverTools sends tools/list to a server and returns the discovered tools.
func (c *MCPClient) discoverTools(_ context.Context, serverName string, conn *serverConn) ([]MCPTool, error) {
	resp, err := conn.sendRequest("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result toolsListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parsing tools/list result: %w", err)
	}

	var tools []MCPTool
	for _, t := range result.Tools {
		tools = append(tools, MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerName:  serverName,
		})
	}

	return tools, nil
}

// Tools returns all tools discovered from connected MCP servers.
func (c *MCPClient) Tools() []MCPTool {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]MCPTool, len(c.tools))
	copy(result, c.tools)
	return result
}

// CallTool invokes a tool on the specified MCP server and returns the text result.
func (c *MCPClient) CallTool(_ context.Context, serverName, toolName string, args json.RawMessage) (string, error) {
	c.mu.Lock()
	conn, ok := c.servers[serverName]
	c.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("unknown MCP server %q", serverName)
	}

	params := map[string]interface{}{
		"name": toolName,
	}
	if args != nil {
		var a interface{}
		if err := json.Unmarshal(args, &a); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
		params["arguments"] = a
	}

	resp, err := conn.sendRequest("tools/call", params)
	if err != nil {
		return "", err
	}

	var result toolCallResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing tools/call result: %w", err)
	}

	// Concatenate all text content blocks.
	var text string
	for _, c := range result.Content {
		if c.Type == "text" {
			if text != "" {
				text += "\n"
			}
			text += c.Text
		}
	}

	return text, nil
}

// Close shuts down all MCP server connections.
func (c *MCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	for name, conn := range c.servers {
		if err := conn.close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("closing server %q: %w", name, err)
		}
	}
	c.servers = make(map[string]*serverConn)
	c.tools = nil
	return firstErr
}

// ToToolDefinitions converts discovered MCP tools to provider.ToolDefinition
// format so they can be sent to the primary model alongside built-in tools.
// Tool names are prefixed with "mcp_{serverName}_" to avoid collisions.
func (c *MCPClient) ToToolDefinitions() []provider.ToolDefinition {
	c.mu.Lock()
	defer c.mu.Unlock()

	defs := make([]provider.ToolDefinition, 0, len(c.tools))
	for _, t := range c.tools {
		name := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
		defs = append(defs, provider.ToolDefinition{
			Name:        name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return defs
}

// sendRequest sends a JSON-RPC request and waits for the response.
func (sc *serverConn) sendRequest(method string, params interface{}) (json.RawMessage, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	id := sc.nextID.Add(1)

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	data = append(data, '\n')

	if _, err := sc.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	// Read lines until we get a response with our ID.
	// Skip any notifications (lines without an id matching ours).
	for {
		if !sc.scanner.Scan() {
			if err := sc.scanner.Err(); err != nil {
				return nil, fmt.Errorf("reading response: %w", err)
			}
			return nil, fmt.Errorf("server closed connection")
		}

		line := sc.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Skip non-JSON lines.
			continue
		}

		if resp.ID != id {
			// Not our response; could be a notification. Skip.
			continue
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("server error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		return resp.Result, nil
	}
}

// close shuts down a server connection.
func (sc *serverConn) close() error {
	if sc.stdin != nil {
		sc.stdin.Close()
	}
	if sc.stdout != nil {
		sc.stdout.Close()
	}
	if sc.process != nil {
		// Kill the process if it hasn't exited.
		_ = sc.process.Process.Kill()
		return sc.process.Wait()
	}
	return nil
}
