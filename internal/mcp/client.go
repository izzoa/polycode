// Package mcp implements a client for the Model Context Protocol.
// It supports connecting to MCP servers via stdio (subprocess) transport,
// discovering tools, and invoking them on behalf of the primary model.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/izzoa/polycode/internal/auth"
	"github.com/izzoa/polycode/internal/config"
	"github.com/izzoa/polycode/internal/provider"
)

const defaultCallTimeout = 30 * time.Second

// MCPTool represents a tool discovered from an MCP server.
type MCPTool struct {
	Name        string
	Description string
	InputSchema map[string]any
	ServerName  string // which server provides this tool
	ReadOnly    bool   // true if server or tool annotation marks it read-only
}

// MCPResource represents a resource exposed by an MCP server.
type MCPResource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	ServerName  string
}

// MCPPrompt represents a prompt template exposed by an MCP server.
type MCPPrompt struct {
	Name        string
	Description string
	Arguments   []MCPPromptArgument
	ServerName  string
}

// MCPPromptArgument describes an argument for a prompt template.
type MCPPromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// ServerStatus reports the connection status of a single MCP server.
type ServerStatus struct {
	Name      string
	Connected bool
	Error     error
	ToolCount int
}

// MCPClient manages connections to one or more MCP servers.
type MCPClient struct {
	configs      []config.MCPServerConfig
	servers      map[string]*serverConn
	serverErrors map[string]error // per-server last connection error
	tools        []MCPTool
	toolIndex    map[string]struct{ Server, Tool string } // prefixed name → (server, tool)
	resources    []MCPResource
	prompts      []MCPPrompt
	mu           sync.Mutex
	reconnectMu  map[string]*sync.Mutex // per-server reconnect serialization
	debug        *debugLogger
	callCount    atomic.Int64 // total MCP tool calls made
	onNotify     func(serverName, method string, params json.RawMessage)
}

// muxResponse is sent through per-request channels by the background reader.
type muxResponse struct {
	result json.RawMessage
	err    error
}

// serverConn holds the state for a single MCP server connection.
// For stdio transport, process/stdin/stdout/scanner are populated.
// For HTTP transport, http is populated and stdio fields are nil.
type serverConn struct {
	config   config.MCPServerConfig
	process  *exec.Cmd      // nil for HTTP
	stdin    io.WriteCloser // nil for HTTP
	stdout   io.ReadCloser  // nil for HTTP
	scanner  *bufio.Scanner
	nextID   atomic.Int64
	mu       sync.Mutex // serialises writes to stdin
	dead     atomic.Bool // set on timeout or connection failure; triggers reconnect
	exitCh   chan struct{} // closed when subprocess exits (background watcher)
	http     *httpConn     // non-nil for HTTP transport
	debug    *debugLogger  // shared debug logger from MCPClient

	// Multiplexed reader state (stdio only).
	pending       map[int64]chan muxResponse // request ID → response channel
	pendingMu     sync.Mutex
	readerRunning bool
	onNotify      func(method string, params json.RawMessage)
}

// jsonrpcRequest is a JSON-RPC 2.0 request.
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  any `json:"params,omitempty"`
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

// toolAnnotations holds MCP tool annotations (spec 2024-11-05).
type toolAnnotations struct {
	ReadOnlyHint bool `json:"readOnlyHint"`
}

// toolsListResult is the result payload from tools/list.
type toolsListResult struct {
	Tools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema map[string]any  `json:"inputSchema"`
		Annotations *toolAnnotations `json:"annotations,omitempty"`
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
func NewMCPClient(configs []config.MCPServerConfig, debug bool) *MCPClient {
	return &MCPClient{
		configs:      configs,
		servers:      make(map[string]*serverConn),
		serverErrors: make(map[string]error),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
		reconnectMu:  make(map[string]*sync.Mutex),
		debug:        newDebugLogger(debug),
	}
}

// Connect starts all configured MCP servers and discovers their tools.
// Individual server failures are isolated — other servers continue to work.
// Returns a non-nil error summarizing any failures, but the client remains
// usable for servers that connected successfully.
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear existing state to make Connect idempotent
	c.tools = nil
	c.toolIndex = make(map[string]struct{ Server, Tool string })

	var errs []string

	for _, cfg := range c.configs {
		var conn *serverConn
		var err error

		if cfg.URL != "" {
			conn, err = c.connectHTTP(ctx, cfg)
		} else {
			conn, err = c.connectStdio(ctx, cfg)
		}
		if err != nil {
			connErr := fmt.Errorf("connecting: %w", err)
			c.serverErrors[cfg.Name] = connErr
			errs = append(errs, fmt.Sprintf("%s: %v", cfg.Name, err))
			continue
		}
		c.servers[cfg.Name] = conn
		delete(c.serverErrors, cfg.Name) // clear any previous error
	}

	// Discover tools from all connected servers in config order (deterministic).
	for _, cfg := range c.configs {
		conn, ok := c.servers[cfg.Name]
		if !ok {
			continue // server failed to connect
		}
		name := cfg.Name

		tools, err := c.discoverTools(ctx, name, conn)
		if err != nil {
			discErr := fmt.Errorf("tool discovery: %w", err)
			c.serverErrors[name] = discErr
			errs = append(errs, fmt.Sprintf("%s: tool discovery: %v", name, err))
			conn.close()
			delete(c.servers, name)
			continue
		}
		// Build the lookup index and filter collisions from both toolIndex and tools.
		for _, t := range tools {
			prefixed := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
			if existing, ok := c.toolIndex[prefixed]; ok && existing.Server != t.ServerName {
				log.Printf("Warning: MCP tool name collision: %q from server %q shadows tool from server %q — skipping duplicate",
					prefixed, t.ServerName, existing.Server)
				continue // skip from both toolIndex and c.tools
			}
			c.toolIndex[prefixed] = struct{ Server, Tool string }{t.ServerName, t.Name}
			c.tools = append(c.tools, t)
		}

		// Discover resources and prompts (best-effort).
		c.resources = append(c.resources, c.discoverResources(ctx, name, conn)...)
		c.prompts = append(c.prompts, c.discoverPrompts(ctx, name, conn)...)

		// Wire notification callback for dynamic tool refresh (stdio only).
		if conn.http == nil {
			serverName := name // capture for closure
			conn.onNotify = func(method string, params json.RawMessage) {
				c.debug.LogNotification(serverName, method)
				if method == "notifications/tools/list_changed" {
					c.refreshToolsForServer(serverName)
					if c.onNotify != nil {
						c.onNotify(serverName, method, params)
					}
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("MCP server errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Status returns the connection status of all configured MCP servers.
func (c *MCPClient) Status() []ServerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build a set of connected server names.
	connected := make(map[string]bool)
	for name := range c.servers {
		connected[name] = true
	}

	// Count tools per server.
	toolCounts := make(map[string]int)
	for _, t := range c.tools {
		toolCounts[t.ServerName]++
	}

	var statuses []ServerStatus
	for _, cfg := range c.configs {
		s := ServerStatus{Name: cfg.Name}
		if connected[cfg.Name] {
			s.Connected = true
			s.ToolCount = toolCounts[cfg.Name]
		}
		if err, ok := c.serverErrors[cfg.Name]; ok {
			s.Error = err
		}
		statuses = append(statuses, s)
	}
	return statuses
}

// connectStdio spawns a subprocess and performs the MCP initialize handshake.
func (c *MCPClient) connectStdio(ctx context.Context, cfg config.MCPServerConfig) (*serverConn, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)

	// Apply environment variables: inherit parent env, then overlay config env.
	if len(cfg.Env) > 0 {
		cmd.Env = os.Environ()
		store := auth.NewStore()
		for k, v := range cfg.Env {
			// Support keyring references: $KEYRING:key_name
			if strings.HasPrefix(v, "$KEYRING:") {
				keyName := v[9:]
				if secret, err := store.Get(keyName); err == nil {
					v = secret
				}
			}
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

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

	exitCh := make(chan struct{})
	conn := &serverConn{
		config:  cfg,
		process: cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: func() *bufio.Scanner {
			s := bufio.NewScanner(stdout)
			s.Buffer(make([]byte, 64*1024), 4*1024*1024) // 4MB max line size
			return s
		}(),
		exitCh:  exitCh,
		debug:   c.debug,
	}

	// Background goroutine to detect process exit reliably.
	go func() {
		_ = cmd.Wait()
		close(exitCh)
	}()

	// Send initialize request.
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "polycode",
			"version": "1.0.0",
		},
	}

	_, err = conn.sendRequest(ctx, "initialize", initParams)
	if err != nil {
		conn.close()
		return nil, fmt.Errorf("initialize handshake: %w", err)
	}

	// Send initialized notification (no response expected).
	notification := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, _ := json.Marshal(notification)
	data = append(data, '\n')
	conn.debug.LogRequest(cfg.Name, "notifications/initialized", "")
	conn.mu.Lock()
	_, err = conn.stdin.Write(data)
	conn.mu.Unlock()
	if err != nil {
		conn.close()
		return nil, fmt.Errorf("sending initialized notification: %w", err)
	}

	return conn, nil
}

// connectHTTP establishes an HTTP/SSE transport connection to an MCP server.
func (c *MCPClient) connectHTTP(ctx context.Context, cfg config.MCPServerConfig) (*serverConn, error) {
	hc := &httpConn{
		url:        cfg.URL,
		serverName: cfg.Name,
		client:     &http.Client{},
		debug:      c.debug,
	}

	// Send initialize handshake via HTTP.
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "polycode",
			"version": "1.0.0",
		},
	}

	_, err := hc.sendRequest(ctx, "initialize", initParams)
	if err != nil {
		return nil, fmt.Errorf("HTTP initialize handshake: %w", err)
	}

	// Send initialized notification.
	notifReq := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	body, _ := json.Marshal(notifReq)
	hc.debug.LogRequest(cfg.Name, "notifications/initialized", "")
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if hc.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", hc.sessionID)
	}
	resp, err := hc.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending initialized notification: %w", err)
	}
	resp.Body.Close()

	conn := &serverConn{
		config: cfg,
		http:   hc,
		debug:  c.debug,
	}

	return conn, nil
}

// discoverTools sends tools/list to a server and returns the discovered tools.
func (c *MCPClient) discoverTools(ctx context.Context, serverName string, conn *serverConn) ([]MCPTool, error) {
	resp, err := conn.sendRequest(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	var result toolsListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parsing tools/list result: %w", err)
	}

	serverReadOnly := conn.config.ReadOnly
	var tools []MCPTool
	for _, t := range result.Tools {
		// A tool is read-only if the server is marked read_only OR if the
		// tool itself has readOnlyHint annotation.
		readOnly := serverReadOnly
		if t.Annotations != nil && t.Annotations.ReadOnlyHint {
			readOnly = true
		}
		tools = append(tools, MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerName:  serverName,
			ReadOnly:    readOnly,
		})
	}

	return tools, nil
}

// discoverResources sends resources/list to a server and returns discovered resources.
// Silently returns nil if the server doesn't support resources.
func (c *MCPClient) discoverResources(ctx context.Context, serverName string, conn *serverConn) []MCPResource {
	resp, err := conn.sendRequest(ctx, "resources/list", map[string]any{})
	if err != nil {
		return nil // server may not support resources
	}

	var result struct {
		Resources []struct {
			URI         string `json:"uri"`
			Name        string `json:"name"`
			Description string `json:"description"`
			MimeType    string `json:"mimeType"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil
	}

	var resources []MCPResource
	for _, r := range result.Resources {
		resources = append(resources, MCPResource{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MimeType,
			ServerName:  serverName,
		})
	}
	return resources
}

// discoverPrompts sends prompts/list to a server and returns discovered prompts.
// Silently returns nil if the server doesn't support prompts.
func (c *MCPClient) discoverPrompts(ctx context.Context, serverName string, conn *serverConn) []MCPPrompt {
	resp, err := conn.sendRequest(ctx, "prompts/list", map[string]any{})
	if err != nil {
		return nil // server may not support prompts
	}

	var result struct {
		Prompts []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Arguments   []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Required    bool   `json:"required"`
			} `json:"arguments"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil
	}

	var prompts []MCPPrompt
	for _, p := range result.Prompts {
		prompt := MCPPrompt{
			Name:        p.Name,
			Description: p.Description,
			ServerName:  serverName,
		}
		for _, a := range p.Arguments {
			prompt.Arguments = append(prompt.Arguments, MCPPromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}
		prompts = append(prompts, prompt)
	}
	return prompts
}

// Resources returns all resources discovered from connected MCP servers.
func (c *MCPClient) Resources() []MCPResource {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]MCPResource, len(c.resources))
	copy(result, c.resources)
	return result
}

// Prompts returns all prompts discovered from connected MCP servers.
func (c *MCPClient) Prompts() []MCPPrompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]MCPPrompt, len(c.prompts))
	copy(result, c.prompts)
	return result
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
// A per-call timeout is applied from the server's Timeout config (default 30s).
// If the server process has died, a reconnect is attempted automatically.
func (c *MCPClient) CallTool(ctx context.Context, serverName, toolName string, args json.RawMessage) (string, error) {
	c.mu.Lock()
	conn, ok := c.servers[serverName]
	c.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("unknown MCP server %q", serverName)
	}

	// Auto-reconnect if the server process has died.
	if !conn.isAlive() {
		newConn, err := c.reconnectServer(ctx, conn.config)
		if err != nil {
			return "", fmt.Errorf("server %q died and reconnect failed: %w", serverName, err)
		}
		conn = newConn
	}

	// Apply per-call timeout from server config.
	timeout := defaultCallTimeout
	if conn.config.Timeout > 0 {
		timeout = time.Duration(conn.config.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := map[string]any{
		"name": toolName,
	}
	if args != nil {
		var a any
		if err := json.Unmarshal(args, &a); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
		params["arguments"] = a
	}

	c.callCount.Add(1)

	resp, err := conn.sendRequest(ctx, "tools/call", params)
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

// SetNotificationHandler sets a callback that is invoked when any MCP server
// sends a notification. This is used to handle notifications/tools/list_changed.
// Must be called before Connect().
func (c *MCPClient) SetNotificationHandler(handler func(serverName, method string, params json.RawMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onNotify = handler
}

// refreshToolsForServer re-discovers tools from a connected server and updates
// the tools list and index. Called when a tools/list_changed notification arrives.
func (c *MCPClient) refreshToolsForServer(serverName string) {
	c.mu.Lock()
	conn, ok := c.servers[serverName]
	c.mu.Unlock()
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tools, err := c.discoverTools(ctx, serverName, conn)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Replace tools for this server.
	var kept []MCPTool
	for _, t := range c.tools {
		if t.ServerName != serverName {
			kept = append(kept, t)
		}
	}
	c.tools = append(kept, tools...)

	// Rebuild index for this server.
	for key, entry := range c.toolIndex {
		if entry.Server == serverName {
			delete(c.toolIndex, key)
		}
	}
	for _, t := range tools {
		prefixed := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
		c.toolIndex[prefixed] = struct{ Server, Tool string }{t.ServerName, t.Name}
	}
}

// TestConnection creates a temporary connection to an MCP server using the
// given config, performs the initialize handshake, discovers tools, and tears
// down the connection. Returns the tool count or an error. Does not modify
// any MCPClient state — safe to call from the wizard with staged config.
func TestConnection(ctx context.Context, cfg config.MCPServerConfig) (int, error) {
	// Create a temporary client just for the test.
	tmpClient := &MCPClient{
		servers:      make(map[string]*serverConn),
		serverErrors: make(map[string]error),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
	}

	var conn *serverConn
	var err error
	if cfg.URL != "" {
		conn, err = tmpClient.connectHTTP(ctx, cfg)
	} else {
		conn, err = tmpClient.connectStdio(ctx, cfg)
	}
	if err != nil {
		return 0, err
	}
	defer conn.close()

	tools, err := tmpClient.discoverTools(ctx, cfg.Name, conn)
	if err != nil {
		return 0, err
	}

	return len(tools), nil
}

// CallCount returns the total number of MCP tool calls made.
func (c *MCPClient) CallCount() int64 {
	return c.callCount.Load()
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
	if c.debug != nil {
		c.debug.Close()
	}
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

// ReadOnlyToolDefinitions returns tool definitions for MCP tools that are
// marked as read-only (via server config or tool annotation). These are safe
// to include in fan-out queries to non-primary providers.
func (c *MCPClient) ReadOnlyToolDefinitions() []provider.ToolDefinition {
	c.mu.Lock()
	defer c.mu.Unlock()

	var defs []provider.ToolDefinition
	for _, t := range c.tools {
		if !t.ReadOnly {
			continue
		}
		name := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
		defs = append(defs, provider.ToolDefinition{
			Name:        name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return defs
}

// ResolveToolCall takes a prefixed tool name (e.g. "mcp_filesystem_read_file")
// and returns the (serverName, toolName) pair via the lookup map built during
// tool discovery. This avoids ambiguous underscore-based string parsing.
func (c *MCPClient) ResolveToolCall(prefixedName string) (serverName, toolName string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.toolIndex[prefixedName]
	if !found {
		return "", "", false
	}
	return entry.Server, entry.Tool, true
}

// IsServerReadOnly returns true if the named server is configured as read-only.
func (c *MCPClient) IsServerReadOnly(serverName string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn, ok := c.servers[serverName]
	if !ok {
		return false
	}
	return conn.config.ReadOnly
}

// startReader spawns the single background goroutine that reads all JSON-RPC
// lines from stdout and dispatches them: responses go to per-ID channels,
// notifications go to the onNotify callback. Safe to call multiple times —
// only the first call starts the goroutine.
func (sc *serverConn) startReader() {
	sc.pendingMu.Lock()
	if sc.readerRunning {
		sc.pendingMu.Unlock()
		return
	}
	sc.readerRunning = true
	if sc.pending == nil {
		sc.pending = make(map[int64]chan muxResponse)
	}
	sc.pendingMu.Unlock()

	go func() {
		for sc.scanner.Scan() {
			line := sc.scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Try to parse as a JSON-RPC message.
			var msg struct {
				ID     *int64          `json:"id"`
				Method string          `json:"method"`
				Result json.RawMessage `json:"result"`
				Error  *jsonrpcError   `json:"error"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}

			if msg.ID == nil {
				// Notification (no ID).
				if sc.onNotify != nil {
					sc.onNotify(msg.Method, msg.Params)
				}
				continue
			}

			// Response — deliver to the waiting caller.
			sc.pendingMu.Lock()
			ch, ok := sc.pending[*msg.ID]
			if ok {
				delete(sc.pending, *msg.ID)
			}
			sc.pendingMu.Unlock()

			if !ok {
				continue // orphaned response — caller timed out
			}

			if msg.Error != nil {
				ch <- muxResponse{err: fmt.Errorf("server error %d: %s", msg.Error.Code, msg.Error.Message)}
			} else {
				ch <- muxResponse{result: msg.Result}
			}
		}

		// Scanner stopped — deliver errors to all pending callers.
		scanErr := sc.scanner.Err()
		sc.dead.Store(true)

		sc.pendingMu.Lock()
		for id, ch := range sc.pending {
			if scanErr != nil {
				ch <- muxResponse{err: fmt.Errorf("reading response: %w", scanErr)}
			} else {
				ch <- muxResponse{err: fmt.Errorf("server closed connection")}
			}
			delete(sc.pending, id)
		}
		sc.pendingMu.Unlock()
	}()
}

// sendRequest sends a JSON-RPC request and waits for the response.
// For stdio: uses the multiplexed reader to receive the response by ID.
// For HTTP: delegates to httpConn.
func (sc *serverConn) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	// HTTP transport — delegate entirely to httpConn.
	if sc.http != nil {
		return sc.http.sendRequest(ctx, method, params)
	}

	if sc.dead.Load() {
		sc.debug.LogResponse(sc.config.Name, method, "", "connection is dead")
		return nil, fmt.Errorf("connection is dead (timed out or failed)")
	}

	// Ensure the background reader is running.
	sc.startReader()

	id := sc.nextID.Add(1)

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		sc.debug.LogResponse(sc.config.Name, method, "", err.Error())
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	data = append(data, '\n')

	// Register response channel before writing to avoid races.
	ch := make(chan muxResponse, 1)
	sc.pendingMu.Lock()
	sc.pending[id] = ch
	sc.pendingMu.Unlock()

	// Log the outgoing request.
	sc.debug.LogRequest(sc.config.Name, method, string(data))

	// Write request (serialised to prevent interleaving).
	sc.mu.Lock()
	_, writeErr := sc.stdin.Write(data)
	sc.mu.Unlock()

	if writeErr != nil {
		sc.pendingMu.Lock()
		delete(sc.pending, id)
		sc.pendingMu.Unlock()
		sc.dead.Store(true)
		sc.debug.LogResponse(sc.config.Name, method, "", writeErr.Error())
		return nil, fmt.Errorf("writing request: %w", writeErr)
	}

	select {
	case r := <-ch:
		if r.err != nil {
			sc.debug.LogResponse(sc.config.Name, method, "", r.err.Error())
		} else {
			sc.debug.LogResponse(sc.config.Name, method, string(r.result), "")
		}
		return r.result, r.err
	case <-ctx.Done():
		// Remove our pending entry so the reader doesn't try to deliver.
		sc.pendingMu.Lock()
		delete(sc.pending, id)
		sc.pendingMu.Unlock()
		sc.dead.Store(true)
		if sc.stdout != nil {
			sc.stdout.Close()
		}
		sc.debug.LogResponse(sc.config.Name, method, "", "request timed out")
		return nil, fmt.Errorf("request timed out: %w", ctx.Err())
	}
}

// isAlive returns true if the server connection is still usable.
// Checks the dead flag (set on timeout/write failure) and subprocess exit status.
func (sc *serverConn) isAlive() bool {
	// HTTP transport.
	if sc.http != nil {
		return sc.http.isAlive()
	}
	// Dead flag is set on timeout or write failure.
	if sc.dead.Load() {
		return false
	}
	// Check if the subprocess has exited via the exit channel.
	if sc.exitCh != nil {
		select {
		case <-sc.exitCh:
			return false // process exited
		default:
			return true // still running
		}
	}
	// No subprocess and no exit channel — pipe-based connection (e.g. tests).
	return true
}

// serverReconnectMu returns the per-server mutex for serializing reconnects.
func (c *MCPClient) serverReconnectMu(name string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	mu, ok := c.reconnectMu[name]
	if !ok {
		mu = &sync.Mutex{}
		c.reconnectMu[name] = mu
	}
	return mu
}

// reconnectServer closes a dead server connection and establishes a new one,
// re-discovering tools, resources, and prompts. Serialized per-server to
// prevent concurrent reconnects from interleaving and duplicating metadata.
func (c *MCPClient) reconnectServer(ctx context.Context, cfg config.MCPServerConfig) (*serverConn, error) {
	// Serialize reconnects for this server.
	smu := c.serverReconnectMu(cfg.Name)
	smu.Lock()
	defer smu.Unlock()

	c.mu.Lock()
	// Close and remove old connection if present.
	if old, ok := c.servers[cfg.Name]; ok {
		old.close()
		delete(c.servers, cfg.Name)
	}
	// Clean out old tools, resources, prompts, and index entries for this server.
	c.removeServerMetadata(cfg.Name)
	c.mu.Unlock()

	var newConn *serverConn
	var err error
	if cfg.URL != "" {
		newConn, err = c.connectHTTP(ctx, cfg)
	} else {
		newConn, err = c.connectStdio(ctx, cfg)
	}
	if err != nil {
		c.mu.Lock()
		c.serverErrors[cfg.Name] = fmt.Errorf("reconnect: %w", err)
		c.mu.Unlock()
		return nil, err
	}

	tools, err := c.discoverTools(ctx, cfg.Name, newConn)
	if err != nil {
		newConn.close()
		c.mu.Lock()
		c.serverErrors[cfg.Name] = fmt.Errorf("reconnect tool discovery: %w", err)
		c.mu.Unlock()
		return nil, err
	}

	// Discover resources and prompts (best-effort).
	resources := c.discoverResources(ctx, cfg.Name, newConn)
	prompts := c.discoverPrompts(ctx, cfg.Name, newConn)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.servers[cfg.Name] = newConn
	delete(c.serverErrors, cfg.Name) // clear error on success
	for _, t := range tools {
		prefixed := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
		if existing, ok := c.toolIndex[prefixed]; ok && existing.Server != t.ServerName {
			log.Printf("Warning: MCP tool name collision: %q from server %q shadows tool from server %q — skipping duplicate",
				prefixed, t.ServerName, existing.Server)
			continue // skip from both toolIndex and c.tools
		}
		c.toolIndex[prefixed] = struct{ Server, Tool string }{t.ServerName, t.Name}
		c.tools = append(c.tools, t)
	}
	c.resources = append(c.resources, resources...)
	c.prompts = append(c.prompts, prompts...)

	return newConn, nil
}

// removeServerMetadata removes tools, resources, prompts, and index entries
// for a server. Must be called with c.mu held.
func (c *MCPClient) removeServerMetadata(serverName string) {
	var keptTools []MCPTool
	for _, t := range c.tools {
		if t.ServerName != serverName {
			keptTools = append(keptTools, t)
		}
	}
	c.tools = keptTools

	for key, entry := range c.toolIndex {
		if entry.Server == serverName {
			delete(c.toolIndex, key)
		}
	}

	var keptResources []MCPResource
	for _, r := range c.resources {
		if r.ServerName != serverName {
			keptResources = append(keptResources, r)
		}
	}
	c.resources = keptResources

	var keptPrompts []MCPPrompt
	for _, p := range c.prompts {
		if p.ServerName != serverName {
			keptPrompts = append(keptPrompts, p)
		}
	}
	c.prompts = keptPrompts
}

// Reconnect forces a reconnection to a specific server by name.
// Works for both currently-connected and initially-failed servers.
func (c *MCPClient) Reconnect(ctx context.Context, serverName string) error {
	// Look up config under lock so it's safe with concurrent Reconfigure.
	c.mu.Lock()
	var cfgCopy config.MCPServerConfig
	found := false
	for i := range c.configs {
		if c.configs[i].Name == serverName {
			cfgCopy = c.configs[i]
			found = true
			break
		}
	}
	c.mu.Unlock()
	if !found {
		return fmt.Errorf("unknown MCP server %q", serverName)
	}
	_, err := c.reconnectServer(ctx, cfgCopy)
	return err
}

// Reconfigure diffs the current config against newConfigs and applies changes:
// - New servers are connected
// - Removed servers are disconnected
// - Changed servers are reconnected
// Updates c.configs so Status() and Reconnect() use the new set.
func (c *MCPClient) Reconfigure(ctx context.Context, newConfigs []config.MCPServerConfig) error {
	// Snapshot old configs and update to new configs under lock.
	c.mu.Lock()
	oldMap := make(map[string]config.MCPServerConfig)
	for _, cfg := range c.configs {
		oldMap[cfg.Name] = cfg
	}
	c.configs = make([]config.MCPServerConfig, len(newConfigs))
	copy(c.configs, newConfigs)
	c.mu.Unlock()

	newMap := make(map[string]config.MCPServerConfig)
	for _, cfg := range newConfigs {
		newMap[cfg.Name] = cfg
	}

	var errs []string

	// Disconnect removed servers.
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			c.DisconnectServer(name)
		}
	}

	// Connect new servers.
	for name, cfg := range newMap {
		if _, existed := oldMap[name]; !existed {
			_, err := c.reconnectServer(ctx, cfg)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}

	// Reconnect changed servers (command, args, URL, or env differ).
	for name, newCfg := range newMap {
		oldCfg, existed := oldMap[name]
		if !existed {
			continue // already handled as new
		}
		if mcpConfigChanged(oldCfg, newCfg) {
			_, err := c.reconnectServer(ctx, newCfg)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("reconfigure errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// mcpConfigChanged returns true if two MCPServerConfigs differ in a way that
// requires reconnection.
func mcpConfigChanged(a, b config.MCPServerConfig) bool {
	if a.Command != b.Command || a.URL != b.URL {
		return true
	}
	if a.ReadOnly != b.ReadOnly || a.Timeout != b.Timeout {
		return true
	}
	if len(a.Args) != len(b.Args) {
		return true
	}
	for i := range a.Args {
		if a.Args[i] != b.Args[i] {
			return true
		}
	}
	if len(a.Env) != len(b.Env) {
		return true
	}
	for k, v := range a.Env {
		if b.Env[k] != v {
			return true
		}
	}
	return false
}

// DisconnectServer removes and closes a single server connection and all its metadata.
func (c *MCPClient) DisconnectServer(serverName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, ok := c.servers[serverName]; ok {
		conn.close()
		delete(c.servers, serverName)
	}
	c.removeServerMetadata(serverName)
	delete(c.serverErrors, serverName)
}

// close performs a graceful shutdown of a server connection.
// It closes stdin (signalling EOF), waits briefly for the process to exit,
// then force-kills if it doesn't comply within 3 seconds.
func (sc *serverConn) close() error {
	sc.dead.Store(true)

	// HTTP transport — lightweight close.
	if sc.http != nil {
		return sc.http.close()
	}

	// Send shutdown notification before closing (best-effort, per MCP spec).
	if sc.stdin != nil {
		notification := map[string]any{
			"jsonrpc": "2.0",
			"method":  "notifications/cancelled",
		}
		if data, err := json.Marshal(notification); err == nil {
			data = append(data, '\n')
			sc.stdin.Write(data) //nolint:errcheck // best-effort
		}
		sc.stdin.Close()
	}

	if sc.process != nil && sc.exitCh != nil {
		// Wait for the background goroutine to detect process exit.
		select {
		case <-sc.exitCh:
			// Process exited cleanly.
		case <-time.After(3 * time.Second):
			// Force kill after timeout.
			_ = sc.process.Process.Kill()
			<-sc.exitCh // wait for background goroutine to finish
		}
	}

	if sc.stdout != nil {
		sc.stdout.Close()
	}
	return nil
}
